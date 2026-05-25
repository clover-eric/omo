package configgen

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"omo/internal/protocol"
)

var (
	ErrInvalidProfile = errors.New("invalid service profile")
	ErrNoRollback     = errors.New("no previous service configuration to roll back")
	ErrConfigWrite    = errors.New("service configuration write failed")
)

type Validator interface {
	Validate(ctx context.Context, path string) error
}

type JSONValidator struct{}

type Options struct {
	ConfigPath string
	BackupDir  string
	Registry   *protocol.Registry
	Validator  Validator
}

type Manager struct {
	configPath string
	backupDir  string
	registry   *protocol.Registry
	validator  Validator
}

type Result struct {
	ProfileID          string `json:"profileId"`
	ProfileVersion     string `json:"profileVersion,omitempty"`
	ProfileDisplayName string `json:"profileDisplayName,omitempty"`
	ExpertProtocol     string `json:"expertProtocol,omitempty"`
	ConfigVersion      string `json:"configVersion"`
	ConfigPath         string `json:"configPath"`
	BackupPath         string `json:"backupPath,omitempty"`
	RolledBack         bool   `json:"rolledBack"`
	ListenPort         int    `json:"listenPort"`
}

type document struct {
	Log       map[string]any   `json:"log"`
	Inbounds  []map[string]any `json:"inbounds"`
	Outbounds []map[string]any `json:"outbounds"`
	Route     map[string]any   `json:"route"`
}

func NewManager(options Options) (*Manager, error) {
	registry := options.Registry
	if registry == nil {
		var err error
		registry, err = protocol.DefaultRegistry()
		if err != nil {
			return nil, err
		}
	}
	validator := options.Validator
	if validator == nil {
		validator = JSONValidator{}
	}
	configPath := strings.TrimSpace(options.ConfigPath)
	if configPath == "" {
		configPath = filepath.Join("data", "sing-box", "config.json")
	}
	backupDir := strings.TrimSpace(options.BackupDir)
	if backupDir == "" {
		backupDir = filepath.Join(filepath.Dir(configPath), "backups")
	}
	return &Manager{
		configPath: filepath.Clean(configPath),
		backupDir:  filepath.Clean(backupDir),
		registry:   registry,
		validator:  validator,
	}, nil
}

func (m *Manager) Apply(ctx context.Context, profileID string) (Result, error) {
	if m == nil {
		return Result{}, errors.New("config manager is nil")
	}
	profile, err := m.registry.Get(strings.TrimSpace(profileID))
	if err != nil {
		return Result{}, fmt.Errorf("%w: %s", ErrInvalidProfile, profileID)
	}
	version := time.Now().UTC().Format("20060102150405")
	rendered, err := render(profile, version)
	if err != nil {
		return Result{}, err
	}

	if err := os.MkdirAll(filepath.Dir(m.configPath), 0o755); err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrConfigWrite, err)
	}
	if err := os.MkdirAll(m.backupDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrConfigWrite, err)
	}

	tmpPath := m.configPath + ".tmp-" + version
	if err := os.WriteFile(tmpPath, rendered, 0o600); err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrConfigWrite, err)
	}
	defer func() { _ = os.Remove(tmpPath) }()

	if err := m.validator.Validate(ctx, tmpPath); err != nil {
		return Result{}, err
	}

	backupPath, err := m.backupCurrent(version)
	if err != nil {
		return Result{}, err
	}
	if err := os.Rename(tmpPath, m.configPath); err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrConfigWrite, err)
	}
	if err := m.validator.Validate(ctx, m.configPath); err != nil {
		_ = m.restorePrevious()
		return Result{}, err
	}

	return Result{
		ProfileID:          profile.ID,
		ProfileVersion:     profile.Version,
		ProfileDisplayName: profile.DisplayName,
		ExpertProtocol:     profile.ExpertProtocol,
		ConfigVersion:      version,
		ConfigPath:         m.configPath,
		BackupPath:         backupPath,
		RolledBack:         false,
		ListenPort:         listenPort(profile.ID),
	}, nil
}

