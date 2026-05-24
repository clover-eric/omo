package backup

import (
	"archive/zip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"omo/internal/store"
)

func TestBackupCreateListAndRestore(t *testing.T) {
	ctx := context.Background()
	appStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer appStore.Close()

	if err := appStore.SetSetting(ctx, "restore.marker", "before-backup"); err != nil {
		t.Fatalf("set marker: %v", err)
	}
	service := NewService(appStore, filepath.Join(t.TempDir(), "backups"))
	created, err := service.Create(ctx)
	if err != nil {
		t.Fatalf("create backup: %v", err)
	}
	assertEncryptedArchive(t, created.Backup.Path)
	assertKeyFile(t, service.keyPath)
	if created.Backup.Status != "ready" || created.Backup.Checksum == "" || created.Job.Status != "succeeded" {
		t.Fatalf("expected ready backup and succeeded job, got %#v", created)
	}
	list, err := service.List(ctx)
	if err != nil {
		t.Fatalf("list backups: %v", err)
	}
	if len(list.Backups) != 1 || list.Backups[0].ID != created.Backup.ID {
		t.Fatalf("expected created backup in list, got %#v", list.Backups)
	}
	if err := appStore.SetSetting(ctx, "restore.marker", "after-backup"); err != nil {
		t.Fatalf("change marker: %v", err)
	}
	if _, err := service.Restore(ctx, created.Backup.ID, RestoreRequest{Confirm: false}); err != ErrConfirmationRequired {
		t.Fatalf("expected restore confirmation error, got %v", err)
	}
	restored, err := service.Restore(ctx, created.Backup.ID, RestoreRequest{Confirm: true})
	if err != nil {
		t.Fatalf("restore backup: %v", err)
	}
	if !restored.Restored || restored.Job.Status != "succeeded" {
		t.Fatalf("expected restored result, got %#v", restored)
	}
	value, ok, err := appStore.GetSetting(ctx, "restore.marker")
	if err != nil {
		t.Fatalf("read restored marker: %v", err)
	}
	if !ok || value != "before-backup" {
		t.Fatalf("expected restored marker, got ok=%v value=%q", ok, value)
	}
}

func TestBackupIncludesAndRestoresConfiguredFiles(t *testing.T) {
	ctx := context.Background()
	appStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer appStore.Close()

	configDir := t.TempDir()
	caddyPath := filepath.Join(configDir, "Caddyfile")
	singBoxPath := filepath.Join(configDir, "sing-box.json")
	if err := os.WriteFile(caddyPath, []byte("ops.example.com\n"), 0o600); err != nil {
		t.Fatalf("write caddy config: %v", err)
	}
	if err := os.WriteFile(singBoxPath, []byte(`{"managedBy":"omo"}`+"\n"), 0o600); err != nil {
		t.Fatalf("write sing-box config: %v", err)
	}

	service := NewServiceWithOptions(Options{
		Store:     appStore,
		BackupDir: filepath.Join(t.TempDir(), "backups"),
		Version:   "test-version",
		Files: []FileSpec{
			{Label: "caddy-config", Path: caddyPath},
			{Label: "sing-box-config", Path: singBoxPath},
		},
	})
	created, err := service.Create(ctx)
	if err != nil {
		t.Fatalf("create backup: %v", err)
	}
	meta := readBackupManifest(t, created.Backup.Path, service.keyPath)
	if meta.Runtime.AppVersion != "test-version" || len(meta.Files) != 2 {
		t.Fatalf("expected runtime metadata and two files, got %#v", meta)
	}
	if err := os.WriteFile(caddyPath, []byte("changed\n"), 0o600); err != nil {
		t.Fatalf("change caddy config: %v", err)
	}
	if err := os.WriteFile(singBoxPath, []byte(`{"changed":true}`+"\n"), 0o600); err != nil {
		t.Fatalf("change sing-box config: %v", err)
	}
	if _, err := service.Restore(ctx, created.Backup.ID, RestoreRequest{Confirm: true}); err != nil {
		t.Fatalf("restore backup: %v", err)
	}
	caddyContent, err := os.ReadFile(caddyPath)
	if err != nil {
		t.Fatalf("read restored caddy config: %v", err)
	}
	if string(caddyContent) != "ops.example.com\n" {
		t.Fatalf("expected restored caddy config, got %q", string(caddyContent))
	}
	singBoxContent, err := os.ReadFile(singBoxPath)
	if err != nil {
		t.Fatalf("read restored sing-box config: %v", err)
	}
	if string(singBoxContent) != `{"managedBy":"omo"}`+"\n" {
		t.Fatalf("expected restored sing-box config, got %q", string(singBoxContent))
	}
}

