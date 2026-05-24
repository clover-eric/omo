package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"omo/internal/auth"
	"omo/internal/store"
)

const (
	JobKindBootstrap = "bootstrap"

	settingInitTokenHash      = "bootstrap.init_token_hash"
	settingInitTokenExpiresAt = "bootstrap.init_token_expires_at"
	settingDomain             = "bootstrap.domain"
	settingPhase1Complete     = "bootstrap.phase1_complete"
)

var (
	ErrAlreadyInitialized = errors.New("bootstrap already initialized")
	ErrInvalidToken       = errors.New("invalid bootstrap token")
	ErrTokenExpired       = errors.New("bootstrap token expired")
	ErrPasswordMismatch   = errors.New("password confirmation does not match")
	ErrInvalidInput       = errors.New("invalid bootstrap input")
	ErrJobRunning         = errors.New("bootstrap job already running")
	ErrRetryRequired      = errors.New("bootstrap retry confirmation required")
)

type Service struct {
	store      *store.Store
	phase2Hook Phase2Hook
	mu         sync.Mutex
}

type Phase2Hook interface {
	Run(ctx context.Context, domain string) (Phase2Result, error)
}

type Phase2Result struct {
	ResolvedIPs       []string `json:"resolvedIps"`
	PortsAvailable    []int    `json:"portsAvailable"`
	CertificateIssuer string   `json:"certificateIssuer,omitempty"`
	Message           string   `json:"message"`
	EntryMode         string   `json:"entryMode"`
	SecurityState     string   `json:"securityState"`
	Warnings          []string `json:"warnings,omitempty"`
}

type NoopPhase2Hook struct{}

func (NoopPhase2Hook) Run(context.Context, string) (Phase2Result, error) {
	return Phase2Result{
		Message:       "Phase 2 系统集成尚未配置，已完成本地初始化闭环。",
		EntryMode:     "local",
		SecurityState: "ready",
	}, nil
}

type Phase2FallbackError struct {
	Code    string
	Message string
	Result  Phase2Result
	Cause   error
}

func (e Phase2FallbackError) Error() string {
	if e.Cause != nil {
		return e.Code + ": " + e.Cause.Error()
	}
	return e.Code + ": " + e.Message
}

func (e Phase2FallbackError) Unwrap() error {
	return e.Cause
}

type StartRequest struct {
	Token           string `json:"token"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmPassword"`
	Domain          string `json:"domain"`
	Retry           bool   `json:"retry"`
}

type StartResult struct {
	Job              store.Job `json:"job"`
	SessionToken     string    `json:"-"`
	SessionExpiresAt time.Time `json:"-"`
	RedirectTo       string    `json:"redirectTo"`
}

type Status struct {
	State           State      `json:"state"`
	Initialized     bool       `json:"initialized"`
	Phase1Complete  bool       `json:"phase1Complete"`
	RequiresToken   bool       `json:"requiresToken"`
	Domain          string     `json:"domain,omitempty"`
	LatestJob       *store.Job `json:"latestJob,omitempty"`
	NextRequirement string     `json:"nextRequirement,omitempty"`
	Phase2Result    any        `json:"phase2Result,omitempty"`
}

type InitToken struct {
	Token     string
	ExpiresAt time.Time
	Generated bool
}

func NewService(store *store.Store) *Service {
	return NewServiceWithPhase2(store, NoopPhase2Hook{})
}

func NewServiceWithPhase2(store *store.Store, hook Phase2Hook) *Service {
	if hook == nil {
		hook = NoopPhase2Hook{}
	}
	return &Service{store: store, phase2Hook: hook}
}

func (s *Service) EnsureInitToken(ctx context.Context) (*InitToken, error) {
	admins, err := s.store.AdminCount(ctx)
	if err != nil {
		return nil, err
	}
	if admins > 0 {
		return nil, nil
	}

	envToken := strings.TrimSpace(getenv("OMO_INIT_TOKEN"))
	existingHash, ok, err := s.store.GetSetting(ctx, settingInitTokenHash)
	if err != nil {
		return nil, err
	}
	if ok && existingHash != "" {
		if envToken != "" && auth.HashToken(envToken) != existingHash {
			expiresAt := time.Now().UTC().Add(2 * time.Hour)
			if err := s.store.SetSetting(ctx, settingInitTokenHash, auth.HashToken(envToken)); err != nil {
				return nil, err
			}
			if err := s.store.SetSetting(ctx, settingInitTokenExpiresAt, expiresAt.Format(time.RFC3339Nano)); err != nil {
				return nil, err
			}
			return &InitToken{Token: envToken, ExpiresAt: expiresAt, Generated: true}, nil
		}
		return nil, nil
	}

	token := envToken
	if token == "" {
		var err error
		token, err = auth.GenerateToken(32)
		if err != nil {
			return nil, err
		}
	}

	expiresAt := time.Now().UTC().Add(2 * time.Hour)
	if err := s.store.SetSetting(ctx, settingInitTokenHash, auth.HashToken(token)); err != nil {
		return nil, err
	}
	if err := s.store.SetSetting(ctx, settingInitTokenExpiresAt, expiresAt.Format(time.RFC3339Nano)); err != nil {
		return nil, err
	}

	return &InitToken{Token: token, ExpiresAt: expiresAt, Generated: true}, nil
}

