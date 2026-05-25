package bootstrap

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"omo/internal/store"
)

type failingPhase2Hook struct{}

func (failingPhase2Hook) Run(context.Context, string) (Phase2Result, error) {
	return Phase2Result{}, errors.New("DOMAIN_NOT_RESOLVED")
}

type sequencePhase2Hook struct {
	calls int
}

func (h *sequencePhase2Hook) Run(context.Context, string) (Phase2Result, error) {
	h.calls++
	if h.calls == 1 {
		return Phase2Result{}, errors.New("DOMAIN_NOT_RESOLVED")
	}
	return Phase2Result{Message: "ok"}, nil
}

type fallbackPhase2Hook struct{}

func (fallbackPhase2Hook) Run(context.Context, string) (Phase2Result, error) {
	result := Phase2Result{
		Message:       "Caddy 暂不可用，已保留临时初始化入口，请安装或修复 Caddy 后重试 HTTPS 入口配置。",
		EntryMode:     "temporary_http",
		SecurityState: "degraded",
		Warnings:      []string{"当前未启用正式 HTTPS 面板入口。"},
	}
	return Phase2Result{}, Phase2FallbackError{
		Code:    "CADDY_UNAVAILABLE",
		Message: result.Message,
		Result:  result,
		Cause:   exec.ErrNotFound,
	}
}

func TestBootstrapFailureCanRetry(t *testing.T) {
	ctx := context.Background()
	appStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer appStore.Close()

	hook := &sequencePhase2Hook{}
	service := NewServiceWithPhase2(appStore, hook)
	token, err := service.EnsureInitToken(ctx)
	if err != nil {
		t.Fatalf("ensure token: %v", err)
	}

	req := StartRequest{
		Token:           token.Token,
		Username:        "admin",
		Password:        "StrongPassw0rd!",
		ConfirmPassword: "StrongPassw0rd!",
		Domain:          "ops.example.com",
	}
	if _, err := service.Start(ctx, req); err == nil {
		t.Fatal("expected phase2 failure")
	}

	if _, err := service.Start(ctx, req); !errors.Is(err, ErrRetryRequired) {
		t.Fatalf("expected retry required, got %v", err)
	}

	req.Retry = true
	if _, err := service.Start(ctx, req); err != nil {
		t.Fatalf("expected retry to continue initialization, got %v", err)
	}
	if hook.calls != 2 {
		t.Fatalf("expected phase2 hook to run twice, got %d", hook.calls)
	}
}

func TestBootstrapWritesReadyMarker(t *testing.T) {
	ctx := context.Background()
	appStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer appStore.Close()

	marker := filepath.Join(t.TempDir(), "bootstrap-ready")
	t.Setenv("OMO_BOOTSTRAP_READY_MARKER", marker)

	service := NewService(appStore)
	token, err := service.EnsureInitToken(ctx)
	if err != nil {
		t.Fatalf("ensure token: %v", err)
	}

	_, err = service.Start(ctx, StartRequest{
		Token:           token.Token,
		Username:        "admin",
		Password:        "StrongPassw0rd!",
		ConfirmPassword: "StrongPassw0rd!",
		Domain:          "ops.example.com",
	})
	if err != nil {
		t.Fatalf("start bootstrap: %v", err)
	}

	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("expected ready marker: %v", err)
	}

	status, err := service.Status(ctx)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !status.Initialized || !status.Phase1Complete {
		t.Fatalf("expected initialized status after bootstrap, got %#v", status)
	}
	if status.Domain != "ops.example.com" {
		t.Fatalf("expected bootstrap domain, got %q", status.Domain)
	}
}

func TestEnsureInitTokenRefreshesFromInstallerEnvironment(t *testing.T) {
	ctx := context.Background()
	appStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer appStore.Close()

	t.Setenv("OMO_INIT_TOKEN", "first-token")
	service := NewService(appStore)
	first, err := service.EnsureInitToken(ctx)
	if err != nil {
		t.Fatalf("ensure first token: %v", err)
	}
	if first == nil || first.Token != "first-token" {
		t.Fatalf("expected first env token, got %#v", first)
	}

	t.Setenv("OMO_INIT_TOKEN", "second-token")
	second, err := service.EnsureInitToken(ctx)
	if err != nil {
		t.Fatalf("ensure refreshed token: %v", err)
	}
	if second == nil || second.Token != "second-token" {
		t.Fatalf("expected refreshed env token, got %#v", second)
	}

	if err := service.validateToken(ctx, "second-token"); err != nil {
		t.Fatalf("expected refreshed token to validate: %v", err)
	}
	if err := service.validateToken(ctx, "first-token"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected old token to be invalid, got %v", err)
	}
}

