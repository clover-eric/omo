package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"omo/internal/caddy"
)

type CaddyPhase2Hook struct {
	Manager     *caddy.Manager
	ExpectedIPs []string
	IPProbe     caddy.PublicIPProbe
	Upstream    string
	TLSWait     time.Duration
	TLSPoll     time.Duration
}

func (h CaddyPhase2Hook) Run(ctx context.Context, domain string) (Phase2Result, error) {
	if h.Manager == nil {
		return Phase2Result{}, fmt.Errorf("caddy manager is not configured")
	}
	if !h.Manager.Available() {
		result := Phase2Result{
			Message:       "Caddy 暂不可用，已保留临时初始化入口，请安装或修复 Caddy 后重试 HTTPS 入口配置。",
			EntryMode:     "temporary_http",
			SecurityState: "degraded",
			Warnings:      []string{"当前未启用正式 HTTPS 面板入口。", "请确认 Caddy 已安装且可执行，再重新运行初始化入口配置。"},
		}
		return Phase2Result{}, Phase2FallbackError{
			Code:    "CADDY_UNAVAILABLE",
			Message: result.Message,
			Result:  result,
			Cause:   exec.ErrNotFound,
		}
	}
	upstream := h.Upstream
	if upstream == "" {
		upstream = "127.0.0.1:8080"
	}

	expectedIPs := h.ExpectedIPs
	if len(expectedIPs) == 0 {
		expectedIPs = h.IPProbe.ExpectedIPs(ctx)
	}
	domainCheck, err := h.Manager.CheckDomain(ctx, domain, expectedIPs)
	if err != nil {
		return Phase2Result{}, fmt.Errorf("%s: %w", domainCheck.InternalCode, err)
	}
	portChecks, err := h.Manager.CheckPorts(ctx, 80, 443)
	if err != nil {
		if h.configContainsDomain(domain) {
			cert := h.waitForCertificate(ctx, domain)
			if cert.Available {
				return h.readyResult(domainCheck.ResolvedIPs, []int{80, 443}, cert, "HTTPS entry is already active."), nil
			}
			return Phase2Result{}, h.tlsNotReadyError(domainCheck.ResolvedIPs, nil, cert, err)
		}
		return Phase2Result{}, err
	}
	available := make([]int, 0, len(portChecks))
	for _, check := range portChecks {
		if check.Available {
			available = append(available, check.Port)
		}
	}

	rendered := h.Manager.RenderConfig(domain, upstream)
	if err := h.Manager.ApplyConfig(ctx, rendered); err != nil {
		if isCaddyUnavailable(err) {
			result := Phase2Result{
				ResolvedIPs:    domainCheck.ResolvedIPs,
				PortsAvailable: available,
				Message:        "Caddy 暂不可用，已保留临时初始化入口，请安装或修复 Caddy 后重试 HTTPS 入口配置。",
				EntryMode:      "temporary_http",
				SecurityState:  "degraded",
				Warnings:       []string{"当前未启用正式 HTTPS 面板入口。", "请确认 Caddy 已安装且可执行，再重新运行初始化入口配置。"},
			}
			return Phase2Result{}, Phase2FallbackError{
				Code:    "CADDY_UNAVAILABLE",
				Message: result.Message,
				Result:  result,
				Cause:   err,
			}
		}
		return Phase2Result{}, err
	}

	cert := h.Manager.CertificateStatus(ctx, domain)
	if !cert.Available {
		cert = h.waitForCertificate(ctx, domain)
	}
	if !cert.Available {
		return Phase2Result{}, h.tlsNotReadyError(domainCheck.ResolvedIPs, available, cert, nil)
	}
	return h.readyResult(domainCheck.ResolvedIPs, available, cert, cert.UserMessage), nil
}

func isCaddyUnavailable(err error) bool {
	return errors.Is(err, exec.ErrNotFound)
}

func (h CaddyPhase2Hook) waitForCertificate(ctx context.Context, domain string) caddy.CertificateStatus {
	wait := h.TLSWait
	if wait == 0 {
		wait = 2 * time.Minute
	}
	poll := h.TLSPoll
	if poll == 0 {
		poll = 3 * time.Second
	}
	if wait < 0 {
		return h.Manager.CertificateStatus(ctx, domain)
	}

	deadline := time.NewTimer(wait)
	defer deadline.Stop()
	ticker := time.NewTicker(poll)
	defer ticker.Stop()

	last := h.Manager.CertificateStatus(ctx, domain)
	if last.Available {
		return last
	}
	for {
		select {
		case <-ctx.Done():
			return last
		case <-deadline.C:
			return last
		case <-ticker.C:
			last = h.Manager.CertificateStatus(ctx, domain)
			if last.Available {
				return last
			}
		}
	}
}

func (h CaddyPhase2Hook) tlsNotReadyError(resolved []string, ports []int, cert caddy.CertificateStatus, cause error) Phase2FallbackError {
	message := "HTTPS entry was configured, but a valid TLS handshake is not ready yet. Keep the temporary initialization entry open, confirm TCP 80/443 are reachable from the public internet, then retry."
	result := Phase2Result{
		ResolvedIPs:    resolved,
		PortsAvailable: ports,
		Message:        message,
		EntryMode:      "temporary_http",
		SecurityState:  "pending_tls",
		Warnings: []string{
			"OMO did not switch to the HTTPS dashboard because the domain certificate is not ready.",
			"Check Caddy logs and cloud security group rules for TCP 80 and 443.",
			cert.UserMessage,
		},
	}
	if cause == nil {
		cause = errors.New(cert.UserMessage)
	}
	return Phase2FallbackError{
		Code:    "TLS_CERTIFICATE_NOT_READY",
		Message: message,
		Result:  result,
		Cause:   cause,
	}
}

func (h CaddyPhase2Hook) readyResult(resolved []string, ports []int, cert caddy.CertificateStatus, message string) Phase2Result {
	return Phase2Result{
		ResolvedIPs:       resolved,
		PortsAvailable:    ports,
		CertificateIssuer: cert.Issuer,
		Message:           message,
		EntryMode:         "https_domain",
		SecurityState:     "ready",
	}
}

func (h CaddyPhase2Hook) configContainsDomain(domain string) bool {
	if h.Manager == nil || strings.TrimSpace(h.Manager.ConfigPath) == "" {
		return false
	}
	data, err := os.ReadFile(h.Manager.ConfigPath)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), strings.TrimSpace(domain))
}