func (s *Service) Status(ctx context.Context) (Status, error) {
	latest, err := s.store.LatestJob(ctx, JobKindBootstrap)
	if err != nil {
		return Status{}, err
	}

	admins, err := s.store.AdminCount(ctx)
	if err != nil {
		return Status{}, err
	}

	domain, _, err := s.store.GetSetting(ctx, settingDomain)
	if err != nil {
		return Status{}, err
	}
	_, phase1Complete, err := s.store.GetSetting(ctx, settingPhase1Complete)
	if err != nil {
		return Status{}, err
	}
	_, tokenRequired, err := s.store.GetSetting(ctx, settingInitTokenHash)
	if err != nil {
		return Status{}, err
	}
	phase2Raw, hasPhase2Result, err := s.store.GetSetting(ctx, "bootstrap.phase2_result")
	if err != nil {
		return Status{}, err
	}
	var phase2Result any
	if hasPhase2Result && phase2Raw != "" {
		var decoded Phase2Result
		if err := json.Unmarshal([]byte(phase2Raw), &decoded); err == nil {
			phase2Result = decoded
		}
	}

	state := StateUninitialized
	if latest != nil && (latest.Status == "running" || latest.Status == "failed") {
		state = State(latest.State)
	} else if latest != nil && latest.Status == "succeeded" {
		state = State(latest.State)
	} else if admins > 0 {
		state = StatePanelHTTPSEnable
	}

	next := ""
	if admins > 0 {
		next = "Phase 2 将接入域名解析、Caddy 和 HTTPS 配置。"
	}

	return Status{
		State:           state,
		Initialized:     false,
		Phase1Complete:  phase1Complete,
		RequiresToken:   tokenRequired,
		Domain:          domain,
		LatestJob:       latest,
		NextRequirement: next,
		Phase2Result:    phase2Result,
	}, nil
}

func (s *Service) Start(ctx context.Context, req StartRequest) (StartResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	admins, err := s.store.AdminCount(ctx)
	if err != nil {
		return StartResult{}, err
	}

	latest, err := s.store.LatestJob(ctx, JobKindBootstrap)
	if err != nil {
		return StartResult{}, err
	} else if latest != nil && latest.Status == "running" {
		return StartResult{}, ErrJobRunning
	} else if latest != nil && latest.Status == "failed" && !req.Retry {
		return StartResult{}, ErrRetryRequired
	}

	if err := s.validateStartRequest(ctx, req, admins); err != nil {
		return StartResult{}, err
	}

	job, err := s.store.CreateJob(ctx, JobKindBootstrap, string(StateUninitialized), "queued", 0, "初始化任务已创建。")
	if err != nil {
		return StartResult{}, err
	}
	if _, err := s.store.AppendJobEvent(ctx, job.ID, JobKindBootstrap, string(StateUninitialized), "queued", 0, "初始化任务已创建。", ""); err != nil {
		return StartResult{}, err
	}

	if err := s.store.MarkJobStarted(ctx, job.ID); err != nil {
		return StartResult{}, err
	}

	admin, sessionToken, sessionExpiresAt, err := s.ensureAdminAndSession(ctx, job.ID, req, admins)
	if err != nil {
		return StartResult{}, err
	}

	_ = admin

	domain := strings.TrimSpace(req.Domain)
	if err := s.store.SetSetting(ctx, settingDomain, domain); err != nil {
		return StartResult{}, err
	}

	if err := s.step(ctx, job.ID, StateDomainVerify, "running", 58, "正在校验域名解析和入口端口。", ""); err != nil {
		return StartResult{}, err
	}
	phase2Result, err := s.phase2Hook.Run(ctx, domain)
	if err != nil {
		var fallback Phase2FallbackError
		if errors.As(err, &fallback) {
			if err := s.store.SetSetting(ctx, "bootstrap.phase2_result", jsonString(fallback.Result)); err != nil {
				return StartResult{}, err
			}
			_ = s.fail(ctx, job.ID, StateTLSProvision, fallback.Code, fallback.Message)
			return StartResult{}, err
		}
		_ = s.fail(ctx, job.ID, StateDomainVerify, "DOMAIN_OR_CADDY_FAILED", userFacingPhase2Error(err))
		return StartResult{}, err
	}
	if err := s.step(ctx, job.ID, StateDomainVerify, "succeeded", 70, "域名解析和端口检查通过。", ""); err != nil {
		return StartResult{}, err
	}
	if err := s.step(ctx, job.ID, StateTLSProvision, "succeeded", 84, "证书状态已记录。", ""); err != nil {
		return StartResult{}, err
	}
	if err := s.step(ctx, job.ID, StatePanelHTTPSEnable, "succeeded", 94, "面板 HTTPS 入口配置已应用。", ""); err != nil {
		return StartResult{}, err
	}
	if err := s.store.SetSetting(ctx, "bootstrap.phase2_result", jsonString(phase2Result)); err != nil {
		return StartResult{}, err
	}
	if err := s.store.SetSetting(ctx, settingPhase1Complete, "true"); err != nil {
		return StartResult{}, err
	}
	if err := s.store.DeleteSetting(ctx, settingInitTokenHash); err != nil {
		return StartResult{}, err
	}
	if err := s.store.DeleteSetting(ctx, settingInitTokenExpiresAt); err != nil {
		return StartResult{}, err
	}
	if err := markReady(); err != nil {
		_ = s.fail(ctx, job.ID, StatePanelHTTPSEnable, "READY_MARKER_FAILED", "初始化已完成，但临时入口切换标记写入失败，请检查数据目录权限。")
		return StartResult{}, err
	}

	if err := s.step(ctx, job.ID, StatePanelHTTPSEnable, "succeeded", 100, "初始化入口配置完成，等待接入核心安装阶段。", ""); err != nil {
		return StartResult{}, err
	}

	latest, err = s.store.LatestJob(ctx, JobKindBootstrap)
	if err != nil {
		return StartResult{}, err
	}
	return StartResult{Job: *latest, SessionToken: sessionToken, SessionExpiresAt: sessionExpiresAt, RedirectTo: "https://" + domain + "/dashboard"}, nil
}

