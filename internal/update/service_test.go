package update

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"omo/internal/backup"
	"omo/internal/store"
)

func TestUpdateCheckReturnsUnconfiguredState(t *testing.T) {
	ctx := context.Background()
	appStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer appStore.Close()

	result, err := NewService(appStore, "0.1.0").Check(ctx)
	if err != nil {
		t.Fatalf("check update: %v", err)
	}
	if result.Configured || result.UpdateAvailable || result.CurrentVersion != "0.1.0" {
		t.Fatalf("expected unconfigured update check, got %#v", result)
	}
}

func TestUpdateCheckReadsHTTPSManifest(t *testing.T) {
	ctx := context.Background()
	appStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer appStore.Close()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"version":"0.2.0",
			"channel":"stable",
			"summary":"Operational maintenance release.",
			"checksumSha256":"manifest-checksum",
			"signature":"manifest-signature",
			"artifacts":[{"os":"` + runtime.GOOS + `","arch":"` + runtime.GOARCH + `","url":"https://releases.example/omo","checksumSha256":"artifact-checksum","signature":"artifact-signature"}]
		}`))
	}))
	defer server.Close()

	service := NewServiceWithClient(appStore, "0.1.0", server.Client())
	if err := service.SaveManifestURL(ctx, server.URL); err != nil {
		t.Fatalf("save manifest url: %v", err)
	}
	result, err := service.Check(ctx)
	if err != nil {
		t.Fatalf("check update: %v", err)
	}
	if !result.Configured || !result.UpdateAvailable || result.LatestVersion != "0.2.0" {
		t.Fatalf("expected available update, got %#v", result)
	}
	if result.ChecksumSHA256 != "artifact-checksum" || result.Signature != "artifact-signature" {
		t.Fatalf("expected artifact verification metadata, got %#v", result)
	}
}

func TestUpdateManifestURLRequiresHTTPS(t *testing.T) {
	ctx := context.Background()
	appStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer appStore.Close()

	if err := NewService(appStore, "0.1.0").SaveManifestURL(ctx, "http://updates.example/manifest.json"); err != ErrInvalidManifestURL {
		t.Fatalf("expected invalid manifest url, got %v", err)
	}
}

func TestUpdateApplyCreatesBackupVerifiesArtifactAndStoresRollback(t *testing.T) {
	ctx := context.Background()
	appStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer appStore.Close()

	tmp := t.TempDir()
	currentBinary := filepath.Join(tmp, "omo")
	if err := os.WriteFile(currentBinary, []byte("old-binary"), 0o755); err != nil {
		t.Fatalf("write current binary: %v", err)
	}
	artifact := makeReleaseArchive(t, []byte("new-binary"))
	checksum := sha256File(t, artifact)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/manifest.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"version":"0.2.0",
				"channel":"stable",
				"summary":"Operational maintenance release.",
				"artifacts":[{"os":"` + runtime.GOOS + `","arch":"` + runtime.GOARCH + `","url":"` + "https://" + r.Host + `/omo.tar.gz","checksumSha256":"` + checksum + `","signature":"verified-by-test"}]
			}`))
		case "/omo.tar.gz":
			http.ServeFile(w, r, artifact)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := NewServiceWithOptions(Options{
		Store: appStore,
		CurrentVersion: "0.1.0",
		Client: server.Client(),
		Backup: fakeBackupRunner{},
		BinaryPath: currentBinary,
		WorkDir: filepath.Join(tmp, "updates"),
		Restarter: fakeRestarter{},
		HealthChecker: fakeHealthChecker{},
		SignatureVerifier: fakeSignatureVerifier{},
	})
	if err := service.SaveManifestURL(ctx, server.URL+"/manifest.json"); err != nil {
		t.Fatalf("save manifest url: %v", err)
	}

	result, err := service.Apply(ctx, ApplyRequest{Confirm: true})
	if err != nil {
		t.Fatalf("apply update: %v", err)
	}
	if !result.Applied || result.RolledBack || result.Version != "0.2.0" || result.BackupID == "" {
		t.Fatalf("unexpected apply result: %#v", result)
	}
	body, err := os.ReadFile(currentBinary)
	if err != nil {
		t.Fatalf("read current binary: %v", err)
	}
	if string(body) != "new-binary" {
		t.Fatalf("expected new binary, got %q", string(body))
	}
	rollbackPath, ok, err := appStore.GetSetting(ctx, rollbackPathSettingKey)
	if err != nil || !ok || rollbackPath == "" {
		t.Fatalf("expected rollback path setting, ok=%v value=%q err=%v", ok, rollbackPath, err)
	}
	rollbackBody, err := os.ReadFile(rollbackPath)
	if err != nil {
		t.Fatalf("read rollback binary: %v", err)
	}
	if string(rollbackBody) != "old-binary" {
		t.Fatalf("expected old rollback binary, got %q", string(rollbackBody))
	}
}

func TestUpdateApplyRequiresConfirmation(t *testing.T) {
	result, err := NewService(nil, "0.1.0").Apply(context.Background(), ApplyRequest{Confirm: false})
	if err != ErrConfirmationRequired {
		t.Fatalf("expected confirmation required, got result=%#v err=%v", result, err)
	}
}

