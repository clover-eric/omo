package subscription

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"omo/internal/auth"
	"omo/internal/store"
)

var (
	ErrInvalidInput         = errors.New("invalid subscription input")
	ErrSubscriptionNotFound = errors.New("subscription not found")
)

type Store interface {
	CreateDistributionToken(ctx context.Context, name string, tokenHash string, expiresAt *time.Time) (store.DistributionToken, error)
	ListDistributionTokens(ctx context.Context) ([]store.DistributionToken, error)
	DistributionTokenByHash(ctx context.Context, tokenHash string, now time.Time) (*store.DistributionToken, error)
	RotateDistributionToken(ctx context.Context, id string, tokenHash string) (*store.DistributionToken, error)
	UpdateDistributionToken(ctx context.Context, id string, name *string, status *string, expiresAt *time.Time, clearExpiresAt bool) (*store.DistributionToken, error)
	DeleteDistributionToken(ctx context.Context, id string) (bool, error)
	RecordSubscriptionRequest(ctx context.Context, distributionTokenID string, clientHint string, remoteAddrHash string) error
}

type serviceInstanceReader interface {
	ListServiceInstances(ctx context.Context) ([]store.ServiceInstance, error)
}

type Service struct {
	store   Store
	baseURL string
}

type CreateRequest struct {
	Name      string `json:"name"`
	ExpiresAt string `json:"expiresAt"`
}

type UpdateRequest struct {
	Name           *string `json:"name"`
	Status         *string `json:"status"`
	ExpiresAt      *string `json:"expiresAt"`
	ClearExpiresAt bool    `json:"clearExpiresAt"`
}

type TokenResult struct {
	Subscription store.DistributionToken `json:"subscription"`
	Token        string                  `json:"token"`
	URL          string                  `json:"url"`
}

type ListResult struct {
	Subscriptions []store.DistributionToken `json:"subscriptions"`
}

type UpdateResult struct {
	Subscription store.DistributionToken `json:"subscription"`
}

type DeleteResult struct {
	Deleted bool `json:"deleted"`
}

type PublicRequest struct {
	Token      string
	Format     string
	ClientHint string
	RemoteAddr string
	BaseURL    string
}

type PublicResponse struct {
	ContentType string
	Body        []byte
}

func NewService(appStore Store, baseURL string) *Service {
	return &Service{store: appStore, baseURL: strings.TrimRight(baseURL, "/")}
}

func (s *Service) List(ctx context.Context) (ListResult, error) {
	records, err := s.store.ListDistributionTokens(ctx)
	if err != nil {
		return ListResult{}, err
	}
	return ListResult{Subscriptions: records}, nil
}

func (s *Service) Create(ctx context.Context, req CreateRequest, baseURL ...string) (TokenResult, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" || len(name) > 80 {
		return TokenResult{}, ErrInvalidInput
	}
	var expiresAt *time.Time
	if strings.TrimSpace(req.ExpiresAt) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(req.ExpiresAt))
		if err != nil {
			return TokenResult{}, ErrInvalidInput
		}
		expiresAt = &parsed
	}
	token, err := auth.GenerateToken(32)
	if err != nil {
		return TokenResult{}, err
	}
	record, err := s.store.CreateDistributionToken(ctx, name, auth.HashToken(token), expiresAt)
	if err != nil {
		return TokenResult{}, err
	}
	return TokenResult{Subscription: record, Token: token, URL: s.publicURL(token, firstBaseURL(baseURL))}, nil
}

func (s *Service) Rotate(ctx context.Context, id string, baseURL ...string) (TokenResult, error) {
	token, err := auth.GenerateToken(32)
	if err != nil {
		return TokenResult{}, err
	}
	record, err := s.store.RotateDistributionToken(ctx, strings.TrimSpace(id), auth.HashToken(token))
	if err != nil {
		return TokenResult{}, err
	}
	if record == nil {
		return TokenResult{}, ErrSubscriptionNotFound
	}
	return TokenResult{Subscription: *record, Token: token, URL: s.publicURL(token, firstBaseURL(baseURL))}, nil
}