func (s *Service) ensureAdminAndSession(ctx context.Context, jobID string, req StartRequest, adminCount int) (store.Admin, string, time.Time, error) {
	if adminCount == 0 {
		if err := s.step(ctx, jobID, StatePreflightCheck, "running", 15, "正在执行初始化预检。", ""); err != nil {
			return store.Admin{}, "", time.Time{}, err
		}
		if err := s.step(ctx, jobID, StatePreflightCheck, "succeeded", 30, "初始化预检通过。", ""); err != nil {
			return store.Admin{}, "", time.Time{}, err
		}
		if err := s.step(ctx, jobID, StateAdminCreate, "running", 40, "正在创建管理员账户。", ""); err != nil {
			return store.Admin{}, "", time.Time{}, err
		}

		passwordHash, err := auth.HashPassword(req.Password)
		if err != nil {
			_ = s.fail(ctx, jobID, StateAdminCreate, "PASSWORD_HASH_FAILED", "管理员密码处理失败。")
			return store.Admin{}, "", time.Time{}, err
		}
		admin, err := s.store.CreateAdmin(ctx, strings.TrimSpace(req.Username), passwordHash)
		if err != nil {
			_ = s.fail(ctx, jobID, StateAdminCreate, "ADMIN_CREATE_FAILED", "管理员创建失败，请检查用户名是否已存在。")
			return store.Admin{}, "", time.Time{}, err
		}
		if err := s.step(ctx, jobID, StateAdminCreate, "succeeded", 45, "管理员账户已创建。", ""); err != nil {
			return store.Admin{}, "", time.Time{}, err
		}
		token, expiresAt, err := s.createSession(ctx, jobID, admin.ID)
		return admin, token, expiresAt, err
	}

	admin, err := s.store.AdminByUsername(ctx, strings.TrimSpace(req.Username))
	if err != nil {
		return store.Admin{}, "", time.Time{}, err
	}
	if admin == nil || !auth.VerifyPassword(req.Password, admin.PasswordHash) {
		return store.Admin{}, "", time.Time{}, ErrInvalidToken
	}
	if err := s.step(ctx, jobID, StatePreflightCheck, "succeeded", 20, "检测到可重试的初始化状态。", ""); err != nil {
		return store.Admin{}, "", time.Time{}, err
	}
	if err := s.step(ctx, jobID, StateAdminCreate, "succeeded", 45, "管理员账户已存在，继续入口配置。", ""); err != nil {
		return store.Admin{}, "", time.Time{}, err
	}
	token, expiresAt, err := s.createSession(ctx, jobID, admin.ID)
	return *admin, token, expiresAt, err
}

