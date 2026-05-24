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
	"net/http"
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

type TokenResult struct {
	Subscription store.DistributionToken `json:"subscription"`
	Token        string                  `json:"token"`
	URL          string                  `json:"url"`
}

type ListResult struct {
	Subscriptions []store.DistributionToken `json:"subscriptions"`
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

func (s *Service) Create(ctx context.Context, req CreateRequest) (TokenResult, error) {
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
	return TokenResult{Subscription: record, Token: token, URL: s.publicURL(token)}, nil
}

func (s *Service) Rotate(ctx context.Context, id string) (TokenResult, error) {
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
	return TokenResult{Subscription: *record, Token: token, URL: s.publicURL(token)}, nil
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
		return PublicResponse{ContentType: "text/plain; charset=utf-8", Body: []byte(publicURL + "\n")}, nil
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
	default:
		return "html"
	}
}

func (s *Service) publicURL(token string) string {
	base := s.baseURL
	if base == "" {
		base = "http://127.0.0.1:8080"
	}
	return base + "/s/" + token
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
		if instance.Status == "active" {
			active = append(active, instance)
		}
	}
	return active, nil
}

func singBoxDescriptor(name string, publicURL string, instances []store.ServiceInstance) map[string]any {
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
		"inbounds": []map[string]any{},
		"outbounds": []map[string]any{{
			"type": "direct",
			"tag":  "direct",
		}},
		"route": map[string]any{
			"final": "direct",
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
	for _, service := range serviceDescriptors(instances) {
		b.WriteString(fmt.Sprintf("# service: %s profile=%s port=%d config=%s\n", service["name"], service["profileId"], service["listenPort"], service["configVersion"]))
	}
	b.WriteString(fmt.Sprintf("proxies: []\nproxy-groups:\n  - name: %q\n    type: select\n    proxies:\n      - DIRECT\nrules:\n  - MATCH,DIRECT\n", name))
	return b.String()
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
	return `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>OMO Smart Subscription</title>
  <style>
    body{font-family:Inter,system-ui,sans-serif;margin:0;background:#f6f7f9;color:#1b1f27}
    main{max-width:720px;margin:0 auto;padding:32px 20px}
    h1{font-size:28px;margin:0 0 8px}
    p{color:#607085;line-height:1.5}
    .links{display:grid;gap:10px;margin-top:24px}
    a{border:1px solid #cfd7e3;border-radius:8px;color:#0b5d70;padding:12px 14px;text-decoration:none;background:#fff}
    code{background:#eef3f5;border-radius:6px;padding:2px 6px}
  </style>
</head>
<body>
  <main>
    <p>OMO Boundary Operations</p>
    <h1>` + escapedName + `</h1>
    <p>Select an import format for authorized configuration distribution.</p>
    <div class="links">
      <a href="` + escapedURL + `?format=sing-box">sing-box format</a>
      <a href="` + escapedURL + `?format=clash">Clash/Mihomo format</a>
      <a href="` + escapedURL + `?format=uri">Direct subscription URL</a>
      <a href="` + escapedURL + `?format=qr">QR code SVG</a>
    </div>
    <p><code>` + escapedURL + `</code></p>
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
