package diagnostics

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"omo/internal/core/singbox"
	"omo/internal/settings"
	"omo/internal/store"
)

const (
	JobKindDiagnostics = "diagnostics"

	StateQueued        = "DIAGNOSTICS_QUEUED"
	StateSystemCheck   = "SYSTEM_RESOURCE_CHECK"
	StateServiceCheck  = "SERVICE_HEALTH_CHECK"
	StateReportPersist = "DIAGNOSTIC_REPORT_PERSIST"
)

type Store interface {
	CreateJob(ctx context.Context, kind string, state string, status string, progress int, message string) (store.Job, error)
	MarkJobStarted(ctx context.Context, jobID string) error
	UpdateJob(ctx context.Context, jobID string, state string, status string, progress int, message string, errorCode string, finished bool) error
	AppendJobEvent(ctx context.Context, jobID string, kind string, state string, status string, progress int, message string, errorCode string) (store.JobEvent, error)
	ListJobEventsAfter(ctx context.Context, kind string, afterID int64) ([]store.JobEvent, error)
	LatestJob(ctx context.Context, kind string) (*store.Job, error)
	CreateDiagnosticReport(ctx context.Context, status string, summary string, reportJSON string) (store.DiagnosticReport, error)
	LatestDiagnosticReport(ctx context.Context) (*store.DiagnosticReport, error)
	GetSetting(ctx context.Context, key string) (string, bool, error)
}

type Service struct {
	store            Store
	resolver         Resolver
	dialer           Dialer
	tlsCheck         TLSChecker
	core             CoreStatusProvider
	externalProvider ExternalProvider
	ports            []int
}

type Options struct {
	Store            Store
	Resolver         Resolver
	Dialer           Dialer
	TLSCheck         TLSChecker
	Core             CoreStatusProvider
	ExternalProvider ExternalProvider
	Ports            []int
}

type Resolver interface {
	LookupHost(ctx context.Context, host string) ([]string, error)
}

type Dialer interface {
	DialContext(ctx context.Context, network string, address string) (net.Conn, error)
}

type TLSChecker interface {
	Check(ctx context.Context, domain string) TLSResult
}

type CoreStatusProvider interface {
	Status(ctx context.Context) (singbox.Status, error)
}

type ExternalProvider interface {
	Check(ctx context.Context, config ExternalProviderConfig) DiagnosticCheck
}

type ExternalProviderConfig struct {
	Name           string
	EndpointURL    string
	APIKey         string
	TimeoutSeconds int
}

type netResolver struct{}

type defaultTLSChecker struct{}

type httpExternalProvider struct {
	client *http.Client
}

type RunResult struct {
	Job    store.Job        `json:"job"`
	Report DiagnosticReport `json:"report"`
}

type LatestResult struct {
	Report    *DiagnosticReport `json:"report"`
	LatestJob *store.Job        `json:"latestJob,omitempty"`
}

type DiagnosticReport struct {
	ID        string           `json:"id"`
	Status    string           `json:"status"`
	Summary   string           `json:"summary"`
	Checks    []DiagnosticCheck `json:"checks"`
	System    SystemSnapshot   `json:"system"`
	CreatedAt time.Time        `json:"createdAt"`
}

type DiagnosticCheck struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Status   string `json:"status"`
	Message  string `json:"message"`
	Evidence string `json:"evidence,omitempty"`
}

type SystemSnapshot struct {
	Hostname       string `json:"hostname"`
	OS             string `json:"os"`
	Architecture   string `json:"architecture"`
	CPUCount       int    `json:"cpuCount"`
	GoVersion      string `json:"goVersion"`
	ProcessID      int    `json:"processId"`
	MemoryAllocMB  uint64 `json:"memoryAllocMb"`
	MemorySystemMB uint64 `json:"memorySystemMb"`
}

func NewService(appStore Store) *Service {
	return NewServiceWithOptions(Options{Store: appStore})
}

