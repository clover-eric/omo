package auth

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"omo/internal/store"
)

func TestLoginRateLimitPersistsAcrossServiceInstances(t *testing.T) {
	ctx := context.Background()
	appStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer appStore.Close()

	hash, err := HashPassword("StrongPassw0rd!")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if _, err := appStore.CreateAdmin(ctx, "admin", hash); err != nil {
		t.Fatalf("create admin: %v", err)
	}

	firstService := NewService(appStore)
	for i := 0; i < loginFailureLockThreshold; i++ {
		_, err := firstService.Login(ctx, LoginRequest{Username: "admin", Password: "wrong-password"})
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("failure %d expected invalid credentials, got %v", i+1, err)
		}
	}

	restartedService := NewService(appStore)
	_, err = restartedService.Login(ctx, LoginRequest{Username: "admin", Password: "StrongPassw0rd!"})
	if !errors.Is(err, ErrLoginLocked) {
		t.Fatalf("expected persisted lock after service restart, got %v", err)
	}
}

func TestSuccessfulLoginClearsPersistentRateLimit(t *testing.T) {
	ctx := context.Background()
	appStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer appStore.Close()

	hash, err := HashPassword("StrongPassw0rd!")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if _, err := appStore.CreateAdmin(ctx, "admin", hash); err != nil {
		t.Fatalf("create admin: %v", err)
	}

	authService := NewService(appStore)
	_, err = authService.Login(ctx, LoginRequest{Username: "admin", Password: "wrong-password"})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
	if record, err := appStore.LoginRateLimit(ctx, "admin"); err != nil || record == nil || record.FailureCount != 1 {
		t.Fatalf("expected one persisted failure, record=%#v err=%v", record, err)
	}

	if _, err := authService.Login(ctx, LoginRequest{Username: "admin", Password: "StrongPassw0rd!"}); err != nil {
		t.Fatalf("expected successful login, got %v", err)
	}
	record, err := appStore.LoginRateLimit(ctx, "admin")
	if err != nil {
		t.Fatalf("read login rate limit: %v", err)
	}
	if record != nil {
		t.Fatalf("expected successful login to clear rate limit, got %#v", record)
	}
}