func (m *Manager) Rollback(ctx context.Context, profileID string) (Result, error) {
	if m == nil {
		return Result{}, errors.New("config manager is nil")
	}
	profile, err := m.registry.Get(strings.TrimSpace(profileID))
	if err != nil {
		return Result{}, fmt.Errorf("%w: %s", ErrInvalidProfile, profileID)
	}
	previous := m.previousPath()
	if _, err := os.Stat(previous); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Result{}, ErrNoRollback
		}
		return Result{}, err
	}
	version := time.Now().UTC().Format("20060102150405")
	if err := os.MkdirAll(m.backupDir, 0o755); err != nil {
		return Result{}, err
	}
	tmpPath := m.configPath + ".rollback-" + version
	if err := copyFile(previous, tmpPath, 0o600); err != nil {
		return Result{}, err
	}
	defer func() { _ = os.Remove(tmpPath) }()
	if err := m.validator.Validate(ctx, tmpPath); err != nil {
		return Result{}, err
	}
	backupPath, err := m.backupCurrentHistory("rollback-" + version)
	if err != nil {
		return Result{}, err
	}
	if err := os.Rename(tmpPath, m.configPath); err != nil {
		return Result{}, err
	}
	if err := m.validator.Validate(ctx, m.configPath); err != nil {
		return Result{}, err
	}

	return Result{
		ProfileID:          profile.ID,
		ProfileVersion:     profile.Version,
		ProfileDisplayName: profile.DisplayName,
		ExpertProtocol:     profile.ExpertProtocol,
		ConfigVersion:      version,
		ConfigPath:         m.configPath,
		BackupPath:         backupPath,
		RolledBack:         true,
		ListenPort:         listenPort(profile.ID),
	}, nil
}

func (m *Manager) backupCurrent(version string) (string, error) {
	if _, err := os.Stat(m.configPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	previous := m.previousPath()
	if err := copyFile(m.configPath, previous, 0o600); err != nil {
		return "", err
	}
	backupPath := filepath.Join(m.backupDir, "config-"+version+".json")
	if err := copyFile(m.configPath, backupPath, 0o600); err != nil {
		return "", err
	}
	return backupPath, nil
}

func (m *Manager) backupCurrentHistory(version string) (string, error) {
	if _, err := os.Stat(m.configPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	backupPath := filepath.Join(m.backupDir, "config-"+version+".json")
	if err := copyFile(m.configPath, backupPath, 0o600); err != nil {
		return "", err
	}
	return backupPath, nil
}

func (m *Manager) restorePrevious() error {
	previous := m.previousPath()
	if _, err := os.Stat(previous); err != nil {
		return err
	}
	return copyFile(previous, m.configPath, 0o600)
}

func (m *Manager) previousPath() string {
	return m.configPath + ".previous"
}

func render(profile protocol.ServiceProfile, version string) ([]byte, error) {
	secret, err := secureHex(24)
	if err != nil {
		return nil, err
	}
	inbound := map[string]any{
		"type":        "mixed",
		"tag":         "omo-" + profile.ID + "-" + version,
		"listen":      "127.0.0.1",
		"listen_port": listenPort(profile.ID),
		"users": []map[string]any{{
			"username": "omo",
			"password": secret,
		}},
	}
	doc := document{
		Log: map[string]any{
			"level":     "info",
			"timestamp": true,
		},
		Inbounds: []map[string]any{inbound},
		Outbounds: []map[string]any{{
			"type": "direct",
			"tag":  "direct",
		}},
		Route: map[string]any{
			"final": "direct",
		},
	}
	payload, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	payload = append(payload, '\n')
	return payload, nil
}

func listenPort(profileID string) int {
	switch profileID {
	case "standard-secure-access":
		return 21080
	case "high-throughput-access":
		return 21081
	case "broad-compatibility-access":
		return 21082
	case "lightweight-fallback-access":
		return 21083
	case "mobile-optimized-access":
		return 21084
	default:
		return 21090
	}
}

func secureHex(size int) (string, error) {
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func (JSONValidator) Validate(_ context.Context, path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var decoded any
	if err := json.Unmarshal(content, &decoded); err != nil {
		return err
	}
	return nil
}

func copyFile(src string, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
