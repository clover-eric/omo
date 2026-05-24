package caddy

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Manager struct {
	ConfigPath          string
	Runner              Runner
	Resolver            Resolver
	Dialer              Dialer
	CertificateStatusFn func(ctx context.Context, domain string) CertificateStatus
}

type Runner interface {
	Run(ctx context.Context, name string, args ...string) error
}

type LookPathRunner interface {
	Runner
	LookPath(name string) (string, error)
}

type Resolver interface {
	LookupHost(ctx context.Context, host string) ([]string, error)
}

type Dialer interface {
	DialContext(ctx context.Context, network string, address string) (net.Conn, error)
}

type SystemRunner struct{}

func (SystemRunner) Run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s failed: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (SystemRunner) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

type NetResolver struct{}

func (NetResolver) LookupHost(ctx context.Context, host string) ([]string, error) {
	return net.DefaultResolver.LookupHost(ctx, host)
}

func NewManager(configPath string) *Manager {
	return &Manager{
		ConfigPath: configPath,
		Runner:     SystemRunner{},
		Resolver:   NetResolver{},
		Dialer:     &net.Dialer{Timeout: 2 * time.Second},
	}
}

type DomainCheck struct {
	Domain       string   `json:"domain"`
	ResolvedIPs  []string `json:"resolvedIps"`
	ExpectedIPs  []string `json:"expectedIps"`
	Matches      bool     `json:"matches"`
	UserMessage  string   `json:"userMessage"`
	InternalCode string   `json:"internalCode,omitempty"`
}

type PortCheck struct {
	Port        int    `json:"port"`
	Available   bool   `json:"available"`
	UserMessage string `json:"userMessage"`
}

type CertificateStatus struct {
	Domain      string     `json:"domain"`
	Available   bool       `json:"available"`
	Issuer      string     `json:"issuer,omitempty"`
	NotBefore   *time.Time `json:"notBefore,omitempty"`
	NotAfter    *time.Time `json:"notAfter,omitempty"`
	UserMessage string     `json:"userMessage"`
}

func (m *Manager) CheckDomain(ctx context.Context, domain string, expectedIPs []string) (DomainCheck, error) {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return DomainCheck{Domain: domain, Matches: false, UserMessage: "域名不能为空。", InternalCode: "DOMAIN_EMPTY"}, ErrDomainInvalid
	}

	hosts, err := m.resolver().LookupHost(ctx, domain)
	if err != nil {
		return DomainCheck{Domain: domain, ExpectedIPs: expectedIPs, Matches: false, UserMessage: "域名暂未解析到当前服务器，请检查 DNS 记录后重试。", InternalCode: "DOMAIN_LOOKUP_FAILED"}, err
	}
	matches := len(expectedIPs) == 0
	if len(expectedIPs) > 0 {
		expected := map[string]bool{}
		for _, ip := range expectedIPs {
			expected[ip] = true
		}
		for _, ip := range hosts {
			if expected[ip] {
				matches = true
				break
			}
		}
	}

	message := "域名解析检查通过。"
	code := ""
	if !matches {
		message = "域名暂未解析到当前服务器，请检查 DNS 记录后重试。"
		code = "DOMAIN_NOT_RESOLVED"
	}

	check := DomainCheck{Domain: domain, ResolvedIPs: hosts, ExpectedIPs: expectedIPs, Matches: matches, UserMessage: message, InternalCode: code}
	if !matches {
		return check, ErrDomainNotResolved
	}
	return check, nil
}

func (m *Manager) Available() bool {
	return m.lookPath("caddy") == nil
}

func (m *Manager) CheckPorts(ctx context.Context, ports ...int) ([]PortCheck, error) {
	var checks []PortCheck
	var unavailable []string
	for _, port := range ports {
		address := fmt.Sprintf("127.0.0.1:%d", port)
		conn, err := m.dialer().DialContext(ctx, "tcp", address)
		if err == nil {
			_ = conn.Close()
			checks = append(checks, PortCheck{Port: port, Available: false, UserMessage: fmt.Sprintf("端口 %d 已被占用。", port)})
			unavailable = append(unavailable, fmt.Sprint(port))
			continue
		}
		checks = append(checks, PortCheck{Port: port, Available: true, UserMessage: fmt.Sprintf("端口 %d 可用于入口服务。", port)})
	}
	if len(unavailable) > 0 {
		return checks, fmt.Errorf("%w: %s", ErrPortUnavailable, strings.Join(unavailable, ","))
	}
	return checks, nil
}