func TestBackupIncludesCertificateMetadataOnly(t *testing.T) {
	ctx := context.Background()
	appStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer appStore.Close()

	if err := appStore.SetSetting(ctx, "bootstrap.domain", "ops.example.com"); err != nil {
		t.Fatalf("set domain: %v", err)
	}
	if err := appStore.SetSetting(ctx, "bootstrap.phase2_result", `{"certificateIssuer":"Test CA","securityState":"ready","entryMode":"https_domain"}`); err != nil {
		t.Fatalf("set phase2 result: %v", err)
	}

	service := NewServiceWithOptions(Options{
		Store:     appStore,
		BackupDir: filepath.Join(t.TempDir(), "backups"),
		Version:   "test-version",
	})
	created, err := service.Create(ctx)
	if err != nil {
		t.Fatalf("create backup: %v", err)
	}
	meta := readBackupManifest(t, created.Backup.Path, service.keyPath)
	if len(meta.Certificates) != 1 {
		t.Fatalf("expected certificate metadata, got %#v", meta.Certificates)
	}
	certificate := meta.Certificates[0]
	if certificate.Domain != "ops.example.com" || certificate.Issuer != "Test CA" || !certificate.Available || !certificate.MetadataOnly {
		t.Fatalf("expected certificate metadata only, got %#v", certificate)
	}
	if certificate.Note == "" || strings.Contains(strings.ToLower(certificate.Note), "private key material is included") {
		t.Fatalf("expected private key exclusion note, got %#v", certificate)
	}
}

func readBackupManifest(t *testing.T, archivePath string, keyPath string) manifest {
	t.Helper()
	workDir := t.TempDir()
	readablePath, err := prepareReadableArchive(archivePath, workDir, keyPath)
	if err != nil {
		t.Fatalf("prepare readable archive: %v", err)
	}
	reader, err := zip.OpenReader(readablePath)
	if err != nil {
		t.Fatalf("open backup archive: %v", err)
	}
	defer reader.Close()
	for _, file := range reader.File {
		if file.Name != "manifest.json" {
			continue
		}
		src, err := file.Open()
		if err != nil {
			t.Fatalf("open manifest: %v", err)
		}
		defer src.Close()
		var meta manifest
		if err := json.NewDecoder(src).Decode(&meta); err != nil {
			t.Fatalf("decode manifest: %v", err)
		}
		return meta
	}
	t.Fatal("manifest not found")
	return manifest{}
}

func assertEncryptedArchive(t *testing.T, archivePath string) {
	t.Helper()
	if filepath.Ext(archivePath) != ".enc" {
		t.Fatalf("expected encrypted archive extension, got %q", archivePath)
	}
	if !isEncryptedArchive(archivePath) {
		t.Fatalf("expected encrypted archive, got %q", archivePath)
	}
	reader, err := zip.OpenReader(archivePath)
	if err == nil {
		_ = reader.Close()
		t.Fatal("expected encrypted archive not to open as plain zip")
	}
}

func assertKeyFile(t *testing.T, keyPath string) {
	t.Helper()
	data, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read backup key file: %v", err)
	}
	if len(strings.TrimSpace(string(data))) != 64 {
		t.Fatalf("expected 32-byte hex key, got %q", strings.TrimSpace(string(data)))
	}
}
