package api

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"omo/internal/audit"
	"omo/internal/auth"
	"omo/internal/backup"
	"omo/internal/bootstrap"
	"omo/internal/configgen"
	"omo/internal/core/singbox"
	"omo/internal/diagnostics"
	"omo/internal/pairing"
	"omo/internal/store"
	"omo/internal/subscription"
	"omo/internal/update"
)

type fallbackBootstrapHook struct{}

func (fallbackBootstrapHook) Run(context.Context, string) (bootstrap.Phase2Result, error) {
	result := bootstrap.Phase2Result{
		Message:       "Caddy 暂不可用，已保留临时初始化入口，请安装或修复 Caddy 后重试 HTTPS 入口配置。",
		EntryMode:     "temporary_http",
		SecurityState: "degraded",
	}
	return bootstrap.Phase2Result{}, bootstrap.Phase2FallbackError{
		Code:    "CADDY_UNAVAILABLE",
		Message: result.Message,
		Result:  result,
		Cause:   exec.ErrNotFound,
	}
}

func TestHealthEndpoint(t *testing.T) {
	router := testRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/system/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"success":true`) {
		t.Fatalf("expected success envelope, got %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"service":"omo"`) {
		t.Fatalf("expected omo service, got %s", rec.Body.String())
	}
	if cookie := findCookie(rec.Result().Cookies(), csrfCookieName); cookie == nil || cookie.HttpOnly {
		t.Fatalf("expected readable csrf cookie, got %#v", cookie)
	}
}

func TestCSRFEndpointSetsReadableCookie(t *testing.T) {
	router := testRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/security/csrf", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"csrfReady":true`) {
		t.Fatalf("expected csrf ready response, got %s", rec.Body.String())
	}
	cookie := findCookie(rec.Result().Cookies(), csrfCookieName)
	if cookie == nil {
		t.Fatal("expected csrf cookie")
	}
	if cookie.HttpOnly {
		t.Fatalf("expected csrf cookie readable by browser API client, got %#v", cookie)
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("expected SameSite=Lax csrf cookie, got %#v", cookie)
	}
}

