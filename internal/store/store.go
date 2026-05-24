package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type Store struct {
	db   *sql.DB
	path string
}

type Admin struct {
	ID           string
	Username     string
	PasswordHash string
}

type Session struct {
	ID        string
	AdminID   string
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
}

type LoginRateLimit struct {
	Username     string
	FailureCount int
	LockedUntil  *time.Time
	UpdatedAt    time.Time
}

type DistributionToken struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Status    string     `json:"status"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

type ServiceInstance struct {
	ID            string    `json:"id"`
	ProfileID     string    `json:"profileId"`
	DisplayName   string    `json:"displayName"`
	ListenPort    int       `json:"listenPort"`
	Status        string    `json:"status"`
	ConfigVersion string    `json:"configVersion"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type DiagnosticReport struct {
	ID         string
	Status     string
	Summary    string
	ReportJSON string
	CreatedAt  time.Time
}

type PairingCode struct {
	ID        string     `json:"id"`
	NodeID    string     `json:"nodeId"`
	NodeName  string     `json:"nodeName"`
	Domain    string     `json:"domain"`
	CodeHash  string     `json:"-"`
	PublicKey string     `json:"-"`
	Signature string     `json:"-"`
	Status    string     `json:"status"`
	ExpiresAt time.Time  `json:"expiresAt"`
	UsedAt    *time.Time `json:"usedAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

type CascadeNode struct {
	ID                  string    `json:"id"`
	Name                string    `json:"name"`
	Domain              string    `json:"domain"`
	Status              string    `json:"status"`
	Role                string    `json:"role"`
	TrustKeyFingerprint string    `json:"trustKeyFingerprint,omitempty"`
	Online              bool      `json:"online"`
	LatencyMS           int       `json:"latencyMs"`
	ThroughputMbps      float64   `json:"throughputMbps"`
	LastError           string    `json:"lastError,omitempty"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

type CascadePair struct {
	ID           string    `json:"id"`
	SourceNodeID string    `json:"sourceNodeId"`
	TargetNodeID string    `json:"targetNodeId"`
	Status       string    `json:"status"`
	ConfigState  string    `json:"configState"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type CascadeHealthSample struct {
	NodeID         string    `json:"nodeId"`
	Status         string    `json:"status"`
	Online         bool      `json:"online"`
	LatencyMS      int       `json:"latencyMs"`
	ThroughputMbps float64   `json:"throughputMbps"`
	LastError      string    `json:"lastError,omitempty"`
	SampledAt      time.Time `json:"sampledAt"`
}

type BackupRecord struct {
	ID          string     `json:"id"`
	Status      string     `json:"status"`
	Path        string     `json:"path"`
	Checksum    string     `json:"checksum,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

type AuditLog struct {
	ID           string    `json:"id"`
	ActorAdminID string    `json:"actorAdminId,omitempty"`
	Action       string    `json:"action"`
	ResourceType string    `json:"resourceType"`
	ResourceID   string    `json:"resourceId,omitempty"`
	DetailsJSON  string    `json:"-"`
	CreatedAt    time.Time `json:"createdAt"`
}

type Job struct {
	ID                string     `json:"id"`
	Kind              string     `json:"kind"`
	State             string     `json:"state"`
	Status            string     `json:"status"`
	Progress          int        `json:"progress"`
	UserMessage       string     `json:"userMessage"`
	InternalErrorCode string     `json:"internalErrorCode,omitempty"`
	StartedAt         *time.Time `json:"startedAt,omitempty"`
	FinishedAt        *time.Time `json:"finishedAt,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

type JobEvent struct {
	ID        int64     `json:"id"`
	JobID     string    `json:"jobId"`
	Kind      string    `json:"kind"`
	State     string    `json:"state"`
	Status    string    `json:"status"`
	Progress  int       `json:"progress"`
	Message   string    `json:"message"`
	ErrorCode string    `json:"errorCode,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

func Open(ctx context.Context, path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	db, err := openSQLite(path)
	if err != nil {
		return nil, err
	}

	store := &Store{db: db, path: path}
	if err := store.Migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func openSQLite(path string) (*sql.DB, error) {
	return sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) DatabasePath() string {
	return s.path
}

func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		return err
	}

	names, err := fs.Glob(migrationFiles, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)

	for _, name := range names {
		applied, err := s.migrationApplied(ctx, name)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		content, err := migrationFiles.ReadFile(name)
		if err != nil {
			return err
		}
		up, err := upMigrationSQL(string(content))
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}

		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, up); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("%s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`, name, nowString()); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}

func (s *Store) migrationApplied(ctx context.Context, version string) (bool, error) {
	var found string
	err := s.db.QueryRowContext(ctx, `SELECT version FROM schema_migrations WHERE version = ?`, version).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func upMigrationSQL(content string) (string, error) {
	upMarker := "-- +goose Up"
	downMarker := "-- +goose Down"
	upIndex := strings.Index(content, upMarker)
	if upIndex < 0 {
		return "", errors.New("missing goose up marker")
	}
	up := content[upIndex+len(upMarker):]
	downIndex := strings.Index(up, downMarker)
	if downIndex >= 0 {
		up = up[:downIndex]
	}
	return strings.TrimSpace(up), nil
}

func (s *Store) GetSetting(ctx context.Context, key string) (string, bool, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return value, err == nil, err
}

func (s *Store) SetSetting(ctx context.Context, key string, value string) error {
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key,
		value,
		nowString(),
	)
	return err
}

func (s *Store) DeleteSetting(ctx context.Context, key string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM settings WHERE key = ?`, key)
	return err
}

func (s *Store) AdminCount(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM admins`).Scan(&count)
	return count, err
}

func (s *Store) CreateAdmin(ctx context.Context, username string, passwordHash string) (Admin, error) {
	admin := Admin{
		ID:           randomID("adm"),
		Username:     username,
		PasswordHash: passwordHash,
	}
	now := nowString()
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO admins (id, username, password_hash, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		admin.ID,
		admin.Username,
		admin.PasswordHash,
		now,
		now,
	)
	return admin, err
}

func (s *Store) AdminByUsername(ctx context.Context, username string) (*Admin, error) {
	var admin Admin
	err := s.db.QueryRowContext(
		ctx,
		`SELECT id, username, password_hash FROM admins WHERE username = ?`,
		username,
	).Scan(&admin.ID, &admin.Username, &admin.PasswordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &admin, nil
}

func (s *Store) AdminBySessionTokenHash(ctx context.Context, tokenHash string, now time.Time) (*Admin, error) {
	var admin Admin
	err := s.db.QueryRowContext(
		ctx,
		`SELECT admins.id, admins.username, admins.password_hash
		 FROM sessions
		 JOIN admins ON admins.id = sessions.admin_id
		 WHERE sessions.token_hash = ?
		   AND sessions.revoked_at IS NULL
		   AND sessions.expires_at > ?
		 LIMIT 1`,
		tokenHash,
		now.UTC().Format(time.RFC3339Nano),
	).Scan(&admin.ID, &admin.Username, &admin.PasswordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &admin, nil
}

func (s *Store) CreateSession(ctx context.Context, adminID string, tokenHash string, expiresAt time.Time) error {
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO sessions (id, admin_id, token_hash, expires_at, created_at) VALUES (?, ?, ?, ?, ?)`,
		randomID("ses"),
		adminID,
		tokenHash,
		expiresAt.UTC().Format(time.RFC3339Nano),
		nowString(),
	)
	return err
}

func (s *Store) RevokeSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(
		ctx,
		`UPDATE sessions SET revoked_at = COALESCE(revoked_at, ?) WHERE token_hash = ?`,
		nowString(),
		tokenHash,
	)
	return err
}

func (s *Store) LoginRateLimit(ctx context.Context, username string) (*LoginRateLimit, error) {
	var record LoginRateLimit
	var lockedUntil sql.NullString
	var updatedAt string
	err := s.db.QueryRowContext(
		ctx,
		`SELECT username, failure_count, locked_until, updated_at FROM login_rate_limits WHERE username = ?`,
		username,
	).Scan(&record.Username, &record.FailureCount, &lockedUntil, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if lockedUntil.Valid {
		parsed, _ := time.Parse(time.RFC3339Nano, lockedUntil.String)
		record.LockedUntil = &parsed
	}
	record.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return &record, nil
}

func (s *Store) RecordLoginFailure(ctx context.Context, username string, lockThreshold int, lockDuration time.Duration, now time.Time) (*LoginRateLimit, error) {
	now = now.UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var currentCount int
	var currentLockedUntil sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT failure_count, locked_until FROM login_rate_limits WHERE username = ?`, username).Scan(&currentCount, &currentLockedUntil)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
		currentCount = 0
		currentLockedUntil = sql.NullString{}
	}
	if err != nil {
		return nil, err
	}

	failureCount := 1
	var lockedUntil *time.Time
	if currentCount > 0 {
		failureCount = currentCount + 1
	}
	if currentLockedUntil.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, currentLockedUntil.String)
		if err == nil && !now.Before(parsed) {
			failureCount = 1
		}
	}
	if failureCount >= lockThreshold {
		until := now.Add(lockDuration)
		lockedUntil = &until
	}

	var lockedUntilValue any
	if lockedUntil != nil {
		lockedUntilValue = lockedUntil.UTC().Format(time.RFC3339Nano)
	}
	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO login_rate_limits (username, failure_count, locked_until, updated_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT(username) DO UPDATE
		 SET failure_count = excluded.failure_count,
		     locked_until = excluded.locked_until,
		     updated_at = excluded.updated_at`,
		username,
		failureCount,
		lockedUntilValue,
		now.Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &LoginRateLimit{
		Username:     username,
		FailureCount: failureCount,
		LockedUntil:  lockedUntil,
		UpdatedAt:    now,
	}, nil
}

func (s *Store) ClearLoginRateLimit(ctx context.Context, username string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM login_rate_limits WHERE username = ?`, username)
	return err
}

