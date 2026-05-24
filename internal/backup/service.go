package backup

import (
	"archive/zip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"omo/internal/store"
)

const (
	JobKindBackupCreate = "backup_create"
	JobKindBackupRestore = "backup_restore"
	StateBackupCreate = "BACKUP_CREATE"
	StateBackupRestore = "BACKUP_RESTORE"
	encryptedArchiveMagic = "OMOBAK1\n"
)

var (
	ErrBackupNotFound = errors.New("backup record not found")
	ErrConfirmationRequired = errors.New("backup restore confirmation required")
	ErrInvalidBackup = errors.New("backup archive is invalid")
)

type Store interface {
	DatabasePath() string
	GetSetting(ctx context.Context, key string) (string, bool, error)
	CreateBackupRecord(ctx context.Context, status string, path string) (store.BackupRecord, error)
	CompleteBackupRecord(ctx context.Context, id string, status string, checksum string, completedAt time.Time) (*store.BackupRecord, error)
	BackupRecordByID(ctx context.Context, id string) (*store.BackupRecord, error)
	ListBackupRecords(ctx context.Context) ([]store.BackupRecord, error)
	BackupDatabaseSnapshot(ctx context.Context, dst string) error
	RestoreDatabaseSnapshot(ctx context.Context, snapshotPath string) error
	CreateJob(ctx context.Context, kind string, state string, status string, progress int, message string) (store.Job, error)
	MarkJobStarted(ctx context.Context, jobID string) error
	UpdateJob(ctx context.Context, jobID string, state string, status string, progress int, message string, errorCode string, finished bool) error
	AppendJobEvent(ctx context.Context, jobID string, kind string, state string, status string, progress int, message string, errorCode string) (store.JobEvent, error)
	LatestJob(ctx context.Context, kind string) (*store.Job, error)
	AppendAuditLog(ctx context.Context, adminID *string, action string, resourceType string, resourceID string, detailsJSON string) error
}

type Service struct {
	store Store
	dir string
	files []FileSpec
	version string
	keyPath string
}

type Options struct {
	Store Store
	BackupDir string
	Version string
	Files []FileSpec
	KeyPath string
}

type FileSpec struct {
	Label string
	Path string
}

type ListResult struct {
	Backups []store.BackupRecord `json:"backups"`
}

type CreateResult struct {
	Backup store.BackupRecord `json:"backup"`
	Job store.Job `json:"job"`
}

type RestoreRequest struct {
	Confirm bool `json:"confirm"`
}

type RestoreResult struct {
	Backup store.BackupRecord `json:"backup"`
	Job store.Job `json:"job"`
	Restored bool `json:"restored"`
}

type manifest struct {
	Version string `json:"version"`
	CreatedAt string `json:"createdAt"`
	DatabaseFile string `json:"databaseFile"`
	SourceDatabase string `json:"sourceDatabase"`
	Runtime RuntimeMetadata `json:"runtime"`
	Files []fileManifest `json:"files"`
	Certificates []certificateManifest `json:"certificates,omitempty"`
}

type RuntimeMetadata struct {
	AppVersion string `json:"appVersion"`
	GoOS string `json:"goos"`
	GoArch string `json:"goarch"`
}

type fileManifest struct {
	Label string `json:"label"`
	SourcePath string `json:"sourcePath"`
	ArchivePath string `json:"archivePath"`
	ChecksumSHA256 string `json:"checksumSha256"`
	SizeBytes int64 `json:"sizeBytes"`
}

type certificateManifest struct {
	Domain string `json:"domain"`
	Available bool `json:"available"`
	Issuer string `json:"issuer,omitempty"`
	Source string `json:"source"`
	CapturedAt string `json:"capturedAt"`
	MetadataOnly bool `json:"metadataOnly"`
	Note string `json:"note"`
}

func NewService(appStore Store, backupDir string) *Service {
	return NewServiceWithOptions(Options{Store: appStore, BackupDir: backupDir, Version: "development"})
}

func NewServiceWithOptions(opts Options) *Service {
	backupDir := strings.TrimSpace(opts.BackupDir)
	if backupDir == "" {
		backupDir = filepath.Join("data", "backups")
	}
	version := strings.TrimSpace(opts.Version)
	if version == "" {
		version = "development"
	}
	keyPath := strings.TrimSpace(opts.KeyPath)
	if keyPath == "" {
		keyPath = filepath.Join(backupDir, "omo-backup.key")
	}
	return &Service{store: opts.Store, dir: backupDir, files: normalizedFiles(opts.Files), version: version, keyPath: keyPath}
}