func (s *Service) createSession(ctx context.Context, jobID string, adminID string) (string, time.Time, error) {
	sessionToken, err := auth.GenerateToken(32)
	if err != nil {
		_ = s.fail(ctx, jobID, StateAdminCreate, "SESSION_CREATE_FAILED", "管理员会话创建失败。")
		return "", time.Time{}, err
	}
	sessionExpiresAt := time.Now().UTC().Add(24 * time.Hour)
	if err := s.store.CreateSession(ctx, adminID, auth.HashToken(sessionToken), sessionExpiresAt); err != nil {
		_ = s.fail(ctx, jobID, StateAdminCreate, "SESSION_CREATE_FAILED", "管理员会话创建失败。")
		return "", time.Time{}, err
	}
	return sessionToken, sessionExpiresAt, nil
}

func userFacingPhase2Error(err error) string {
	message := err.Error()
	if strings.Contains(message, "DOMAIN_NOT_RESOLVED") ||
		strings.Contains(message, "DOMAIN_LOOKUP_FAILED") ||
		strings.Contains(message, "domain not resolved") ||
		strings.Contains(message, "no such host") {
		return "域名暂未解析到当前服务器，请检查 DNS 记录后重试。"
	}
	if strings.Contains(message, "port unavailable") {
		return "80/443 端口不可用，请释放端口后重试。"
	}
	if strings.Contains(message, "caddy ") || strings.Contains(message, "Caddyfile") || strings.Contains(message, "certificate") {
		return "HTTPS 入口配置失败，旧配置已保留。Caddy 返回：" + message
	}
	return "域名、端口或 HTTPS 入口配置失败，旧配置已保留。错误详情：" + message
}

func (s *Service) StreamEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming is not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	lastID := parseLastEventID(r)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	fmt.Fprint(w, "retry: 1000\n\n")
	flusher.Flush()

	for {
		events, err := s.store.ListJobEventsAfter(r.Context(), JobKindBootstrap, lastID)
		if err != nil {
			fmt.Fprintf(w, "event: error\ndata: %s\n\n", jsonString(map[string]string{"message": "读取初始化事件失败"}))
			flusher.Flush()
			return
		}
		for _, event := range events {
			payload, _ := json.Marshal(event)
			fmt.Fprintf(w, "id: %d\nevent: bootstrap\ndata: %s\n\n", event.ID, payload)
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

func (s *Service) validateStartRequest(ctx context.Context, req StartRequest, admins int) error {
	if err := s.validateToken(ctx, req.Token); err != nil {
		return err
	}

	username := strings.TrimSpace(req.Username)
	if len(username) < 3 || len(username) > 64 {
		return ErrInvalidInput
	}
	if req.Password != req.ConfirmPassword {
		return ErrPasswordMismatch
	}
	if admins == 0 {
		if err := auth.ValidatePassword(username, req.Password); err != nil {
			return err
		}
	} else if !req.Retry {
		return ErrAlreadyInitialized
	} else if req.Password == "" {
		return ErrInvalidInput
	}
	if strings.TrimSpace(req.Domain) == "" {
		return ErrInvalidInput
	}
	return nil
}

func (s *Service) validateToken(ctx context.Context, token string) error {
	expected, ok, err := s.store.GetSetting(ctx, settingInitTokenHash)
	if err != nil {
		return err
	}
	if !ok || expected == "" || auth.HashToken(strings.TrimSpace(token)) != expected {
		return ErrInvalidToken
	}

	expiresRaw, ok, err := s.store.GetSetting(ctx, settingInitTokenExpiresAt)
	if err != nil {
		return err
	}
	if ok && expiresRaw != "" {
		expiresAt, err := time.Parse(time.RFC3339Nano, expiresRaw)
		if err != nil || time.Now().UTC().After(expiresAt) {
			return ErrTokenExpired
		}
	}
	return nil
}

func (s *Service) step(ctx context.Context, jobID string, state State, status string, progress int, message string, errorCode string) error {
	finished := status == "succeeded" || status == "failed"
	if err := s.store.UpdateJob(ctx, jobID, string(state), status, progress, message, errorCode, finished); err != nil {
		return err
	}
	_, err := s.store.AppendJobEvent(ctx, jobID, JobKindBootstrap, string(state), status, progress, message, errorCode)
	return err
}

func (s *Service) fail(ctx context.Context, jobID string, state State, code string, message string) error {
	return s.step(ctx, jobID, state, "failed", 100, message, code)
}

func parseLastEventID(r *http.Request) int64 {
	raw := r.Header.Get("Last-Event-ID")
	if raw == "" {
		raw = r.URL.Query().Get("since")
	}
	id, _ := strconv.ParseInt(raw, 10, 64)
	return id
}

func jsonString(v any) string {
	payload, _ := json.Marshal(v)
	return string(payload)
}

var getenv = func(key string) string {
	return os.Getenv(key)
}

func markReady() error {
	path := strings.TrimSpace(getenv("OMO_BOOTSTRAP_READY_MARKER"))
	if path == "" {
		return nil
	}
	return os.WriteFile(path, []byte(time.Now().UTC().Format(time.RFC3339Nano)+"\n"), 0o600)
}
