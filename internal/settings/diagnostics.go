package settings

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
)

const (
	DiagnosticsExternalProviderKey       = "diagnostics.external_provider"
	DiagnosticsExternalProviderAPIKeyKey = "diagnostics.external_provider.api_key"
	defaultDiagnosticsProviderTimeout    = 3
)

var ErrInvalidDiagnosticsProvider = errors.New("invalid diagnostics external provider settings")

type WritableStore interface {
	GetSetting(ctx context.Context, key string) (string, bool, error)
	SetSetting(ctx context.Context, key string, value string) error
	DeleteSetting(ctx context.Context, key string) error
}

type DiagnosticsExternalProviderSettings struct {
	Enabled          bool   `json:"enabled"`
	Name             string `json:"name"`
	EndpointURL      string `json:"endpointUrl"`
	TimeoutSeconds   int    `json:"timeoutSeconds"`
	APIKeyConfigured bool   `json:"apiKeyConfigured"`
}

type DiagnosticsExternalProviderUpdate struct {
	Enabled        bool   `json:"enabled"`
	Name           string `json:"name"`
	EndpointURL    string `json:"endpointUrl"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
	APIKey         string `json:"apiKey,omitempty"`
	ClearAPIKey    bool   `json:"clearApiKey,omitempty"`
}

type diagnosticsExternalProviderRecord struct {
	Enabled        bool   `json:"enabled"`
	Name           string `json:"name"`
	EndpointURL    string `json:"endpointUrl"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
}

func LoadDiagnosticsExternalProvider(ctx context.Context, appStore Store) (DiagnosticsExternalProviderSettings, error) {
	config, _, err := loadDiagnosticsExternalProviderRecord(ctx, appStore)
	if err != nil {
		return DiagnosticsExternalProviderSettings{}, err
	}
	_, keyConfigured, err := appStore.GetSetting(ctx, DiagnosticsExternalProviderAPIKeyKey)
	if err != nil {
		return DiagnosticsExternalProviderSettings{}, err
	}
	return settingsFromRecord(config, keyConfigured), nil
}

func LoadDiagnosticsExternalProviderSecret(ctx context.Context, appStore Store) (string, bool, error) {
	return appStore.GetSetting(ctx, DiagnosticsExternalProviderAPIKeyKey)
}

func SaveDiagnosticsExternalProvider(ctx context.Context, appStore WritableStore, update DiagnosticsExternalProviderUpdate) (DiagnosticsExternalProviderSettings, error) {
	record := diagnosticsExternalProviderRecord{
		Enabled:        update.Enabled,
		Name:           strings.TrimSpace(update.Name),
		EndpointURL:    strings.TrimSpace(update.EndpointURL),
		TimeoutSeconds: update.TimeoutSeconds,
	}
	if record.Name == "" {
		record.Name = "Operator provider"
	}
	if record.TimeoutSeconds == 0 {
		record.TimeoutSeconds = defaultDiagnosticsProviderTimeout
	}
	if err := validateDiagnosticsExternalProvider(record); err != nil {
		return DiagnosticsExternalProviderSettings{}, err
	}

	payload, err := json.Marshal(record)
	if err != nil {
		return DiagnosticsExternalProviderSettings{}, err
	}
	if err := appStore.SetSetting(ctx, DiagnosticsExternalProviderKey, string(payload)); err != nil {
		return DiagnosticsExternalProviderSettings{}, err
	}

	key := strings.TrimSpace(update.APIKey)
	if update.ClearAPIKey {
		if err := appStore.DeleteSetting(ctx, DiagnosticsExternalProviderAPIKeyKey); err != nil {
			return DiagnosticsExternalProviderSettings{}, err
		}
	} else if key != "" {
		if len(key) > 4096 {
			return DiagnosticsExternalProviderSettings{}, ErrInvalidDiagnosticsProvider
		}
		if err := appStore.SetSetting(ctx, DiagnosticsExternalProviderAPIKeyKey, key); err != nil {
			return DiagnosticsExternalProviderSettings{}, err
		}
	}

	_, keyConfigured, err := appStore.GetSetting(ctx, DiagnosticsExternalProviderAPIKeyKey)
	if err != nil {
		return DiagnosticsExternalProviderSettings{}, err
	}
	return settingsFromRecord(record, keyConfigured), nil
}

func loadDiagnosticsExternalProviderRecord(ctx context.Context, appStore Store) (diagnosticsExternalProviderRecord, bool, error) {
	value, ok, err := appStore.GetSetting(ctx, DiagnosticsExternalProviderKey)
	if err != nil {
		return diagnosticsExternalProviderRecord{}, false, err
	}
	if !ok || strings.TrimSpace(value) == "" {
		return diagnosticsExternalProviderRecord{Name: "Operator provider", TimeoutSeconds: defaultDiagnosticsProviderTimeout}, false, nil
	}
	var record diagnosticsExternalProviderRecord
	if err := json.Unmarshal([]byte(value), &record); err != nil {
		return diagnosticsExternalProviderRecord{}, false, err
	}
	record.Name = strings.TrimSpace(record.Name)
	record.EndpointURL = strings.TrimSpace(record.EndpointURL)
	if record.Name == "" {
		record.Name = "Operator provider"
	}
	if record.TimeoutSeconds == 0 {
		record.TimeoutSeconds = defaultDiagnosticsProviderTimeout
	}
	if err := validateDiagnosticsExternalProvider(record); err != nil {
		return diagnosticsExternalProviderRecord{}, false, err
	}
	return record, true, nil
}

func validateDiagnosticsExternalProvider(record diagnosticsExternalProviderRecord) error {
	if len(record.Name) > 80 || record.TimeoutSeconds < 1 || record.TimeoutSeconds > 10 {
		return ErrInvalidDiagnosticsProvider
	}
	if record.Enabled {
		parsed, err := url.Parse(record.EndpointURL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			return ErrInvalidDiagnosticsProvider
		}
	}
	return nil
}

func settingsFromRecord(record diagnosticsExternalProviderRecord, apiKeyConfigured bool) DiagnosticsExternalProviderSettings {
	return DiagnosticsExternalProviderSettings{
		Enabled:          record.Enabled,
		Name:             record.Name,
		EndpointURL:      record.EndpointURL,
		TimeoutSeconds:   record.TimeoutSeconds,
		APIKeyConfigured: apiKeyConfigured,
	}
}
