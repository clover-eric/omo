package diagnostics

import (
	"context"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"omo/internal/core/singbox"
	"omo/internal/store"
)

func TestRunPersistsReportAndEvents(t *testing.T) {
	ctx := context.Background()
	appStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer appStore.Close()

	service := NewService(appStore)
	result, err := service.Run(ctx)
	if err != nil {
		t.Fatalf("run diagnostics: %v", err)
	}
	if result.Job.Kind != JobKindDiagnostics || result.Job.Status != "succeeded" {
		t.Fatalf("expected succeeded diagnostics job, got %#v", result.Job)
	}
	if result.Report.ID == "" || len(result.Report.Checks) == 0 {
		t.Fatalf("expected persisted report with checks, got %#v", result.Report)
	}

	latest, err := service.Latest(ctx)
	if err != nil {
		t.Fatalf("latest diagnostics: %v", err)
	}
	if latest.Report == nil || latest.Report.ID != result.Report.ID {
		t.Fatalf("expected latest report %q, got %#v", result.Report.ID, latest.Report)
	}

	events, err := service.Events(ctx, 0)
	if err != nil {
		t.Fatalf("events: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected diagnostics job events")
	}
}

func TestRunIncludesConfiguredProviders(t *testing.T) {
	ctx := context.Background()
	appStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer appStore.Close()
	if err := appStore.SetSetting(ctx, "bootstrap.domain", "ops.example.com"); err != nil {
		t.Fatalf("set domain: %v", err)
	}

	service := NewServiceWithOptions(Options{
		Store:    appStore,
		Resolver: fakeResolver{hosts: []string{"203.0.113.10"}},
		Dialer:   fakeDialer{},
		TLSCheck: fakeTLSChecker{},
		Core:     fakeCore{status: singbox.Status{Installed: true, Healthy: true, Version: "1.12.0", Path: "/usr/local/bin/sing-box", Message: "access core ready"}},
		Ports:    []int{80, 443},
	})
	result, err := service.Run(ctx)
	if err != nil {
		t.Fatalf("run diagnostics: %v", err)
	}

	for _, id := range []string{"domain-dns", "domain-tls", "local-port-80", "local-port-443", "access-core"} {
		if !hasCheck(result.Report.Checks, id) {
			t.Fatalf("expected check %s in %#v", id, result.Report.Checks)
		}
	}
	if result.Report.Status != "ok" {
		t.Fatalf("expected ok report, got %s", result.Report.Status)
	}
}

func TestRunIncludesOptionalExternalProviderOnlyWhenEnabled(t *testing.T) {
	ctx := context.Background()
	appStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer appStore.Close()

	service := NewServiceWithOptions(Options{
		Store:            appStore,
		ExternalProvider: fakeExternalProvider{},
	})
	withoutProvider, err := service.Run(ctx)
	if err != nil {
		t.Fatalf("run diagnostics without provider: %v", err)
	}
	if hasCheck(withoutProvider.Report.Checks, "external-provider") {
		t.Fatalf("external provider should not run without explicit settings: %#v", withoutProvider.Report.Checks)
	}

	if err := appStore.SetSetting(ctx, "diagnostics.external_provider", `{"enabled":true,"name":"Operator IP quality","endpointUrl":"https://provider.example/check","timeoutSeconds":2}`); err != nil {
		t.Fatalf("set provider: %v", err)
	}
	if err := appStore.SetSetting(ctx, "diagnostics.external_provider.api_key", "secret-value"); err != nil {
		t.Fatalf("set provider key: %v", err)
	}
	withProvider, err := service.Run(ctx)
	if err != nil {
		t.Fatalf("run diagnostics with provider: %v", err)
	}
	if !hasCheck(withProvider.Report.Checks, "external-provider") {
		t.Fatalf("expected external provider check in %#v", withProvider.Report.Checks)
	}
}

type fakeResolver struct {
	hosts []string
}

func (r fakeResolver) LookupHost(context.Context, string) ([]string, error) {
	return r.hosts, nil
}

type fakeDialer struct{}

func (fakeDialer) DialContext(context.Context, string, string) (net.Conn, error) {
	return fakeConn{}, nil
}

type fakeConn struct{}

func (fakeConn) Read([]byte) (int, error)         { return 0, nil }
func (fakeConn) Write([]byte) (int, error)        { return 0, nil }
func (fakeConn) Close() error                     { return nil }
func (fakeConn) LocalAddr() net.Addr              { return fakeAddr("local") }
func (fakeConn) RemoteAddr() net.Addr             { return fakeAddr("remote") }
func (fakeConn) SetDeadline(time.Time) error      { return nil }
func (fakeConn) SetReadDeadline(time.Time) error  { return nil }
func (fakeConn) SetWriteDeadline(time.Time) error { return nil }

type fakeAddr string

func (a fakeAddr) Network() string { return string(a) }
func (a fakeAddr) String() string  { return string(a) }

type fakeTLSChecker struct{}

func (fakeTLSChecker) Check(context.Context, string) TLSResult {
	return TLSResult{Status: "ok", Message: "tls ready", Evidence: "issuer=test"}
}

type fakeCore struct {
	status singbox.Status
}

func (c fakeCore) Status(context.Context) (singbox.Status, error) {
	return c.status, nil
}

type fakeExternalProvider struct{}

func (fakeExternalProvider) Check(_ context.Context, config ExternalProviderConfig) DiagnosticCheck {
	if config.APIKey != "secret-value" {
		return DiagnosticCheck{ID: "external-provider", Label: config.Name, Status: "warning", Message: "provider key missing"}
	}
	return DiagnosticCheck{ID: "external-provider", Label: config.Name, Status: "ok", Message: "provider ready", Evidence: config.EndpointURL}
}

func hasCheck(checks []DiagnosticCheck, id string) bool {
	for _, check := range checks {
		if strings.EqualFold(check.ID, id) {
			return true
		}
	}
	return false
}