func NewServiceWithOptions(options Options) *Service {
	resolver := options.Resolver
	if resolver == nil {
		resolver = netResolver{}
	}
	dialer := options.Dialer
	if dialer == nil {
		dialer = &net.Dialer{Timeout: 2 * time.Second}
	}
	tlsCheck := options.TLSCheck
	if tlsCheck == nil {
		tlsCheck = defaultTLSChecker{}
	}
	core := options.Core
	if core == nil {
		core = singbox.NewDetector(singbox.Options{})
	}
	externalProvider := options.ExternalProvider
	if externalProvider == nil {
		externalProvider = httpExternalProvider{client: &http.Client{}}
	}
	ports := options.Ports
	if len(ports) == 0 {
		ports = []int{80, 443}
	}
	return &Service{
		store:            options.Store,
		resolver:         resolver,
		dialer:           dialer,
		tlsCheck:         tlsCheck,
		core:             core,
		externalProvider: externalProvider,
		ports:            append([]int(nil), ports...),
	}
}

func (s *Service) Run(ctx context.Context) (RunResult, error) {
	if s == nil || s.store == nil {
		return RunResult{}, errors.New("diagnostics service is unavailable")
	}
	job, err := s.store.CreateJob(ctx, JobKindDiagnostics, StateQueued, "queued", 0, "Server checkup job created.")
	if err != nil {
		return RunResult{}, err
	}
	if _, err := s.store.AppendJobEvent(ctx, job.ID, JobKindDiagnostics, StateQueued, "queued", 0, "Server checkup job created.", ""); err != nil {
		return RunResult{}, err
	}
	if err := s.store.MarkJobStarted(ctx, job.ID); err != nil {
		return RunResult{}, err
	}
	if err := s.step(ctx, job.ID, StateSystemCheck, "running", 30, "Collecting local system resource evidence.", ""); err != nil {
		return RunResult{}, err
	}
	report := s.collectReport(ctx)
	if err := s.step(ctx, job.ID, StateServiceCheck, "running", 70, "Checking OMO service runtime health.", ""); err != nil {
		return RunResult{}, err
	}
	payload, err := json.Marshal(report)
	if err != nil {
		_ = s.step(ctx, job.ID, StateReportPersist, "failed", 100, "Server checkup report could not be encoded.", "DIAGNOSTIC_REPORT_ENCODE_FAILED")
		return RunResult{}, err
	}
	if err := s.step(ctx, job.ID, StateReportPersist, "running", 88, "Persisting server checkup report.", ""); err != nil {
		return RunResult{}, err
	}
	stored, err := s.store.CreateDiagnosticReport(ctx, report.Status, report.Summary, string(payload))
	if err != nil {
		_ = s.step(ctx, job.ID, StateReportPersist, "failed", 100, "Server checkup report could not be saved.", "DIAGNOSTIC_REPORT_SAVE_FAILED")
		return RunResult{}, err
	}
	report.ID = stored.ID
	report.CreatedAt = stored.CreatedAt
	if err := s.step(ctx, job.ID, StateReportPersist, "succeeded", 100, "Server checkup completed.", ""); err != nil {
		return RunResult{}, err
	}
	latest, err := s.store.LatestJob(ctx, JobKindDiagnostics)
	if err != nil {
		return RunResult{}, err
	}
	return RunResult{Job: *latest, Report: report}, nil
}

func (s *Service) Latest(ctx context.Context) (LatestResult, error) {
	if s == nil || s.store == nil {
		return LatestResult{}, errors.New("diagnostics service is unavailable")
	}
	record, err := s.store.LatestDiagnosticReport(ctx)
	if err != nil {
		return LatestResult{}, err
	}
	var report *DiagnosticReport
	if record != nil {
		decoded, err := decodeReport(*record)
		if err != nil {
			return LatestResult{}, err
		}
		report = &decoded
	}
	latestJob, err := s.store.LatestJob(ctx, JobKindDiagnostics)
	if err != nil {
		return LatestResult{}, err
	}
	return LatestResult{Report: report, LatestJob: latestJob}, nil
}