func (s *Service) Update(ctx context.Context, id string, req UpdateRequest) (UpdateResult, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return UpdateResult{}, ErrSubscriptionNotFound
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" || len(name) > 80 {
			return UpdateResult{}, ErrInvalidInput
		}
		*req.Name = name
	}
	if req.Status != nil {
		status := strings.TrimSpace(*req.Status)
		if status != "active" && status != "disabled" {
			return UpdateResult{}, ErrInvalidInput
		}
		*req.Status = status
	}
	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		raw := strings.TrimSpace(*req.ExpiresAt)
		if raw == "" {
			req.ClearExpiresAt = true
		} else {
			parsed, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				return UpdateResult{}, ErrInvalidInput
			}
			expiresAt = &parsed
		}
	}
	record, err := s.store.UpdateDistributionToken(ctx, id, req.Name, req.Status, expiresAt, req.ClearExpiresAt)
	if err != nil {
		return UpdateResult{}, err
	}
	if record == nil {
		return UpdateResult{}, ErrSubscriptionNotFound
	}
	return UpdateResult{Subscription: *record}, nil
}

func (s *Service) Delete(ctx context.Context, id string) (DeleteResult, error) {
	deleted, err := s.store.DeleteDistributionToken(ctx, strings.TrimSpace(id))
	if err != nil {
		return DeleteResult{}, err
	}
	if !deleted {
		return DeleteResult{}, ErrSubscriptionNotFound
	}
	return DeleteResult{Deleted: true}, nil
}

func (s *Service) PublicContent(ctx context.Context, req PublicRequest) (PublicResponse, error) {
	token := strings.TrimSpace(req.Token)
	if token == "" {
		return PublicResponse{}, ErrSubscriptionNotFound
	}
	record, err := s.store.DistributionTokenByHash(ctx, auth.HashToken(token), time.Now().UTC())
	if err != nil {
		return PublicResponse{}, err
	}
	if record == nil {
		return PublicResponse{}, ErrSubscriptionNotFound
	}
	remoteHash := hashRemoteAddr(req.RemoteAddr)
	if err := s.store.RecordSubscriptionRequest(ctx, record.ID, strings.TrimSpace(req.ClientHint), remoteHash); err != nil {
		return PublicResponse{}, err
	}
	baseURL := strings.TrimRight(req.BaseURL, "/")
	if baseURL == "" {
		baseURL = s.baseURL
	}
	publicURL := baseURL + "/s/" + token
	instances, err := s.activeServiceInstances(ctx)
	if err != nil {
		return PublicResponse{}, err
	}
	format := normalizeFormat(req.Format, req.ClientHint)
	switch format {
	case "sing-box":
		return jsonResponse(singBoxDescriptor(record.Name, publicURL, instances))
	case "clash":
		return PublicResponse{ContentType: "text/plain; charset=utf-8", Body: []byte(clashDescriptor(record.Name, publicURL, instances))}, nil
	case "uri":
		return PublicResponse{ContentType: "text/plain; charset=utf-8", Body: []byte(uriDescriptor(publicURL, instances))}, nil
	case "qr":
		body, err := qrSVG(publicURL)
		if err != nil {
			return PublicResponse{}, err
		}
		return PublicResponse{ContentType: "image/svg+xml; charset=utf-8", Body: body}, nil
	default:
		return PublicResponse{ContentType: "text/html; charset=utf-8", Body: []byte(importPage(record.Name, publicURL))}, nil
	}
}

func normalizeFormat(format string, clientHint string) string {
	format = strings.ToLower(strings.TrimSpace(format))
	if format != "" {
		return format
	}
	hint := strings.ToLower(clientHint)
	switch {
	case strings.Contains(hint, "sing-box"):
		return "sing-box"
	case strings.Contains(hint, "clash"), strings.Contains(hint, "mihomo"):
		return "clash"
	case strings.Contains(hint, "shadowrocket"), strings.Contains(hint, "stash"), strings.Contains(hint, "nekobox"):
		return "uri"
	default:
		return "html"
	}
}

func (s *Service) publicURL(token string, overrideBaseURL string) string {
	base := strings.TrimRight(strings.TrimSpace(overrideBaseURL), "/")
	if base == "" {
		base = s.baseURL
	}
	if base == "" {
		base = "http://127.0.0.1:8080"
	}
	return base + "/s/" + token
}

func firstBaseURL(baseURLs []string) string {
	if len(baseURLs) == 0 {
		return ""
	}
	return baseURLs[0]
}

