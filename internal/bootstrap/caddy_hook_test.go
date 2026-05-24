package bootstrap

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"

	"omo/internal/caddy"
)

type phase2Runner struct{}

func (phase2Runner) Run(context.Context, string, ...string) error {
	return nil
}

func (phase2Runner) LookPath(string) (string, error) {
	return "caddy", nil
}

type phase2Resolver struct{}

func (phase2Resolver) LookupHost(context.Context, string) ([]string, error) {
	return []string{"203.0.113.10"}, nil
}

type phase2Dialer struct{}

func (phase2Dialer) DialContext(context.Context, string, string) (net.Conn, error) {
	return nil, errors.New("connection refused")
}

func TestCaddyPhase2RequiresTLSCertificateBeforeReady(t *testing.T) {
	ctx := context.Background()
	configPath := filepath.Join(t.TempDir(), "omo.caddy")
	manager := caddy.NewManager(configPath)
	manager.Runner = phase2Runner{}
	manager.Resolver = phase2Resolver{}
	manager.Dialer = phase2Dialer{}
	manager.CertificateStatusFn = func(context.Context, string) caddy.CertificateStatus {
		return caddy.CertificateStatus{
			Domain:      "ops.example.com",
			Available:   false,
			UserMessage: "certificate is not ready",
		}
	}

	hook := CaddyPhase2Hook{
		Manager:     manager,
		ExpectedIPs: []string{"203.0.113.10"},
		TLSWait:     -1,
	}
	_, err := hook.Run(ctx, "ops.example.com")

	var fallback Phase2FallbackError
	if !errors.As(err, &fallback) {
		t.Fatalf("expected TLS fallback error, got %v", err)
	}
	if fallback.Code != "TLS_CERTIFICATE_NOT_READY" {
		t.Fatalf("expected TLS not ready code, got %q", fallback.Code)
	}
	if fallback.Result.SecurityState != "pending_tls" || fallback.Result.EntryMode != "temporary_http" {
		t.Fatalf("expected pending temporary entry result, got %#v", fallback.Result)
	}
	if _, statErr := os.Stat(configPath); statErr != nil {
		t.Fatalf("expected Caddy config to remain for certificate provisioning: %v", statErr)
	}
}
