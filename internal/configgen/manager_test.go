package configgen

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type validatorFunc func(context.Context, string) error

func (f validatorFunc) Validate(ctx context.Context, path string) error {
	return f(ctx, path)
}

type healthFunc func(context.Context, string) error

func (f healthFunc) Check(ctx context.Context, address string) error {
	return f(ctx, address)
}

func TestApplyWritesConfigAndBackupsPrevious(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	oldConfig := []byte(`{"log":{"level":"warn"},"outbounds":[{"type":"direct","tag":"old"}]}` + "\n")
	if err := os.WriteFile(configPath, oldConfig, 0o600); err != nil {
		t.Fatalf("write old config: %v", err)
	}
	manager := testManager(t, configPath, JSONValidator{})

	result, err := manager.Apply(ctx, "standard-secure-access")
	if err != nil {
		t.Fatalf("apply config: %v", err)
	}
	if result.ProfileID != "standard-secure-access" || result.ConfigPath != configPath {
		t.Fatalf("unexpected apply result: %#v", result)
	}
	current, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read current config: %v", err)
	}
	if !strings.Contains(string(current), "omo-standard-secure-access") {
		t.Fatalf("expected rendered service profile tag, got %s", string(current))
	}
	if !strings.Contains(string(current), `"type": "trojan"`) || !strings.Contains(string(current), `"path": "/omo-access/standard-secure-access"`) || !strings.Contains(string(current), `"password"`) {
		t.Fatalf("expected runnable access inbound, got %s", string(current))
	}
	previous, err := os.ReadFile(configPath + ".previous")
	if err != nil {
		t.Fatalf("read previous config: %v", err)
	}
	if string(previous) != string(oldConfig) {
		t.Fatalf("expected previous config backup, got %s", string(previous))
	}
	if result.BackupPath == "" {
		t.Fatal("expected versioned backup path")
	}
	if _, err := os.Stat(result.BackupPath); err != nil {
		t.Fatalf("expected versioned backup file: %v", err)
	}
}

func TestApplyRejectsProfilesThatAreNotDistributionReady(t *testing.T) {
	ctx := context.Background()
	configPath := filepath.Join(t.TempDir(), "config.json")
	manager := testManager(t, configPath, JSONValidator{})

	if _, err := manager.Apply(ctx, "mobile-optimized-access"); !errors.Is(err, ErrUnsupportedProfile) {
		t.Fatalf("expected unsupported profile error, got %v", err)
	}
}

func TestRollbackRestoresPreviousConfig(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	oldConfig := []byte(`{"log":{"level":"warn"},"outbounds":[{"type":"direct","tag":"old"}]}` + "\n")
	if err := os.WriteFile(configPath, oldConfig, 0o600); err != nil {
		t.Fatalf("write old config: %v", err)
	}
	manager := testManager(t, configPath, JSONValidator{})
	if _, err := manager.Apply(ctx, "standard-secure-access"); err != nil {
		t.Fatalf("apply config: %v", err)
	}

	result, err := manager.Rollback(ctx, "standard-secure-access")
	if err != nil {
		t.Fatalf("rollback config: %v", err)
	}
	if !result.RolledBack {
		t.Fatalf("expected rollback result, got %#v", result)
	}
	current, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read current config: %v", err)
	}
	if string(current) != string(oldConfig) {
		t.Fatalf("expected old config after rollback, got %s", string(current))
	}
}

func TestApplyRestoresPreviousConfigAfterPostApplyValidationFailure(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	oldConfig := []byte(`{"log":{"level":"warn"},"outbounds":[{"type":"direct","tag":"old"}]}` + "\n")
	if err := os.WriteFile(configPath, oldConfig, 0o600); err != nil {
		t.Fatalf("write old config: %v", err)
	}
	calls := 0
	validator := validatorFunc(func(_ context.Context, _ string) error {
		calls++
		if calls == 2 {
			return errors.New("post-apply validation failed")
		}
		return nil
	})
	manager := testManager(t, configPath, validator)

	if _, err := manager.Apply(ctx, "standard-secure-access"); err == nil {
		t.Fatal("expected post-apply validation failure")
	}
	current, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read current config: %v", err)
	}
	if string(current) != string(oldConfig) {
		t.Fatalf("expected automatic rollback to old config, got %s", string(current))
	}
}

func TestApplyReportsConfigWriteFailure(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "missing", "config.json")
	if err := os.WriteFile(filepath.Join(dir, "missing"), []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("write blocking file: %v", err)
	}
	manager := testManager(t, configPath, JSONValidator{})

	if _, err := manager.Apply(ctx, "standard-secure-access"); !errors.Is(err, ErrConfigWrite) {
		t.Fatalf("expected config write error, got %v", err)
	}
}

func TestApplyRequiresRunningEntryWhenHealthCheckerIsConfigured(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	oldConfig := []byte(`{"log":{"level":"warn"},"outbounds":[{"type":"direct","tag":"old"}]}` + "\n")
	if err := os.WriteFile(configPath, oldConfig, 0o600); err != nil {
		t.Fatalf("write old config: %v", err)
	}
	manager, err := NewManager(Options{
		ConfigPath: configPath,
		BackupDir:  filepath.Join(filepath.Dir(configPath), "backups"),
		Validator:  JSONValidator{},
		EntryHealth: healthFunc(func(_ context.Context, address string) error {
			if address != "127.0.0.1:21080" {
				t.Fatalf("unexpected health address %s", address)
			}
			return errors.New("entry is not listening")
		}),
	})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	if _, err := manager.Apply(ctx, "standard-secure-access"); err == nil || !strings.Contains(err.Error(), "entry is not listening") {
		t.Fatalf("expected entry health failure, got %v", err)
	}
	current, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read current config: %v", err)
	}
	if string(current) != string(oldConfig) {
		t.Fatalf("expected old config after failed health check, got %s", string(current))
	}
}

func testManager(t *testing.T, configPath string, validator Validator) *Manager {
	t.Helper()
	manager, err := NewManager(Options{
		ConfigPath: configPath,
		BackupDir:  filepath.Join(filepath.Dir(configPath), "backups"),
		Validator:  validator,
	})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	return manager
}
