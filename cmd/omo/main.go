package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"omo/internal/api"
	"omo/internal/audit"
	"omo/internal/auth"
	"omo/internal/backup"
	"omo/internal/bootstrap"
	"omo/internal/caddy"
	"omo/internal/configgen"
	"omo/internal/core/singbox"
	"omo/internal/diagnostics"
	"omo/internal/pairing"
	"omo/internal/protocol"
	"omo/internal/store"
	"omo/internal/subscription"
	"omo/internal/update"
	"omo/internal/version"
)

//go:embed all:web
var embeddedFiles embed.FS

func main() {
	if err := run(); err != nil {
		slog.Error("omo stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "serve" {
		args = args[1:]
	}

	flags := flag.NewFlagSet("omo serve", flag.ExitOnError)
	addr := flags.String("addr", "127.0.0.1:8080", "HTTP listen address")
	dataPath := flags.String("data", "data/omo.db", "SQLite database path")
	caddyConfig := flags.String("caddy-config", "data/Caddyfile", "Caddy config path")
	caddyUpstream := flags.String("caddy-upstream", "", "Panel upstream address used in generated Caddy config")
	expectedIP := flags.String("expected-ip", "", "Expected public server IP for domain verification")
	singBoxPath := flags.String("sing-box", "", "sing-box binary path")
	singBoxConfigPath := flags.String("sing-box-config", "data/sing-box/config.json", "OMO-managed sing-box configuration path")
	backupDir := flags.String("backup-dir", "data/backups", "OMO backup archive directory")
	updateManifestURL := flags.String("update-manifest", "", "HTTPS update manifest URL")
	updateWorkDir := flags.String("update-work-dir", "data/updates", "OMO update workspace directory")
	publicURL := flags.String("public-url", "", "Public base URL used in generated subscription links")
	if err := flags.Parse(args); err != nil {
		return err
	}

	staticFS, err := fs.Sub(embeddedFiles, "web")
	if err != nil {
		return err
	}

	ctx := context.Background()
	appStore, err := store.Open(ctx, *dataPath)
	if err != nil {
		return err
	}
	defer appStore.Close()

	caddyManager := caddy.NewManager(*caddyConfig)
	var expectedIPs []string
	if *expectedIP != "" {
		expectedIPs = []string{*expectedIP}
	}
	upstream := *caddyUpstream
	if upstream == "" {
		upstream = *addr
	}
	bootstrapSvc := bootstrap.NewServiceWithPhase2(appStore, bootstrap.CaddyPhase2Hook{
		Manager:     caddyManager,
		ExpectedIPs: expectedIPs,
		Upstream:    upstream,
	})
	authSvc := auth.NewService(appStore)
	singBoxDetector := singbox.NewDetector(singbox.Options{BinaryPath: *singBoxPath})
	profileRegistry, err := protocol.DefaultRegistry()
	if err != nil {
		return err
	}
	configManager, err := configgen.NewManager(configgen.Options{
		ConfigPath: *singBoxConfigPath,
		Registry:   profileRegistry,
	})
	if err != nil {
		return err
	}
	configSvc := configgen.NewService(configManager, appStore)
	subscriptionSvc := subscription.NewService(appStore, *publicURL)
	diagnosticsSvc := diagnostics.NewServiceWithOptions(diagnostics.Options{Store: appStore, Core: singBoxDetector})
	pairingSvc := pairing.NewService(appStore)
	backupSvc := backup.NewServiceWithOptions(backup.Options{
		Store:     appStore,
		BackupDir: *backupDir,
		Version:   version.Info(),
		Files: []backup.FileSpec{
			{Label: "caddy-config", Path: *caddyConfig},
			{Label: "sing-box-config", Path: *singBoxConfigPath},
		},
	})
	auditSvc := audit.NewService(appStore)
	updateHealthURL := "http://" + upstream + "/api/system/health"
	binaryPath, _ := os.Executable()
	updateSvc := update.NewServiceWithOptions(update.Options{
		Store:          appStore,
		CurrentVersion: version.Info(),
		Backup:         backupSvc,
		BinaryPath:     binaryPath,
		WorkDir:        *updateWorkDir,
		RestartCommand: []string{"systemctl", "restart", "omo.service"},
		HealthURL:      updateHealthURL,
	})
	if strings.TrimSpace(*updateManifestURL) != "" {
		if err := updateSvc.SaveManifestURL(ctx, *updateManifestURL); err != nil {
			return err
		}
	}
	initToken, err := bootstrapSvc.EnsureInitToken(ctx)
	if err != nil {
		return err
	}

	server := &http.Server{
		Addr:              *addr,
		Handler:           api.NewRouter(api.Config{StaticFS: staticFS, Bootstrap: bootstrapSvc, Auth: authSvc, Store: appStore, SingBox: singBoxDetector, Profiles: profileRegistry, ConfigGen: configSvc, Subscriptions: subscriptionSvc, Diagnostics: diagnosticsSvc, Pairing: pairingSvc, Backup: backupSvc, Audit: auditSvc, Update: updateSvc, Version: version.Info()}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("omo server listening", "addr", *addr)
		if initToken != nil && initToken.Generated {
			slog.Info("omo initialization link", "url", initURL(*addr, initToken.Token), "expiresAt", initToken.ExpiresAt.Format(time.RFC3339))
		}
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-stopCh:
		slog.Info("shutdown signal received", "signal", sig.String())
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

func initURL(addr string, token string) string {
	host := strings.TrimSpace(os.Getenv("OMO_INIT_URL_HOST"))
	if host == "" {
		host = publicListenHost(addr)
	}

	host = stripURLScheme(host)
	if _, _, err := net.SplitHostPort(host); err != nil {
		if _, port, splitErr := net.SplitHostPort(addr); splitErr == nil && port != "" {
			host = net.JoinHostPort(strings.Trim(host, "[]"), port)
		}
	}

	link := url.URL{Scheme: "http", Host: host, Path: "/init"}
	query := link.Query()
	query.Set("token", token)
	link.RawQuery = query.Encode()
	return link.String()
}

func publicListenHost(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}

func stripURLScheme(host string) string {
	if strings.HasPrefix(host, "http://") {
		return strings.TrimPrefix(host, "http://")
	}
	if strings.HasPrefix(host, "https://") {
		return strings.TrimPrefix(host, "https://")
	}
	return host
}
