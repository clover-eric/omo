package caddy

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"
)

type PublicIPProbe struct {
	Client *http.Client
}

func (p PublicIPProbe) ExpectedIPs(ctx context.Context) []string {
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}

	var ips []string
	for _, url := range []string{"https://api.ipify.org", "https://api64.ipify.org"} {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		body := make([]byte, 128)
		n, _ := resp.Body.Read(body)
		_ = resp.Body.Close()
		ip := strings.TrimSpace(string(body[:n]))
		if parsed := net.ParseIP(ip); parsed != nil {
			ips = append(ips, parsed.String())
		}
	}
	return uniqueStrings(ips)
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