func (s *Service) activeServiceInstances(ctx context.Context) ([]store.ServiceInstance, error) {
	reader, ok := s.store.(serviceInstanceReader)
	if !ok {
		return nil, nil
	}
	instances, err := reader.ListServiceInstances(ctx)
	if err != nil {
		return nil, err
	}
	active := make([]store.ServiceInstance, 0, len(instances))
	for _, instance := range instances {
		if distributionReady(instance) {
			active = append(active, instance)
		}
	}
	sort.SliceStable(active, func(i, j int) bool {
		if !active[i].UpdatedAt.Equal(active[j].UpdatedAt) {
			return active[i].UpdatedAt.After(active[j].UpdatedAt)
		}
		if !active[i].CreatedAt.Equal(active[j].CreatedAt) {
			return active[i].CreatedAt.After(active[j].CreatedAt)
		}
		return active[i].ID > active[j].ID
	})
	if len(active) > 1 {
		return active[:1], nil
	}
	return active, nil
}

func distributionReady(instance store.ServiceInstance) bool {
	return instance.Status == "active" &&
		distributionSupportedProfile(instance.ProfileID) &&
		instance.ListenPort > 0 &&
		strings.TrimSpace(instance.AccessUsername) != "" &&
		strings.TrimSpace(instance.AccessPassword) != "" &&
		strings.TrimSpace(instance.AccessPath) != "" &&
		isAppliedConfigVersion(instance.ConfigVersion)
}

func distributionSupportedProfile(profileID string) bool {
	return strings.TrimSpace(profileID) == "standard-secure-access"
}