func (s *Service) List(ctx context.Context) (ListResult, error) {
	records, err := s.store.ListBackupRecords(ctx)
	if err != nil {
		return ListResult{}, err
	}
	return ListResult{Backups: records}, nil
}

func (s *Service) Create(ctx context.Context) (CreateResult, error) {
	if s == nil || s.store == nil {
		return CreateResult{}, errors.New("backup service is unavailable")
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return CreateResult{}, err
	}
	job, err := s.store.CreateJob(ctx, JobKindBackupCreate, StateBackupCreate, "queued", 0, "Backup job created.")
	if err != nil {
		return CreateResult{}, err
	}
	_, _ = s.store.AppendJobEvent(ctx, job.ID, JobKindBackupCreate, StateBackupCreate, "queued", 0, "Backup job created.", "")
	if err := s.store.MarkJobStarted(ctx, job.ID); err != nil {
		return CreateResult{}, err
	}

	archivePath := filepath.Join(s.dir, time.Now().UTC().Format("20060102T150405Z")+".omo-backup.enc")
	record, err := s.store.CreateBackupRecord(ctx, "running", archivePath)
	if err != nil {
		_ = s.failJob(ctx, job.ID, JobKindBackupCreate, StateBackupCreate, "Backup record could not be created.", "BACKUP_RECORD_FAILED")
		return CreateResult{}, err
	}
	_, _ = s.store.AppendJobEvent(ctx, job.ID, JobKindBackupCreate, StateBackupCreate, "running", 25, "Preparing consistent database snapshot.", "")

	workDir, err := os.MkdirTemp(s.dir, ".work-"+record.ID+"-")
	if err != nil {
		_ = s.failJob(ctx, job.ID, JobKindBackupCreate, StateBackupCreate, "Backup workspace could not be prepared.", "BACKUP_WORKSPACE_FAILED")
		return CreateResult{}, err
	}
	defer os.RemoveAll(workDir)

	dbSnapshot := filepath.Join(workDir, "omo.db")
	if err := s.store.BackupDatabaseSnapshot(ctx, dbSnapshot); err != nil {
		_ = s.failJob(ctx, job.ID, JobKindBackupCreate, StateBackupCreate, "Database snapshot could not be created.", "BACKUP_DATABASE_SNAPSHOT_FAILED")
		return CreateResult{}, err
	}
	_, _ = s.store.AppendJobEvent(ctx, job.ID, JobKindBackupCreate, StateBackupCreate, "running", 65, "Writing backup archive.", "")

	files, err := s.collectFiles()
	if err != nil {
		_ = s.failJob(ctx, job.ID, JobKindBackupCreate, StateBackupCreate, "Backup file metadata could not be prepared.", "BACKUP_FILE_METADATA_FAILED")
		return CreateResult{}, err
	}
	certificates, err := s.collectCertificates(ctx)
	if err != nil {
		_ = s.failJob(ctx, job.ID, JobKindBackupCreate, StateBackupCreate, "Certificate metadata could not be prepared.", "BACKUP_CERTIFICATE_METADATA_FAILED")
		return CreateResult{}, err
	}
	plainArchive := filepath.Join(workDir, "omo-backup.zip")
	if err := writeArchive(plainArchive, dbSnapshot, files, manifest{
		Version: "1",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		DatabaseFile: "omo.db",
		SourceDatabase: filepath.Base(s.store.DatabasePath()),
		Runtime: RuntimeMetadata{AppVersion: s.version, GoOS: runtime.GOOS, GoArch: runtime.GOARCH},
		Certificates: certificates,
	}); err != nil {
		_ = s.failJob(ctx, job.ID, JobKindBackupCreate, StateBackupCreate, "Backup archive could not be written.", "BACKUP_ARCHIVE_FAILED")
		return CreateResult{}, err
	}
	if err := encryptArchive(plainArchive, archivePath, s.keyPath); err != nil {
		_ = s.failJob(ctx, job.ID, JobKindBackupCreate, StateBackupCreate, "Backup archive could not be encrypted.", "BACKUP_ENCRYPTION_FAILED")
		return CreateResult{}, err
	}
	checksum, err := fileSHA256(archivePath)
	if err != nil {
		_ = s.failJob(ctx, job.ID, JobKindBackupCreate, StateBackupCreate, "Backup checksum could not be calculated.", "BACKUP_CHECKSUM_FAILED")
		return CreateResult{}, err
	}
	completed, err := s.store.CompleteBackupRecord(ctx, record.ID, "ready", checksum, time.Now().UTC())
	if err != nil {
		_ = s.failJob(ctx, job.ID, JobKindBackupCreate, StateBackupCreate, "Backup record could not be completed.", "BACKUP_RECORD_COMPLETE_FAILED")
		return CreateResult{}, err
	}
	_ = s.store.AppendAuditLog(ctx, nil, "backup_created", "backup", record.ID, `{"status":"ready"}`)
	if err := s.completeJob(ctx, job.ID, JobKindBackupCreate, StateBackupCreate, "Backup completed."); err != nil {
		return CreateResult{}, err
	}
	latest, err := s.store.LatestJob(ctx, JobKindBackupCreate)
	if err != nil {
		return CreateResult{}, err
	}
	return CreateResult{Backup: *completed, Job: *latest}, nil
}

