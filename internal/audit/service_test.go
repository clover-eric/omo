package audit

import (
	"context"
	"path/filepath"
	"testing"

	"omo/internal/store"
)

func TestAuditServiceListsParsedDetails(t *testing.T) {
	ctx := context.Background()
	appStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer appStore.Close()

	if err := appStore.AppendAuditLog(ctx, nil, "backup_created", "backup", "bak_test", `{"status":"ready"}`); err != nil {
		t.Fatalf("append json audit log: %v", err)
	}
	if err := appStore.AppendAuditLog(ctx, nil, "legacy_event", "system", "legacy", `not-json`); err != nil {
		t.Fatalf("append legacy audit log: %v", err)
	}

	result, err := NewService(appStore).List(ctx, 10)
	if err != nil {
		t.Fatalf("list audit logs: %v", err)
	}
	if len(result.Logs) != 2 {
		t.Fatalf("expected two audit logs, got %#v", result.Logs)
	}
	if result.Logs[0].Details["raw"] != "not-json" {
		t.Fatalf("expected raw fallback details for latest log, got %#v", result.Logs[0])
	}
	if result.Logs[1].Details["status"] != "ready" {
		t.Fatalf("expected parsed json details, got %#v", result.Logs[1])
	}
}
