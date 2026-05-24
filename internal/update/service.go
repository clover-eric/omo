package update

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"omo/internal/backup"
	"omo/internal/store"
)

const (
	ManifestURLSettingKey = "update.manifest_url"
	rollbackPathSettingKey = "update.rollback_path"
	rollbackVersionSettingKey = "update.rollback_version"
	currentVersionSettingKey = "update.current_version"
	defaultTimeout = 5 * time.Second
	JobKindUpdateApply = "update_apply"
	JobKindUpdateRollback = "update_rollback"
	StateUpdateApply = "UPDATE_APPLY"
	StateUpdateRollback = "UPDATE_ROLLBACK"
)

var (
	ErrInvalidManifestURL = errors.New("invalid update manifest URL")
	ErrConfirmationRequired = errors.New("update confirmation required")
	ErrUpdateUnavailable = errors.New("update is unavailable")
	ErrArtifactVerificationFailed = errors.New("update artifact verification failed")
	ErrNoRollback = errors.New("update rollback is unavailable")
)

type Store interface {
	GetSetting(ctx context.Context, key string) (string, bool, error)
	SetSetting(ctx context.Context, key string, value string) error
	DeleteSetting(ctx context.Context, key string) error
	CreateJob(ctx context.Context, kind string, state string, status string, progress int, message string) (store.Job, error)
	MarkJobStarted(ctx context.Context, jobID string) error
	UpdateJob(ctx context.Context, jobID string, state string, status string, progress int, message string, errorCode string, finished bool) error
	AppendJobEvent(ctx context.Context, jobID string, kind string, state string, status string, progress int, message string, errorCode string) (store.JobEvent, error)
	LatestJob(ctx context.Context, kind string) (*store.Job, error)
	AppendAuditLog(ctx context.Context, adminID *string, action string, resourceType string, resourceID string, detailsJSON string) error
}

type BackupRunner interface {
	Create(ctx context.Context) (backup.CreateResult, error)
}

type SignatureVerifier interface {
	Verify(ctx context.Context, artifactPath string, signature string, workDir string) error
}

type Restarter interface {
	Restart(ctx context.Context) error
}

type HealthChecker interface {
	Check(ctx context.Context) error
}

type Service struct {
	store Store
	client *http.Client
	currentVersion string
	backup BackupRunner
	binaryPath string
	workDir string
	restarter Restarter
	health HealthChecker
	verifier SignatureVerifier
	restartTimeout time.Duration
	healthTimeout time.Duration
}

type Options struct {
	Store Store
	CurrentVersion string
	Client *http.Client
	Backup BackupRunner
	BinaryPath string
	WorkDir string
	RestartCommand []string
	HealthURL string
	Restarter Restarter
	HealthChecker HealthChecker
	SignatureVerifier SignatureVerifier
}