func (s *Service) Restore(ctx context.Context, id string, req RestoreRequest) (RestoreResult, error) {
	if s == nil || s.store == nil {
		return RestoreResult{}, errors.New("backup service is unavailable")
	}
	if !req.Confirm {
		return RestoreResult{}, ErrConfirmationRequired
	}
	record, err := s.store.BackupRecordByID(ctx, strings.TrimSpace(id))
	if err != nil {
		return RestoreResult{}, err
	}
	if record == nil || record.Status != "ready" {
		return RestoreResult{}, ErrBackupNotFound
	}
	checksum, err := fileSHA256(record.Path)
	if err != nil {
		return RestoreResult{}, err
	}
	if !strings.EqualFold(checksum, record.Checksum) {
		return RestoreResult{}, ErrInvalidBackup
	}

	job, err := s.store.CreateJob(ctx, JobKindBackupRestore, StateBackupRestore, "queued", 0, "Backup restore job created.")
	if err != nil {
		return RestoreResult{}, err
	}
	_, _ = s.store.AppendJobEvent(ctx, job.ID, JobKindBackupRestore, StateBackupRestore, "queued", 0, "Backup restore job created.", "")
	if err := s.store.MarkJobStarted(ctx, job.ID); err != nil {
		return RestoreResult{}, err
	}
	_, _ = s.store.AppendJobEvent(ctx, job.ID, JobKindBackupRestore, StateBackupRestore, "running", 40, "Verifying backup archive.", "")

	workDir, err := os.MkdirTemp(filepath.Dir(record.Path), ".restore-"+record.ID+"-")
	if err != nil {
		_ = s.failJob(ctx, job.ID, JobKindBackupRestore, StateBackupRestore, "Restore workspace could not be prepared.", "RESTORE_WORKSPACE_FAILED")
		return RestoreResult{}, err
	}
	defer os.RemoveAll(workDir)
	readableArchive, err := prepareReadableArchive(record.Path, workDir, s.keyPath)
	if err != nil {
		_ = s.failJob(ctx, job.ID, JobKindBackupRestore, StateBackupRestore, "Backup archive could not be decrypted.", "RESTORE_ARCHIVE_DECRYPT_FAILED")
		return RestoreResult{}, ErrInvalidBackup
	}
	archiveContent, err := extractArchive(readableArchive, workDir)
	if err != nil {
		_ = s.failJob(ctx, job.ID, JobKindBackupRestore, StateBackupRestore, "Backup archive could not be read.", "RESTORE_ARCHIVE_INVALID")
		return RestoreResult{}, ErrInvalidBackup
	}
	_, _ = s.store.AppendJobEvent(ctx, job.ID, JobKindBackupRestore, StateBackupRestore, "running", 60, "Restoring managed configuration files.", "")
	restoredFiles, err := restoreFiles(archiveContent.Manifest.Files, workDir, s.files)
	if err != nil {
		_ = s.failJob(ctx, job.ID, JobKindBackupRestore, StateBackupRestore, "Managed configuration restore failed; current database was preserved.", "RESTORE_FILES_FAILED")
		return RestoreResult{}, err
	}
	_, _ = s.store.AppendJobEvent(ctx, job.ID, JobKindBackupRestore, StateBackupRestore, "running", 75, "Restoring database snapshot.", "")
	if err := s.store.RestoreDatabaseSnapshot(ctx, archiveContent.DatabasePath); err != nil {
		_ = rollbackRestoredFiles(restoredFiles)
		_ = s.failJob(ctx, job.ID, JobKindBackupRestore, StateBackupRestore, "Database restore failed; current data was preserved when possible.", "RESTORE_DATABASE_FAILED")
		return RestoreResult{}, err
	}
	if _, err := s.store.CompleteBackupRecord(ctx, record.ID, "ready", record.Checksum, time.Now().UTC()); err != nil {
		return RestoreResult{}, err
	}
	restoredJob, err := s.store.CreateJob(ctx, JobKindBackupRestore, StateBackupRestore, "queued", 90, "Backup restore finalization started.")
	if err != nil {
		return RestoreResult{}, err
	}
	_, _ = s.store.AppendJobEvent(ctx, restoredJob.ID, JobKindBackupRestore, StateBackupRestore, "queued", 90, "Backup restore finalization started.", "")
	_ = s.store.MarkJobStarted(ctx, restoredJob.ID)
	_ = s.store.AppendAuditLog(ctx, nil, "backup_restored", "backup", record.ID, `{"restored":true}`)
	if err := s.completeJob(ctx, restoredJob.ID, JobKindBackupRestore, StateBackupRestore, "Backup restored after operator confirmation."); err != nil {
		return RestoreResult{}, err
	}
	latest, err := s.store.LatestJob(ctx, JobKindBackupRestore)
	if err != nil {
		return RestoreResult{}, err
	}
	return RestoreResult{Backup: *record, Job: *latest, Restored: true}, nil
}

