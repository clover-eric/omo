package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"omo/internal/audit"
	"omo/internal/auth"
	"omo/internal/backup"
	"omo/internal/bootstrap"
	"omo/internal/configgen"
	"omo/internal/core/singbox"
	"omo/internal/diagnostics"
	"omo/internal/pairing"
	"omo/internal/protocol"
	"omo/internal/settings"
	"omo/internal/store"
	"omo/internal/subscription"
	"omo/internal/update"
)

const csrfCookieName = "omo_csrf"

type Config struct {
	StaticFS      fs.FS
	Bootstrap     *bootstrap.Service
	Auth          *auth.Service
	Store         stateStore
	SingBox       *singbox.Detector
	Profiles      *protocol.Registry
	ConfigGen     *configgen.Service
	Subscriptions *subscription.Service
	Diagnostics   *diagnostics.Service
	Pairing       *pairing.Service
	Backup        *backup.Service
	Audit         *audit.Service
	Update        *update.Service
	Version       string
}

type settingsStore interface {
	GetSetting(ctx context.Context, key string) (string, bool, error)
	SetSetting(ctx context.Context, key string, value string) error
	DeleteSetting(ctx context.Context, key string) error
}

type stateStore interface {
	settingsStore
	AdminCount(ctx context.Context) (int, error)
	EnsureServiceProfile(ctx context.Context, profileID string, version string, displayName string, expertProtocol string) error
	CreateServiceInstance(ctx context.Context, profileID string, displayName string, listenPort int, status string, configVersion string) (store.ServiceInstance, error)
	ListServiceInstances(ctx context.Context) ([]store.ServiceInstance, error)
	UpdateServiceInstance(ctx context.Context, id string, displayName *string, listenPort *int, status *string, configVersion *string) (*store.ServiceInstance, error)
}

type Envelope struct {
	Success   bool      `json:"success"`
	Data      any       `json:"data"`
	Error     *APIError `json:"error"`
	RequestID string    `json:"requestId"`
}

type APIError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details"`
}

type HealthResponse struct {
	Status    string `json:"status"`
	Service   string `json:"service"`
	Version   string `json:"version"`
	Timestamp string `json:"timestamp"`
}

