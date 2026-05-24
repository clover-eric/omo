package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeSettingsStore map[string]string

func (s fakeSettingsStore) GetSetting(_ context.Context, key string) (string, bool, error) {
	value, ok := s[key]
	return value, ok, nil
}

func (s fakeSettingsStore) SetSetting(_ context.Context, key string, value string) error {
	s[key] = value
	return nil
}

func (s fakeSettingsStore) DeleteSetting(_ context.Context, key string) error {
	delete(s, key)
	return nil
}

func TestPanelAccessRedirectsAfterBootstrap(t *testing.T) {
	store := fakeSettingsStore{
		"bootstrap.phase1_complete": "true",
		"bootstrap.domain":          "ops.example.com",
	}
	handler := panelAccessMiddleware(store)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/dashboard", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected redirect, got %d", rec.Code)
	}
	if location := rec.Header().Get("Location"); location != "https://ops.example.com/dashboard" {
		t.Fatalf("unexpected redirect location %q", location)
	}
}

func TestPanelAccessAllowsAPI(t *testing.T) {
	store := fakeSettingsStore{
		"bootstrap.phase1_complete": "true",
		"bootstrap.domain":          "ops.example.com",
	}
	handler := panelAccessMiddleware(store)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/api/system/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected api pass-through, got %d", rec.Code)
	}
}