func (s *Service) collectFiles() ([]fileBackup, error) {
	files := make([]fileBackup, 0, len(s.files))
	for _, spec := range s.files {
		if strings.TrimSpace(spec.Path) == "" {
			continue
		}
		info, err := os.Stat(spec.Path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			continue
		}
		checksum, err := fileSHA256(spec.Path)
		if err != nil {
			return nil, err
		}
		archivePath := filepath.ToSlash(filepath.Join("files", safeName(spec.Label), filepath.Base(spec.Path)))
		files = append(files, fileBackup{
			SourcePath: spec.Path,
			Manifest: fileManifest{
				Label: spec.Label,
				SourcePath: spec.Path,
				ArchivePath: archivePath,
				ChecksumSHA256: checksum,
				SizeBytes: info.Size(),
			},
		})
	}
	return files, nil
}

func (s *Service) collectCertificates(ctx context.Context) ([]certificateManifest, error) {
	domain, ok, err := s.store.GetSetting(ctx, "bootstrap.domain")
	if err != nil {
		return nil, err
	}
	domain = strings.TrimSpace(domain)
	if !ok || domain == "" {
		return nil, nil
	}
	certificate := certificateManifest{
		Domain: domain,
		Source: "bootstrap.phase2_result",
		CapturedAt: time.Now().UTC().Format(time.RFC3339),
		MetadataOnly: true,
		Note: "Certificate metadata only; private key material is not included.",
	}
	raw, ok, err := s.store.GetSetting(ctx, "bootstrap.phase2_result")
	if err != nil {
		return nil, err
	}
	if ok && strings.TrimSpace(raw) != "" {
		var phase2 struct {
			CertificateIssuer string `json:"certificateIssuer"`
			SecurityState string `json:"securityState"`
			EntryMode string `json:"entryMode"`
		}
		if err := json.Unmarshal([]byte(raw), &phase2); err == nil {
			certificate.Issuer = strings.TrimSpace(phase2.CertificateIssuer)
			certificate.Available = certificate.Issuer != "" && phase2.SecurityState != "degraded" && phase2.EntryMode != "temporary_http"
		}
	}
	return []certificateManifest{certificate}, nil
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

type fileBackup struct {
	SourcePath string
	Manifest fileManifest
}

func writeArchive(path string, dbSnapshot string, files []fileBackup, meta manifest) error {
	tmpPath := path + ".tmp"
	out, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	zipWriter := zip.NewWriter(out)
	meta.Files = make([]fileManifest, 0, len(files))
	for _, file := range files {
		meta.Files = append(meta.Files, file.Manifest)
	}
	metaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		_ = zipWriter.Close()
		_ = out.Close()
		return err
	}
	metaBytes = append(metaBytes, '\n')
	if err := addBytes(zipWriter, "manifest.json", metaBytes); err != nil {
		_ = zipWriter.Close()
		_ = out.Close()
		return err
	}
	if err := addFile(zipWriter, "omo.db", dbSnapshot); err != nil {
		_ = zipWriter.Close()
		_ = out.Close()
		return err
	}
	for _, file := range files {
		if err := addFile(zipWriter, file.Manifest.ArchivePath, file.SourcePath); err != nil {
			_ = zipWriter.Close()
			_ = out.Close()
			return err
		}
	}
	if err := zipWriter.Close(); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func addBytes(zipWriter *zip.Writer, name string, data []byte) error {
	writer, err := zipWriter.Create(name)
	if err != nil {
		return err
	}
	_, err = writer.Write(data)
	return err
}

func addFile(zipWriter *zip.Writer, name string, src string) error {
	writer, err := zipWriter.Create(name)
	if err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	_, err = io.Copy(writer, in)
	return err
}

func encryptArchive(plainPath string, encryptedPath string, keyPath string) error {
	key, err := loadOrCreateKey(keyPath)
	if err != nil {
		return err
	}
	plain, err := os.ReadFile(plainPath)
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	sealed := gcm.Seal(nil, nonce, plain, []byte(encryptedArchiveMagic))
	tmpPath := encryptedPath + ".tmp"
	out, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := out.Write([]byte(encryptedArchiveMagic)); err != nil {
		_ = out.Close()
		return err
	}
	if _, err := out.Write(nonce); err != nil {
		_ = out.Close()
		return err
	}
	if _, err := out.Write(sealed); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, encryptedPath)
}

