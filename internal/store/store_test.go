package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenAppliesMigrations(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	if err := store.SetSetting(ctx, "test.key", "value"); err != nil {
		t.Fatalf("set setting: %v", err)
	}

	value, ok, err := store.GetSetting(ctx, "test.key")
	if err != nil {
		t.Fatalf("get setting: %v", err)
	}
	if !ok || value != "value" {
		t.Fatalf("expected stored setting, got ok=%v value=%q", ok, value)
	}
}

func TestBackupRecordsAndDatabaseRestore(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "omo.db")
	appStore, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer appStore.Close()

	if err := appStore.SetSetting(ctx, "restore.marker", "before-backup"); err != nil {
		t.Fatalf("set marker: %v", err)
	}
	snapshot := filepath.Join(t.TempDir(), "snapshot.db")
	if err := appStore.BackupDatabaseSnapshot(ctx, snapshot); err != nil {
		t.Fatalf("backup snapshot: %v", err)
	}
	record, err := appStore.CreateBackupRecord(ctx, "running", snapshot)
	if err != nil {
		t.Fatalf("create backup record: %v", err)
	}
	completed, err := appStore.CompleteBackupRecord(ctx, record.ID, "ready", "checksum", time.Now().UTC())
	if err != nil {
		t.Fatalf("complete backup record: %v", err)
	}
	if completed == nil || completed.Status != "ready" || completed.CompletedAt == nil {
		t.Fatalf("expected completed backup record, got %#v", completed)
	}
	records, err := appStore.ListBackupRecords(ctx)
	if err != nil {
		t.Fatalf("list backup records: %v", err)
	}
	if len(records) != 1 || records[0].ID != record.ID {
		t.Fatalf("expected one backup record, got %#v", records)
	}

	if err := appStore.SetSetting(ctx, "restore.marker", "after-backup"); err != nil {
		t.Fatalf("change marker: %v", err)
	}
	if err := appStore.RestoreDatabaseSnapshot(ctx, snapshot); err != nil {
		t.Fatalf("restore snapshot: %v", err)
	}
	value, ok, err := appStore.GetSetting(ctx, "restore.marker")
	if err != nil {
		t.Fatalf("read marker after restore: %v", err)
	}
	if !ok || value != "before-backup" {
		t.Fatalf("expected restored marker, got ok=%v value=%q", ok, value)
	}
}

func TestAuditLogsPersistAndListLatestFirst(t *testing.T) {
	ctx := context.Background()
	appStore, err := Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer appStore.Close()

	admin, err := appStore.CreateAdmin(ctx, "admin", "hash")
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	if err := appStore.AppendAuditLog(ctx, &admin.ID, "auth.login", "admin", admin.ID, "{}"); err != nil {
		t.Fatalf("append login audit: %v", err)
	}
	if err := appStore.AppendAuditLog(ctx, nil, "backup_created", "backup", "bak_test", `{"status":"ready"}`); err != nil {
		t.Fatalf("append backup audit: %v", err)
	}
	logs, err := appStore.ListAuditLogs(ctx, 1)
	if err != nil {
		t.Fatalf("list audit logs: %v", err)
	}
	if len(logs) != 1 || logs[0].Action != "backup_created" || logs[0].ResourceID != "bak_test" {
		t.Fatalf("expected latest backup audit log, got %#v", logs)
	}
}

func TestLoginRateLimitPersistsFailuresAndClears(t *testing.T) {
	ctx := context.Background()
	appStore, err := Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer appStore.Close()

	now := time.Date(2026, 5, 23, 8, 0, 0, 0, time.UTC)
	record, err := appStore.RecordLoginFailure(ctx, "admin", 3, 5*time.Minute, now)
	if err != nil {
		t.Fatalf("record first failure: %v", err)
	}
	if record.FailureCount != 1 || record.LockedUntil != nil {
		t.Fatalf("expected one unlocked failure, got %#v", record)
	}

	record, err = appStore.RecordLoginFailure(ctx, "admin", 3, 5*time.Minute, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("record second failure: %v", err)
	}
	if record.FailureCount != 2 || record.LockedUntil != nil {
		t.Fatalf("expected two unlocked failures, got %#v", record)
	}

	record, err = appStore.RecordLoginFailure(ctx, "admin", 3, 5*time.Minute, now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("record lock failure: %v", err)
	}
	if record.FailureCount != 3 || record.LockedUntil == nil {
		t.Fatalf("expected locked failure record, got %#v", record)
	}
	if !record.LockedUntil.Equal(now.Add(7 * time.Minute)) {
		t.Fatalf("expected locked until %s, got %v", now.Add(7*time.Minute), record.LockedUntil)
	}

	stored, err := appStore.LoginRateLimit(ctx, "admin")
	if err != nil {
		t.Fatalf("read login rate limit: %v", err)
	}
	if stored == nil || stored.FailureCount != 3 || stored.LockedUntil == nil {
		t.Fatalf("expected persisted lock record, got %#v", stored)
	}

	record, err = appStore.RecordLoginFailure(ctx, "admin", 3, 5*time.Minute, now.Add(8*time.Minute))
	if err != nil {
		t.Fatalf("record failure after lock expiry: %v", err)
	}
	if record.FailureCount != 1 || record.LockedUntil != nil {
		t.Fatalf("expected expired lockout to restart failure count, got %#v", record)
	}

	if err := appStore.ClearLoginRateLimit(ctx, "admin"); err != nil {
		t.Fatalf("clear login rate limit: %v", err)
	}
	stored, err = appStore.LoginRateLimit(ctx, "admin")
	if err != nil {
		t.Fatalf("read after clear: %v", err)
	}
	if stored != nil {
		t.Fatalf("expected cleared login rate limit, got %#v", stored)
	}
}