func isAppliedConfigVersion(version string) bool {
	version = strings.TrimSpace(version)
	if len(version) != len("20060102150405") {
		return false
	}
	for _, ch := range version {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func singBoxDescriptor(name string, publicURL string, instances []store.ServiceInstance) map[string]any {
	outbounds := singBoxOutbounds(publicURL, instances)
	final := "direct"
	if len(outbounds) > 1 {
		if tag, ok := outbounds[0]["tag"].(string); ok && tag != "" {
			final = tag
		}
	}
	return map[string]any{
		"_omo": map[string]any{
			"name":      name,
			"source":    publicURL,
			"managedBy": "omo",
			"services":  serviceDescriptors(instances),
		},
		"log": map[string]any{
			"level": "info",
		},
		"inbounds":  []map[string]any{},
		"outbounds": outbounds,
		"route": map[string]any{
			"final": final,
		},
		"experimental": map[string]any{
			"cache_file": map[string]any{
				"enabled": true,
			},
		},
	}
}

func clashDescriptor(name string, publicURL string, instances []store.ServiceInstance) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# managed-by: omo\n# source: %s\n", publicURL))
	proxies := proxyDescriptors(publicURL, instances)
	for _, proxy := range proxies {
		b.WriteString(fmt.Sprintf("# service: %s profile=%s config=%s\n", proxy.Name, proxy.ProfileID, proxy.ConfigVersion))
	}
	b.WriteString("proxies:\n")
	if len(proxies) == 0 {
		b.WriteString("  []\n")
	} else {
		for _, proxy := range proxies {
			b.WriteString(fmt.Sprintf("  - name: %q\n", proxy.Name))
			b.WriteString("    type: trojan\n")
			b.WriteString(fmt.Sprintf("    server: %q\n", proxy.Host))
			b.WriteString(fmt.Sprintf("    port: %d\n", proxy.Port))
			b.WriteString(fmt.Sprintf("    password: %q\n", proxy.Password))
			b.WriteString(fmt.Sprintf("    sni: %q\n", proxy.Host))
			b.WriteString("    skip-cert-verify: false\n")
			b.WriteString("    network: ws\n")
			b.WriteString("    ws-opts:\n")
			b.WriteString(fmt.Sprintf("      path: %q\n", proxy.Path))
			b.WriteString("      headers:\n")
			b.WriteString(fmt.Sprintf("        Host: %q\n", proxy.Host))
		}
	}
	b.WriteString("proxy-groups:\n")
	b.WriteString(fmt.Sprintf("  - name: %q\n    type: select\n    proxies:\n", name))
	for _, proxy := range proxies {
		b.WriteString(fmt.Sprintf("      - %q\n", proxy.Name))
	}
	b.WriteString("      - DIRECT\nrules:\n")
	if len(proxies) == 0 {
		b.WriteString("  - MATCH,DIRECT\n")
	} else {
		b.WriteString(fmt.Sprintf("  - MATCH,%s\n", sanitizeClashPolicyName(name)))
	}
	return b.String()
}

func uriDescriptor(publicURL string, instances []store.ServiceInstance) string {
	proxies := proxyDescriptors(publicURL, instances)
	lines := make([]string, 0, len(proxies))
	for _, proxy := range proxies {
		lines = append(lines, trojanURI(proxy))
	}
	if len(lines) == 0 {
		return "# OMO: no active distribution-ready service is available. Apply a service in Service Library first.\n"
	}
	return strings.Join(lines, "\n") + "\n"
}

type proxyDescriptor struct {
	Name          string
	ProfileID     string
	ConfigVersion string
	Host          string
	Port          int
	Path          string
	Password      string
}

func proxyDescriptors(publicURL string, instances []store.ServiceInstance) []proxyDescriptor {
	host, port := subscriptionHostPort(publicURL)
	if host == "" {
		return nil
	}
	proxies := make([]proxyDescriptor, 0, len(instances))
	for _, instance := range instances {
		password := strings.TrimSpace(instance.AccessPassword)
		if password == "" {
			continue
		}
		path := strings.TrimSpace(instance.AccessPath)
		if path == "" {
			path = "/omo-access/" + strings.Trim(strings.TrimSpace(instance.ProfileID), "/")
		}
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		name := strings.TrimSpace(instance.DisplayName)
		if name == "" {
			name = instance.ProfileID
		}
		proxies = append(proxies, proxyDescriptor{
			Name:          name,
			ProfileID:     instance.ProfileID,
			ConfigVersion: instance.ConfigVersion,
			Host:          host,
			Port:          port,
			Path:          path,
			Password:      password,
		})
	}
	return proxies
}

func singBoxOutbounds(publicURL string, instances []store.ServiceInstance) []map[string]any {
	proxies := proxyDescriptors(publicURL, instances)
	outbounds := make([]map[string]any, 0, len(proxies)+1)
	for _, proxy := range proxies {
		outbounds = append(outbounds, map[string]any{
			"type":        "trojan",
			"tag":         proxy.Name,
			"server":      proxy.Host,
			"server_port": proxy.Port,
			"password":    proxy.Password,
			"tls": map[string]any{
				"enabled":     true,
				"server_name": proxy.Host,
			},
			"transport": map[string]any{
				"type": "ws",
				"path": proxy.Path,
				"headers": map[string]string{
					"Host": proxy.Host,
				},
			},
		})
	}
	outbounds = append(outbounds, map[string]any{
		"type": "direct",
		"tag":  "direct",
	})
	return outbounds
}

func trojanURI(proxy proxyDescriptor) string {
	u := url.URL{
		Scheme:   "trojan",
		User:     url.User(proxy.Password),
		Host:     net.JoinHostPort(proxy.Host, strconv.Itoa(proxy.Port)),
		Fragment: proxy.Name,
	}
	query := u.Query()
	query.Set("security", "tls")
	query.Set("sni", proxy.Host)
	query.Set("type", "ws")
	query.Set("host", proxy.Host)
	query.Set("path", proxy.Path)
	u.RawQuery = query.Encode()
	return u.String()
}

func subscriptionHostPort(publicURL string) (string, int) {
	parsed, err := url.Parse(strings.TrimSpace(publicURL))
	if err != nil || parsed.Host == "" {
		return "", 0
	}
	host := parsed.Hostname()
	port := parsed.Port()
	if host == "" {
		return "", 0
	}
	if port != "" {
		parsedPort, err := strconv.Atoi(port)
		if err == nil && parsedPort > 0 && parsedPort <= 65535 {
			return host, parsedPort
		}
	}
	if parsed.Scheme == "http" {
		return host, 80
	}
	return host, 443
}

func sanitizeClashPolicyName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "\r", " ")
	name = strings.ReplaceAll(name, "\n", " ")
	name = strings.ReplaceAll(name, ",", " ")
	if name == "" {
		return "OMO"
	}
	return name
}

func serviceDescriptors(instances []store.ServiceInstance) []map[string]any {
	services := make([]map[string]any, 0, len(instances))
	for _, instance := range instances {
		services = append(services, map[string]any{
			"id":            instance.ID,
			"name":          instance.DisplayName,
			"profileId":     instance.ProfileID,
			"listenPort":    instance.ListenPort,
			"status":        instance.Status,
			"configVersion": instance.ConfigVersion,
			"updatedAt":     instance.UpdatedAt.Format(time.RFC3339),
		})
	}
	return services
}