func prepareReadableArchive(path string, workDir string, keyPath string) (string, error) {
	if !isEncryptedArchive(path) {
		return path, nil
	}
	key, err := readKey(keyPath)
	if err != nil {
		return "", err
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	magic := []byte(encryptedArchiveMagic)
	if len(payload) <= len(magic) || string(payload[:len(magic)]) != encryptedArchiveMagic {
		return "", ErrInvalidBackup
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	rest := payload[len(magic):]
	if len(rest) <= gcm.NonceSize() {
		return "", ErrInvalidBackup
	}
	nonce := rest[:gcm.NonceSize()]
	ciphertext := rest[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, magic)
	if err != nil {
		return "", err
	}
	plainPath := filepath.Join(workDir, "decrypted.omo-backup.zip")
	if err := os.WriteFile(plainPath, plain, 0o600); err != nil {
		return "", err
	}
	return plainPath, nil
}

func isEncryptedArchive(path string) bool {
	in, err := os.Open(path)
	if err != nil {
		return false
	}
	defer in.Close()
	magic := make([]byte, len(encryptedArchiveMagic))
	n, err := io.ReadFull(in, magic)
	return err == nil && n == len(magic) && string(magic) == encryptedArchiveMagic
}

func loadOrCreateKey(path string) ([]byte, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("backup encryption key path is required")
	}
	if key, err := readKey(path); err == nil {
		return key, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	payload := []byte(hex.EncodeToString(key) + "\n")
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		return nil, err
	}
	return key, nil
}

func readKey(path string) ([]byte, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("backup encryption key path is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	keyText := strings.TrimSpace(string(data))
	key, err := hex.DecodeString(keyText)
	if err != nil || len(key) != 32 {
		return nil, ErrInvalidBackup
	}
	return key, nil
}

type archiveContent struct {
	DatabasePath string
	Manifest manifest
}

func extractArchive(archivePath string, dstDir string) (archiveContent, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return archiveContent{}, err
	}
	defer reader.Close()
	content := archiveContent{}
	for _, file := range reader.File {
		cleanName := filepath.ToSlash(filepath.Clean(file.Name))
		if cleanName == "." || strings.HasPrefix(cleanName, "../") || filepath.IsAbs(cleanName) {
			continue
		}
		if cleanName != "omo.db" && cleanName != "manifest.json" && !strings.HasPrefix(cleanName, "files/") {
			continue
		}
		src, err := file.Open()
		if err != nil {
			return archiveContent{}, err
		}
		dst := filepath.Join(dstDir, filepath.FromSlash(cleanName))
		if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
			_ = src.Close()
			return archiveContent{}, err
		}
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			_ = src.Close()
			return archiveContent{}, err
		}
		if _, err := io.Copy(out, src); err != nil {
			_ = src.Close()
			_ = out.Close()
			return archiveContent{}, err
		}
		_ = src.Close()
		if err := out.Close(); err != nil {
			return archiveContent{}, err
		}
		if cleanName == "omo.db" {
			content.DatabasePath = dst
		}
		if cleanName == "manifest.json" {
			payload, err := os.ReadFile(dst)
			if err != nil {
				return archiveContent{}, err
			}
			if err := json.Unmarshal(payload, &content.Manifest); err != nil {
				return archiveContent{}, err
			}
		}
	}
	if content.DatabasePath == "" {
		return archiveContent{}, fmt.Errorf("%w: missing database snapshot", ErrInvalidBackup)
	}
	return content, nil
}