func (s *Service) Events(ctx context.Context, afterID int64) ([]store.JobEvent, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("diagnostics service is unavailable")
	}
	return s.store.ListJobEventsAfter(ctx, JobKindDiagnostics, afterID)
}

func (s *Service) step(ctx context.Context, jobID string, state string, status string, progress int, message string, errorCode string) error {
	finished := status == "succeeded" || status == "failed"
	if err := s.store.UpdateJob(ctx, jobID, state, status, progress, message, errorCode, finished); err != nil {
		return err
	}
	_, err := s.store.AppendJobEvent(ctx, jobID, JobKindDiagnostics, state, status, progress, message, errorCode)
	return err
}

func (s *Service) collectReport(ctx context.Context) DiagnosticReport {
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	hostname, _ := os.Hostname()
	system := SystemSnapshot{
		Hostname:       hostname,
		OS:             runtime.GOOS,
		Architecture:   runtime.GOARCH,
		CPUCount:       runtime.NumCPU(),
		GoVersion:      runtime.Version(),
		ProcessID:      os.Getpid(),
		MemoryAllocMB:  bytesToMB(memory.Alloc),
		MemorySystemMB: bytesToMB(memory.Sys),
	}
	checks := []DiagnosticCheck{
		{
			ID:       "runtime",
			Label:    "OMO runtime",
			Status:   "ok",
			Message:  "OMO process is responding to an authorized server checkup request.",
			Evidence: fmt.Sprintf("pid=%d go=%s", system.ProcessID, system.GoVersion),
		},
		{
			ID:       "cpu",
			Label:    "CPU availability",
			Status:   statusForCPU(system.CPUCount),
			Message:  messageForCPU(system.CPUCount),
			Evidence: fmt.Sprintf("logical_cpu=%d", system.CPUCount),
		},
		{
			ID:       "memory",
			Label:    "Process memory",
			Status:   "ok",
			Message:  "OMO process memory snapshot was collected.",
			Evidence: fmt.Sprintf("alloc_mb=%d system_mb=%d", system.MemoryAllocMB, system.MemorySystemMB),
		},
		{
			ID:       "loopback",
			Label:    "Local network stack",
			Status:   statusForLoopback(),
			Message:  "Local loopback name resolution was checked.",
			Evidence: "localhost",
		},
	}
	checks = append(checks, s.domainChecks(ctx)...)
	checks = append(checks, s.portChecks(ctx)...)
	checks = append(checks, s.coreCheck(ctx))
	checks = append(checks, s.externalProviderCheck(ctx)...)
	status := reportStatus(checks)
	return DiagnosticReport{
		Status:  status,
		Summary: summaryFor(status),
		Checks:  checks,
		System:  system,
	}
}

func (s *Service) domainChecks(ctx context.Context) []DiagnosticCheck {
	domain, ok, err := s.store.GetSetting(ctx, "bootstrap.domain")
	domain = strings.TrimSpace(domain)
	if err != nil {
		return []DiagnosticCheck{{
			ID:      "domain-settings",
			Label:   "Configured domain",
			Status:  "warning",
			Message: "Configured panel domain could not be read.",
		}}
	}
	if !ok || domain == "" {
		return []DiagnosticCheck{{
			ID:      "domain-settings",
			Label:   "Configured domain",
			Status:  "warning",
			Message: "No configured panel domain is available for DNS or TLS checks.",
		}}
	}

	var checks []DiagnosticCheck
	hosts, err := s.resolver.LookupHost(ctx, domain)
	if err != nil || len(hosts) == 0 {
		checks = append(checks, DiagnosticCheck{
			ID:       "domain-dns",
			Label:    "Domain DNS",
			Status:   "warning",
			Message:  "Configured panel domain did not resolve during this server checkup.",
			Evidence: domain,
		})
	} else {
		checks = append(checks, DiagnosticCheck{
			ID:       "domain-dns",
			Label:    "Domain DNS",
			Status:   "ok",
			Message:  "Configured panel domain resolves.",
			Evidence: domain + " -> " + strings.Join(hosts, ","),
		})
	}

	tlsResult := s.tlsCheck.Check(ctx, domain)
	checks = append(checks, DiagnosticCheck{
		ID:       "domain-tls",
		Label:    "Domain TLS",
		Status:   tlsResult.Status,
		Message:  tlsResult.Message,
		Evidence: tlsResult.Evidence,
	})
	return checks
}

