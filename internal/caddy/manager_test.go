package caddy

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeRunner struct {
	failReload bool
	lookPath   error
	calls      []string
}

func (r *fakeRunner) Run(_ context.Context, name string, args ...string) error {
	r.calls = append(r.calls, name+" "+join(args))
	if r.failReload && len(args) > 0 && args[0] == "reload" {
		return errors.New("reload failed")
	}
	return nil
}

func (r *fakeRunner) LookPath(string) (string, error) {
	return "", r.lookPath
}

type fakeResolver struct {
	hosts []string
	err   error
}

func (r fakeResolver) LookupHost(context.Context, string) ([]string, error) {
	return r.hosts, r.err
}

type fakeDialer struct {
	busy map[string]bool
}

func (d fakeDialer) DialContext(_ context.Context, _ string, address string) (net.Conn, error) {
	if d.busy[address] {
		left, right := net.Pipe()
		_ = right.Close()
		return left, nil
	}
	return nil, errors.New("connection refused")
}

func TestCheckDomainMatchesExpectedIP(t *testing.T) {
	manager := NewManager("")
	manager.Resolver = fakeResolver{hosts: []string{"203.0.113.10"}}

	check, err := manager.CheckDomain(context.Background(), "ops.example.com", []string{"203.0.113.10"})
	if err != nil {
		t.Fatalf("expected domain match: %v", err)
	}
	if !check.Matches {
		t.Fatal("expected domain to match")
	}
}

func TestCheckPortsDetectsBusyPort(t *testing.T) {
	manager := NewManager("")
	manager.Dialer = fakeDialer{busy: map[string]bool{"127.0.0.1:443": true}}

	checks, err := manager.CheckPorts(context.Background(), 80, 443)
	if !errors.Is(err, ErrPortUnavailable) {
		t.Fatalf("expected port unavailable error, got %v", err)
	}
	if len(checks) != 2 || checks[1].Available {
		t.Fatalf("expected busy 443 check, got %#v", checks)
	}
}

func TestApplyConfigRollbackOnReloadFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Caddyfile")
	if err := os.WriteFile(path, []byte("old config"), 0o600); err != nil {
		t.Fatalf("write old config: %v", err)
	}

	runner := &fakeRunner{failReload: true}
	manager := NewManager(path)
	manager.Runner = runner

	err := manager.ApplyConfig(context.Background(), "new config")
	if err == nil {
		t.Fatal("expected reload failure")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(data) != "old config" {
		t.Fatalf("expected rollback to old config, got %q", string(data))
	}
}

func TestApplyConfigUsesCaddyfileAdapter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "omo.caddy")
	runner := &fakeRunner{}
	manager := NewManager(path)
	manager.Runner = runner

	if err := manager.ApplyConfig(context.Background(), "ops.example.com {\n\treverse_proxy 127.0.0.1:8080\n}\n"); err != nil {
		t.Fatalf("apply config: %v", err)
	}

	if len(runner.calls) != 2 {
		t.Fatalf("expected validate and reload calls, got %#v", runner.calls)
	}
	if !strings.Contains(runner.calls[0], "validate --config") || !strings.Contains(runner.calls[0], "--adapter caddyfile") {
		t.Fatalf("expected validate to use caddyfile adapter, got %q", runner.calls[0])
	}
	if !strings.Contains(runner.calls[1], "reload --config") || !strings.Contains(runner.calls[1], "--adapter caddyfile") {
		t.Fatalf("expected reload to use caddyfile adapter, got %q", runner.calls[1])
	}
}

func TestAvailableChecksCaddyBinary(t *testing.T) {
	manager := NewManager("")
	manager.Runner = &fakeRunner{lookPath: errors.New("not found")}

	if manager.Available() {
		t.Fatal("expected manager to report caddy unavailable")
	}
}

func join(args []string) string {
	out := ""
	for i, arg := range args {
		if i > 0 {
			out += " "
		}
		out += arg
	}
	return out
}
