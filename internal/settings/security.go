package settings

import (
	"context"
	"net"
	"strings"
)

type Store interface {
	GetSetting(ctx context.Context, key string) (string, bool, error)
}

type PanelAccessPolicy struct {
	Ready        bool
	Domain       string
	HTTPSOnly    bool
	AllowLocalIP bool
}

func LoadPanelAccessPolicy(ctx context.Context, appStore Store) (PanelAccessPolicy, error) {
	_, ready, err := appStore.GetSetting(ctx, "bootstrap.phase1_complete")
	if err != nil {
		return PanelAccessPolicy{}, err
	}
	domain, _, err := appStore.GetSetting(ctx, "bootstrap.domain")
	if err != nil {
		return PanelAccessPolicy{}, err
	}
	return PanelAccessPolicy{
		Ready:        ready,
		Domain:       domain,
		HTTPSOnly:    ready && domain != "",
		AllowLocalIP: !ready,
	}, nil
}

func HostMatchesDomain(host string, domain string) bool {
	host = strings.TrimSpace(host)
	domain = strings.TrimSpace(domain)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	domain = strings.TrimSuffix(strings.ToLower(domain), ".")
	return host == domain
}