func (s *Service) portChecks(ctx context.Context) []DiagnosticCheck {
	checks := make([]DiagnosticCheck, 0, len(s.ports))
	for _, port := range s.ports {
		address := net.JoinHostPort("127.0.0.1", fmt.Sprint(port))
		conn, err := s.dialer.DialContext(ctx, "tcp", address)
		if err != nil {
			checks = append(checks, DiagnosticCheck{
				ID:       fmt.Sprintf("local-port-%d", port),
				Label:    fmt.Sprintf("Local port %d", port),
				Status:   "warning",
				Message:  "Expected local entry port is not reachable from this host.",
				Evidence: address,
			})
			continue
		}
		_ = conn.Close()
		checks = append(checks, DiagnosticCheck{
			ID:       fmt.Sprintf("local-port-%d", port),
			Label:    fmt.Sprintf("Local port %d", port),
			Status:   "ok",
			Message:  "Expected local entry port is reachable from this host.",
			Evidence: address,
		})
	}
	return checks
}

func (s *Service) coreCheck(ctx context.Context) DiagnosticCheck {
	status, err := s.core.Status(ctx)
	if err != nil {
		return DiagnosticCheck{
			ID:      "access-core",
			Label:   "Access core",
			Status:  "warning",
			Message: "Access core status could not be read.",
		}
	}
	check := DiagnosticCheck{
		ID:      "access-core",
		Label:   "Access core",
		Status:  "warning",
		Message: status.Message,
	}
	if status.Healthy {
		check.Status = "ok"
	}
	if status.Path != "" || status.Version != "" {
		check.Evidence = strings.TrimSpace(status.Path + " " + status.Version)
	}
	return check
}

func (s *Service) externalProviderCheck(ctx context.Context) []DiagnosticCheck {
	provider, err := settings.LoadDiagnosticsExternalProvider(ctx, s.store)
	if err != nil {
		return []DiagnosticCheck{{
			ID:      "external-provider-settings",
			Label:   "External provider settings",
			Status:  "warning",
			Message: "Optional provider settings could not be read.",
		}}
	}
	if !provider.Enabled {
		return nil
	}
	apiKey, _, err := settings.LoadDiagnosticsExternalProviderSecret(ctx, s.store)
	if err != nil {
		return []DiagnosticCheck{{
			ID:      "external-provider-settings",
			Label:   "External provider settings",
			Status:  "warning",
			Message: "Optional provider credential could not be read.",
		}}
	}
	return []DiagnosticCheck{s.externalProvider.Check(ctx, ExternalProviderConfig{
		Name:           provider.Name,
		EndpointURL:    provider.EndpointURL,
		APIKey:         apiKey,
		TimeoutSeconds: provider.TimeoutSeconds,
	})}
}

func decodeReport(record store.DiagnosticReport) (DiagnosticReport, error) {
	var report DiagnosticReport
	if err := json.Unmarshal([]byte(record.ReportJSON), &report); err != nil {
		return DiagnosticReport{}, err
	}
	report.ID = record.ID
	report.Status = record.Status
	report.Summary = record.Summary
	report.CreatedAt = record.CreatedAt
	return report, nil
}

func bytesToMB(value uint64) uint64 {
	return value / 1024 / 1024
}

func statusForCPU(cpuCount int) string {
	if cpuCount < 1 {
		return "error"
	}
	if cpuCount < 2 {
		return "warning"
	}
	return "ok"
}