func (m *Manager) RenderConfig(domain string, upstream string) string {
	return fmt.Sprintf(`%s {
	encode zstd gzip
	header {
		Strict-Transport-Security "max-age=31536000; includeSubDomains"
		X-Content-Type-Options "nosniff"
		Referrer-Policy "same-origin"
	}
	reverse_proxy %s
}
`, strings.TrimSpace(domain), strings.TrimSpace(upstream))
}

func (m *Manager) ApplyConfig(ctx context.Context, rendered string) error {
	configPath := m.ConfigPath
	if configPath == "" {
		return ErrConfigPathEmpty
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return err
	}

	previous, hadPrevious, err := readMaybe(configPath)
	if err != nil {
		return err
	}

	tmp := configPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(rendered), 0o644); err != nil {
		return err
	}

	if err := m.runner().Run(ctx, "caddy", "validate", "--config", tmp, "--adapter", "caddyfile"); err != nil {
		_ = os.Remove(tmp)
		return err
	}

	if err := os.Rename(tmp, configPath); err != nil {
		_ = os.Remove(tmp)
		return err
	}

	if err := m.runner().Run(ctx, "caddy", "reload", "--config", configPath, "--adapter", "caddyfile"); err != nil {
		if hadPrevious {
			_ = os.WriteFile(configPath, previous, 0o644)
			_ = m.runner().Run(ctx, "caddy", "reload", "--config", configPath, "--adapter", "caddyfile")
		}
		return err
	}

	return nil
}

func (m *Manager) CertificateStatus(ctx context.Context, domain string) CertificateStatus {
	if m.CertificateStatusFn != nil {
		return m.CertificateStatusFn(ctx, domain)
	}
	dialer := tls.Dialer{NetDialer: &net.Dialer{Timeout: 3 * time.Second}, Config: &tls.Config{ServerName: domain, MinVersion: tls.VersionTLS12}}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(domain, "443"))
	if err != nil {
		return CertificateStatus{Domain: domain, Available: false, UserMessage: "暂未读取到可用证书状态。"}
	}
	defer conn.Close()

	tlsConn, ok := conn.(*tls.Conn)
	if !ok || len(tlsConn.ConnectionState().PeerCertificates) == 0 {
		return CertificateStatus{Domain: domain, Available: false, UserMessage: "暂未读取到可用证书状态。"}
	}
	cert := tlsConn.ConnectionState().PeerCertificates[0]
	notBefore := cert.NotBefore.UTC()
	notAfter := cert.NotAfter.UTC()
	return CertificateStatus{
		Domain:      domain,
		Available:   true,
		Issuer:      cert.Issuer.CommonName,
		NotBefore:   &notBefore,
		NotAfter:    &notAfter,
		UserMessage: "证书状态正常。",
	}
}

func (m *Manager) runner() Runner {
	if m.Runner == nil {
		return SystemRunner{}
	}
	return m.Runner
}

func (m *Manager) lookPath(name string) error {
	if runner, ok := m.runner().(LookPathRunner); ok {
		_, err := runner.LookPath(name)
		return err
	}
	_, err := exec.LookPath(name)
	return err
}

func (m *Manager) resolver() Resolver {
	if m.Resolver == nil {
		return NetResolver{}
	}
	return m.Resolver
}

func (m *Manager) dialer() Dialer {
	if m.Dialer == nil {
		return &net.Dialer{Timeout: 2 * time.Second}
	}
	return m.Dialer
}

func readMaybe(path string) ([]byte, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	return data, err == nil, err
}

var (
	ErrDomainInvalid     = errors.New("domain invalid")
	ErrDomainNotResolved = errors.New("domain not resolved")
	ErrPortUnavailable   = errors.New("port unavailable")
	ErrConfigPathEmpty   = errors.New("caddy config path empty")
)