type CheckResult struct {
	Configured bool `json:"configured"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion string `json:"latestVersion,omitempty"`
	UpdateAvailable bool `json:"updateAvailable"`
	Channel string `json:"channel,omitempty"`
	Summary string `json:"summary"`
	ManifestURL string `json:"manifestUrl,omitempty"`
	ChecksumSHA256 string `json:"checksumSha256,omitempty"`
	Signature string `json:"signature,omitempty"`
	ArtifactURL string `json:"artifactUrl,omitempty"`
	CheckedAt time.Time `json:"checkedAt"`
	Platform string `json:"platform"`
}

type Manifest struct {
	Version string `json:"version"`
	Channel string `json:"channel"`
	Summary string `json:"summary"`
	ChecksumSHA256 string `json:"checksumSha256"`
	Signature string `json:"signature"`
	Artifacts []Artifact `json:"artifacts"`
}

type Artifact struct {
	OS string `json:"os"`
	Arch string `json:"arch"`
	URL string `json:"url"`
	ChecksumSHA256 string `json:"checksumSha256"`
	Signature string `json:"signature"`
}

type ApplyRequest struct {
	Confirm bool `json:"confirm"`
}

type RollbackRequest struct {
	Confirm bool `json:"confirm"`
}

type JobResult struct {
	Job store.Job `json:"job"`
	Version string `json:"version,omitempty"`
	PreviousVersion string `json:"previousVersion,omitempty"`
	BackupID string `json:"backupId,omitempty"`
	Applied bool `json:"applied"`
	RolledBack bool `json:"rolledBack"`
	ArtifactURL string `json:"artifactUrl,omitempty"`
	ChecksumSHA256 string `json:"checksumSha256,omitempty"`
}

func NewService(appStore Store, currentVersion string) *Service {
	return NewServiceWithOptions(Options{Store: appStore, CurrentVersion: currentVersion})
}

func NewServiceWithClient(appStore Store, currentVersion string, client *http.Client) *Service {
	return NewServiceWithOptions(Options{Store: appStore, CurrentVersion: currentVersion, Client: client})
}

func NewServiceWithOptions(opts Options) *Service {
	currentVersion := strings.TrimSpace(opts.CurrentVersion)
	if currentVersion == "" {
		currentVersion = "development"
	}
	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}
	binaryPath := strings.TrimSpace(opts.BinaryPath)
	if binaryPath == "" {
		if current, err := os.Executable(); err == nil {
			binaryPath = current
		}
	}
	workDir := strings.TrimSpace(opts.WorkDir)
	if workDir == "" {
		workDir = filepath.Join("data", "updates")
	}
	restarter := opts.Restarter
	if restarter == nil && len(opts.RestartCommand) > 0 {
		restarter = commandRestarter{command: opts.RestartCommand}
	}
	if restarter == nil {
		restarter = noopRestarter{}
	}
	health := opts.HealthChecker
	if health == nil && strings.TrimSpace(opts.HealthURL) != "" {
		health = httpHealthChecker{client: client, url: strings.TrimSpace(opts.HealthURL)}
	}
	if health == nil {
		health = noopHealthChecker{}
	}
	verifier := opts.SignatureVerifier
	if verifier == nil {
		verifier = cosignVerifier{client: client}
	}
	return &Service{
		store: opts.Store,
		client: client,
		currentVersion: currentVersion,
		backup: opts.Backup,
		binaryPath: binaryPath,
		workDir: workDir,
		restarter: restarter,
		health: health,
		verifier: verifier,
		restartTimeout: 30 * time.Second,
		healthTimeout: 20 * time.Second,
	}
}

func (s *Service) Check(ctx context.Context) (CheckResult, error) {
	currentVersion := "development"
	if s != nil && strings.TrimSpace(s.currentVersion) != "" {
		currentVersion = s.currentVersion
	}
	result := CheckResult{
		Configured: false,
		CurrentVersion: currentVersion,
		UpdateAvailable: false,
		Summary: "Update manifest is not configured.",
		CheckedAt: time.Now().UTC(),
		Platform: runtime.GOOS + "/" + runtime.GOARCH,
	}
	if s == nil || s.store == nil {
		return result, nil
	}
	manifestURL, ok, err := s.store.GetSetting(ctx, ManifestURLSettingKey)
	if err != nil {
		return CheckResult{}, err
	}
	manifestURL = strings.TrimSpace(manifestURL)
	if !ok || manifestURL == "" {
		return result, nil
	}
	if err := validateManifestURL(manifestURL); err != nil {
		return CheckResult{}, err
	}
	manifest, err := s.fetchManifest(ctx, manifestURL)
	if err != nil {
		return CheckResult{}, err
	}
	artifact := manifest.ArtifactFor(runtime.GOOS, runtime.GOARCH)
	result.Configured = true
	result.ManifestURL = manifestURL
	result.LatestVersion = manifest.Version
	result.Channel = manifest.Channel
	result.Summary = manifest.Summary
	result.UpdateAvailable = manifest.Version != "" && manifest.Version != s.currentVersion
	result.ChecksumSHA256 = manifest.ChecksumSHA256
	result.Signature = manifest.Signature
	if artifact != nil {
		result.ArtifactURL = artifact.URL
		if artifact.ChecksumSHA256 != "" {
			result.ChecksumSHA256 = artifact.ChecksumSHA256
		}
		if artifact.Signature != "" {
			result.Signature = artifact.Signature
		}
	}
	return result, nil
}

func (s *Service) Apply(ctx context.Context, req ApplyRequest) (JobResult, error) {
	if !req.Confirm {
		return JobResult{}, ErrConfirmationRequired
	}
	if s == nil || s.store == nil {
		return JobResult{}, ErrUpdateUnavailable
	}
	check, manifest, artifact, err := s.checkWithManifest(ctx)
	if err != nil {
		return JobResult{}, err
	}
	if !check.Configured || !check.UpdateAvailable || artifact == nil {
		return JobResult{}, ErrUpdateUnavailable
	}
	if strings.TrimSpace(artifact.URL) == "" || strings.TrimSpace(artifact.ChecksumSHA256) == "" {
		return JobResult{}, ErrUpdateUnavailable
	}
	if err := validateArtifactURL(artifact.URL); err != nil {
		return JobResult{}, err
	}
	signature := strings.TrimSpace(artifact.Signature)
	if signature == "" {
		signature = strings.TrimSpace(manifest.Signature)
	}
	if signature == "" {
		return JobResult{}, ErrArtifactVerificationFailed
	}
	if s.backup == nil {
		return JobResult{}, errors.New("pre-update backup service is unavailable")
	}
	job, err := s.startJob(ctx, JobKindUpdateApply, StateUpdateApply, "Update apply job created.")
	if err != nil {
		return JobResult{}, err
	}
	backupResult, err := s.backup.Create(ctx)
	if err != nil {
		_ = s.failJob(ctx, job.ID, JobKindUpdateApply, StateUpdateApply, "Pre-update backup failed.", "UPDATE_BACKUP_FAILED")
		return JobResult{}, err
	}
	_ = s.step(ctx, job.ID, JobKindUpdateApply, StateUpdateApply, 25, "Pre-update backup completed.")

	if err := os.MkdirAll(s.workDir, 0o700); err != nil {
		_ = s.failJob(ctx, job.ID, JobKindUpdateApply, StateUpdateApply, "Update workspace could not be prepared.", "UPDATE_WORKSPACE_FAILED")
		return JobResult{}, err
	}
	workDir, err := os.MkdirTemp(s.workDir, ".apply-")
	if err != nil {
		_ = s.failJob(ctx, job.ID, JobKindUpdateApply, StateUpdateApply, "Update workspace could not be prepared.", "UPDATE_WORKSPACE_FAILED")
		return JobResult{}, err
	}
	defer os.RemoveAll(workDir)
	artifactPath := filepath.Join(workDir, "artifact")
	if err := s.downloadFile(ctx, artifact.URL, artifactPath); err != nil {
		_ = s.failJob(ctx, job.ID, JobKindUpdateApply, StateUpdateApply, "Update artifact download failed.", "UPDATE_DOWNLOAD_FAILED")
		return JobResult{}, err
	}
	_ = s.step(ctx, job.ID, JobKindUpdateApply, StateUpdateApply, 45, "Update artifact downloaded.")
	if err := verifyChecksum(artifactPath, artifact.ChecksumSHA256); err != nil {
		_ = s.failJob(ctx, job.ID, JobKindUpdateApply, StateUpdateApply, "Update artifact checksum verification failed.", "UPDATE_CHECKSUM_FAILED")
		return JobResult{}, ErrArtifactVerificationFailed
	}
	if err := s.verifier.Verify(ctx, artifactPath, signature, workDir); err != nil {
		_ = s.failJob(ctx, job.ID, JobKindUpdateApply, StateUpdateApply, "Update artifact signature verification failed.", "UPDATE_SIGNATURE_FAILED")
		return JobResult{}, ErrArtifactVerificationFailed
	}
	_ = s.step(ctx, job.ID, JobKindUpdateApply, StateUpdateApply, 60, "Update artifact verification completed.")
	newBinary := filepath.Join(workDir, "omo")
	if err := prepareBinary(artifactPath, newBinary); err != nil {
		_ = s.failJob(ctx, job.ID, JobKindUpdateApply, StateUpdateApply, "Update binary could not be prepared.", "UPDATE_BINARY_PREPARE_FAILED")
		return JobResult{}, err
	}
	rollbackPath, err := s.replaceBinary(newBinary)
	if err != nil {
		_ = s.failJob(ctx, job.ID, JobKindUpdateApply, StateUpdateApply, "Update binary replacement failed.", "UPDATE_REPLACE_FAILED")
		return JobResult{}, err
	}
	_ = s.step(ctx, job.ID, JobKindUpdateApply, StateUpdateApply, 75, "Update binary replaced.")
	if err := s.restartAndCheck(ctx); err != nil {
		_ = restoreBinary(rollbackPath, s.binaryPath)
		_ = s.restartAndCheck(ctx)
		_ = s.failJob(ctx, job.ID, JobKindUpdateApply, StateUpdateApply, "Update health check failed; previous binary was restored.", "UPDATE_AUTO_ROLLBACK_COMPLETED")
		return JobResult{}, err
	}
	_ = s.store.SetSetting(ctx, rollbackPathSettingKey, rollbackPath)
	_ = s.store.SetSetting(ctx, rollbackVersionSettingKey, s.currentVersion)
	_ = s.store.SetSetting(ctx, currentVersionSettingKey, manifest.Version)
	_ = s.store.AppendAuditLog(ctx, nil, "update_applied", "update", manifest.Version, `{"applied":true}`)
	if err := s.completeJob(ctx, job.ID, JobKindUpdateApply, StateUpdateApply, "Update applied and health check passed."); err != nil {
		return JobResult{}, err
	}
	latest, err := s.store.LatestJob(ctx, JobKindUpdateApply)
	if err != nil {
		return JobResult{}, err
	}
	return JobResult{
		Job: *latest,
		Version: manifest.Version,
		PreviousVersion: s.currentVersion,
		BackupID: backupResult.Backup.ID,
		Applied: true,
		RolledBack: false,
		ArtifactURL: artifact.URL,
		ChecksumSHA256: artifact.ChecksumSHA256,
	}, nil
}

func (s *Service) Rollback(ctx context.Context, req RollbackRequest) (JobResult, error) {
	if !req.Confirm {
		return JobResult{}, ErrConfirmationRequired
	}
	if s == nil || s.store == nil {
		return JobResult{}, ErrUpdateUnavailable
	}
	rollbackPath, ok, err := s.store.GetSetting(ctx, rollbackPathSettingKey)
	if err != nil {
		return JobResult{}, err
	}
	if !ok || strings.TrimSpace(rollbackPath) == "" {
		return JobResult{}, ErrNoRollback
	}
	if _, err := os.Stat(rollbackPath); err != nil {
		return JobResult{}, ErrNoRollback
	}
	job, err := s.startJob(ctx, JobKindUpdateRollback, StateUpdateRollback, "Update rollback job created.")
	if err != nil {
		return JobResult{}, err
	}
	if err := os.MkdirAll(s.workDir, 0o700); err != nil {
		_ = s.failJob(ctx, job.ID, JobKindUpdateRollback, StateUpdateRollback, "Update rollback workspace could not be prepared.", "UPDATE_ROLLBACK_WORKSPACE_FAILED")
		return JobResult{}, err
	}
	currentCopy := filepath.Join(s.workDir, "rollback-current-"+time.Now().UTC().Format("20060102T150405Z"))
	if err := copyFile(s.binaryPath, currentCopy, 0o755); err != nil {
		_ = s.failJob(ctx, job.ID, JobKindUpdateRollback, StateUpdateRollback, "Current binary could not be preserved before rollback.", "UPDATE_ROLLBACK_PRESERVE_FAILED")
		return JobResult{}, err
	}
	if err := restoreBinary(rollbackPath, s.binaryPath); err != nil {
		_ = s.failJob(ctx, job.ID, JobKindUpdateRollback, StateUpdateRollback, "Update rollback binary restore failed.", "UPDATE_ROLLBACK_RESTORE_FAILED")
		return JobResult{}, err
	}
	_ = s.step(ctx, job.ID, JobKindUpdateRollback, StateUpdateRollback, 70, "Previous update binary restored.")
	if err := s.restartAndCheck(ctx); err != nil {
		_ = restoreBinary(currentCopy, s.binaryPath)
		_ = s.restartAndCheck(ctx)
		_ = s.failJob(ctx, job.ID, JobKindUpdateRollback, StateUpdateRollback, "Update rollback failed; active binary was restored.", "UPDATE_ROLLBACK_FAILED")
		return JobResult{}, err
	}
	previousVersion, _, _ := s.store.GetSetting(ctx, rollbackVersionSettingKey)
	_ = s.store.SetSetting(ctx, currentVersionSettingKey, previousVersion)
	_ = s.store.AppendAuditLog(ctx, nil, "update_rolled_back", "update", previousVersion, `{"rolledBack":true}`)
	if err := s.completeJob(ctx, job.ID, JobKindUpdateRollback, StateUpdateRollback, "Update rollback completed and health check passed."); err != nil {
		return JobResult{}, err
	}
	latest, err := s.store.LatestJob(ctx, JobKindUpdateRollback)
	if err != nil {
		return JobResult{}, err
	}
	return JobResult{Job: *latest, Version: previousVersion, Applied: false, RolledBack: true}, nil
}

func (s *Service) SaveManifestURL(ctx context.Context, manifestURL string) error {
	manifestURL = strings.TrimSpace(manifestURL)
	if manifestURL == "" {
		return s.store.DeleteSetting(ctx, ManifestURLSettingKey)
	}
	if err := validateManifestURL(manifestURL); err != nil {
		return err
	}
	return s.store.SetSetting(ctx, ManifestURLSettingKey, manifestURL)
}

func (s *Service) checkWithManifest(ctx context.Context) (CheckResult, Manifest, *Artifact, error) {
	check, err := s.Check(ctx)
	if err != nil {
		return CheckResult{}, Manifest{}, nil, err
	}
	if !check.Configured {
		return check, Manifest{}, nil, nil
	}
	manifestURL, _, err := s.store.GetSetting(ctx, ManifestURLSettingKey)
	if err != nil {
		return CheckResult{}, Manifest{}, nil, err
	}
	manifest, err := s.fetchManifest(ctx, manifestURL)
	if err != nil {
		return CheckResult{}, Manifest{}, nil, err
	}
	return check, manifest, manifest.ArtifactFor(runtime.GOOS, runtime.GOARCH), nil
}

func (s *Service) fetchManifest(ctx context.Context, manifestURL string) (Manifest, error) {
	client := s.client
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return Manifest{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "omo-update-check/0.1")
	resp, err := client.Do(req)
	if err != nil {
		return Manifest{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return Manifest{}, errors.New("update manifest endpoint returned non-success status")
	}
	var manifest Manifest
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&manifest); err != nil {
		return Manifest{}, err
	}
	manifest.Version = strings.TrimSpace(manifest.Version)
	manifest.Channel = strings.TrimSpace(manifest.Channel)
	manifest.Summary = strings.TrimSpace(manifest.Summary)
	manifest.ChecksumSHA256 = strings.TrimSpace(manifest.ChecksumSHA256)
	manifest.Signature = strings.TrimSpace(manifest.Signature)
	if manifest.Version == "" {
		return Manifest{}, errors.New("update manifest version is required")
	}
	if manifest.Summary == "" {
		manifest.Summary = "Update manifest retrieved."
	}
	return manifest, nil
}

func (m Manifest) ArtifactFor(goos string, goarch string) *Artifact {
	for _, artifact := range m.Artifacts {
		if artifact.OS == goos && artifact.Arch == goarch {
			copy := artifact
			copy.URL = strings.TrimSpace(copy.URL)
			copy.ChecksumSHA256 = strings.TrimSpace(copy.ChecksumSHA256)
			copy.Signature = strings.TrimSpace(copy.Signature)
			return &copy
		}
	}
	return nil
}

func validateArtifactURL(value string) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return ErrUpdateUnavailable
	}
	return nil
}

func (s *Service) downloadFile(ctx context.Context, sourceURL string, dst string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return errors.New("update artifact endpoint returned non-success status")
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, io.LimitReader(resp.Body, 512<<20)); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func verifyChecksum(path string, expected string) error {
	actual, err := fileSHA256(path)
	if err != nil {
		return err
	}
	if !strings.EqualFold(actual, strings.TrimSpace(expected)) {
		return ErrArtifactVerificationFailed
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	in, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer in.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, in); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func prepareBinary(artifactPath string, dst string) error {
	if err := extractBinaryFromTarGz(artifactPath, dst); err == nil {
		return nil
	}
	return copyFile(artifactPath, dst, 0o755)
}

func extractBinaryFromTarGz(artifactPath string, dst string) error {
	in, err := os.Open(artifactPath)
	if err != nil {
		return err
	}
	defer in.Close()
	gz, err := gzip.NewReader(in)
	if err != nil {
		return err
	}
	defer gz.Close()
	reader := tar.NewReader(gz)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if header == nil || header.FileInfo().IsDir() {
			continue
		}
		if filepath.Base(header.Name) != "omo" {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
			return err
		}
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, reader); err != nil {
			_ = out.Close()
			return err
		}
		return out.Close()
	}
	return errors.New("omo binary not found in update archive")
}

func (s *Service) replaceBinary(newBinary string) (string, error) {
	if strings.TrimSpace(s.binaryPath) == "" {
		return "", errors.New("update binary path is not configured")
	}
	info, err := os.Stat(s.binaryPath)
	if err != nil {
		return "", err
	}
	rollbackDir := filepath.Join(s.workDir, "rollback")
	if err := os.MkdirAll(rollbackDir, 0o700); err != nil {
		return "", err
	}
	rollbackPath := filepath.Join(rollbackDir, "omo-"+time.Now().UTC().Format("20060102T150405Z")+".previous")
	if err := copyFile(s.binaryPath, rollbackPath, info.Mode().Perm()); err != nil {
		return "", err
	}
	tmpPath := s.binaryPath + ".new"
	if err := copyFile(newBinary, tmpPath, info.Mode().Perm()); err != nil {
		return "", err
	}
	if err := os.Remove(s.binaryPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	if err := os.Rename(tmpPath, s.binaryPath); err != nil {
		_ = copyFile(rollbackPath, s.binaryPath, info.Mode().Perm())
		return "", err
	}
	return rollbackPath, nil
}

func restoreBinary(source string, target string) error {
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	return copyFile(source, target, info.Mode().Perm())
}

func copyFile(src string, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func (s *Service) restartAndCheck(ctx context.Context) error {
	restartCtx, cancel := context.WithTimeout(ctx, s.restartTimeout)
	defer cancel()
	if err := s.restarter.Restart(restartCtx); err != nil {
		return err
	}
	healthCtx, healthCancel := context.WithTimeout(ctx, s.healthTimeout)
	defer healthCancel()
	return s.health.Check(healthCtx)
}

func (s *Service) startJob(ctx context.Context, kind string, state string, message string) (store.Job, error) {
	job, err := s.store.CreateJob(ctx, kind, state, "queued", 0, message)
	if err != nil {
		return store.Job{}, err
	}
	_, _ = s.store.AppendJobEvent(ctx, job.ID, kind, state, "queued", 0, message, "")
	if err := s.store.MarkJobStarted(ctx, job.ID); err != nil {
		return store.Job{}, err
	}
	return job, nil
}

func (s *Service) step(ctx context.Context, jobID string, kind string, state string, progress int, message string) error {
	if err := s.store.UpdateJob(ctx, jobID, state, "running", progress, message, "", false); err != nil {
		return err
	}
	_, err := s.store.AppendJobEvent(ctx, jobID, kind, state, "running", progress, message, "")
	return err
}

func (s *Service) failJob(ctx context.Context, jobID string, kind string, state string, message string, code string) error {
	if err := s.store.UpdateJob(ctx, jobID, state, "failed", 100, message, code, true); err != nil {
		return err
	}
	_, err := s.store.AppendJobEvent(ctx, jobID, kind, state, "failed", 100, message, code)
	return err
}

func (s *Service) completeJob(ctx context.Context, jobID string, kind string, state string, message string) error {
	if err := s.store.UpdateJob(ctx, jobID, state, "succeeded", 100, message, "", true); err != nil {
		return err
	}
	_, err := s.store.AppendJobEvent(ctx, jobID, kind, state, "succeeded", 100, message, "")
	return err
}

type noopRestarter struct{}

func (noopRestarter) Restart(context.Context) error {
	return nil
}

type commandRestarter struct {
	command []string
}

func (r commandRestarter) Restart(ctx context.Context) error {
	if len(r.command) == 0 {
		return nil
	}
	cmd := exec.CommandContext(ctx, r.command[0], r.command[1:]...)
	return cmd.Run()
}

type noopHealthChecker struct{}

func (noopHealthChecker) Check(context.Context) error {
	return nil
}

type httpHealthChecker struct {
	client *http.Client
	url string
}

func (h httpHealthChecker) Check(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.url, nil)
	if err != nil {
		return err
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New("update health endpoint returned non-success status")
	}
	return nil
}

type cosignVerifier struct {
	client *http.Client
}

func (v cosignVerifier) Verify(ctx context.Context, artifactPath string, signature string, workDir string) error {
	signature = strings.TrimSpace(signature)
	if signature == "" {
		return ErrArtifactVerificationFailed
	}
	bundlePath := signature
	if strings.HasPrefix(signature, "https://") {
		dst := filepath.Join(workDir, "signature.sigstore.json")
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, signature, nil)
		if err != nil {
			return err
		}
		resp, err := v.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return ErrArtifactVerificationFailed
		}
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, io.LimitReader(resp.Body, 4<<20)); err != nil {
			_ = out.Close()
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}
		bundlePath = dst
	} else if strings.HasPrefix(signature, "{") {
		bundlePath = filepath.Join(workDir, "signature.sigstore.json")
		if err := os.WriteFile(bundlePath, []byte(signature), 0o600); err != nil {
			return err
		}
	}
	if _, err := os.Stat(bundlePath); err != nil {
		return ErrArtifactVerificationFailed
	}
	if _, err := exec.LookPath("cosign"); err != nil {
		return ErrArtifactVerificationFailed
	}
	cmd := exec.CommandContext(ctx, "cosign", "verify-blob", "--bundle", bundlePath, artifactPath)
	if err := cmd.Run(); err != nil {
		return ErrArtifactVerificationFailed
	}
	return nil
}

func validateManifestURL(value string) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return ErrInvalidManifestURL
	}
	return nil
}
