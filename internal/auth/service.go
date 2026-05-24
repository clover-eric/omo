package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"omo/internal/store"
)

const SessionCookieName = "omo_session"

const (
	loginFailureLockThreshold = 5
	loginFailureLockDuration  = 5 * time.Minute
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrLoginLocked        = errors.New("login temporarily locked")
)

type Service struct {
	store *store.Store
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResult struct {
	Admin            AdminView `json:"admin"`
	SessionToken     string    `json:"-"`
	SessionExpiresAt time.Time `json:"-"`
}

type AdminView struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

func NewService(store *store.Store) *Service {
	return &Service{store: store}
}

func (s *Service) Login(ctx context.Context, req LoginRequest) (LoginResult, error) {
	username := normalizeUsername(req.Username)
	if username == "" || req.Password == "" {
		return LoginResult{}, ErrInvalidCredentials
	}

	locked, err := s.isLocked(ctx, username)
	if err != nil {
		return LoginResult{}, err
	}
	if locked {
		return LoginResult{}, ErrLoginLocked
	}

	admin, err := s.store.AdminByUsername(ctx, username)
	if err != nil {
		return LoginResult{}, err
	}
	if admin == nil || !VerifyPassword(req.Password, admin.PasswordHash) {
		if _, err := s.recordFailure(ctx, username); err != nil {
			return LoginResult{}, err
		}
		return LoginResult{}, ErrInvalidCredentials
	}

	if err := s.clearFailure(ctx, username); err != nil {
		return LoginResult{}, err
	}

	token, err := GenerateToken(32)
	if err != nil {
		return LoginResult{}, err
	}
	expiresAt := time.Now().UTC().Add(24 * time.Hour)
	if err := s.store.CreateSession(ctx, admin.ID, HashToken(token), expiresAt); err != nil {
		return LoginResult{}, err
	}
	_ = s.store.AppendAuditLog(ctx, &admin.ID, "auth.login", "admin", admin.ID, "{}")

	return LoginResult{
		Admin:            AdminView{ID: admin.ID, Username: admin.Username},
		SessionToken:     token,
		SessionExpiresAt: expiresAt,
	}, nil
}

func (s *Service) AdminForToken(ctx context.Context, token string) (*AdminView, error) {
	if token == "" {
		return nil, nil
	}
	admin, err := s.store.AdminBySessionTokenHash(ctx, HashToken(token), time.Now().UTC())
	if err != nil || admin == nil {
		return nil, err
	}
	view := AdminView{ID: admin.ID, Username: admin.Username}
	return &view, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.store.RevokeSession(ctx, HashToken(token))
}

func (s *Service) isLocked(ctx context.Context, username string) (bool, error) {
	state, err := s.store.LoginRateLimit(ctx, username)
	if err != nil || state == nil || state.LockedUntil == nil {
		return false, err
	}
	return time.Now().UTC().Before(*state.LockedUntil), nil
}

func (s *Service) recordFailure(ctx context.Context, username string) (*store.LoginRateLimit, error) {
	return s.store.RecordLoginFailure(ctx, username, loginFailureLockThreshold, loginFailureLockDuration, time.Now().UTC())
}

func (s *Service) clearFailure(ctx context.Context, username string) error {
	return s.store.ClearLoginRateLimit(ctx, username)
}

func normalizeUsername(username string) string {
	return strings.TrimSpace(username)
}