func messageForCPU(cpuCount int) string {
	if cpuCount < 1 {
		return "CPU information is unavailable."
	}
	if cpuCount < 2 {
		return "Single logical CPU detected; service capacity may be limited."
	}
	return "CPU capacity is suitable for baseline operations."
}

func statusForLoopback() string {
	addrs, err := net.LookupHost("localhost")
	if err != nil || len(addrs) == 0 {
		return "warning"
	}
	return "ok"
}

type TLSResult struct {
	Status   string
	Message  string
	Evidence string
}

func (netResolver) LookupHost(ctx context.Context, host string) ([]string, error) {
	return net.DefaultResolver.LookupHost(ctx, host)
}

func (defaultTLSChecker) Check(ctx context.Context, domain string) TLSResult {
	dialer := tls.Dialer{NetDialer: &net.Dialer{Timeout: 3 * time.Second}, Config: &tls.Config{ServerName: domain, MinVersion: tls.VersionTLS12}}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(domain, "443"))
	if err != nil {
		return TLSResult{
			Status:   "warning",
			Message:  "Configured panel domain TLS status could not be read.",
			Evidence: domain + ":443",
		}
	}
	defer conn.Close()

	tlsConn, ok := conn.(*tls.Conn)
	if !ok || len(tlsConn.ConnectionState().PeerCertificates) == 0 {
		return TLSResult{
			Status:   "warning",
			Message:  "Configured panel domain did not return certificate details.",
			Evidence: domain + ":443",
		}
	}
	cert := tlsConn.ConnectionState().PeerCertificates[0]
	return TLSResult{
		Status:   "ok",
		Message:  "Configured panel domain returned a TLS certificate.",
		Evidence: fmt.Sprintf("issuer=%s not_after=%s", cert.Issuer.CommonName, cert.NotAfter.UTC().Format(time.RFC3339)),
	}
}

func (p httpExternalProvider) Check(ctx context.Context, config ExternalProviderConfig) DiagnosticCheck {
	label := strings.TrimSpace(config.Name)
	if label == "" {
		label = "Operator provider"
	}
	timeout := time.Duration(config.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, config.EndpointURL, nil)
	if err != nil {
		return DiagnosticCheck{
			ID:      "external-provider",
			Label:   label,
			Status:  "warning",
			Message: "Optional provider request could not be prepared.",
		}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "omo-diagnostics/1")
	if strings.TrimSpace(config.APIKey) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(config.APIKey))
	}

	client := p.client
	if client == nil {
		client = &http.Client{}
	}
	resp, err := client.Do(req)
	if err != nil {
		return DiagnosticCheck{
			ID:       "external-provider",
			Label:    label,
			Status:   "warning",
			Message:  "Optional provider did not return a health response.",
			Evidence: config.EndpointURL,
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return DiagnosticCheck{
			ID:       "external-provider",
			Label:    label,
			Status:   "warning",
			Message:  "Optional provider returned an advisory status.",
			Evidence: fmt.Sprintf("status=%d endpoint=%s", resp.StatusCode, config.EndpointURL),
		}
	}
	return DiagnosticCheck{
		ID:       "external-provider",
		Label:    label,
		Status:   "ok",
		Message:  "Optional provider returned a health response.",
		Evidence: fmt.Sprintf("status=%d endpoint=%s", resp.StatusCode, config.EndpointURL),
	}
}

func reportStatus(checks []DiagnosticCheck) string {
	status := "ok"
	for _, check := range checks {
		if check.Status == "error" {
			return "error"
		}
		if check.Status == "warning" {
			status = "warning"
		}
	}
	return status
}

func summaryFor(status string) string {
	switch status {
	case "error":
		return "Server checkup found an issue that needs operator attention."
	case "warning":
		return "Server checkup completed with advisory findings."
	default:
		return "Server checkup completed successfully."
	}
}