func (s *Store) EnsureServiceProfile(ctx context.Context, profileID string, version string, displayName string, expertProtocol string) error {
	profileID = strings.TrimSpace(profileID)
	version = strings.TrimSpace(version)
	displayName = strings.TrimSpace(displayName)
	expertProtocol = strings.TrimSpace(expertProtocol)
	if profileID == "" || version == "" || displayName == "" {
		return errors.New("service profile metadata is incomplete")
	}
	now := nowString()
	moduleID := "mod_omo_access_services"
	if _, err := s.db.ExecContext(
		ctx,
		`INSERT INTO service_modules (id, name, version, status, created_at, updated_at)
		 VALUES (?, ?, ?, 'active', ?, ?)
		 ON CONFLICT(id) DO UPDATE SET version = excluded.version, status = 'active', updated_at = excluded.updated_at`,
		moduleID,
		"OMO managed access services",
		version,
		now,
		now,
	); err != nil {
		return err
	}
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO service_profiles (id, module_id, profile_key, version, display_name, expert_protocol, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, 'active', ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   module_id = excluded.module_id,
		   version = excluded.version,
		   display_name = excluded.display_name,
		   expert_protocol = excluded.expert_protocol,
		   status = 'active',
		   updated_at = excluded.updated_at`,
		profileID,
		moduleID,
		profileID,
		version,
		displayName,
		expertProtocol,
		now,
		now,
	)
	return err
}

func (s *Store) CreateServiceInstance(ctx context.Context, profileID string, displayName string, listenPort int, status string, configVersion string) (ServiceInstance, error) {
	record := ServiceInstance{
		ID:            randomID("svc"),
		ProfileID:     strings.TrimSpace(profileID),
		DisplayName:   strings.TrimSpace(displayName),
		ListenPort:    listenPort,
		Status:        strings.TrimSpace(status),
		ConfigVersion: strings.TrimSpace(configVersion),
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if record.DisplayName == "" {
		record.DisplayName = record.ProfileID
	}
	now := nowString()
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO service_instances (id, profile_id, display_name, listen_port, status, config_version, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		record.ID,
		record.ProfileID,
		record.DisplayName,
		record.ListenPort,
		record.Status,
		record.ConfigVersion,
		now,
		now,
	)
	if err != nil {
		return ServiceInstance{}, err
	}
	record.CreatedAt, _ = time.Parse(time.RFC3339Nano, now)
	record.UpdatedAt = record.CreatedAt
	return record, nil
}

func (s *Store) ListServiceInstances(ctx context.Context) ([]ServiceInstance, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, profile_id, display_name, listen_port, status, config_version, created_at, updated_at
		 FROM service_instances
		 ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []ServiceInstance
	for rows.Next() {
		record, err := scanServiceInstance(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *Store) UpdateServiceInstance(ctx context.Context, id string, displayName *string, listenPort *int, status *string, configVersion *string) (*ServiceInstance, error) {
	current, err := s.ServiceInstanceByID(ctx, id)
	if err != nil || current == nil {
		return current, err
	}
	nextName := current.DisplayName
	nextPort := current.ListenPort
	nextStatus := current.Status
	nextVersion := current.ConfigVersion
	if displayName != nil {
		nextName = strings.TrimSpace(*displayName)
	}
	if listenPort != nil {
		nextPort = *listenPort
	}
	if status != nil {
		nextStatus = strings.TrimSpace(*status)
	}
	if configVersion != nil {
		nextVersion = strings.TrimSpace(*configVersion)
	}
	_, err = s.db.ExecContext(
		ctx,
		`UPDATE service_instances
		 SET display_name = ?, listen_port = ?, status = ?, config_version = ?, updated_at = ?
		 WHERE id = ?`,
		nextName,
		nextPort,
		nextStatus,
		nextVersion,
		nowString(),
		id,
	)
	if err != nil {
		return nil, err
	}
	return s.ServiceInstanceByID(ctx, id)
}

func (s *Store) ActivateServiceInstancesForProfile(ctx context.Context, profileID string, displayName string, listenPort int, configVersion string) ([]ServiceInstance, error) {
	profileID = strings.TrimSpace(profileID)
	displayName = strings.TrimSpace(displayName)
	configVersion = strings.TrimSpace(configVersion)
	if displayName == "" {
		displayName = profileID
	}
	if configVersion == "" {
		configVersion = "active"
	}
	result, err := s.db.ExecContext(
		ctx,
		`UPDATE service_instances
		 SET status = 'active', display_name = CASE WHEN display_name = '' THEN ? ELSE display_name END, listen_port = ?, config_version = ?, updated_at = ?
		 WHERE profile_id = ? AND status IN ('planned', 'active')`,
		displayName,
		listenPort,
		configVersion,
		nowString(),
		profileID,
	)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		if _, err := s.CreateServiceInstance(ctx, profileID, displayName, listenPort, "active", configVersion); err != nil {
			return nil, err
		}
	}
	return s.ServiceInstancesByProfile(ctx, profileID)
}

func (s *Store) DeactivateServiceInstancesForProfile(ctx context.Context, profileID string, configVersion string) ([]ServiceInstance, error) {
	profileID = strings.TrimSpace(profileID)
	configVersion = strings.TrimSpace(configVersion)
	if configVersion == "" {
		configVersion = "rolled_back"
	}
	if _, err := s.db.ExecContext(
		ctx,
		`UPDATE service_instances
		 SET status = 'planned', config_version = ?, updated_at = ?
		 WHERE profile_id = ? AND status = 'active'`,
		configVersion,
		nowString(),
		profileID,
	); err != nil {
		return nil, err
	}
	return s.ServiceInstancesByProfile(ctx, profileID)
}

func (s *Store) ServiceInstancesByProfile(ctx context.Context, profileID string) ([]ServiceInstance, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, profile_id, display_name, listen_port, status, config_version, created_at, updated_at
		 FROM service_instances
		 WHERE profile_id = ?
		 ORDER BY created_at DESC`,
		strings.TrimSpace(profileID),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []ServiceInstance
	for rows.Next() {
		record, err := scanServiceInstance(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *Store) ServiceInstanceByID(ctx context.Context, id string) (*ServiceInstance, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT id, profile_id, display_name, listen_port, status, config_version, created_at, updated_at
		 FROM service_instances WHERE id = ?`,
		id,
	)
	record, err := scanServiceInstance(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *Store) CreateDistributionToken(ctx context.Context, name string, tokenHash string, expiresAt *time.Time) (DistributionToken, error) {
	record := DistributionToken{
		ID:        randomID("sub"),
		Name:      name,
		Status:    "active",
		ExpiresAt: expiresAt,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	var expires any
	if expiresAt != nil {
		expires = expiresAt.UTC().Format(time.RFC3339Nano)
	}
	now := nowString()
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO distribution_tokens (id, token_hash, name, status, expires_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		record.ID,
		tokenHash,
		record.Name,
		record.Status,
		expires,
		now,
		now,
	)
	if err != nil {
		return DistributionToken{}, err
	}
	return record, nil
}

func (s *Store) ListDistributionTokens(ctx context.Context) ([]DistributionToken, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, name, status, expires_at, created_at, updated_at
		 FROM distribution_tokens
		 ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []DistributionToken
	for rows.Next() {
		record, err := scanDistributionToken(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *Store) DistributionTokenByHash(ctx context.Context, tokenHash string, now time.Time) (*DistributionToken, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT id, name, status, expires_at, created_at, updated_at
		 FROM distribution_tokens
		 WHERE token_hash = ?
		   AND status = 'active'
		   AND (expires_at IS NULL OR expires_at > ?)
		 LIMIT 1`,
		tokenHash,
		now.UTC().Format(time.RFC3339Nano),
	)
	record, err := scanDistributionToken(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *Store) RotateDistributionToken(ctx context.Context, id string, tokenHash string) (*DistributionToken, error) {
	now := nowString()
	result, err := s.db.ExecContext(
		ctx,
		`UPDATE distribution_tokens SET token_hash = ?, updated_at = ? WHERE id = ? AND status = 'active'`,
		tokenHash,
		now,
		id,
	)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, nil
	}
	row := s.db.QueryRowContext(ctx, `SELECT id, name, status, expires_at, created_at, updated_at FROM distribution_tokens WHERE id = ?`, id)
	record, err := scanDistributionToken(row)
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *Store) RecordSubscriptionRequest(ctx context.Context, distributionTokenID string, clientHint string, remoteAddrHash string) error {
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO subscription_requests (id, distribution_token_id, client_hint, remote_addr_hash, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		randomID("sreq"),
		distributionTokenID,
		clientHint,
		remoteAddrHash,
		nowString(),
	)
	return err
}

func (s *Store) CreateDiagnosticReport(ctx context.Context, status string, summary string, reportJSON string) (DiagnosticReport, error) {
	record := DiagnosticReport{
		ID:         randomID("diag"),
		Status:     status,
		Summary:    summary,
		ReportJSON: reportJSON,
		CreatedAt:  time.Now().UTC(),
	}
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO diagnostic_reports (id, status, summary, report_json, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		record.ID,
		record.Status,
		record.Summary,
		record.ReportJSON,
		record.CreatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return DiagnosticReport{}, err
	}
	return record, nil
}

func (s *Store) LatestDiagnosticReport(ctx context.Context) (*DiagnosticReport, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT id, status, summary, report_json, created_at
		 FROM diagnostic_reports
		 ORDER BY created_at DESC
		 LIMIT 1`,
	)
	record, err := scanDiagnosticReport(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *Store) CreatePairingCode(ctx context.Context, nodeID string, nodeName string, domain string, codeHash string, publicKey string, signature string, expiresAt time.Time) (PairingCode, error) {
	record := PairingCode{
		ID:        randomID("paircode"),
		NodeID:    nodeID,
		NodeName:  nodeName,
		Domain:    domain,
		CodeHash:  codeHash,
		PublicKey: publicKey,
		Signature: signature,
		Status:    "active",
		ExpiresAt: expiresAt.UTC(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	now := nowString()
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO pairing_codes (id, node_id, node_name, domain, code_hash, public_key, signature, status, expires_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.ID,
		record.NodeID,
		record.NodeName,
		record.Domain,
		record.CodeHash,
		record.PublicKey,
		record.Signature,
		record.Status,
		record.ExpiresAt.Format(time.RFC3339Nano),
		now,
		now,
	)
	if err != nil {
		return PairingCode{}, err
	}
	record.CreatedAt, _ = time.Parse(time.RFC3339Nano, now)
	record.UpdatedAt = record.CreatedAt
	return record, nil
}

func (s *Store) PairingCodeByHash(ctx context.Context, codeHash string, now time.Time) (*PairingCode, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT id, node_id, node_name, domain, code_hash, public_key, signature, status, expires_at, used_at, created_at, updated_at
		 FROM pairing_codes
		 WHERE code_hash = ? AND status = 'active' AND expires_at > ?
		 LIMIT 1`,
		codeHash,
		now.UTC().Format(time.RFC3339Nano),
	)
	record, err := scanPairingCode(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *Store) AnyPairingCodeByHash(ctx context.Context, codeHash string) (*PairingCode, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT id, node_id, node_name, domain, code_hash, public_key, signature, status, expires_at, used_at, created_at, updated_at
		 FROM pairing_codes
		 WHERE code_hash = ?
		 LIMIT 1`,
		codeHash,
	)
	record, err := scanPairingCode(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *Store) MarkPairingCodeUsed(ctx context.Context, id string, now time.Time) error {
	_, err := s.db.ExecContext(
		ctx,
		`UPDATE pairing_codes SET status = 'used', used_at = ?, updated_at = ? WHERE id = ? AND status = 'active'`,
		now.UTC().Format(time.RFC3339Nano),
		now.UTC().Format(time.RFC3339Nano),
		id,
	)
	return err
}

func (s *Store) CreateCascadeNode(ctx context.Context, name string, domain string, status string, role string, fingerprint string) (CascadeNode, error) {
	record := CascadeNode{
		ID:                  randomID("node"),
		Name:                name,
		Domain:              domain,
		Status:              status,
		Role:                role,
		TrustKeyFingerprint: fingerprint,
		CreatedAt:           time.Now().UTC(),
		UpdatedAt:           time.Now().UTC(),
	}
	now := nowString()
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO cascade_nodes (id, name, domain, status, role, trust_key_fingerprint, online, latency_ms, throughput_mbps, last_error, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, 0, 0, 0, '', ?, ?)`,
		record.ID,
		record.Name,
		record.Domain,
		record.Status,
		record.Role,
		record.TrustKeyFingerprint,
		now,
		now,
	)
	if err != nil {
		return CascadeNode{}, err
	}
	record.CreatedAt, _ = time.Parse(time.RFC3339Nano, now)
	record.UpdatedAt = record.CreatedAt
	return record, nil
}

func (s *Store) ListCascadeNodes(ctx context.Context) ([]CascadeNode, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, name, domain, status, role, trust_key_fingerprint, online, latency_ms, throughput_mbps, last_error, created_at, updated_at
		 FROM cascade_nodes
		 ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []CascadeNode
	for rows.Next() {
		record, err := scanCascadeNode(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *Store) UpdateCascadeNode(ctx context.Context, id string, name string, status string) (*CascadeNode, error) {
	current, err := s.CascadeNodeByID(ctx, id)
	if err != nil || current == nil {
		return current, err
	}
	if name == "" {
		name = current.Name
	}
	if status == "" {
		status = current.Status
	}
	_, err = s.db.ExecContext(ctx, `UPDATE cascade_nodes SET name = ?, status = ?, updated_at = ? WHERE id = ?`, name, status, nowString(), id)
	if err != nil {
		return nil, err
	}
	return s.CascadeNodeByID(ctx, id)
}

func (s *Store) DeleteCascadeNode(ctx context.Context, id string) (bool, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM cascade_nodes WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (s *Store) CascadeNodeByID(ctx context.Context, id string) (*CascadeNode, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT id, name, domain, status, role, trust_key_fingerprint, online, latency_ms, throughput_mbps, last_error, created_at, updated_at
		 FROM cascade_nodes WHERE id = ?`,
		id,
	)
	record, err := scanCascadeNode(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *Store) CreateCascadePair(ctx context.Context, sourceNodeID string, targetNodeID string, status string, configState string) (CascadePair, error) {
	record := CascadePair{
		ID:           randomID("cpair"),
		SourceNodeID: sourceNodeID,
		TargetNodeID: targetNodeID,
		Status:       status,
		ConfigState:  configState,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	now := nowString()
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO cascade_pairs (id, source_node_id, target_node_id, status, config_state, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		record.ID,
		record.SourceNodeID,
		record.TargetNodeID,
		record.Status,
		record.ConfigState,
		now,
		now,
	)
	if err != nil {
		return CascadePair{}, err
	}
	record.CreatedAt, _ = time.Parse(time.RFC3339Nano, now)
	record.UpdatedAt = record.CreatedAt
	return record, nil
}

func (s *Store) ListCascadePairs(ctx context.Context) ([]CascadePair, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, source_node_id, target_node_id, status, config_state, created_at, updated_at
		 FROM cascade_pairs
		 ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []CascadePair
	for rows.Next() {
		record, err := scanCascadePair(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *Store) UpdateCascadePairConfigState(ctx context.Context, id string, status string, configState string) (*CascadePair, error) {
	current, err := s.CascadePairByID(ctx, id)
	if err != nil || current == nil {
		return current, err
	}
	if status == "" {
		status = current.Status
	}
	if configState == "" {
		configState = current.ConfigState
	}
	_, err = s.db.ExecContext(ctx, `UPDATE cascade_pairs SET status = ?, config_state = ?, updated_at = ? WHERE id = ?`, status, configState, nowString(), id)
	if err != nil {
		return nil, err
	}
	return s.CascadePairByID(ctx, id)
}

func (s *Store) CascadePairByID(ctx context.Context, id string) (*CascadePair, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT id, source_node_id, target_node_id, status, config_state, created_at, updated_at
		 FROM cascade_pairs WHERE id = ?`,
		id,
	)
	record, err := scanCascadePair(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *Store) RecordCascadeHealthSample(ctx context.Context, nodeID string, online bool, latencyMS int, throughputMbps float64, lastError string, sampledAt time.Time) (CascadeHealthSample, error) {
	status := "offline"
	if online {
		status = "online"
	}
	onlineValue := 0
	if online {
		onlineValue = 1
	}
	sampledAt = sampledAt.UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return CascadeHealthSample{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(
		ctx,
		`UPDATE cascade_nodes
		 SET online = ?, latency_ms = ?, throughput_mbps = ?, last_error = ?, updated_at = ?
		 WHERE id = ?`,
		onlineValue,
		latencyMS,
		throughputMbps,
		lastError,
		sampledAt.Format(time.RFC3339Nano),
		nodeID,
	); err != nil {
		return CascadeHealthSample{}, err
	}
	if err := tx.Commit(); err != nil {
		return CascadeHealthSample{}, err
	}
	return CascadeHealthSample{NodeID: nodeID, Status: status, Online: online, LatencyMS: latencyMS, ThroughputMbps: throughputMbps, LastError: lastError, SampledAt: sampledAt}, nil
}

func (s *Store) CreateBackupRecord(ctx context.Context, status string, path string) (BackupRecord, error) {
	record := BackupRecord{
		ID:        randomID("bak"),
		Status:    status,
		Path:      path,
		CreatedAt: time.Now().UTC(),
	}
	now := nowString()
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO backup_records (id, status, path, checksum, created_at, completed_at)
		 VALUES (?, ?, ?, '', ?, NULL)`,
		record.ID,
		record.Status,
		record.Path,
		now,
	)
	if err != nil {
		return BackupRecord{}, err
	}
	record.CreatedAt, _ = time.Parse(time.RFC3339Nano, now)
	return record, nil
}

func (s *Store) CompleteBackupRecord(ctx context.Context, id string, status string, checksum string, completedAt time.Time) (*BackupRecord, error) {
	completedAt = completedAt.UTC()
	result, err := s.db.ExecContext(
		ctx,
		`UPDATE backup_records SET status = ?, checksum = ?, completed_at = ? WHERE id = ?`,
		status,
		checksum,
		completedAt.Format(time.RFC3339Nano),
		id,
	)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, nil
	}
	return s.BackupRecordByID(ctx, id)
}

func (s *Store) BackupRecordByID(ctx context.Context, id string) (*BackupRecord, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT id, status, path, COALESCE(checksum, ''), created_at, completed_at
		 FROM backup_records WHERE id = ?`,
		id,
	)
	record, err := scanBackupRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *Store) ListBackupRecords(ctx context.Context) ([]BackupRecord, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, status, path, COALESCE(checksum, ''), created_at, completed_at
		 FROM backup_records
		 ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []BackupRecord
	for rows.Next() {
		record, err := scanBackupRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *Store) BackupDatabaseSnapshot(ctx context.Context, dst string) error {
	if strings.TrimSpace(dst) == "" {
		return errors.New("backup snapshot path is required")
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	_ = os.Remove(dst)
	_, err := s.db.ExecContext(ctx, "VACUUM INTO "+sqliteQuote(dst))
	return err
}

func (s *Store) RestoreDatabaseSnapshot(ctx context.Context, snapshotPath string) error {
	if strings.TrimSpace(s.path) == "" {
		return errors.New("store database path is unknown")
	}
	if strings.TrimSpace(snapshotPath) == "" {
		return errors.New("restore snapshot path is required")
	}
	if _, err := os.Stat(snapshotPath); err != nil {
		return err
	}
	currentBackup := s.path + ".pre-restore-" + time.Now().UTC().Format("20060102150405")
	if err := s.BackupDatabaseSnapshot(ctx, currentBackup); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := s.db.Close(); err != nil {
		return err
	}
	if err := copyFile(snapshotPath, s.path); err != nil {
		db, openErr := openSQLite(s.path)
		if openErr == nil {
			s.db = db
		}
		return err
	}
	_ = os.Remove(s.path + "-wal")
	_ = os.Remove(s.path + "-shm")
	db, err := openSQLite(s.path)
	if err != nil {
		_ = copyFile(currentBackup, s.path)
		reopened, reopenErr := openSQLite(s.path)
		if reopenErr == nil {
			s.db = reopened
			_ = s.Migrate(ctx)
		}
		return err
	}
	s.db = db
	return s.Migrate(ctx)
}

func (s *Store) AppendAuditLog(ctx context.Context, adminID *string, action string, resourceType string, resourceID string, detailsJSON string) error {
	var actor any
	if adminID != nil {
		actor = *adminID
	}
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO audit_logs (id, actor_admin_id, action, resource_type, resource_id, details_json, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		randomID("aud"),
		actor,
		action,
		resourceType,
		resourceID,
		detailsJSON,
		nowString(),
	)
	return err
}

func (s *Store) ListAuditLogs(ctx context.Context, limit int) ([]AuditLog, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, actor_admin_id, action, resource_type, COALESCE(resource_id, ''), details_json, created_at
		 FROM audit_logs
		 ORDER BY created_at DESC
		 LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []AuditLog
	for rows.Next() {
		record, err := scanAuditLog(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *Store) CreateJob(ctx context.Context, kind string, state string, status string, progress int, message string) (Job, error) {
	job := Job{
		ID:          randomID("job"),
		Kind:        kind,
		State:       state,
		Status:      status,
		Progress:    progress,
		UserMessage: message,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	now := nowString()
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO jobs (id, kind, state, status, progress, user_message, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID,
		job.Kind,
		job.State,
		job.Status,
		job.Progress,
		job.UserMessage,
		now,
		now,
	)
	if err != nil {
		return Job{}, err
	}
	return job, nil
}

func (s *Store) UpdateJob(ctx context.Context, jobID string, state string, status string, progress int, message string, errorCode string, finished bool) error {
	now := nowString()
	var finishedAt any
	if finished {
		finishedAt = now
	}

	_, err := s.db.ExecContext(
		ctx,
		`UPDATE jobs
		 SET state = ?, status = ?, progress = ?, user_message = ?, internal_error_code = ?, finished_at = COALESCE(?, finished_at), updated_at = ?
		 WHERE id = ?`,
		state,
		status,
		progress,
		message,
		errorCode,
		finishedAt,
		now,
		jobID,
	)
	return err
}

func (s *Store) MarkJobStarted(ctx context.Context, jobID string) error {
	now := nowString()
	_, err := s.db.ExecContext(ctx, `UPDATE jobs SET started_at = COALESCE(started_at, ?), updated_at = ? WHERE id = ?`, now, now, jobID)
	return err
}

func (s *Store) LatestJob(ctx context.Context, kind string) (*Job, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT id, kind, state, status, progress, COALESCE(user_message, ''), COALESCE(internal_error_code, ''), started_at, finished_at, created_at, updated_at
		 FROM jobs WHERE kind = ? ORDER BY created_at DESC LIMIT 1`,
		kind,
	)
	job, err := scanJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (s *Store) AppendJobEvent(ctx context.Context, jobID string, kind string, state string, status string, progress int, message string, errorCode string) (JobEvent, error) {
	now := nowString()
	result, err := s.db.ExecContext(
		ctx,
		`INSERT INTO job_events (job_id, kind, state, status, progress, message, error_code, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		jobID,
		kind,
		state,
		status,
		progress,
		message,
		errorCode,
		now,
	)
	if err != nil {
		return JobEvent{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return JobEvent{}, err
	}
	createdAt, _ := time.Parse(time.RFC3339Nano, now)
	return JobEvent{ID: id, JobID: jobID, Kind: kind, State: state, Status: status, Progress: progress, Message: message, ErrorCode: errorCode, CreatedAt: createdAt}, nil
}

func (s *Store) ListJobEventsAfter(ctx context.Context, kind string, afterID int64) ([]JobEvent, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, job_id, kind, state, status, progress, message, COALESCE(error_code, ''), created_at
		 FROM job_events
		 WHERE kind = ? AND id > ?
		 ORDER BY id ASC
		 LIMIT 200`,
		kind,
		afterID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []JobEvent
	for rows.Next() {
		var event JobEvent
		var createdAt string
		if err := rows.Scan(&event.ID, &event.JobID, &event.Kind, &event.State, &event.Status, &event.Progress, &event.Message, &event.ErrorCode, &createdAt); err != nil {
			return nil, err
		}
		event.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		events = append(events, event)
	}
	return events, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanDistributionToken(row rowScanner) (DistributionToken, error) {
	var record DistributionToken
	var expiresAt sql.NullString
	var createdAt string
	var updatedAt string
	err := row.Scan(&record.ID, &record.Name, &record.Status, &expiresAt, &createdAt, &updatedAt)
	if err != nil {
		return DistributionToken{}, err
	}
	if expiresAt.Valid {
		parsed, _ := time.Parse(time.RFC3339Nano, expiresAt.String)
		record.ExpiresAt = &parsed
	}
	record.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	record.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return record, nil
}

func scanDiagnosticReport(row rowScanner) (DiagnosticReport, error) {
	var record DiagnosticReport
	var createdAt string
	err := row.Scan(&record.ID, &record.Status, &record.Summary, &record.ReportJSON, &createdAt)
	if err != nil {
		return DiagnosticReport{}, err
	}
	record.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	return record, nil
}

func scanServiceInstance(row rowScanner) (ServiceInstance, error) {
	var record ServiceInstance
	var createdAt string
	var updatedAt string
	err := row.Scan(
		&record.ID,
		&record.ProfileID,
		&record.DisplayName,
		&record.ListenPort,
		&record.Status,
		&record.ConfigVersion,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return ServiceInstance{}, err
	}
	record.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	record.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return record, nil
}

func scanPairingCode(row rowScanner) (PairingCode, error) {
	var record PairingCode
	var expiresAt string
	var usedAt sql.NullString
	var createdAt string
	var updatedAt string
	err := row.Scan(
		&record.ID,
		&record.NodeID,
		&record.NodeName,
		&record.Domain,
		&record.CodeHash,
		&record.PublicKey,
		&record.Signature,
		&record.Status,
		&expiresAt,
		&usedAt,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return PairingCode{}, err
	}
	record.ExpiresAt, _ = time.Parse(time.RFC3339Nano, expiresAt)
	if usedAt.Valid {
		parsed, _ := time.Parse(time.RFC3339Nano, usedAt.String)
		record.UsedAt = &parsed
	}
	record.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	record.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return record, nil
}

func scanCascadeNode(row rowScanner) (CascadeNode, error) {
	var record CascadeNode
	var online int
	var createdAt string
	var updatedAt string
	err := row.Scan(
		&record.ID,
		&record.Name,
		&record.Domain,
		&record.Status,
		&record.Role,
		&record.TrustKeyFingerprint,
		&online,
		&record.LatencyMS,
		&record.ThroughputMbps,
		&record.LastError,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return CascadeNode{}, err
	}
	record.Online = online != 0
	record.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	record.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return record, nil
}

func scanCascadePair(row rowScanner) (CascadePair, error) {
	var record CascadePair
	var createdAt string
	var updatedAt string
	err := row.Scan(
		&record.ID,
		&record.SourceNodeID,
		&record.TargetNodeID,
		&record.Status,
		&record.ConfigState,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return CascadePair{}, err
	}
	record.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	record.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return record, nil
}

func scanBackupRecord(row rowScanner) (BackupRecord, error) {
	var record BackupRecord
	var createdAt string
	var completedAt sql.NullString
	err := row.Scan(&record.ID, &record.Status, &record.Path, &record.Checksum, &createdAt, &completedAt)
	if err != nil {
		return BackupRecord{}, err
	}
	record.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	if completedAt.Valid {
		parsed, _ := time.Parse(time.RFC3339Nano, completedAt.String)
		record.CompletedAt = &parsed
	}
	return record, nil
}

func scanAuditLog(row rowScanner) (AuditLog, error) {
	var record AuditLog
	var actorAdminID sql.NullString
	var createdAt string
	err := row.Scan(
		&record.ID,
		&actorAdminID,
		&record.Action,
		&record.ResourceType,
		&record.ResourceID,
		&record.DetailsJSON,
		&createdAt,
	)
	if err != nil {
		return AuditLog{}, err
	}
	if actorAdminID.Valid {
		record.ActorAdminID = actorAdminID.String
	}
	record.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	return record, nil
}

func scanJob(row rowScanner) (Job, error) {
	var job Job
	var startedAt sql.NullString
	var finishedAt sql.NullString
	var createdAt string
	var updatedAt string
	err := row.Scan(
		&job.ID,
		&job.Kind,
		&job.State,
		&job.Status,
		&job.Progress,
		&job.UserMessage,
		&job.InternalErrorCode,
		&startedAt,
		&finishedAt,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return Job{}, err
	}
	if startedAt.Valid {
		parsed, _ := time.Parse(time.RFC3339Nano, startedAt.String)
		job.StartedAt = &parsed
	}
	if finishedAt.Valid {
		parsed, _ := time.Parse(time.RFC3339Nano, finishedAt.String)
		job.FinishedAt = &parsed
	}
	job.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	job.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return job, nil
}

func nowString() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func randomID(prefix string) string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(raw)
}

func sqliteQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
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
