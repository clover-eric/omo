package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"os/exec"

	"omo/internal/caddy"
)

type CaddyPhase2Hook struct {
	Manager     *caddy.Manager
	ExpectedIPs []string
	IPProbe     caddy.PublicIPProbe
	Upstream    string
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
	result := Phase2Result{
		ResolvedIPs:       domainCheck.ResolvedIPs,
		PortsAvailable:    available,
		CertificateIssuer: cert.Issuer,
		Message:           cert.UserMessage,
		EntryMode:         "https_domain",
		SecurityState:     "ready",
	}
	return result, nil
}

func isCaddyUnavailable(err error) bool {
	return errors.Is(err, exec.ErrNotFound)
}
