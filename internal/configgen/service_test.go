package configgen

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"omo/internal/store"
)

func TestServiceApplyAndRollbackUpdatesManagedInstances(t *testing.T) {
	ctx := context.Background()
	appStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer appStore.Close()
	if err := appStore.EnsureServiceProfile(ctx, "standard-secure-access", "2026.05.1", "Standard secure access", "sing-box tls/tcp"); err != nil {
		t.Fatalf("ensure service profile: %v", err)
	}
	planned, err := appStore.CreateServiceInstance(ctx, "standard-secure-access", "Team access", 0, "planned", "2026.05.1")
	if err != nil {
		t.Fatalf("create planned service: %v", err)
	}

	configPath := filepath.Join(t.TempDir(), "sing-box", "config.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("create config dir: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(`{"outbounds":[{"type":"direct","tag":"old"}]}`+"\n"), 0o600); err != nil {
		t.Fatalf("write seed config: %v", err)
	}
	manager, err := NewManager(Options{ConfigPath: configPath})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	service := NewService(manager, appStore)

	applyResult, err := service.Apply(ctx, "standard-secure-access")
	if err != nil {
		t.Fatalf("apply service config: %v", err)
	}
	if len(applyResult.Instances) != 1 || applyResult.Instances[0].ID != planned.ID || applyResult.Instances[0].Status != "active" || applyResult.Instances[0].ListenPort != 21080 {
		t.Fatalf("expected active service instance in apply result, got %#v", applyResult.Instances)
	}

	rollbackResult, err := service.Rollback(ctx, "standard-secure-access")
	if err != nil {
		t.Fatalf("rollback service config: %v", err)
	}
	if len(rollbackResult.Instances) != 1 || rollbackResult.Instances[0].Status != "planned" {
		t.Fatalf("expected planned service instance after rollback, got %#v", rollbackResult.Instances)
	}
}