type restoredFile struct {
	Path        string
	BackupPath  string
	HadOriginal bool
}

func restoreFiles(files []fileManifest, workDir string, allowed []FileSpec) ([]restoredFile, error) {
	allowedFiles := allowedRestoreFiles(allowed)
	restored := []restoredFile{}
	for _, file := range files {
		if strings.TrimSpace(file.SourcePath) == "" || strings.TrimSpace(file.ArchivePath) == "" {
			continue
		}
		if !allowedFiles[restoreFileKey(file.Label, file.SourcePath)] {
			continue
		}
		archivePath := cleanArchivePath(file.ArchivePath)
		if archivePath == "" {
			_ = rollbackRestoredFiles(restored)
			return nil, ErrInvalidBackup
		}
		source := filepath.Join(workDir, filepath.FromSlash(archivePath))
		if _, err := os.Stat(source); err != nil {
			_ = rollbackRestoredFiles(restored)
			return nil, err
		}
		checksum, err := fileSHA256(source)
		if err != nil {
			_ = rollbackRestoredFiles(restored)
			return nil, err
		}
		if file.ChecksumSHA256 != "" && !strings.EqualFold(checksum, file.ChecksumSHA256) {
			_ = rollbackRestoredFiles(restored)
			return nil, ErrInvalidBackup
		}
		if err := os.MkdirAll(filepath.Dir(file.SourcePath), 0o700); err != nil {
			_ = rollbackRestoredFiles(restored)
			return nil, err
		}
		action := restoredFile{Path: file.SourcePath}
		if _, err := os.Stat(file.SourcePath); err == nil {
			backupPath := file.SourcePath + ".pre-restore-" + time.Now().UTC().Format("20060102150405")
			if err := copyFile(file.SourcePath, backupPath); err != nil {
				_ = rollbackRestoredFiles(restored)
				return nil, err
			}
			action.BackupPath = backupPath
			action.HadOriginal = true
		}
		restored = append(restored, action)
		if err := copyFile(source, file.SourcePath); err != nil {
			_ = rollbackRestoredFiles(restored)
			return nil, err
		}
	}
	return restored, nil
}

func rollbackRestoredFiles(files []restoredFile) error {
	var firstErr error
	for i := len(files) - 1; i >= 0; i-- {
		file := files[i]
		if file.HadOriginal {
			if err := copyFile(file.BackupPath, file.Path); err != nil && firstErr == nil {
				firstErr = err
			}
			continue
		}
		if err := os.Remove(file.Path); err != nil && !errors.Is(err, os.ErrNotExist) && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func allowedRestoreFiles(files []FileSpec) map[string]bool {
	allowed := make(map[string]bool, len(files))
	for _, file := range normalizedFiles(files) {
		allowed[restoreFileKey(file.Label, file.Path)] = true
	}
	return allowed
}

func restoreFileKey(label string, path string) string {
	return strings.TrimSpace(label) + "\x00" + filepath.Clean(strings.TrimSpace(path))
}

func cleanArchivePath(path string) string {
	clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
	if clean == "." || strings.HasPrefix(clean, "../") || filepath.IsAbs(clean) || !strings.HasPrefix(clean, "files/") {
		return ""
	}
	return clean
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

func normalizedFiles(files []FileSpec) []FileSpec {
	normalized := make([]FileSpec, 0, len(files))
	seen := map[string]bool{}
	for _, file := range files {
		label := strings.TrimSpace(file.Label)
		path := strings.TrimSpace(file.Path)
		if label == "" || path == "" || seen[path] {
			continue
		}
		seen[path] = true
		normalized = append(normalized, FileSpec{Label: label, Path: path})
	}
	return normalized
}

func safeName(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "file"
	}
	var builder strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			builder.WriteRune(r)
		} else {
			builder.WriteByte('-')
		}
	}
	return builder.String()
}

func copyFile(src string, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