func TestSingBoxStatusEndpoint(t *testing.T) {
	router := testRouterWithSingBox(t, singbox.NewDetector(singbox.Options{
		BinaryPath: filepath.Join(t.TempDir(), "missing-sing-box"),
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/core/singbox/status", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"installed":false`) {
		t.Fatalf("expected missing sing-box status, got %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "未检测到 sing-box 接入核心") {
		t.Fatalf("expected user-facing status message, got %s", rec.Body.String())
	}
}

func TestServiceProfilesEndpoint(t *testing.T) {
	router := testRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/services/profiles", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"id":"standard-secure-access"`) {
		t.Fatalf("expected standard profile, got %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"id":"high-throughput-access"`) {
		t.Fatalf("expected high-throughput profile, got %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"id":"broad-compatibility-access"`) {
		t.Fatalf("expected compatibility profile, got %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"id":"lightweight-fallback-access"`) {
		t.Fatalf("expected lightweight fallback profile, got %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"id":"mobile-optimized-access"`) {
		t.Fatalf("expected mobile optimized profile, got %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "bypass") || strings.Contains(rec.Body.String(), "stealth") {
		t.Fatalf("service profile response contains disallowed wording: %s", rec.Body.String())
	}
}

func TestSystemOverviewEndpoint(t *testing.T) {
	router, _ := testRouterWithServiceStore(t)

	req := httptest.NewRequest(http.MethodGet, "/api/system/overview", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected overview 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"service":"omo"`) || !strings.Contains(rec.Body.String(), `"core"`) {
		t.Fatalf("expected system overview response, got %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"serviceProfiles":5`) {
		t.Fatalf("expected service profile count, got %s", rec.Body.String())
	}
}

func TestServiceInstanceLifecycleEndpoints(t *testing.T) {
	router, _ := testRouterWithServiceStore(t)

	emptyListReq := httptest.NewRequest(http.MethodGet, "/api/services", nil)
	emptyListRec := httptest.NewRecorder()
	router.ServeHTTP(emptyListRec, emptyListReq)
	if emptyListRec.Code != http.StatusOK || !strings.Contains(emptyListRec.Body.String(), `"services":[]`) {
		t.Fatalf("expected empty services array, got %d: %s", emptyListRec.Code, emptyListRec.Body.String())
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/services", strings.NewReader(`{"profileId":"standard-secure-access","displayName":"Team access","listenPort":0}`))
	addCSRF(createReq)
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected create 201, got %d: %s", createRec.Code, createRec.Body.String())
	}
	if !strings.Contains(createRec.Body.String(), `"profileId":"standard-secure-access"`) || !strings.Contains(createRec.Body.String(), `"displayName":"Team access"`) {
		t.Fatalf("expected created service instance, got %s", createRec.Body.String())
	}

	var createPayload Envelope
	if err := json.Unmarshal(createRec.Body.Bytes(), &createPayload); err != nil {
		t.Fatalf("decode service create response: %v", err)
	}
	createData := createPayload.Data.(map[string]any)
	serviceData := createData["service"].(map[string]any)
	serviceID, _ := serviceData["id"].(string)
	if serviceID == "" {
		t.Fatalf("expected service id, got %#v", createPayload.Data)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/services", nil)
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK || !strings.Contains(listRec.Body.String(), serviceID) || !strings.Contains(listRec.Body.String(), `"profiles"`) {
		t.Fatalf("expected service list with profiles, got %d: %s", listRec.Code, listRec.Body.String())
	}

	updateReq := httptest.NewRequest(http.MethodPatch, "/api/services/"+serviceID, strings.NewReader(`{"displayName":"Disabled team access","status":"disabled","listenPort":8443}`))
	addCSRF(updateReq)
	updateRec := httptest.NewRecorder()
	router.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK || !strings.Contains(updateRec.Body.String(), `"status":"disabled"`) || !strings.Contains(updateRec.Body.String(), `"listenPort":8443`) {
		t.Fatalf("expected service update, got %d: %s", updateRec.Code, updateRec.Body.String())
	}
}

func TestServiceInstanceEndpointsValidateInput(t *testing.T) {
	router, _ := testRouterWithServiceStore(t)

	unknownReq := httptest.NewRequest(http.MethodPost, "/api/services", strings.NewReader(`{"profileId":"unknown-profile"}`))
	addCSRF(unknownReq)
	unknownRec := httptest.NewRecorder()
	router.ServeHTTP(unknownRec, unknownReq)
	if unknownRec.Code != http.StatusNotFound || !strings.Contains(unknownRec.Body.String(), `"code":"SERVICE_PROFILE_NOT_FOUND"`) {
		t.Fatalf("expected unknown profile rejection, got %d: %s", unknownRec.Code, unknownRec.Body.String())
	}

	invalidReq := httptest.NewRequest(http.MethodPost, "/api/services", strings.NewReader(`{"profileId":"standard-secure-access","status":"running"}`))
	addCSRF(invalidReq)
	invalidRec := httptest.NewRecorder()
	router.ServeHTTP(invalidRec, invalidReq)
	if invalidRec.Code != http.StatusBadRequest || !strings.Contains(invalidRec.Body.String(), `"code":"INVALID_SERVICE_INPUT"`) {
		t.Fatalf("expected invalid status rejection, got %d: %s", invalidRec.Code, invalidRec.Body.String())
	}
}

func TestServiceConfigApplyAndRollbackEndpoints(t *testing.T) {
	router := testRouterWithConfigGen(t)
	csrf := "test-csrf-token"

	applyReq := httptest.NewRequest(http.MethodPost, "/api/services/standard-secure-access/apply", nil)
	addCSRFWithValue(applyReq, csrf)
	applyRec := httptest.NewRecorder()
	router.ServeHTTP(applyRec, applyReq)

	if applyRec.Code != http.StatusAccepted {
		t.Fatalf("expected apply 202, got %d: %s", applyRec.Code, applyRec.Body.String())
	}
	if !strings.Contains(applyRec.Body.String(), `"kind":"service_config_apply"`) {
		t.Fatalf("expected apply job response, got %s", applyRec.Body.String())
	}

	rollbackReq := httptest.NewRequest(http.MethodPost, "/api/services/standard-secure-access/rollback", nil)
	addCSRFWithValue(rollbackReq, csrf)
	rollbackRec := httptest.NewRecorder()
	router.ServeHTTP(rollbackRec, rollbackReq)

	if rollbackRec.Code != http.StatusAccepted {
		t.Fatalf("expected rollback 202, got %d: %s", rollbackRec.Code, rollbackRec.Body.String())
	}
	if !strings.Contains(rollbackRec.Body.String(), `"rolledBack":true`) {
		t.Fatalf("expected rollback result, got %s", rollbackRec.Body.String())
	}
}

func TestServiceConfigApplyRejectsUnknownProfile(t *testing.T) {
	router := testRouterWithConfigGen(t)

	req := httptest.NewRequest(http.MethodPost, "/api/services/unknown-profile/apply", nil)
	addCSRF(req)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"SERVICE_PROFILE_NOT_FOUND"`) {
		t.Fatalf("expected profile not found error, got %s", rec.Body.String())
	}
}

func TestServiceConfigApplyReportsWriteFailure(t *testing.T) {
	ctx := context.Background()
	appStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = appStore.Close() })
	blocker := filepath.Join(t.TempDir(), "sing-box")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	manager, err := configgen.NewManager(configgen.Options{ConfigPath: filepath.Join(blocker, "config.json")})
	if err != nil {
		t.Fatalf("new config manager: %v", err)
	}
	router := NewRouter(Config{
		StaticFS:  fstest.MapFS{},
		Store:     appStore,
		ConfigGen: configgen.NewService(manager, appStore),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/services/standard-secure-access/apply", nil)
	addCSRF(req)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "SERVICE_CONFIG_WRITE_FAILED") {
		t.Fatalf("expected write failure response, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSubscriptionCreateListRotateAndPublicEndpoint(t *testing.T) {
	router := testRouterWithSubscriptions(t)

	emptyListReq := httptest.NewRequest(http.MethodGet, "/api/subscriptions", nil)
	emptyListRec := httptest.NewRecorder()
	router.ServeHTTP(emptyListRec, emptyListReq)
	if emptyListRec.Code != http.StatusOK || !strings.Contains(emptyListRec.Body.String(), `"subscriptions":[]`) {
		t.Fatalf("expected empty subscriptions array, got %d: %s", emptyListRec.Code, emptyListRec.Body.String())
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/subscriptions", strings.NewReader(`{"name":"Operations devices"}`))
	addCSRF(createReq)
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected create 201, got %d: %s", createRec.Code, createRec.Body.String())
	}
	if !strings.Contains(createRec.Body.String(), `"token":"`) {
		t.Fatalf("expected one-time token in create response, got %s", createRec.Body.String())
	}

	var createPayload Envelope
	if err := json.Unmarshal(createRec.Body.Bytes(), &createPayload); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	raw := createPayload.Data.(map[string]any)
	token, _ := raw["token"].(string)
	subscriptionData, _ := raw["subscription"].(map[string]any)
	id, _ := subscriptionData["id"].(string)
	if token == "" || id == "" {
		t.Fatalf("expected token and id, got %#v", createPayload.Data)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/subscriptions", nil)
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK || !strings.Contains(listRec.Body.String(), `"Operations devices"`) {
		t.Fatalf("expected subscription list, got %d: %s", listRec.Code, listRec.Body.String())
	}

	publicReq := httptest.NewRequest(http.MethodGet, "/s/"+token+"?format=sing-box", nil)
	publicRec := httptest.NewRecorder()
	router.ServeHTTP(publicRec, publicReq)
	if publicRec.Code != http.StatusOK {
		t.Fatalf("expected public subscription 200, got %d: %s", publicRec.Code, publicRec.Body.String())
	}
	if contentType := publicRec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected json subscription content type, got %q", contentType)
	}
	if !strings.Contains(publicRec.Body.String(), `"managedBy": "omo"`) {
		t.Fatalf("expected managed subscription descriptor, got %s", publicRec.Body.String())
	}

	rotateReq := httptest.NewRequest(http.MethodPost, "/api/subscriptions/"+id+"/rotate", nil)
	addCSRF(rotateReq)
	rotateRec := httptest.NewRecorder()
	router.ServeHTTP(rotateRec, rotateReq)
	if rotateRec.Code != http.StatusOK {
		t.Fatalf("expected rotate 200, got %d: %s", rotateRec.Code, rotateRec.Body.String())
	}
	if !strings.Contains(rotateRec.Body.String(), `"token":"`) {
		t.Fatalf("expected rotated token response, got %s", rotateRec.Body.String())
	}
}

func TestPublicSubscriptionQRCodeOutput(t *testing.T) {
	router := testRouterWithSubscriptions(t)

	createReq := httptest.NewRequest(http.MethodPost, "/api/subscriptions", strings.NewReader(`{"name":"QR import"}`))
	addCSRF(createReq)
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected create 201, got %d: %s", createRec.Code, createRec.Body.String())
	}
	var createPayload Envelope
	if err := json.Unmarshal(createRec.Body.Bytes(), &createPayload); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	raw := createPayload.Data.(map[string]any)
	token, _ := raw["token"].(string)

	publicReq := httptest.NewRequest(http.MethodGet, "/s/"+token+"?format=qr", nil)
	publicRec := httptest.NewRecorder()
	router.ServeHTTP(publicRec, publicReq)
	if publicRec.Code != http.StatusOK {
		t.Fatalf("expected qr 200, got %d: %s", publicRec.Code, publicRec.Body.String())
	}
	if contentType := publicRec.Header().Get("Content-Type"); !strings.Contains(contentType, "image/svg+xml") {
		t.Fatalf("expected svg content type, got %q", contentType)
	}
	if !strings.Contains(publicRec.Body.String(), "<svg") {
		t.Fatalf("expected svg body, got %s", publicRec.Body.String())
	}
}

func TestPublicSubscriptionUnknownClientGetsImportPage(t *testing.T) {
	router := testRouterWithSubscriptions(t)

	createReq := httptest.NewRequest(http.MethodPost, "/api/subscriptions", strings.NewReader(`{"name":"Manual import"}`))
	addCSRF(createReq)
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected create 201, got %d: %s", createRec.Code, createRec.Body.String())
	}
	var createPayload Envelope
	if err := json.Unmarshal(createRec.Body.Bytes(), &createPayload); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	raw := createPayload.Data.(map[string]any)
	token, _ := raw["token"].(string)

	publicReq := httptest.NewRequest(http.MethodGet, "/s/"+token, nil)
	publicReq.Header.Set("User-Agent", "UnknownClient/1.0")
	publicRec := httptest.NewRecorder()
	router.ServeHTTP(publicRec, publicReq)
	if publicRec.Code != http.StatusOK {
		t.Fatalf("expected import page 200, got %d: %s", publicRec.Code, publicRec.Body.String())
	}
	if contentType := publicRec.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
		t.Fatalf("expected html content type, got %q", contentType)
	}
	if !strings.Contains(publicRec.Body.String(), "Select an import format") {
		t.Fatalf("expected manual import page, got %s", publicRec.Body.String())
	}
}

func TestDiagnosticsRunLatestAndEventsEndpoints(t *testing.T) {
	router := testRouterWithDiagnostics(t)

	runReq := httptest.NewRequest(http.MethodPost, "/api/diagnostics/run", nil)
	addCSRF(runReq)
	runRec := httptest.NewRecorder()
	router.ServeHTTP(runRec, runReq)
	if runRec.Code != http.StatusAccepted {
		t.Fatalf("expected diagnostics run 202, got %d: %s", runRec.Code, runRec.Body.String())
	}
	if !strings.Contains(runRec.Body.String(), `"kind":"diagnostics"`) {
		t.Fatalf("expected diagnostics job response, got %s", runRec.Body.String())
	}
	if !strings.Contains(runRec.Body.String(), `"checks"`) {
		t.Fatalf("expected diagnostics checks, got %s", runRec.Body.String())
	}

	latestReq := httptest.NewRequest(http.MethodGet, "/api/diagnostics/latest", nil)
	latestRec := httptest.NewRecorder()
	router.ServeHTTP(latestRec, latestReq)
	if latestRec.Code != http.StatusOK {
		t.Fatalf("expected diagnostics latest 200, got %d: %s", latestRec.Code, latestRec.Body.String())
	}
	if !strings.Contains(latestRec.Body.String(), `"summary"`) {
		t.Fatalf("expected latest diagnostics summary, got %s", latestRec.Body.String())
	}

	eventsReq := httptest.NewRequest(http.MethodGet, "/api/diagnostics/events?since=0", nil)
	eventsCtx, cancel := context.WithCancel(eventsReq.Context())
	time.AfterFunc(20*time.Millisecond, cancel)
	eventsReq = eventsReq.WithContext(eventsCtx)
	eventsRec := httptest.NewRecorder()
	router.ServeHTTP(eventsRec, eventsReq)
	if eventsRec.Code != http.StatusOK {
		t.Fatalf("expected diagnostics events 200, got %d: %s", eventsRec.Code, eventsRec.Body.String())
	}
	if !strings.Contains(eventsRec.Body.String(), "event: diagnostics") {
		t.Fatalf("expected diagnostics SSE event, got %s", eventsRec.Body.String())
	}
}

func TestSettingsDiagnosticsExternalProviderEndpoints(t *testing.T) {
	router := testRouterWithSettings(t)

	getReq := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected settings 200, got %d: %s", getRec.Code, getRec.Body.String())
	}
	if !strings.Contains(getRec.Body.String(), `"apiKeyConfigured":false`) {
		t.Fatalf("expected redacted provider settings, got %s", getRec.Body.String())
	}

	patchBody := strings.NewReader(`{"diagnosticsExternalProvider":{"enabled":true,"name":"Operator quality provider","endpointUrl":"https://provider.example/check","timeoutSeconds":2,"apiKey":"secret-token"},"updateManifestUrl":"https://updates.example/manifest.json"}`)
	patchReq := httptest.NewRequest(http.MethodPatch, "/api/settings", patchBody)
	addCSRF(patchReq)
	patchRec := httptest.NewRecorder()
	router.ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("expected settings patch 200, got %d: %s", patchRec.Code, patchRec.Body.String())
	}
	if !strings.Contains(patchRec.Body.String(), `"apiKeyConfigured":true`) || !strings.Contains(patchRec.Body.String(), `"updateManifestUrl":"https://updates.example/manifest.json"`) || strings.Contains(patchRec.Body.String(), "secret-token") {
		t.Fatalf("expected configured secret to be redacted, got %s", patchRec.Body.String())
	}

	invalidReq := httptest.NewRequest(http.MethodPatch, "/api/settings", strings.NewReader(`{"diagnosticsExternalProvider":{"enabled":true,"endpointUrl":"http://provider.example/check","timeoutSeconds":2}}`))
	addCSRF(invalidReq)
	invalidRec := httptest.NewRecorder()
	router.ServeHTTP(invalidRec, invalidReq)
	if invalidRec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid provider 400, got %d: %s", invalidRec.Code, invalidRec.Body.String())
	}

	invalidUpdateReq := httptest.NewRequest(http.MethodPatch, "/api/settings", strings.NewReader(`{"updateManifestUrl":"http://updates.example/manifest.json"}`))
	addCSRF(invalidUpdateReq)
	invalidUpdateRec := httptest.NewRecorder()
	router.ServeHTTP(invalidUpdateRec, invalidUpdateReq)
	if invalidUpdateRec.Code != http.StatusBadRequest || !strings.Contains(invalidUpdateRec.Body.String(), `"code":"UPDATE_MANIFEST_INVALID"`) {
		t.Fatalf("expected invalid update manifest 400, got %d: %s", invalidUpdateRec.Code, invalidUpdateRec.Body.String())
	}
}

func TestCascadePairingEndpoints(t *testing.T) {
	router := testRouterWithPairing(t)

	createReq := httptest.NewRequest(http.MethodPost, "/api/pairing/code", strings.NewReader(`{"nodeName":"Exit node","domain":"exit.example.com","ttlMinutes":10}`))
	addCSRF(createReq)
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected code create 201, got %d: %s", createRec.Code, createRec.Body.String())
	}
	if !strings.Contains(createRec.Body.String(), `"code":"`) {
		t.Fatalf("expected one-time pairing code, got %s", createRec.Body.String())
	}
	var createPayload Envelope
	if err := json.Unmarshal(createRec.Body.Bytes(), &createPayload); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	createData := createPayload.Data.(map[string]any)
	code, _ := createData["code"].(string)

	acceptReq := httptest.NewRequest(http.MethodPost, "/api/pairing/accept", strings.NewReader(`{"exitDomain":"exit.example.com","code":"`+code+`"}`))
	addCSRF(acceptReq)
	acceptRec := httptest.NewRecorder()
	router.ServeHTTP(acceptRec, acceptReq)
	if acceptRec.Code != http.StatusAccepted {
		t.Fatalf("expected accept 202, got %d: %s", acceptRec.Code, acceptRec.Body.String())
	}
	if !strings.Contains(acceptRec.Body.String(), `"configState":"pending_apply"`) {
		t.Fatalf("expected pending cascade config, got %s", acceptRec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/cascade/nodes", nil)
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK || !strings.Contains(listRec.Body.String(), `"Exit node"`) {
		t.Fatalf("expected cascade node list, got %d: %s", listRec.Code, listRec.Body.String())
	}

	var acceptPayload Envelope
	if err := json.Unmarshal(acceptRec.Body.Bytes(), &acceptPayload); err != nil {
		t.Fatalf("decode accept response: %v", err)
	}
	acceptData := acceptPayload.Data.(map[string]any)
	node := acceptData["node"].(map[string]any)
	nodeID, _ := node["id"].(string)
	pair := acceptData["pair"].(map[string]any)
	pairID, _ := pair["id"].(string)
	if nodeID == "" {
		t.Fatalf("expected node id in %#v", acceptPayload.Data)
	}

	planReq := httptest.NewRequest(http.MethodPost, "/api/cascade/pairs/"+pairID+"/plan", nil)
	addCSRF(planReq)
	planRec := httptest.NewRecorder()
	router.ServeHTTP(planRec, planReq)
	if planRec.Code != http.StatusOK || !strings.Contains(planRec.Body.String(), `"configState":"planned"`) {
		t.Fatalf("expected cascade config plan, got %d: %s", planRec.Code, planRec.Body.String())
	}

	applyWithoutConfirmReq := httptest.NewRequest(http.MethodPost, "/api/cascade/pairs/"+pairID+"/apply", strings.NewReader(`{"confirm":false}`))
	addCSRF(applyWithoutConfirmReq)
	applyWithoutConfirmRec := httptest.NewRecorder()
	router.ServeHTTP(applyWithoutConfirmRec, applyWithoutConfirmReq)
	if applyWithoutConfirmRec.Code != http.StatusBadRequest || !strings.Contains(applyWithoutConfirmRec.Body.String(), `"code":"CASCADE_CONFIRMATION_REQUIRED"`) {
		t.Fatalf("expected confirmation required, got %d: %s", applyWithoutConfirmRec.Code, applyWithoutConfirmRec.Body.String())
	}

	applyReq := httptest.NewRequest(http.MethodPost, "/api/cascade/pairs/"+pairID+"/apply", strings.NewReader(`{"confirm":true}`))
	addCSRF(applyReq)
	applyRec := httptest.NewRecorder()
	router.ServeHTTP(applyRec, applyReq)
	if applyRec.Code != http.StatusAccepted || !strings.Contains(applyRec.Body.String(), `"configState":"applied"`) {
		t.Fatalf("expected cascade config apply, got %d: %s", applyRec.Code, applyRec.Body.String())
	}

	sampleReq := httptest.NewRequest(http.MethodPost, "/api/cascade/health/sample", nil)
	addCSRF(sampleReq)
	sampleRec := httptest.NewRecorder()
	router.ServeHTTP(sampleRec, sampleReq)
	if sampleRec.Code != http.StatusAccepted || !strings.Contains(sampleRec.Body.String(), `"samples"`) {
		t.Fatalf("expected cascade health samples, got %d: %s", sampleRec.Code, sampleRec.Body.String())
	}

	updateReq := httptest.NewRequest(http.MethodPatch, "/api/cascade/nodes/"+nodeID, strings.NewReader(`{"name":"Trusted exit","status":"disabled"}`))
	addCSRF(updateReq)
	updateRec := httptest.NewRecorder()
	router.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK || !strings.Contains(updateRec.Body.String(), `"Trusted exit"`) {
		t.Fatalf("expected cascade node update, got %d: %s", updateRec.Code, updateRec.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/cascade/nodes/"+nodeID, nil)
	addCSRF(deleteReq)
	deleteRec := httptest.NewRecorder()
	router.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK || !strings.Contains(deleteRec.Body.String(), `"deleted":true`) {
		t.Fatalf("expected cascade node delete, got %d: %s", deleteRec.Code, deleteRec.Body.String())
	}
}

func TestBackupCreateListAndRestoreEndpoints(t *testing.T) {
	router, appStore := testRouterWithBackupStore(t)
	if err := appStore.SetSetting(context.Background(), "restore.marker", "before-backup"); err != nil {
		t.Fatalf("set marker: %v", err)
	}

	emptyListReq := httptest.NewRequest(http.MethodGet, "/api/backups", nil)
	emptyListRec := httptest.NewRecorder()
	router.ServeHTTP(emptyListRec, emptyListReq)
	if emptyListRec.Code != http.StatusOK || !strings.Contains(emptyListRec.Body.String(), `"backups":[]`) {
		t.Fatalf("expected empty backups array, got %d: %s", emptyListRec.Code, emptyListRec.Body.String())
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/backups", nil)
	addCSRF(createReq)
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusAccepted {
		t.Fatalf("expected backup create 202, got %d: %s", createRec.Code, createRec.Body.String())
	}
	if !strings.Contains(createRec.Body.String(), `"status":"ready"`) || !strings.Contains(createRec.Body.String(), `"kind":"backup_create"`) {
		t.Fatalf("expected ready backup result, got %s", createRec.Body.String())
	}
	var createPayload Envelope
	if err := json.Unmarshal(createRec.Body.Bytes(), &createPayload); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	createData := createPayload.Data.(map[string]any)
	backupData := createData["backup"].(map[string]any)
	backupID, _ := backupData["id"].(string)
	if backupID == "" {
		t.Fatalf("expected backup id, got %#v", createPayload.Data)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/backups", nil)
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK || !strings.Contains(listRec.Body.String(), backupID) {
		t.Fatalf("expected backup list, got %d: %s", listRec.Code, listRec.Body.String())
	}

	if err := appStore.SetSetting(context.Background(), "restore.marker", "after-backup"); err != nil {
		t.Fatalf("change marker: %v", err)
	}
	restoreWithoutConfirmReq := httptest.NewRequest(http.MethodPost, "/api/backups/"+backupID+"/restore", strings.NewReader(`{"confirm":false}`))
	addCSRF(restoreWithoutConfirmReq)
	restoreWithoutConfirmRec := httptest.NewRecorder()
	router.ServeHTTP(restoreWithoutConfirmRec, restoreWithoutConfirmReq)
	if restoreWithoutConfirmRec.Code != http.StatusBadRequest || !strings.Contains(restoreWithoutConfirmRec.Body.String(), `"code":"BACKUP_RESTORE_CONFIRMATION_REQUIRED"`) {
		t.Fatalf("expected restore confirmation required, got %d: %s", restoreWithoutConfirmRec.Code, restoreWithoutConfirmRec.Body.String())
	}

	restoreReq := httptest.NewRequest(http.MethodPost, "/api/backups/"+backupID+"/restore", strings.NewReader(`{"confirm":true}`))
	addCSRF(restoreReq)
	restoreRec := httptest.NewRecorder()
	router.ServeHTTP(restoreRec, restoreReq)
	if restoreRec.Code != http.StatusAccepted || !strings.Contains(restoreRec.Body.String(), `"restored":true`) {
		t.Fatalf("expected backup restore, got %d: %s", restoreRec.Code, restoreRec.Body.String())
	}
	value, ok, err := appStore.GetSetting(context.Background(), "restore.marker")
	if err != nil {
		t.Fatalf("read restored marker: %v", err)
	}
	if !ok || value != "before-backup" {
		t.Fatalf("expected restored marker, got ok=%v value=%q", ok, value)
	}
}

func TestAuditListEndpoint(t *testing.T) {
	router, appStore := testRouterWithAuditStore(t)
	emptyReq := httptest.NewRequest(http.MethodGet, "/api/audit?limit=5", nil)
	emptyRec := httptest.NewRecorder()
	router.ServeHTTP(emptyRec, emptyReq)
	if emptyRec.Code != http.StatusOK || !strings.Contains(emptyRec.Body.String(), `"logs":[]`) {
		t.Fatalf("expected empty audit logs array, got %d: %s", emptyRec.Code, emptyRec.Body.String())
	}

	if err := appStore.AppendAuditLog(context.Background(), nil, "backup_created", "backup", "bak_test", `{"status":"ready"}`); err != nil {
		t.Fatalf("append audit log: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/audit?limit=5", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected audit list 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"action":"backup_created"`) || !strings.Contains(rec.Body.String(), `"status":"ready"`) {
		t.Fatalf("expected audit log response, got %s", rec.Body.String())
	}

	invalidReq := httptest.NewRequest(http.MethodGet, "/api/audit?limit=999", nil)
	invalidRec := httptest.NewRecorder()
	router.ServeHTTP(invalidRec, invalidReq)
	if invalidRec.Code != http.StatusBadRequest || !strings.Contains(invalidRec.Body.String(), `"code":"INVALID_AUDIT_LIMIT"`) {
		t.Fatalf("expected invalid limit response, got %d: %s", invalidRec.Code, invalidRec.Body.String())
	}
}

func TestUpdateCheckEndpoint(t *testing.T) {
	router := testRouterWithUpdate(t, update.NewService(nil, "0.1.0"))

	req := httptest.NewRequest(http.MethodGet, "/api/update/check", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected update check 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"configured":false`) || !strings.Contains(rec.Body.String(), `"currentVersion":"0.1.0"`) {
		t.Fatalf("expected unconfigured update response, got %s", rec.Body.String())
	}
}

func TestUpdateCheckEndpointReportsInvalidManifestURL(t *testing.T) {
	appStore, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = appStore.Close() })
	if err := appStore.SetSetting(context.Background(), update.ManifestURLSettingKey, "http://updates.example/manifest.json"); err != nil {
		t.Fatalf("set update manifest url: %v", err)
	}
	router := testRouterWithUpdateStore(t, appStore, update.NewService(appStore, "0.1.0"))

	req := httptest.NewRequest(http.MethodGet, "/api/update/check", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"code":"UPDATE_MANIFEST_INVALID"`) {
		t.Fatalf("expected invalid manifest response, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateApplyEndpointRequiresConfirmation(t *testing.T) {
	router := testRouterWithUpdate(t, update.NewService(nil, "0.1.0"))
	req := httptest.NewRequest(http.MethodPost, "/api/update/apply", strings.NewReader(`{"confirm":false}`))
	addCSRF(req)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"code":"UPDATE_CONFIRMATION_REQUIRED"`) {
		t.Fatalf("expected confirmation required, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPairingExchangeEndpointSkipsBrowserCSRF(t *testing.T) {
	router, pairingService := testRouterWithPairingService(t)

	codeResult, err := pairingService.CreateCode(context.Background(), pairing.CreateCodeRequest{
		NodeName:   "Exit node",
		Domain:     "exit.example.com",
		TTLMinutes: 10,
	})
	if err != nil {
		t.Fatalf("create code: %v", err)
	}
	entry := testPeerNode(t, "entry.example.com")
	body := `{"code":"` + codeResult.Code + `","entryNode":{"nodeId":"` + entry.NodeID + `","nodeName":"Entry node","domain":"` + entry.Domain + `","publicKey":"` + entry.PublicKey + `","fingerprint":"` + entry.Fingerprint + `"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/pairing/exchange", strings.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected exchange 200 without browser csrf, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"exitNode"`) || !strings.Contains(rec.Body.String(), `"configState":"pending_apply"`) {
		t.Fatalf("expected exchange result, got %s", rec.Body.String())
	}

	reuseReq := httptest.NewRequest(http.MethodPost, "/api/pairing/exchange", strings.NewReader(body))
	reuseRec := httptest.NewRecorder()
	router.ServeHTTP(reuseRec, reuseReq)
	if reuseRec.Code != http.StatusNotFound {
		t.Fatalf("expected one-time exchange rejection, got %d: %s", reuseRec.Code, reuseRec.Body.String())
	}
}

func TestBootstrapStartCreatesJob(t *testing.T) {
	ctx := context.Background()
	appStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = appStore.Close() })

	bootstrapSvc := bootstrap.NewService(appStore)
	token, err := bootstrapSvc.EnsureInitToken(ctx)
	if err != nil {
		t.Fatalf("ensure token: %v", err)
	}

	router := NewRouter(Config{
		StaticFS:  fstest.MapFS{"index.html": {Data: []byte("<html>omo</html>")}},
		Bootstrap: bootstrapSvc,
		Auth:      auth.NewService(appStore),
	})

	body := strings.NewReader(`{"token":"` + token.Token + `","username":"admin","password":"StrongPassw0rd!","confirmPassword":"StrongPassw0rd!","domain":"ops.example.com"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/bootstrap/start", body)
	addCSRF(req)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !payload.Success {
		t.Fatalf("expected success response: %s", rec.Body.String())
	}

	count, err := appStore.AdminCount(ctx)
	if err != nil {
		t.Fatalf("admin count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one admin, got %d", count)
	}

	events, err := appStore.ListJobEventsAfter(ctx, bootstrap.JobKindBootstrap, 0)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected bootstrap events")
	}
}

func TestLoginMeAndLogout(t *testing.T) {
	ctx := context.Background()
	appStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = appStore.Close() })

	bootstrapSvc := bootstrap.NewService(appStore)
	token, err := bootstrapSvc.EnsureInitToken(ctx)
	if err != nil {
		t.Fatalf("ensure token: %v", err)
	}

	router := NewRouter(Config{
		StaticFS:  fstest.MapFS{"index.html": {Data: []byte("<html>omo</html>")}},
		Bootstrap: bootstrapSvc,
		Auth:      auth.NewService(appStore),
	})

	startBody := strings.NewReader(`{"token":"` + token.Token + `","username":"admin","password":"StrongPassw0rd!","confirmPassword":"StrongPassw0rd!","domain":"ops.example.com"}`)
	startReq := httptest.NewRequest(http.MethodPost, "/api/bootstrap/start", startBody)
	addCSRF(startReq)
	startRec := httptest.NewRecorder()
	router.ServeHTTP(startRec, startReq)
	if startRec.Code != http.StatusAccepted {
		t.Fatalf("bootstrap expected 202, got %d: %s", startRec.Code, startRec.Body.String())
	}

	loginBody := strings.NewReader(`{"username":"admin","password":"StrongPassw0rd!"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", loginBody)
	addCSRF(loginReq)
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login expected 200, got %d: %s", loginRec.Code, loginRec.Body.String())
	}
	cookies := loginRec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie")
	}

	meReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meReq.AddCookie(cookies[0])
	meRec := httptest.NewRecorder()
	router.ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusOK || !strings.Contains(meRec.Body.String(), `"authenticated":true`) {
		t.Fatalf("expected authenticated me response, got %d: %s", meRec.Code, meRec.Body.String())
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	addCSRF(logoutReq)
	logoutReq.AddCookie(cookies[0])
	logoutRec := httptest.NewRecorder()
	router.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusOK {
		t.Fatalf("logout expected 200, got %d: %s", logoutRec.Code, logoutRec.Body.String())
	}

	meAfterReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meAfterReq.AddCookie(cookies[0])
	meAfterRec := httptest.NewRecorder()
	router.ServeHTTP(meAfterRec, meAfterReq)
	if meAfterRec.Code != http.StatusOK || !strings.Contains(meAfterRec.Body.String(), `"authenticated":false`) {
		t.Fatalf("expected unauthenticated after logout, got %d: %s", meAfterRec.Code, meAfterRec.Body.String())
	}
}

func TestBootstrapStartReturnsCaddyUnavailable(t *testing.T) {
	ctx := context.Background()
	appStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = appStore.Close() })

	bootstrapSvc := bootstrap.NewServiceWithPhase2(appStore, fallbackBootstrapHook{})
	token, err := bootstrapSvc.EnsureInitToken(ctx)
	if err != nil {
		t.Fatalf("ensure token: %v", err)
	}

	router := NewRouter(Config{
		StaticFS:  fstest.MapFS{"index.html": {Data: []byte("<html>omo</html>")}},
		Bootstrap: bootstrapSvc,
		Auth:      auth.NewService(appStore),
	})

	body := strings.NewReader(`{"token":"` + token.Token + `","username":"admin","password":"StrongPassw0rd!","confirmPassword":"StrongPassw0rd!","domain":"ops.example.com"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/bootstrap/start", body)
	addCSRF(req)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"CADDY_UNAVAILABLE"`) {
		t.Fatalf("expected CADDY_UNAVAILABLE response, got %s", rec.Body.String())
	}

	_, err = bootstrapSvc.Start(ctx, bootstrap.StartRequest{
		Token:           token.Token,
		Username:        "admin",
		Password:        "StrongPassw0rd!",
		ConfirmPassword: "StrongPassw0rd!",
		Domain:          "ops.example.com",
	})
	if !errors.Is(err, bootstrap.ErrRetryRequired) {
		t.Fatalf("expected retry required after failed job, got %v", err)
	}
}

func TestPostRequiresCSRFToken(t *testing.T) {
	router := testRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"CSRF_TOKEN_INVALID"`) {
		t.Fatalf("expected csrf error, got %s", rec.Body.String())
	}
}

func testRouter(t *testing.T) http.Handler {
	t.Helper()
	return testRouterWithSingBox(t, nil)
}

func testRouterWithSingBox(t *testing.T, detector *singbox.Detector) http.Handler {
	t.Helper()
	appStore, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = appStore.Close() })
	bootstrapSvc := bootstrap.NewService(appStore)
	return NewRouter(Config{StaticFS: fstest.MapFS{
		"index.html": {Data: []byte("<html>omo</html>")},
	}, Bootstrap: bootstrapSvc, Auth: auth.NewService(appStore), SingBox: detector})
}

func testRouterWithConfigGen(t *testing.T) http.Handler {
	t.Helper()
	appStore, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = appStore.Close() })
	configPath := filepath.Join(t.TempDir(), "sing-box", "config.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("create config dir: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(`{"outbounds":[{"type":"direct","tag":"old"}]}`+"\n"), 0o600); err != nil {
		t.Fatalf("write seed config: %v", err)
	}
	manager, err := configgen.NewManager(configgen.Options{ConfigPath: configPath})
	if err != nil {
		t.Fatalf("new config manager: %v", err)
	}
	bootstrapSvc := bootstrap.NewService(appStore)
	return NewRouter(Config{StaticFS: fstest.MapFS{
		"index.html": {Data: []byte("<html>omo</html>")},
	}, Bootstrap: bootstrapSvc, Auth: auth.NewService(appStore), ConfigGen: configgen.NewService(manager, appStore)})
}

func testRouterWithSubscriptions(t *testing.T) http.Handler {
	t.Helper()
	appStore, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = appStore.Close() })
	bootstrapSvc := bootstrap.NewService(appStore)
	return NewRouter(Config{StaticFS: fstest.MapFS{
		"index.html": {Data: []byte("<html>omo</html>")},
	}, Bootstrap: bootstrapSvc, Auth: auth.NewService(appStore), Subscriptions: subscription.NewService(appStore, "https://ops.example.com")})
}

func testRouterWithDiagnostics(t *testing.T) http.Handler {
	t.Helper()
	appStore, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = appStore.Close() })
	bootstrapSvc := bootstrap.NewService(appStore)
	return NewRouter(Config{StaticFS: fstest.MapFS{
		"index.html": {Data: []byte("<html>omo</html>")},
	}, Bootstrap: bootstrapSvc, Auth: auth.NewService(appStore), Diagnostics: diagnostics.NewService(appStore)})
}

func testRouterWithSettings(t *testing.T) http.Handler {
	t.Helper()
	appStore, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = appStore.Close() })
	bootstrapSvc := bootstrap.NewService(appStore)
	return NewRouter(Config{StaticFS: fstest.MapFS{
		"index.html": {Data: []byte("<html>omo</html>")},
	}, Bootstrap: bootstrapSvc, Auth: auth.NewService(appStore), Store: appStore})
}

func testRouterWithServiceStore(t *testing.T) (http.Handler, *store.Store) {
	t.Helper()
	appStore, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = appStore.Close() })
	bootstrapSvc := bootstrap.NewService(appStore)
	router := NewRouter(Config{StaticFS: fstest.MapFS{
		"index.html": {Data: []byte("<html>omo</html>")},
	}, Bootstrap: bootstrapSvc, Auth: auth.NewService(appStore), Store: appStore, SingBox: singbox.NewDetector(singbox.Options{
		BinaryPath: filepath.Join(t.TempDir(), "missing-sing-box"),
	})})
	return router, appStore
}

func testRouterWithPairing(t *testing.T) http.Handler {
	t.Helper()
	router, _ := testRouterWithPairingService(t)
	return router
}

func testRouterWithPairingService(t *testing.T) (http.Handler, *pairing.Service) {
	t.Helper()
	appStore, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = appStore.Close() })
	bootstrapSvc := bootstrap.NewService(appStore)
	service := pairing.NewServiceWithOptions(appStore, nil, apiFakeHealthSampler{})
	router := NewRouter(Config{StaticFS: fstest.MapFS{
		"index.html": {Data: []byte("<html>omo</html>")},
	}, Bootstrap: bootstrapSvc, Auth: auth.NewService(appStore), Pairing: service})
	return router, service
}

func testRouterWithBackupStore(t *testing.T) (http.Handler, *store.Store) {
	t.Helper()
	appStore, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = appStore.Close() })
	bootstrapSvc := bootstrap.NewService(appStore)
	router := NewRouter(Config{StaticFS: fstest.MapFS{
		"index.html": {Data: []byte("<html>omo</html>")},
	}, Bootstrap: bootstrapSvc, Auth: auth.NewService(appStore), Backup: backup.NewService(appStore, filepath.Join(t.TempDir(), "backups"))})
	return router, appStore
}

func testRouterWithAuditStore(t *testing.T) (http.Handler, *store.Store) {
	t.Helper()
	appStore, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = appStore.Close() })
	bootstrapSvc := bootstrap.NewService(appStore)
	router := NewRouter(Config{StaticFS: fstest.MapFS{
		"index.html": {Data: []byte("<html>omo</html>")},
	}, Bootstrap: bootstrapSvc, Auth: auth.NewService(appStore), Audit: audit.NewService(appStore)})
	return router, appStore
}

func testRouterWithUpdate(t *testing.T, service *update.Service) http.Handler {
	t.Helper()
	appStore, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = appStore.Close() })
	return testRouterWithUpdateStore(t, appStore, service)
}

func testRouterWithUpdateStore(t *testing.T, appStore *store.Store, service *update.Service) http.Handler {
	t.Helper()
	bootstrapSvc := bootstrap.NewService(appStore)
	return NewRouter(Config{StaticFS: fstest.MapFS{
		"index.html": {Data: []byte("<html>omo</html>")},
	}, Bootstrap: bootstrapSvc, Auth: auth.NewService(appStore), Update: service})
}

func addCSRF(req *http.Request) {
	addCSRFWithValue(req, "test-csrf-token")
}

func addCSRFWithValue(req *http.Request, value string) {
	req.Header.Set("X-CSRF-Token", value)
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: value, Path: "/"})
}

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

func TestSPAHandlerServesRouteHTMLWhenAvailable(t *testing.T) {
	router := NewRouter(Config{StaticFS: fstest.MapFS{
		"index.html": {Data: []byte("<html>index</html>")},
		"init.html":  {Data: []byte("<html>init</html>")},
	}})

	req := httptest.NewRequest(http.MethodGet, "/init?token=test", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected init route 200, got %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != "<html>init</html>" {
		t.Fatalf("expected route html, got %q", rec.Body.String())
	}
}

func testPeerNode(t *testing.T, domain string) pairing.PeerNode {
	t.Helper()
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate peer key: %v", err)
	}
	sum := sha256.Sum256(publicKey)
	return pairing.PeerNode{
		NodeID:      "node_test_entry",
		NodeName:    "Entry node",
		Domain:      domain,
		PublicKey:   base64.RawURLEncoding.EncodeToString(publicKey),
		Fingerprint: base64.RawURLEncoding.EncodeToString(sum[:]),
	}
}

type apiFakeHealthSampler struct{}

func (apiFakeHealthSampler) Sample(_ context.Context, node store.CascadeNode) (store.CascadeHealthSample, error) {
	return store.CascadeHealthSample{
		NodeID:         node.ID,
		Status:         "online",
		Online:         true,
		LatencyMS:      18,
		ThroughputMbps: 1.75,
		SampledAt:      time.Now().UTC(),
	}, nil
}