func importPage(name string, publicURL string) string {
	escapedName := html.EscapeString(name)
	escapedURL := html.EscapeString(publicURL)
	directURL := escapedURL + "?format=uri"
	return `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>OMO Configuration Import</title>
  <style>
    :root{color-scheme:light dark}
    *{box-sizing:border-box}
    body{font-family:Inter,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;margin:0;background:#f4f7f8;color:#17232e}
    main{max-width:760px;margin:0 auto;padding:28px 18px 36px}
    .hero{background:#fff;border:1px solid #d8e1e7;border-radius:14px;padding:22px;box-shadow:0 12px 34px rgba(28,43,55,.08)}
    .eyebrow{color:#0f6b78;font-size:12px;font-weight:800;letter-spacing:.04em;margin:0 0 8px;text-transform:uppercase}
    h1{font-size:28px;line-height:1.16;margin:0}
    p{color:#667789;line-height:1.55;margin:10px 0 0}
    .steps{display:grid;gap:10px;margin:18px 0 0}
    .step{align-items:flex-start;background:#f8fbfb;border:1px solid #e0e8ed;border-radius:10px;display:grid;gap:4px;grid-template-columns:30px minmax(0,1fr);padding:12px}
    .step b{align-items:center;background:#e5f4f5;border-radius:999px;color:#0f6b78;display:flex;height:24px;justify-content:center;width:24px}
    .step strong{display:block;font-size:14px}
    .step span{color:#667789;font-size:13px;line-height:1.45}
    .links{display:grid;gap:10px;margin-top:18px}
    a,.copy{align-items:center;background:#0f6b78;border:0;border-radius:10px;color:#fff;display:flex;font:inherit;font-weight:800;justify-content:space-between;min-height:46px;padding:0 14px;text-decoration:none}
    a.secondary{background:#e5f4f5;color:#0f6b78}
    code{background:#111820;border-radius:10px;color:#dbe7ef;display:block;font-size:13px;line-height:1.45;margin-top:16px;overflow-wrap:anywhere;padding:12px}
    .hint{font-size:13px}
    @media (prefers-color-scheme:dark){
      body{background:#101419;color:#eef3f6}
      .hero{background:#151b23;border-color:#2b3542;box-shadow:none}
      p,.step span{color:#9aa7b7}
      .step{background:#111820;border-color:#2b3542}
      a.secondary{background:#1b3038;color:#68d3e3}
    }
  </style>
</head>
<body>
  <main>
    <div class="hero">
      <p class="eyebrow">OMO Import</p>
      <h1>` + escapedName + `</h1>
      <p>Authorized device import page. Choose the format that matches your client.</p>
      <p>Use this page for authorized device import. If scanning does not open your client automatically, choose one format below.</p>
      <p class="hint">This page is for authorized device import. If scanning does not open your client automatically, choose the matching format below.</p>
      <div class="steps">
        <div class="step"><b>1</b><div><strong>Choose client format</strong><span>sing-box uses JSON, Clash/Mihomo uses YAML, and mobile clients can use direct URI output.</span></div></div>
        <div class="step"><b>2</b><div><strong>Import on the device</strong><span>Open the matching link or copy it into the client subscription entry.</span></div></div>
        <div class="step"><b>3</b><div><strong>Manage in OMO</strong><span>Rotate, disable, or delete this distribution entry from the console later.</span></div></div>
      </div>
      <div class="links">
        <a href="` + escapedURL + `?format=sing-box">sing-box JSON <span>Open</span></a>
        <a href="` + escapedURL + `?format=clash">Clash/Mihomo YAML <span>Open</span></a>
        <a class="secondary" href="` + directURL + `">Direct URI <span>Open</span></a>
        <button class="copy" type="button" onclick="navigator.clipboard&&navigator.clipboard.writeText('` + directURL + `');this.textContent='Copied'">Copy direct URI</button>
      </div>
      <code>` + escapedURL + `</code>
    </div>
  </main>
</body>
</html>`
}

func jsonResponse(v any) (PublicResponse, error) {
	payload, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return PublicResponse{}, err
	}
	payload = append(payload, '\n')
	return PublicResponse{ContentType: "application/json; charset=utf-8", Body: payload}, nil
}

func hashRemoteAddr(remoteAddr string) string {
	if strings.TrimSpace(remoteAddr) == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(remoteAddr))
	return hex.EncodeToString(sum[:])
}

func BasicAuthToken(token string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(token))
}

func BaseURLFromRequest(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}