func NewRouter(cfg Config) http.Handler {
	appVersion := strings.TrimSpace(cfg.Version)
	if appVersion == "" {
		appVersion = "development"
	}
	router := chi.NewRouter()
	router.Use(requestIDMiddleware)
	router.Use(csrfMiddleware)
	if cfg.Store != nil {
		router.Use(panelAccessMiddleware(cfg.Store))
	}

	router.Get("/api/security/csrf", func(w http.ResponseWriter, r *http.Request) {
		if !ensureCSRFCookie(w, r) {
			respondError(w, r, http.StatusInternalServerError, "CSRF_TOKEN_PREPARE_FAILED", "Security check preparation failed. Please refresh and retry.", nil)
			return
		}
		respondJSON(w, http.StatusOK, Envelope{
			Success:   true,
			Data:      map[string]bool{"csrfReady": true},
			Error:     nil,
			RequestID: requestIDFrom(r),
		})
	})
	router.Get("/api/system/health", func(w http.ResponseWriter, r *http.Request) {
		respondOK(w, r, HealthResponse{
			Status:    "ok",
			Service:   "omo",
			Version:   appVersion,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		})
	})
	router.Get("/api/system/overview", func(w http.ResponseWriter, r *http.Request) {
		result, err := systemOverview(r.Context(), cfg, appVersion)
		if err != nil {
			respondError(w, r, http.StatusInternalServerError, "SYSTEM_OVERVIEW_FAILED", "Unable to read system overview.", nil)
			return
		}
		respondOK(w, r, result)
	})
	router.Get("/api/core/singbox/status", func(w http.ResponseWriter, r *http.Request) {
		detector := cfg.SingBox
		if detector == nil {
			detector = singbox.NewDetector(singbox.Options{})
		}
		status, err := detector.Status(r.Context())
		if err != nil {
			respondError(w, r, http.StatusInternalServerError, "SINGBOX_STATUS_FAILED", "Unable to read access core status. Please retry later.", nil)
			return
		}
		respondOK(w, r, status)
	})
	router.Get("/api/services/profiles", func(w http.ResponseWriter, r *http.Request) {
		registry := cfg.Profiles
		if registry == nil {
			var err error
			registry, err = protocol.DefaultRegistry()
			if err != nil {
				respondError(w, r, http.StatusInternalServerError, "SERVICE_PROFILES_UNAVAILABLE", "Service profile templates are unavailable. Please retry later.", nil)
				return
			}
		}
		respondOK(w, r, map[string]any{"profiles": registry.List()})
	})
	router.Get("/api/services", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Store == nil {
			respondError(w, r, http.StatusServiceUnavailable, "SERVICES_UNAVAILABLE", "Service management is unavailable.", nil)
			return
		}
		registry, err := serviceRegistry(cfg.Profiles)
		if err != nil {
			respondError(w, r, http.StatusInternalServerError, "SERVICE_PROFILES_UNAVAILABLE", "Service profile templates are unavailable. Please retry later.", nil)
			return
		}
		if err := ensureStoredProfiles(r.Context(), cfg.Store, registry); err != nil {
			respondError(w, r, http.StatusInternalServerError, "SERVICE_PROFILE_SYNC_FAILED", "Service profile metadata could not be prepared.", nil)
			return
		}
		services, err := cfg.Store.ListServiceInstances(r.Context())
		if err != nil {
			respondError(w, r, http.StatusInternalServerError, "SERVICES_LIST_FAILED", "Unable to list managed access services.", nil)
			return
		}
		respondOK(w, r, map[string]any{"services": services, "profiles": registry.List()})
	})
	router.Post("/api/services", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Store == nil {
			respondError(w, r, http.StatusServiceUnavailable, "SERVICES_UNAVAILABLE", "Service management is unavailable.", nil)
			return
		}
		var req serviceCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is not valid JSON.", nil)
			return
		}
		registry, err := serviceRegistry(cfg.Profiles)
		if err != nil {
			respondError(w, r, http.StatusInternalServerError, "SERVICE_PROFILES_UNAVAILABLE", "Service profile templates are unavailable. Please retry later.", nil)
			return
		}
		profile, err := registry.Get(strings.TrimSpace(req.ProfileID))
		if err != nil {
			respondError(w, r, http.StatusNotFound, "SERVICE_PROFILE_NOT_FOUND", "Service profile was not found.", nil)
			return
		}
		if err := ensureStoredProfiles(r.Context(), cfg.Store, registry); err != nil {
			respondError(w, r, http.StatusInternalServerError, "SERVICE_PROFILE_SYNC_FAILED", "Service profile metadata could not be prepared.", nil)
			return
		}
		status := strings.TrimSpace(req.Status)
		if status == "" {
			status = "planned"
		}
		if !validServiceStatus(status) || req.ListenPort < 0 || req.ListenPort > 65535 {
			respondError(w, r, http.StatusBadRequest, "INVALID_SERVICE_INPUT", "Service input is invalid.", nil)
			return
		}
		name := strings.TrimSpace(req.DisplayName)
		if name == "" {
			name = profile.DisplayName
		}
		if len(name) > 80 {
			respondError(w, r, http.StatusBadRequest, "INVALID_SERVICE_INPUT", "Service input is invalid.", nil)
			return
		}
		service, err := cfg.Store.CreateServiceInstance(r.Context(), profile.ID, name, req.ListenPort, status, profile.Version)
		if err != nil {
			respondError(w, r, http.StatusInternalServerError, "SERVICE_CREATE_FAILED", "Managed access service could not be created.", nil)
			return
		}
		respondJSON(w, http.StatusCreated, Envelope{
			Success:   true,
			Data:      map[string]any{"service": service},
			Error:     nil,
			RequestID: requestIDFrom(r),
		})
	})
	router.Patch("/api/services/{id}", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Store == nil {
			respondError(w, r, http.StatusServiceUnavailable, "SERVICES_UNAVAILABLE", "Service management is unavailable.", nil)
			return
		}
		var req serviceUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is not valid JSON.", nil)
			return
		}
		if req.DisplayName != nil {
			name := strings.TrimSpace(*req.DisplayName)
			if name == "" || len(name) > 80 {
				respondError(w, r, http.StatusBadRequest, "INVALID_SERVICE_INPUT", "Service input is invalid.", nil)
				return
			}
		}
		if req.Status != nil && !validServiceStatus(strings.TrimSpace(*req.Status)) {
			respondError(w, r, http.StatusBadRequest, "INVALID_SERVICE_INPUT", "Service input is invalid.", nil)
			return
		}
		if req.ListenPort != nil && (*req.ListenPort < 0 || *req.ListenPort > 65535) {
			respondError(w, r, http.StatusBadRequest, "INVALID_SERVICE_INPUT", "Service input is invalid.", nil)
			return
		}
		service, err := cfg.Store.UpdateServiceInstance(r.Context(), chi.URLParam(r, "id"), req.DisplayName, req.ListenPort, req.Status, req.ConfigVersion)
		if err != nil {
			respondError(w, r, http.StatusInternalServerError, "SERVICE_UPDATE_FAILED", "Managed access service could not be updated.", nil)
			return
		}
		if service == nil {
			respondError(w, r, http.StatusNotFound, "SERVICE_NOT_FOUND", "Managed access service was not found.", nil)
			return
		}
		respondOK(w, r, map[string]any{"service": service})
	})
	router.Post("/api/services/{id}/apply", func(w http.ResponseWriter, r *http.Request) {
		if cfg.ConfigGen == nil {
			respondError(w, r, http.StatusServiceUnavailable, "SERVICE_CONFIG_UNAVAILABLE", "Service configuration management is unavailable.", nil)
			return
		}
		result, err := cfg.ConfigGen.Apply(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			writeServiceConfigError(w, r, err)
			return
		}
		respondJSON(w, http.StatusAccepted, Envelope{
			Success:   true,
			Data:      result,
			Error:     nil,
			RequestID: requestIDFrom(r),
		})
	})
	router.Post("/api/services/{id}/rollback", func(w http.ResponseWriter, r *http.Request) {
		if cfg.ConfigGen == nil {
			respondError(w, r, http.StatusServiceUnavailable, "SERVICE_CONFIG_UNAVAILABLE", "Service configuration management is unavailable.", nil)
			return
		}
		result, err := cfg.ConfigGen.Rollback(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			writeServiceConfigError(w, r, err)
			return
		}
		respondJSON(w, http.StatusAccepted, Envelope{
			Success:   true,
			Data:      result,
			Error:     nil,
			RequestID: requestIDFrom(r),
		})
	})
	router.Get("/api/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Subscriptions == nil {
			respondError(w, r, http.StatusServiceUnavailable, "SUBSCRIPTIONS_UNAVAILABLE", "Smart subscription management is unavailable.", nil)
			return
		}
		result, err := cfg.Subscriptions.List(r.Context())
		if err != nil {
			writeSubscriptionError(w, r, err)
			return
		}
		respondOK(w, r, result)
	})
	router.Post("/api/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Subscriptions == nil {
			respondError(w, r, http.StatusServiceUnavailable, "SUBSCRIPTIONS_UNAVAILABLE", "Smart subscription management is unavailable.", nil)
			return
		}
		var req subscription.CreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is not valid JSON.", nil)
			return
		}
		result, err := cfg.Subscriptions.Create(r.Context(), req)
		if err != nil {
			writeSubscriptionError(w, r, err)
			return
		}
		respondJSON(w, http.StatusCreated, Envelope{
			Success:   true,
			Data:      result,
			Error:     nil,
			RequestID: requestIDFrom(r),
		})
	})
	router.Post("/api/subscriptions/{id}/rotate", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Subscriptions == nil {
			respondError(w, r, http.StatusServiceUnavailable, "SUBSCRIPTIONS_UNAVAILABLE", "Smart subscription management is unavailable.", nil)
			return
		}
		result, err := cfg.Subscriptions.Rotate(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			writeSubscriptionError(w, r, err)
			return
		}
		respondOK(w, r, result)
	})
	router.Patch("/api/subscriptions/{id}", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Subscriptions == nil {
			respondError(w, r, http.StatusServiceUnavailable, "SUBSCRIPTIONS_UNAVAILABLE", "Smart subscription management is unavailable.", nil)
			return
		}
		var req subscription.UpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is not valid JSON.", nil)
			return
		}
		result, err := cfg.Subscriptions.Update(r.Context(), chi.URLParam(r, "id"), req)
		if err != nil {
			writeSubscriptionError(w, r, err)
			return
		}
		respondOK(w, r, result)
	})
	router.Delete("/api/subscriptions/{id}", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Subscriptions == nil {
			respondError(w, r, http.StatusServiceUnavailable, "SUBSCRIPTIONS_UNAVAILABLE", "Smart subscription management is unavailable.", nil)
			return
		}
		result, err := cfg.Subscriptions.Delete(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			writeSubscriptionError(w, r, err)
			return
		}
		respondOK(w, r, result)
	})
	router.Post("/api/diagnostics/run", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Diagnostics == nil {
			respondError(w, r, http.StatusServiceUnavailable, "DIAGNOSTICS_UNAVAILABLE", "Server checkup is unavailable.", nil)
			return
		}
		result, err := cfg.Diagnostics.Run(r.Context())
		if err != nil {
			respondError(w, r, http.StatusInternalServerError, "DIAGNOSTICS_RUN_FAILED", "Server checkup failed. Please retry later.", nil)
			return
		}
		respondJSON(w, http.StatusAccepted, Envelope{
			Success:   true,
			Data:      result,
			Error:     nil,
			RequestID: requestIDFrom(r),
		})
	})
	router.Get("/api/diagnostics/latest", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Diagnostics == nil {
			respondError(w, r, http.StatusServiceUnavailable, "DIAGNOSTICS_UNAVAILABLE", "Server checkup is unavailable.", nil)
			return
		}
		result, err := cfg.Diagnostics.Latest(r.Context())
		if err != nil {
			respondError(w, r, http.StatusInternalServerError, "DIAGNOSTICS_LATEST_FAILED", "Unable to read the latest server checkup.", nil)
			return
		}
		respondOK(w, r, result)
	})
	router.Get("/api/diagnostics/events", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Diagnostics == nil {
			respondError(w, r, http.StatusServiceUnavailable, "DIAGNOSTICS_UNAVAILABLE", "Server checkup is unavailable.", nil)
			return
		}
		streamDiagnosticEvents(w, r, cfg.Diagnostics)
	})
	router.Get("/api/settings", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Store == nil {
			respondError(w, r, http.StatusServiceUnavailable, "SETTINGS_UNAVAILABLE", "Settings are unavailable.", nil)
			return
		}
		result, err := settings.LoadDiagnosticsExternalProvider(r.Context(), cfg.Store)
		if err != nil {
			respondError(w, r, http.StatusInternalServerError, "SETTINGS_READ_FAILED", "Unable to read settings.", nil)
			return
		}
		manifestURL, _, err := cfg.Store.GetSetting(r.Context(), update.ManifestURLSettingKey)
		if err != nil {
			respondError(w, r, http.StatusInternalServerError, "SETTINGS_READ_FAILED", "Unable to read settings.", nil)
			return
		}
		respondOK(w, r, map[string]any{"diagnosticsExternalProvider": result, "updateManifestUrl": strings.TrimSpace(manifestURL)})
	})
	router.Patch("/api/settings", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Store == nil {
			respondError(w, r, http.StatusServiceUnavailable, "SETTINGS_UNAVAILABLE", "Settings are unavailable.", nil)
			return
		}
		var req struct {
			DiagnosticsExternalProvider *settings.DiagnosticsExternalProviderUpdate `json:"diagnosticsExternalProvider"`
			UpdateManifestURL           *string                                     `json:"updateManifestUrl"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is not valid JSON.", nil)
			return
		}
		if req.DiagnosticsExternalProvider == nil && req.UpdateManifestURL == nil {
			respondError(w, r, http.StatusBadRequest, "INVALID_SETTINGS_INPUT", "Settings input is invalid.", nil)
			return
		}
		var result settings.DiagnosticsExternalProviderSettings
		if req.DiagnosticsExternalProvider != nil {
			var err error
			result, err = settings.SaveDiagnosticsExternalProvider(r.Context(), cfg.Store, *req.DiagnosticsExternalProvider)
			if err != nil {
				if errors.Is(err, settings.ErrInvalidDiagnosticsProvider) {
					respondError(w, r, http.StatusBadRequest, "INVALID_DIAGNOSTICS_PROVIDER", "Optional provider settings are invalid.", nil)
					return
				}
				respondError(w, r, http.StatusInternalServerError, "SETTINGS_SAVE_FAILED", "Unable to save settings.", nil)
				return
			}
		} else {
			var err error
			result, err = settings.LoadDiagnosticsExternalProvider(r.Context(), cfg.Store)
			if err != nil {
				respondError(w, r, http.StatusInternalServerError, "SETTINGS_READ_FAILED", "Unable to read settings.", nil)
				return
			}
		}
		if req.UpdateManifestURL != nil {
			if err := saveUpdateManifestURL(r.Context(), cfg.Store, *req.UpdateManifestURL); err != nil {
				writeUpdateError(w, r, err)
				return
			}
		}
		manifestURL, _, err := cfg.Store.GetSetting(r.Context(), update.ManifestURLSettingKey)
		if err != nil {
			respondError(w, r, http.StatusInternalServerError, "SETTINGS_READ_FAILED", "Unable to read settings.", nil)
			return
		}
		respondOK(w, r, map[string]any{"diagnosticsExternalProvider": result, "updateManifestUrl": strings.TrimSpace(manifestURL)})
	})
	router.Post("/api/pairing/code", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Pairing == nil {
			respondError(w, r, http.StatusServiceUnavailable, "PAIRING_UNAVAILABLE", "Cascade pairing is unavailable.", nil)
			return
		}
		var req pairing.CreateCodeRequest
		if r.Body != nil {
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
				respondError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is not valid JSON.", nil)
				return
			}
		}
		result, err := cfg.Pairing.CreateCode(r.Context(), req)
		if err != nil {
			writePairingError(w, r, err)
			return
		}
		respondJSON(w, http.StatusCreated, Envelope{
			Success:   true,
			Data:      result,
			Error:     nil,
			RequestID: requestIDFrom(r),
		})
	})
	router.Post("/api/pairing/accept", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Pairing == nil {
			respondError(w, r, http.StatusServiceUnavailable, "PAIRING_UNAVAILABLE", "Cascade pairing is unavailable.", nil)
			return
		}
		var req pairing.AcceptRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is not valid JSON.", nil)
			return
		}
		result, err := cfg.Pairing.Accept(r.Context(), req)
		if err != nil {
			writePairingError(w, r, err)
			return
		}
		respondJSON(w, http.StatusAccepted, Envelope{
			Success:   true,
			Data:      result,
			Error:     nil,
			RequestID: requestIDFrom(r),
		})
	})
	router.Post("/api/pairing/exchange", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Pairing == nil {
			respondError(w, r, http.StatusServiceUnavailable, "PAIRING_UNAVAILABLE", "Cascade pairing is unavailable.", nil)
			return
		}
		var req pairing.ExchangeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is not valid JSON.", nil)
			return
		}
		result, err := cfg.Pairing.Exchange(r.Context(), req)
		if err != nil {
			writePairingError(w, r, err)
			return
		}
		respondOK(w, r, result)
	})
	router.Get("/api/cascade/nodes", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Pairing == nil {
			respondError(w, r, http.StatusServiceUnavailable, "CASCADE_UNAVAILABLE", "Cascade node management is unavailable.", nil)
			return
		}
		result, err := cfg.Pairing.List(r.Context())
		if err != nil {
			respondError(w, r, http.StatusInternalServerError, "CASCADE_LIST_FAILED", "Unable to list cascade nodes.", nil)
			return
		}
		respondOK(w, r, result)
	})
	router.Patch("/api/cascade/nodes/{id}", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Pairing == nil {
			respondError(w, r, http.StatusServiceUnavailable, "CASCADE_UNAVAILABLE", "Cascade node management is unavailable.", nil)
			return
		}
		var req pairing.UpdateNodeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is not valid JSON.", nil)
			return
		}
		result, err := cfg.Pairing.UpdateNode(r.Context(), chi.URLParam(r, "id"), req)
		if err != nil {
			writePairingError(w, r, err)
			return
		}
		respondOK(w, r, result)
	})
	router.Delete("/api/cascade/nodes/{id}", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Pairing == nil {
			respondError(w, r, http.StatusServiceUnavailable, "CASCADE_UNAVAILABLE", "Cascade node management is unavailable.", nil)
			return
		}
		result, err := cfg.Pairing.DeleteNode(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			writePairingError(w, r, err)
			return
		}
		respondOK(w, r, result)
	})
	router.Post("/api/cascade/pairs/{id}/plan", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Pairing == nil {
			respondError(w, r, http.StatusServiceUnavailable, "CASCADE_UNAVAILABLE", "Cascade node management is unavailable.", nil)
			return
		}
		result, err := cfg.Pairing.PlanConfig(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			writePairingError(w, r, err)
			return
		}
		respondOK(w, r, result)
	})
	router.Post("/api/cascade/pairs/{id}/apply", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Pairing == nil {
			respondError(w, r, http.StatusServiceUnavailable, "CASCADE_UNAVAILABLE", "Cascade node management is unavailable.", nil)
			return
		}
		var req pairing.ApplyConfigRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is not valid JSON.", nil)
			return
		}
		result, err := cfg.Pairing.ApplyConfig(r.Context(), chi.URLParam(r, "id"), req)
		if err != nil {
			writePairingError(w, r, err)
			return
		}
		respondJSON(w, http.StatusAccepted, Envelope{
			Success:   true,
			Data:      result,
			Error:     nil,
			RequestID: requestIDFrom(r),
		})
	})
	router.Post("/api/cascade/health/sample", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Pairing == nil {
			respondError(w, r, http.StatusServiceUnavailable, "CASCADE_UNAVAILABLE", "Cascade node management is unavailable.", nil)
			return
		}
		result, err := cfg.Pairing.SampleHealth(r.Context())
		if err != nil {
			writePairingError(w, r, err)
			return
		}
		respondJSON(w, http.StatusAccepted, Envelope{
			Success:   true,
			Data:      result,
			Error:     nil,
			RequestID: requestIDFrom(r),
		})
	})
	router.Get("/api/backups", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Backup == nil {
			respondError(w, r, http.StatusServiceUnavailable, "BACKUPS_UNAVAILABLE", "Backup management is unavailable.", nil)
			return
		}
		result, err := cfg.Backup.List(r.Context())
		if err != nil {
			writeBackupError(w, r, err)
			return
		}
		respondOK(w, r, result)
	})
	router.Post("/api/backups", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Backup == nil {
			respondError(w, r, http.StatusServiceUnavailable, "BACKUPS_UNAVAILABLE", "Backup management is unavailable.", nil)
			return
		}
		result, err := cfg.Backup.Create(r.Context())
		if err != nil {
			writeBackupError(w, r, err)
			return
		}
		respondJSON(w, http.StatusAccepted, Envelope{
			Success:   true,
			Data:      result,
			Error:     nil,
			RequestID: requestIDFrom(r),
		})
	})
	router.Post("/api/backups/{id}/restore", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Backup == nil {
			respondError(w, r, http.StatusServiceUnavailable, "BACKUPS_UNAVAILABLE", "Backup management is unavailable.", nil)
			return
		}
		var req backup.RestoreRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is not valid JSON.", nil)
			return
		}
		result, err := cfg.Backup.Restore(r.Context(), chi.URLParam(r, "id"), req)
		if err != nil {
			writeBackupError(w, r, err)
			return
		}
		respondJSON(w, http.StatusAccepted, Envelope{
			Success:   true,
			Data:      result,
			Error:     nil,
			RequestID: requestIDFrom(r),
		})
	})
	router.Get("/api/audit", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Audit == nil {
			respondError(w, r, http.StatusServiceUnavailable, "AUDIT_UNAVAILABLE", "Audit log management is unavailable.", nil)
			return
		}
		limit := 100
		if raw := r.URL.Query().Get("limit"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 1 || parsed > 200 {
				respondError(w, r, http.StatusBadRequest, "INVALID_AUDIT_LIMIT", "Audit log limit is invalid.", nil)
				return
			}
			limit = parsed
		}
		result, err := cfg.Audit.List(r.Context(), limit)
		if err != nil {
			respondError(w, r, http.StatusInternalServerError, "AUDIT_LIST_FAILED", "Unable to list audit logs.", nil)
			return
		}
		respondOK(w, r, result)
	})
	router.Get("/api/update/check", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Update == nil {
			respondError(w, r, http.StatusServiceUnavailable, "UPDATE_UNAVAILABLE", "Update management is unavailable.", nil)
			return
		}
		result, err := cfg.Update.Check(r.Context())
		if err != nil {
			writeUpdateError(w, r, err)
			return
		}
		respondOK(w, r, result)
	})
	router.Post("/api/update/apply", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Update == nil {
			respondError(w, r, http.StatusServiceUnavailable, "UPDATE_UNAVAILABLE", "Update management is unavailable.", nil)
			return
		}
		var req update.ApplyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is not valid JSON.", nil)
			return
		}
		result, err := cfg.Update.Apply(r.Context(), req)
		if err != nil {
			writeUpdateError(w, r, err)
			return
		}
		respondJSON(w, http.StatusAccepted, Envelope{
			Success:   true,
			Data:      result,
			Error:     nil,
			RequestID: requestIDFrom(r),
		})
	})
	router.Post("/api/update/rollback", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Update == nil {
			respondError(w, r, http.StatusServiceUnavailable, "UPDATE_UNAVAILABLE", "Update management is unavailable.", nil)
			return
		}
		var req update.RollbackRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is not valid JSON.", nil)
			return
		}
		result, err := cfg.Update.Rollback(r.Context(), req)
		if err != nil {
			writeUpdateError(w, r, err)
			return
		}
		respondJSON(w, http.StatusAccepted, Envelope{
			Success:   true,
			Data:      result,
			Error:     nil,
			RequestID: requestIDFrom(r),
		})
	})

	router.Get("/api/bootstrap/status", func(w http.ResponseWriter, r *http.Request) {
		status, err := cfg.Bootstrap.Status(r.Context())
		if err != nil {
			respondError(w, r, http.StatusInternalServerError, "BOOTSTRAP_STATUS_FAILED", "Unable to read initialization status. Please retry later.", nil)
			return
		}
		respondOK(w, r, status)
	})
	router.Post("/api/bootstrap/start", func(w http.ResponseWriter, r *http.Request) {
		var req bootstrap.StartRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is not valid JSON.", nil)
			return
		}

		result, err := cfg.Bootstrap.Start(r.Context(), req)
		if err != nil {
			writeBootstrapError(w, r, err)
			return
		}

		writeSessionCookie(w, r, result.SessionToken, result.SessionExpiresAt)
		respondJSON(w, http.StatusAccepted, Envelope{
			Success:   true,
			Data:      result,
			Error:     nil,
			RequestID: requestIDFrom(r),
		})
	})
	router.Get("/api/bootstrap/events", func(w http.ResponseWriter, r *http.Request) {
		cfg.Bootstrap.StreamEvents(w, r)
	})
	router.Post("/api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		var req auth.LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is not valid JSON.", nil)
			return
		}

		result, err := cfg.Auth.Login(r.Context(), req)
		if err != nil {
			writeAuthError(w, r, err)
			return
		}

		writeSessionCookie(w, r, result.SessionToken, result.SessionExpiresAt)
		respondOK(w, r, map[string]any{"admin": result.Admin})
	})
	router.Post("/api/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(auth.SessionCookieName)
		if err == nil {
			_ = cfg.Auth.Logout(r.Context(), cookie.Value)
		}
		clearSessionCookie(w, r)
		respondOK(w, r, map[string]bool{"loggedOut": true})
	})
	router.Get("/api/auth/me", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(auth.SessionCookieName)
		if err != nil {
			respondOK(w, r, map[string]any{"authenticated": false})
			return
		}
		admin, err := cfg.Auth.AdminForToken(r.Context(), cookie.Value)
		if err != nil {
			respondError(w, r, http.StatusInternalServerError, "SESSION_LOOKUP_FAILED", "Unable to read the administrator session. Please sign in again.", nil)
			return
		}
		if admin == nil {
			respondOK(w, r, map[string]any{"authenticated": false})
			return
		}
		respondOK(w, r, map[string]any{"authenticated": true, "admin": admin})
	})
	router.Get("/s/{token}", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Subscriptions == nil {
			http.NotFound(w, r)
			return
		}
		result, err := cfg.Subscriptions.PublicContent(r.Context(), subscription.PublicRequest{
			Token:      chi.URLParam(r, "token"),
			Format:     r.URL.Query().Get("format"),
			ClientHint: r.UserAgent(),
			RemoteAddr: r.RemoteAddr,
			BaseURL:    subscription.BaseURLFromRequest(r),
		})
		if err != nil {
			if errors.Is(err, subscription.ErrSubscriptionNotFound) {
				http.NotFound(w, r)
				return
			}
			respondError(w, r, http.StatusInternalServerError, "SUBSCRIPTION_DISTRIBUTION_FAILED", "Subscription distribution failed.", nil)
			return
		}
		w.Header().Set("Content-Type", result.ContentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(result.Body)
	})

	router.NotFound(spaHandler(cfg.StaticFS))
	return router
}

func panelAccessMiddleware(appStore settingsStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/init") || strings.HasPrefix(r.URL.Path, "/login") || strings.HasPrefix(r.URL.Path, "/_app/") {
				next.ServeHTTP(w, r)
				return
			}

			policy, err := settings.LoadPanelAccessPolicy(r.Context(), appStore)
			if err != nil || !policy.HTTPSOnly {
				next.ServeHTTP(w, r)
				return
			}

			if !requestIsHTTPS(r) || !settings.HostMatchesDomain(requestHost(r), policy.Domain) {
				target := "https://" + policy.Domain + r.URL.RequestURI()
				http.Redirect(w, r, target, http.StatusTemporaryRedirect)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func requestIsHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if !trustedForwardedRequest(r) {
		return false
	}
	return strings.EqualFold(firstForwardedValue(r.Header.Get("X-Forwarded-Proto")), "https")
}

func requestHost(r *http.Request) string {
	if trustedForwardedRequest(r) {
		if host := strings.TrimSpace(firstForwardedValue(r.Header.Get("X-Forwarded-Host"))); host != "" {
			return host
		}
	}
	return r.Host
}

func trustedForwardedRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(strings.TrimSpace(host))
	return ip != nil && ip.IsLoopback()
}

func firstForwardedValue(value string) string {
	first, _, _ := strings.Cut(value, ",")
	return strings.TrimSpace(first)
}

func respondOK(w http.ResponseWriter, r *http.Request, data any) {
	_ = ensureCSRFCookie(w, r)
	respondJSON(w, http.StatusOK, Envelope{
		Success:   true,
		Data:      data,
		Error:     nil,
		RequestID: requestIDFrom(r),
	})
}

type serviceCreateRequest struct {
	ProfileID   string `json:"profileId"`
	DisplayName string `json:"displayName"`
	ListenPort  int    `json:"listenPort"`
	Status      string `json:"status"`
}

type serviceUpdateRequest struct {
	DisplayName   *string `json:"displayName"`
	ListenPort    *int    `json:"listenPort"`
	Status        *string `json:"status"`
	ConfigVersion *string `json:"configVersion"`
}

func systemOverview(ctx context.Context, cfg Config, appVersion string) (map[string]any, error) {
	overview := map[string]any{
		"status":    "ok",
		"service":   "omo",
		"version":   appVersion,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	if cfg.Bootstrap != nil {
		status, err := cfg.Bootstrap.Status(ctx)
		if err != nil {
			return nil, err
		}
		overview["bootstrap"] = status
	}
	if cfg.Store != nil {
		admins, err := cfg.Store.AdminCount(ctx)
		if err != nil {
			return nil, err
		}
		services, err := cfg.Store.ListServiceInstances(ctx)
		if err != nil {
			return nil, err
		}
		overview["counts"] = map[string]int{
			"admins":          admins,
			"serviceProfiles": len(profileListOrEmpty(cfg.Profiles)),
			"services":        len(services),
		}
	}
	detector := cfg.SingBox
	if detector == nil {
		detector = singbox.NewDetector(singbox.Options{})
	}
	status, err := detector.Status(ctx)
	if err != nil {
		return nil, err
	}
	overview["core"] = status
	return overview, nil
}

func serviceRegistry(registry *protocol.Registry) (*protocol.Registry, error) {
	if registry != nil {
		return registry, nil
	}
	return protocol.DefaultRegistry()
}

func profileListOrEmpty(registry *protocol.Registry) []protocol.ServiceProfile {
	resolved, err := serviceRegistry(registry)
	if err != nil {
		return nil
	}
	return resolved.List()
}

func ensureStoredProfiles(ctx context.Context, appStore stateStore, registry *protocol.Registry) error {
	for _, profile := range registry.List() {
		if err := appStore.EnsureServiceProfile(ctx, profile.ID, profile.Version, profile.DisplayName, profile.ExpertProtocol); err != nil {
			return err
		}
	}
	return nil
}

func validServiceStatus(status string) bool {
	switch status {
	case "planned", "active", "disabled":
		return true
	default:
		return false
	}
}

func respondJSON(w http.ResponseWriter, status int, body Envelope) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func csrfMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if csrfRequired(r) {
			cookie, err := r.Cookie(csrfCookieName)
			token := r.Header.Get("X-CSRF-Token")
			if err != nil || cookie.Value == "" || token == "" || token != cookie.Value {
				respondError(w, r, http.StatusForbidden, "CSRF_TOKEN_INVALID", "Security check failed. Please refresh the page and retry.", nil)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func csrfRequired(r *http.Request) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return false
	default:
		if r.URL.Path == "/api/pairing/exchange" {
			return false
		}
		return strings.HasPrefix(r.URL.Path, "/api/")
	}
}

func ensureCSRFCookie(w http.ResponseWriter, r *http.Request) bool {
	if cookie, err := r.Cookie(csrfCookieName); err == nil && cookie.Value != "" {
		return true
	}
	token, err := auth.GenerateToken(32)
	if err != nil {
		return false
	}
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestIsHTTPS(r),
	})
	return true
}

func writeSessionCookie(w http.ResponseWriter, r *http.Request, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestIsHTTPS(r),
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestIsHTTPS(r),
	})
}

func respondError(w http.ResponseWriter, r *http.Request, status int, code string, message string, details map[string]any) {
	if details == nil {
		details = map[string]any{}
	}
	respondJSON(w, status, Envelope{
		Success: false,
		Data:    nil,
		Error: &APIError{
			Code:    code,
			Message: message,
			Details: details,
		},
		RequestID: requestIDFrom(r),
	})
}

func writeBootstrapError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, bootstrap.ErrAlreadyInitialized):
		respondError(w, r, http.StatusConflict, "BOOTSTRAP_ALREADY_INITIALIZED", "Initialization is already complete and cannot be repeated.", nil)
	case errors.Is(err, bootstrap.ErrInvalidToken):
		respondError(w, r, http.StatusUnauthorized, "INVALID_INIT_TOKEN", "Initialization link is invalid. Check the server terminal for the latest link.", nil)
	case errors.Is(err, bootstrap.ErrTokenExpired):
		respondError(w, r, http.StatusUnauthorized, "INIT_TOKEN_EXPIRED", "Initialization link has expired. Restart the service to generate a new link.", nil)
	case errors.Is(err, bootstrap.ErrPasswordMismatch):
		respondError(w, r, http.StatusBadRequest, "PASSWORD_CONFIRM_MISMATCH", "The administrator passwords do not match.", nil)
	case errors.Is(err, auth.ErrWeakPassword):
		respondError(w, r, http.StatusBadRequest, "WEAK_ADMIN_PASSWORD", "Administrator password must be at least 8 characters and include letters and numbers.", nil)
	case errors.Is(err, bootstrap.ErrInvalidInput):
		respondError(w, r, http.StatusBadRequest, "INVALID_BOOTSTRAP_INPUT", "Initialization information is incomplete or invalid.", nil)
	case errors.Is(err, bootstrap.ErrJobRunning):
		respondError(w, r, http.StatusConflict, "BOOTSTRAP_JOB_RUNNING", "An initialization job is already running. Check progress shortly.", nil)
	case errors.Is(err, bootstrap.ErrRetryRequired):
		respondError(w, r, http.StatusConflict, "BOOTSTRAP_RETRY_REQUIRED", "The previous initialization attempt failed. Confirm retry before starting again.", nil)
	case strings.Contains(err.Error(), "CADDY_UNAVAILABLE"):
		respondError(w, r, http.StatusServiceUnavailable, "CADDY_UNAVAILABLE", "Caddy is unavailable. The temporary initialization entry remains active; repair Caddy and retry HTTPS entry configuration.", nil)
	case strings.Contains(err.Error(), "TLS_CERTIFICATE_NOT_READY"):
		respondError(w, r, http.StatusServiceUnavailable, "TLS_CERTIFICATE_NOT_READY", "The HTTPS entry was configured, but the TLS certificate is not ready yet. Keep the temporary initialization entry open, confirm TCP 80/443 are reachable, and retry.", nil)
	case strings.Contains(err.Error(), "DOMAIN_NOT_RESOLVED") ||
		strings.Contains(err.Error(), "DOMAIN_LOOKUP_FAILED") ||
		strings.Contains(err.Error(), "no such host"):
		respondError(w, r, http.StatusBadRequest, "DOMAIN_NOT_RESOLVED", "Domain DNS does not currently resolve to this server. Check DNS records and retry.", nil)
	case strings.Contains(err.Error(), "port unavailable"):
		respondError(w, r, http.StatusBadRequest, "PORT_UNAVAILABLE", "Ports 80 or 443 are unavailable. Release the required ports and retry.", nil)
	case strings.Contains(err.Error(), "caddy ") ||
		strings.Contains(err.Error(), "Caddyfile") ||
		strings.Contains(err.Error(), "certificate"):
		respondError(w, r, http.StatusBadGateway, "CADDY_CONFIG_FAILED", "Caddy entry configuration failed. Check the initialization event log for the exact error and retry.", nil)
	default:
		respondError(w, r, http.StatusInternalServerError, "BOOTSTRAP_START_FAILED", "Initialization failed to start. Check service logs and retry.", nil)
	}
}

func writeServiceConfigError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, configgen.ErrInvalidProfile):
		respondError(w, r, http.StatusNotFound, "SERVICE_PROFILE_NOT_FOUND", "Service profile was not found.", nil)
	case errors.Is(err, configgen.ErrNoRollback):
		respondError(w, r, http.StatusConflict, "SERVICE_ROLLBACK_UNAVAILABLE", "No previous service configuration is available for rollback.", nil)
	case errors.Is(err, configgen.ErrConfigWrite):
		respondError(w, r, http.StatusInternalServerError, "SERVICE_CONFIG_WRITE_FAILED", "Service configuration could not be written. Confirm the OMO data directory is writable and retry.", nil)
	default:
		respondError(w, r, http.StatusInternalServerError, "SERVICE_CONFIG_FAILED", "Service configuration operation failed; previous configuration was preserved or restored when possible.", nil)
	}
}

func writeSubscriptionError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, subscription.ErrInvalidInput):
		respondError(w, r, http.StatusBadRequest, "INVALID_SUBSCRIPTION_INPUT", "Subscription input is invalid.", nil)
	case errors.Is(err, subscription.ErrSubscriptionNotFound):
		respondError(w, r, http.StatusNotFound, "SUBSCRIPTION_NOT_FOUND", "Subscription was not found.", nil)
	default:
		respondError(w, r, http.StatusInternalServerError, "SUBSCRIPTION_FAILED", "Smart subscription operation failed.", nil)
	}
}

func writePairingError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, pairing.ErrInvalidInput):
		respondError(w, r, http.StatusBadRequest, "INVALID_CASCADE_INPUT", "Cascade pairing input is invalid.", nil)
	case errors.Is(err, pairing.ErrPairingCodeNotFound):
		respondError(w, r, http.StatusNotFound, "PAIRING_CODE_NOT_FOUND", "Cascade pairing code is unavailable or expired.", nil)
	case errors.Is(err, pairing.ErrCascadeNodeNotFound):
		respondError(w, r, http.StatusNotFound, "CASCADE_NODE_NOT_FOUND", "Cascade node was not found.", nil)
	case errors.Is(err, pairing.ErrPeerExchangeFailed):
		respondError(w, r, http.StatusBadGateway, "CASCADE_PEER_EXCHANGE_FAILED", "Peer cascade exchange failed. Confirm the remote OMO HTTPS entry and retry.", nil)
	case errors.Is(err, pairing.ErrCascadePairNotFound):
		respondError(w, r, http.StatusNotFound, "CASCADE_PAIR_NOT_FOUND", "Cascade link was not found.", nil)
	case errors.Is(err, pairing.ErrConfirmationRequired):
		respondError(w, r, http.StatusBadRequest, "CASCADE_CONFIRMATION_REQUIRED", "Operator confirmation is required before applying cascade configuration.", nil)
	default:
		respondError(w, r, http.StatusInternalServerError, "CASCADE_OPERATION_FAILED", "Cascade operation failed. Please retry later.", nil)
	}
}

func writeBackupError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, backup.ErrBackupNotFound):
		respondError(w, r, http.StatusNotFound, "BACKUP_NOT_FOUND", "Backup was not found or is not ready.", nil)
	case errors.Is(err, backup.ErrConfirmationRequired):
		respondError(w, r, http.StatusBadRequest, "BACKUP_RESTORE_CONFIRMATION_REQUIRED", "Operator confirmation is required before restoring a backup.", nil)
	case errors.Is(err, backup.ErrInvalidBackup):
		respondError(w, r, http.StatusConflict, "BACKUP_INVALID", "Backup archive verification failed.", nil)
	default:
		respondError(w, r, http.StatusInternalServerError, "BACKUP_OPERATION_FAILED", "Backup operation failed. Please retry later.", nil)
	}
}

func writeUpdateError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, update.ErrInvalidManifestURL):
		respondError(w, r, http.StatusBadRequest, "UPDATE_MANIFEST_INVALID", "Update manifest URL must be a valid HTTPS endpoint.", nil)
	case errors.Is(err, update.ErrConfirmationRequired):
		respondError(w, r, http.StatusBadRequest, "UPDATE_CONFIRMATION_REQUIRED", "Operator confirmation is required before applying or rolling back an update.", nil)
	case errors.Is(err, update.ErrUpdateUnavailable):
		respondError(w, r, http.StatusConflict, "UPDATE_UNAVAILABLE", "No applicable update artifact is available for this server.", nil)
	case errors.Is(err, update.ErrArtifactVerificationFailed):
		respondError(w, r, http.StatusConflict, "UPDATE_VERIFICATION_FAILED", "Update artifact verification failed.", nil)
	case errors.Is(err, update.ErrNoRollback):
		respondError(w, r, http.StatusConflict, "UPDATE_ROLLBACK_UNAVAILABLE", "No previous update binary is available for rollback.", nil)
	default:
		respondError(w, r, http.StatusBadGateway, "UPDATE_CHECK_FAILED", "Update manifest check failed. Please verify the configured release channel.", nil)
	}
}

func saveUpdateManifestURL(ctx context.Context, appStore settingsStore, manifestURL string) error {
	manifestURL = strings.TrimSpace(manifestURL)
	if manifestURL == "" {
		return appStore.DeleteSetting(ctx, update.ManifestURLSettingKey)
	}
	parsed, err := url.Parse(manifestURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return update.ErrInvalidManifestURL
	}
	return appStore.SetSetting(ctx, update.ManifestURLSettingKey, manifestURL)
}

func writeAuthError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		respondError(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Administrator username or password is incorrect.", nil)
	case errors.Is(err, auth.ErrLoginLocked):
		respondError(w, r, http.StatusTooManyRequests, "LOGIN_TEMPORARILY_LOCKED", "Too many failed login attempts. Please retry later.", nil)
	default:
		respondError(w, r, http.StatusInternalServerError, "LOGIN_FAILED", "Login failed. Please retry later.", nil)
	}
}

func streamDiagnosticEvents(w http.ResponseWriter, r *http.Request, service *diagnostics.Service) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming is not supported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	lastID := parseEventID(r)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	fmt.Fprint(w, "retry: 1000\n\n")
	flusher.Flush()

	for {
		events, err := service.Events(r.Context(), lastID)
		if err != nil {
			fmt.Fprintf(w, "event: error\ndata: %s\n\n", `{"message":"Unable to read server checkup events."}`)
			flusher.Flush()
			return
		}
		for _, event := range events {
			payload, _ := json.Marshal(event)
			fmt.Fprintf(w, "id: %d\nevent: diagnostics\ndata: %s\n\n", event.ID, payload)
			lastID = event.ID
		}
		flusher.Flush()

		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
	}
}

func parseEventID(r *http.Request) int64 {
	raw := r.Header.Get("Last-Event-ID")
	if raw == "" {
		raw = r.URL.Query().Get("since")
	}
	id, _ := strconv.ParseInt(raw, 10, 64)
	return id
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-Id")
		if requestID == "" {
			requestID = "req_" + time.Now().UTC().Format("20060102150405.000000000")
		}
		ctx := withRequestID(r.Context(), requestID)
		w.Header().Set("X-Request-Id", requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type requestIDKey struct{}

func withRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, requestID)
}

func requestIDFrom(r *http.Request) string {
	requestID, _ := r.Context().Value(requestIDKey{}).(string)
	if requestID == "" {
		return "req_unknown"
	}
	return requestID
}

func spaHandler(staticFS fs.FS) http.HandlerFunc {
	fileServer := http.FileServer(http.FS(staticFS))
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}

		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "." || name == "" {
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}

		if _, err := fs.Stat(staticFS, name); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}

		htmlName := name + ".html"
		if _, err := fs.Stat(staticFS, htmlName); err == nil {
			r.URL.Path = "/" + htmlName
			fileServer.ServeHTTP(w, r)
			return
		}

		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	}
}