func TestUpdateRollbackRestoresPreviousBinary(t *testing.T) {
	ctx := context.Background()
	appStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer appStore.Close()
	tmp := t.TempDir()
	currentBinary := filepath.Join(tmp, "omo")
	rollbackBinary := filepath.Join(tmp, "previous")
	if err := os.WriteFile(currentBinary, []byte("new-binary"), 0o755); err != nil {
		t.Fatalf("write current: %v", err)
	}
	if err := os.WriteFile(rollbackBinary, []byte("old-binary"), 0o755); err != nil {
		t.Fatalf("write rollback: %v", err)
	}
	if err := appStore.SetSetting(ctx, rollbackPathSettingKey, rollbackBinary); err != nil {
		t.Fatalf("set rollback path: %v", err)
	}
	if err := appStore.SetSetting(ctx, rollbackVersionSettingKey, "0.1.0"); err != nil {
		t.Fatalf("set rollback version: %v", err)
	}
	service := NewServiceWithOptions(Options{
		Store: appStore,
		CurrentVersion: "0.2.0",
		BinaryPath: currentBinary,
		WorkDir: filepath.Join(tmp, "updates"),
		Restarter: fakeRestarter{},
		HealthChecker: fakeHealthChecker{},
		SignatureVerifier: fakeSignatureVerifier{},
	})
	result, err := service.Rollback(ctx, RollbackRequest{Confirm: true})
	if err != nil {
		t.Fatalf("rollback update: %v", err)
	}
	if !result.RolledBack || result.Applied || result.Version != "0.1.0" {
		t.Fatalf("unexpected rollback result: %#v", result)
	}
	body, err := os.ReadFile(currentBinary)
	if err != nil {
		t.Fatalf("read current: %v", err)
	}
	if string(body) != "old-binary" {
		t.Fatalf("expected old binary restored, got %q", string(body))
	}
}

func TestUpdateApplyAutoRestoresBinaryOnHealthFailure(t *testing.T) {
	ctx := context.Background()
	appStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer appStore.Close()
	tmp := t.TempDir()
	currentBinary := filepath.Join(tmp, "omo")
	if err := os.WriteFile(currentBinary, []byte("old-binary"), 0o755); err != nil {
		t.Fatalf("write current binary: %v", err)
	}
	artifact := makeReleaseArchive(t, []byte("new-binary"))
	checksum := sha256File(t, artifact)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/manifest.json" {
			_, _ = w.Write([]byte(`{"version":"0.2.0","summary":"test","artifacts":[{"os":"` + runtime.GOOS + `","arch":"` + runtime.GOARCH + `","url":"https://` + r.Host + `/omo.tar.gz","checksumSha256":"` + checksum + `","signature":"verified-by-test"}]}`))
			return
		}
		http.ServeFile(w, r, artifact)
	}))
	defer server.Close()
	service := NewServiceWithOptions(Options{
		Store: appStore,
		CurrentVersion: "0.1.0",
		Client: server.Client(),
		Backup: fakeBackupRunner{},
		BinaryPath: currentBinary,
		WorkDir: filepath.Join(tmp, "updates"),
		Restarter: fakeRestarter{},
		HealthChecker: fakeHealthChecker{err: errHealthFailed},
		SignatureVerifier: fakeSignatureVerifier{},
	})
	if err := service.SaveManifestURL(ctx, server.URL+"/manifest.json"); err != nil {
		t.Fatalf("save manifest url: %v", err)
	}
	if _, err := service.Apply(ctx, ApplyRequest{Confirm: true}); err == nil {
		t.Fatal("expected health failure")
	}
	body, err := os.ReadFile(currentBinary)
	if err != nil {
		t.Fatalf("read current: %v", err)
	}
	if string(body) != "old-binary" {
		t.Fatalf("expected old binary after automatic restore, got %q", string(body))
	}
}

func makeReleaseArchive(t *testing.T, binary []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "omo.tar.gz")
	out, err := os.Create(path)
	if err != nil {
		t.Fatalf("create artifact: %v", err)
	}
	gz := gzip.NewWriter(out)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "omo", Mode: 0o755, Size: int64(len(binary)), ModTime: time.Now()}); err != nil {
		t.Fatalf("write tar header: %v", err)
	}
	if _, err := tw.Write(binary); err != nil {
		t.Fatalf("write tar binary: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("close artifact: %v", err)
	}
	return path
}

func sha256File(t *testing.T, path string) string {
	t.Helper()
	in, err := os.Open(path)
	if err != nil {
		t.Fatalf("open checksum source: %v", err)
	}
	defer in.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, in); err != nil {
		t.Fatalf("hash artifact: %v", err)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

type fakeBackupRunner struct{}

func (fakeBackupRunner) Create(context.Context) (backup.CreateResult, error) {
	return backup.CreateResult{Backup: store.BackupRecord{ID: "bak_test", Status: "ready"}}, nil
}

type fakeSignatureVerifier struct{}

func (fakeSignatureVerifier) Verify(context.Context, string, string, string) error {
	return nil
}

type fakeRestarter struct{}

func (fakeRestarter) Restart(context.Context) error {
	return nil
}

var errHealthFailed = http.ErrAbortHandler

type fakeHealthChecker struct {
	err error
}

func (h fakeHealthChecker) Check(context.Context) error {
	return h.err
}