func TestDiagnosticReportPersistsAndLoadsLatest(t *testing.T) {
	ctx := context.Background()
	appStore, err := Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer appStore.Close()

	first, err := appStore.CreateDiagnosticReport(ctx, "ok", "first report", `{"status":"ok"}`)
	if err != nil {
		t.Fatalf("create first report: %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	second, err := appStore.CreateDiagnosticReport(ctx, "warning", "second report", `{"status":"warning"}`)
	if err != nil {
		t.Fatalf("create second report: %v", err)
	}
	if first.ID == second.ID {
		t.Fatalf("expected unique report ids, got %q", first.ID)
	}

	latest, err := appStore.LatestDiagnosticReport(ctx)
	if err != nil {
		t.Fatalf("latest report: %v", err)
	}
	if latest == nil || latest.ID != second.ID || latest.Summary != "second report" {
		t.Fatalf("expected latest second report, got %#v", latest)
	}
}

func TestServiceInstancesActivateAndDeactivateByProfile(t *testing.T) {
	ctx := context.Background()
	appStore, err := Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer appStore.Close()

	if err := appStore.EnsureServiceProfile(ctx, "standard-secure-access", "2026.05.1", "Standard secure access", "sing-box tls/tcp"); err != nil {
		t.Fatalf("ensure service profile: %v", err)
	}
	planned, err := appStore.CreateServiceInstance(ctx, "standard-secure-access", "Team access", 0, "planned", "2026.05.1")
	if err != nil {
		t.Fatalf("create service instance: %v", err)
	}

	active, err := appStore.ActivateServiceInstancesForProfile(ctx, "standard-secure-access", "Standard secure access", 21080, "cfg001", "omo", "secret", "/omo-access/standard-secure-access")
	if err != nil {
		t.Fatalf("activate service instances: %v", err)
	}
	if len(active) != 1 || active[0].ID != planned.ID || active[0].Status != "active" || active[0].ListenPort != 21080 || active[0].ConfigVersion != "cfg001" || active[0].AccessPassword != "secret" || active[0].AccessPath == "" {
		t.Fatalf("expected active instance update, got %#v", active)
	}

	rolledBack, err := appStore.DeactivateServiceInstancesForProfile(ctx, "standard-secure-access", "cfg002")
	if err != nil {
		t.Fatalf("deactivate service instances: %v", err)
	}
	if len(rolledBack) != 1 || rolledBack[0].Status != "planned" || rolledBack[0].ConfigVersion != "cfg002" {
		t.Fatalf("expected planned instance after rollback, got %#v", rolledBack)
	}
}

func TestActivateServiceInstanceKeepsOnlyOneActiveProfile(t *testing.T) {
	ctx := context.Background()
	appStore, err := Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer appStore.Close()

	if err := appStore.EnsureServiceProfile(ctx, "standard-secure-access", "2026.05.1", "Standard secure access", "sing-box tls/tcp"); err != nil {
		t.Fatalf("ensure standard profile: %v", err)
	}
	if err := appStore.EnsureServiceProfile(ctx, "broad-compatibility-access", "2026.05.1", "Broad compatibility access", "sing-box tcp compatibility"); err != nil {
		t.Fatalf("ensure compatibility profile: %v", err)
	}
	if _, err := appStore.ActivateServiceInstancesForProfile(ctx, "standard-secure-access", "Standard secure access", 21080, "cfg001", "omo", "secret-a", "/omo-access/standard-secure-access"); err != nil {
		t.Fatalf("activate standard: %v", err)
	}
	if _, err := appStore.ActivateServiceInstancesForProfile(ctx, "broad-compatibility-access", "Broad compatibility access", 21082, "cfg002", "omo", "secret-b", "/omo-access/broad-compatibility-access"); err != nil {
		t.Fatalf("activate compatibility: %v", err)
	}
	instances, err := appStore.ListServiceInstances(ctx)
	if err != nil {
		t.Fatalf("list instances: %v", err)
	}
	active := 0
	for _, instance := range instances {
		if instance.Status == "active" {
			active++
			if instance.ProfileID != "broad-compatibility-access" {
				t.Fatalf("expected only compatibility active, got %#v", instance)
			}
		}
	}
	if active != 1 {
		t.Fatalf("expected exactly one active instance, got %d in %#v", active, instances)
	}
}

func TestCascadePairingRecordsPersist(t *testing.T) {
	ctx := context.Background()
	appStore, err := Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer appStore.Close()

	expiresAt := time.Now().UTC().Add(15 * time.Minute)
	code, err := appStore.CreatePairingCode(ctx, "node_remote", "Exit node", "exit.example.com", "hash-value", "public-key", "signature", expiresAt)
	if err != nil {
		t.Fatalf("create pairing code: %v", err)
	}
	found, err := appStore.PairingCodeByHash(ctx, "hash-value", time.Now().UTC())
	if err != nil {
		t.Fatalf("find pairing code: %v", err)
	}
	if found == nil || found.ID != code.ID || found.Status != "active" {
		t.Fatalf("expected active pairing code, got %#v", found)
	}
	if err := appStore.MarkPairingCodeUsed(ctx, code.ID, time.Now().UTC()); err != nil {
		t.Fatalf("mark used: %v", err)
	}
	found, err = appStore.PairingCodeByHash(ctx, "hash-value", time.Now().UTC())
	if err != nil {
		t.Fatalf("find used pairing code: %v", err)
	}
	if found != nil {
		t.Fatalf("expected used pairing code to be unavailable, got %#v", found)
	}
	used, err := appStore.AnyPairingCodeByHash(ctx, "hash-value")
	if err != nil {
		t.Fatalf("find any pairing code: %v", err)
	}
	if used == nil || used.Status != "used" {
		t.Fatalf("expected used pairing code lookup for local reuse detection, got %#v", used)
	}

	local, err := appStore.CreateCascadeNode(ctx, "Local entry", "entry.example.com", "trusted", "entry", "entry-fingerprint")
	if err != nil {
		t.Fatalf("create local node: %v", err)
	}
	node, err := appStore.CreateCascadeNode(ctx, "Exit node", "exit.example.com", "trusted", "exit", "fingerprint")
	if err != nil {
		t.Fatalf("create node: %v", err)
	}
	pair, err := appStore.CreateCascadePair(ctx, local.ID, node.ID, "pending", "pending_apply")
	if err != nil {
		t.Fatalf("create pair: %v", err)
	}
	nodes, err := appStore.ListCascadeNodes(ctx)
	if err != nil {
		t.Fatalf("list nodes: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("expected two node records, got %#v", nodes)
	}
	pairs, err := appStore.ListCascadePairs(ctx)
	if err != nil {
		t.Fatalf("list pairs: %v", err)
	}
	if len(pairs) != 1 || pairs[0].ID != pair.ID || pairs[0].ConfigState != "pending_apply" {
		t.Fatalf("expected pending pair, got %#v", pairs)
	}
	updated, err := appStore.UpdateCascadePairConfigState(ctx, pair.ID, "active", "applied")
	if err != nil {
		t.Fatalf("update pair config state: %v", err)
	}
	if updated == nil || updated.Status != "active" || updated.ConfigState != "applied" {
		t.Fatalf("expected applied pair state, got %#v", updated)
	}
	foundPair, err := appStore.CascadePairByID(ctx, pair.ID)
	if err != nil {
		t.Fatalf("find pair by id: %v", err)
	}
	if foundPair == nil || foundPair.ConfigState != "applied" {
		t.Fatalf("expected persisted applied pair, got %#v", foundPair)
	}
	sample, err := appStore.RecordCascadeHealthSample(ctx, node.ID, true, 42, 1.25, "", time.Now().UTC())
	if err != nil {
		t.Fatalf("record health sample: %v", err)
	}
	if !sample.Online || sample.LatencyMS != 42 || sample.ThroughputMbps != 1.25 {
		t.Fatalf("expected online sample, got %#v", sample)
	}
	updatedNode, err := appStore.CascadeNodeByID(ctx, node.ID)
	if err != nil {
		t.Fatalf("find sampled node: %v", err)
	}
	if updatedNode == nil || !updatedNode.Online || updatedNode.LatencyMS != 42 {
		t.Fatalf("expected sampled node health, got %#v", updatedNode)
	}
}
