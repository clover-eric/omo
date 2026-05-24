package settings

import (
	"context"
	"path/filepath"
	"testing"

	"omo/internal/store"
)

func TestHostMatchesDomain(t *testing.T) {
	if !HostMatchesDomain("ops.example.com:443", "ops.example.com") {
		t.Fatal("expected host with port to match domain")
	}
	if HostMatchesDomain("127.0.0.1:443", "ops.example.com") {
		t.Fatal("expected IP host not to match domain")
	}
}

func TestDiagnosticsExternalProviderSettingsAreExplicitAndRedacted(t *testing.T) {
	ctx := context.Background()
	appStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer appStore.Close()

	defaults, err := LoadDiagnosticsExternalProvider(ctx, appStore)
	if err != nil {
		t.Fatalf("load defaults: %v", err)
	}
	if defaults.Enabled || defaults.APIKeyConfigured {
		t.Fatalf("expected provider disabled and key redacted by default, got %#v", defaults)
	}

	if _, err := SaveDiagnosticsExternalProvider(ctx, appStore, DiagnosticsExternalProviderUpdate{
		Enabled:        true,
		Name:           "Operator quality provider",
		EndpointURL:    "http://provider.example/check",
		TimeoutSeconds: 2,
	}); err != ErrInvalidDiagnosticsProvider {
		t.Fatalf("expected invalid non-https provider, got %v", err)
	}

	saved, err := SaveDiagnosticsExternalProvider(ctx, appStore, DiagnosticsExternalProviderUpdate{
		Enabled:        true,
		Name:           "Operator quality provider",
		EndpointURL:    "https://provider.example/check",
		TimeoutSeconds: 2,
		APIKey:         "secret-token",
	})
	if err != nil {
		t.Fatalf("save provider: %v", err)
	}
	if !saved.Enabled || !saved.APIKeyConfigured {
		t.Fatalf("expected enabled provider with redacted key flag, got %#v", saved)
	}
	if saved.EndpointURL != "https://provider.example/check" {
		t.Fatalf("unexpected endpoint %q", saved.EndpointURL)
	}
}