func TestEnsureInitTokenAllowsInstallerRecoveryAfterAdminExists(t *testing.T) {
	ctx := context.Background()
	appStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer appStore.Close()

	passwordHash := "hash"
	if _, err := appStore.CreateAdmin(ctx, "admin", passwordHash); err != nil {
		t.Fatalf("create admin: %v", err)
	}

	t.Setenv("OMO_INIT_TOKEN", "recovery-token")
	service := NewService(appStore)
	token, err := service.EnsureInitToken(ctx)
	if err != nil {
		t.Fatalf("ensure recovery token: %v", err)
	}
	if token == nil || token.Token != "recovery-token" {
		t.Fatalf("expected recovery env token, got %#v", token)
	}
	if err := service.validateToken(ctx, "recovery-token"); err != nil {
		t.Fatalf("expected recovery token to validate: %v", err)
	}
}

func TestRecoveryTokenCanRestartHTTPSConfigurationAfterAdminExists(t *testing.T) {
	ctx := context.Background()
	appStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer appStore.Close()

	service := NewService(appStore)
	token, err := service.EnsureInitToken(ctx)
	if err != nil {
		t.Fatalf("ensure token: %v", err)
	}
	password := "StrongPassw0rd!"
	if _, err := service.Start(ctx, StartRequest{
		Token:           token.Token,
		Username:        "admin",
		Password:        password,
		ConfirmPassword: password,
		Domain:          "ops.example.com",
	}); err != nil {
		t.Fatalf("initial bootstrap: %v", err)
	}

	t.Setenv("OMO_INIT_TOKEN", "recovery-token")
	recovery, err := service.EnsureInitToken(ctx)
	if err != nil {
		t.Fatalf("ensure recovery token: %v", err)
	}
	if _, err := service.Start(ctx, StartRequest{
		Token:           recovery.Token,
		Username:        "admin",
		Password:        password,
		ConfirmPassword: password,
		Domain:          "ops.example.com",
	}); err != nil {
		t.Fatalf("expected recovery bootstrap to continue without retry flag: %v", err)
	}
}

func TestBootstrapFallbackKeepsRetryableState(t *testing.T) {
	ctx := context.Background()
	appStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer appStore.Close()

	service := NewServiceWithPhase2(appStore, fallbackPhase2Hook{})
	token, err := service.EnsureInitToken(ctx)
	if err != nil {
		t.Fatalf("ensure token: %v", err)
	}

	_, err = service.Start(ctx, StartRequest{
		Token:           token.Token,
		Username:        "admin",
		Password:        "StrongPassw0rd!",
		ConfirmPassword: "StrongPassw0rd!",
		Domain:          "ops.example.com",
	})
	var fallback Phase2FallbackError
	if !errors.As(err, &fallback) {
		t.Fatalf("expected fallback error, got %v", err)
	}

	status, err := service.Status(ctx)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.Phase1Complete {
		t.Fatal("expected degraded fallback to keep bootstrap incomplete")
	}
	if !status.RequiresToken {
		t.Fatal("expected initialization token to remain valid for retry")
	}
	if status.LatestJob == nil || status.LatestJob.Status != "failed" || status.LatestJob.State != string(StateTLSProvision) {
		t.Fatalf("expected failed TLS_PROVISION job, got %#v", status.LatestJob)
	}

	raw, ok, err := appStore.GetSetting(ctx, "bootstrap.phase2_result")
	if err != nil {
		t.Fatalf("phase2 result setting: %v", err)
	}
	if !ok || !strings.Contains(raw, `"securityState":"degraded"`) || !strings.Contains(raw, `"entryMode":"temporary_http"`) {
		t.Fatalf("expected degraded phase2 result, got %q", raw)
	}
}
