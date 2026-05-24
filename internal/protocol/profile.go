package protocol

import (
	"errors"
	"fmt"
)

var (
	ErrDuplicateProfile = errors.New("duplicate service profile")
	ErrProfileNotFound  = errors.New("service profile not found")
)

type ServiceProfile struct {
	ID                string       `json:"id"`
	Version           string       `json:"version"`
	DisplayName       string       `json:"displayName"`
	Category          string       `json:"category"`
	Summary           string       `json:"summary"`
	ExpertProtocol    string       `json:"expertProtocol"`
	Transport         string       `json:"transport"`
	SecurityLayer     string       `json:"securityLayer"`
	RequiresDomain    bool         `json:"requiresDomain"`
	RequiresTLSCert   bool         `json:"requiresTLSCert"`
	RequiresUDP       bool         `json:"requiresUdp"`
	DefaultPortPolicy string       `json:"defaultPortPolicy"`
	Dependencies      []string     `json:"dependencies"`
	ClientFormats     []string     `json:"clientFormats"`
	ScoreWeights      ScoreWeights `json:"scoreWeights"`
	TemplateRef       string       `json:"templateRef"`
	GoldenTestRef     string       `json:"goldenTestRef"`
	RollbackStrategy  string       `json:"rollbackStrategy"`
}

type ScoreWeights struct {
	Latency       int `json:"latency"`
	Throughput    int `json:"throughput"`
	Security      int `json:"security"`
	Stability     int `json:"stability"`
	Compatibility int `json:"compatibility"`
	Resource      int `json:"resource"`
	Operations    int `json:"operations"`
	Resilience    int `json:"resilience"`
}

type Registry struct {
	profiles []ServiceProfile
	byID     map[string]ServiceProfile
}

func DefaultRegistry() (*Registry, error) {
	return NewRegistry(DefaultServiceProfiles())
}

func NewRegistry(profiles []ServiceProfile) (*Registry, error) {
	registry := &Registry{
		profiles: make([]ServiceProfile, 0, len(profiles)),
		byID:     make(map[string]ServiceProfile, len(profiles)),
	}
	for _, profile := range profiles {
		if err := validateProfile(profile); err != nil {
			return nil, err
		}
		if _, exists := registry.byID[profile.ID]; exists {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateProfile, profile.ID)
		}
		copied := cloneProfile(profile)
		registry.profiles = append(registry.profiles, copied)
		registry.byID[copied.ID] = copied
	}
	return registry, nil
}

func (r *Registry) List() []ServiceProfile {
	if r == nil {
		return nil
	}
	profiles := make([]ServiceProfile, 0, len(r.profiles))
	for _, profile := range r.profiles {
		profiles = append(profiles, cloneProfile(profile))
	}
	return profiles
}

func (r *Registry) Get(id string) (ServiceProfile, error) {
	if r == nil {
		return ServiceProfile{}, ErrProfileNotFound
	}
	profile, ok := r.byID[id]
	if !ok {
		return ServiceProfile{}, fmt.Errorf("%w: %s", ErrProfileNotFound, id)
	}
	return cloneProfile(profile), nil
}

func DefaultServiceProfiles() []ServiceProfile {
	return []ServiceProfile{
		{
			ID:              "standard-secure-access",
			Version:         "2026.05.1",
			DisplayName:     "Standard secure access",
			Category:        "standard",
			Summary:         "Balanced boundary access profile for managed infrastructure with a verified domain and certificate.",
			ExpertProtocol:  "sing-box tls/tcp",
			Transport:       "TCP over managed HTTPS entry",
			SecurityLayer:   "Domain certificate plus backend-generated credentials",
			RequiresDomain:  true,
			RequiresTLSCert: true,
			RequiresUDP:     false,
			DefaultPortPolicy: "Reuse the managed HTTPS entry on 443 when the panel entry is ready; otherwise wait for Phase 2 " +
				"entry recovery.",
			Dependencies: []string{
				"bootstrap.phase1_complete",
				"panel.https_domain_ready",
				"sing-box.healthy",
			},
			ClientFormats: []string{
				"sing-box",
				"Clash/Mihomo",
				"v2rayN/v2rayNG",
				"Shadowrocket",
				"Stash",
				"Hiddify",
			},
			ScoreWeights: ScoreWeights{
				Latency:       14,
				Throughput:    14,
				Security:      22,
				Stability:     18,
				Compatibility: 14,
				Resource:      8,
				Operations:    6,
				Resilience:    4,
			},
			TemplateRef:      "singbox/standard-secure-access@2026.05.1",
			GoldenTestRef:    "internal/protocol/testdata/standard-secure-access.golden.json",
			RollbackStrategy: "Keep the previous validated sing-box config active until the new config validates and the health check passes.",
		},
		{
			ID:              "high-throughput-access",
			Version:         "2026.05.1",
			DisplayName:     "High throughput access",
			Category:        "performance",
			Summary:         "Performance-oriented boundary access profile for owned networks where UDP is explicitly allowed.",
			ExpertProtocol:  "sing-box quic/udp",
			Transport:       "QUIC over managed UDP entry",
			SecurityLayer:   "Backend-generated credentials with domain-bound certificate material",
			RequiresDomain:  true,
			RequiresTLSCert: true,
			RequiresUDP:     true,
			DefaultPortPolicy: "Allocate a backend-managed UDP service port and require an operator-visible firewall confirmation " +
				"before apply.",
			Dependencies: []string{
				"bootstrap.phase1_complete",
				"panel.https_domain_ready",
				"sing-box.healthy",
				"udp.entry_allowed",
			},
			ClientFormats: []string{
				"sing-box",
				"NekoBox",
				"SFI/SFA/SFM",
				"Hiddify",
			},
			ScoreWeights: ScoreWeights{
				Latency:       18,
				Throughput:    26,
				Security:      16,
				Stability:     12,
				Compatibility: 8,
				Resource:      8,
				Operations:    6,
				Resilience:    6,
			},
			TemplateRef:      "singbox/high-throughput-access@2026.05.1",
			GoldenTestRef:    "internal/protocol/testdata/high-throughput-access.golden.json",
			RollbackStrategy: "Close the newly allocated UDP entry and restore the previous service config if validation or health checks fail.",
		},
		{
			ID:                "broad-compatibility-access",
			Version:           "2026.05.1",
			DisplayName:       "Broad compatibility access",
			Category:          "compatibility",
			Summary:           "Compatibility-first boundary access profile for teams that need predictable client support.",
			ExpertProtocol:    "sing-box tcp compatibility",
			Transport:         "TCP over managed service entry",
			SecurityLayer:     "Backend-generated credentials with optional domain certificate binding",
			RequiresDomain:    false,
			RequiresTLSCert:   false,
			RequiresUDP:       false,
			DefaultPortPolicy: "Prefer a backend-managed TCP service port; use the HTTPS entry only after domain readiness is confirmed.",
			Dependencies: []string{
				"bootstrap.phase1_complete",
				"sing-box.healthy",
			},
			ClientFormats: []string{
				"sing-box",
				"Clash/Mihomo",
				"v2rayN/v2rayNG",
				"Shadowrocket",
				"Stash",
				"NekoBox",
			},
			ScoreWeights: ScoreWeights{
				Latency:       12,
				Throughput:    12,
				Security:      16,
				Stability:     16,
				Compatibility: 26,
				Resource:      8,
				Operations:    6,
				Resilience:    4,
			},
			TemplateRef:      "singbox/broad-compatibility-access@2026.05.1",
			GoldenTestRef:    "internal/protocol/testdata/broad-compatibility-access.golden.json",
			RollbackStrategy: "Keep the previous instance active and discard the pending service entry if client compatibility validation fails.",
		},
		{
			ID:                "lightweight-fallback-access",
			Version:           "2026.05.1",
			DisplayName:       "Lightweight fallback access",
			Category:          "fallback",
			Summary:           "Resource-conscious boundary access profile for low-capacity servers or constrained client environments.",
			ExpertProtocol:    "sing-box lightweight tcp",
			Transport:         "TCP over backend-managed fallback entry",
			SecurityLayer:     "Backend-generated credentials with minimal runtime overhead",
			RequiresDomain:    false,
			RequiresTLSCert:   false,
			RequiresUDP:       false,
			DefaultPortPolicy: "Allocate a conservative backend-managed TCP service port after checking for local conflicts.",
			Dependencies: []string{
				"bootstrap.phase1_complete",
				"sing-box.healthy",
				"system.resources_available",
			},
			ClientFormats: []string{
				"sing-box",
				"Clash/Mihomo",
				"v2rayN/v2rayNG",
				"NekoBox",
				"Hiddify",
			},
			ScoreWeights: ScoreWeights{
				Latency:       10,
				Throughput:    8,
				Security:      16,
				Stability:     18,
				Compatibility: 16,
				Resource:      20,
				Operations:    8,
				Resilience:    4,
			},
			TemplateRef:      "singbox/lightweight-fallback-access@2026.05.1",
			GoldenTestRef:    "internal/protocol/testdata/lightweight-fallback-access.golden.json",
			RollbackStrategy: "Preserve the previous active entry and remove the fallback instance if resource or validation checks fail.",
		},
		{
			ID:                "mobile-optimized-access",
			Version:           "2026.05.1",
			DisplayName:       "Mobile optimized access",
			Category:          "mobility",
			Summary:           "Resilience-first boundary access profile for authorized mobile devices on changing networks.",
			ExpertProtocol:    "sing-box mobile resilient",
			Transport:         "Adaptive managed service entry with reconnect-oriented settings",
			SecurityLayer:     "Backend-generated credentials with domain certificate binding when available",
			RequiresDomain:    false,
			RequiresTLSCert:   false,
			RequiresUDP:       false,
			DefaultPortPolicy: "Prefer an already healthy managed entry; otherwise allocate a backend-managed TCP service port.",
			Dependencies: []string{
				"bootstrap.phase1_complete",
				"sing-box.healthy",
				"client.mobile_profile_present",
			},
			ClientFormats: []string{
				"sing-box",
				"SFI/SFA/SFM",
				"Shadowrocket",
				"Stash",
				"Hiddify",
				"NekoBox",
			},
			ScoreWeights: ScoreWeights{
				Latency:       16,
				Throughput:    10,
				Security:      16,
				Stability:     18,
				Compatibility: 12,
				Resource:      8,
				Operations:    6,
				Resilience:    14,
			},
			TemplateRef:      "singbox/mobile-optimized-access@2026.05.1",
			GoldenTestRef:    "internal/protocol/testdata/mobile-optimized-access.golden.json",
			RollbackStrategy: "Restore the previous mobile-capable entry if reconnect validation or client import checks fail.",
		},
	}
}

func validateProfile(profile ServiceProfile) error {
	switch {
	case profile.ID == "":
		return errors.New("service profile id is required")
	case profile.Version == "":
		return fmt.Errorf("service profile %s version is required", profile.ID)
	case profile.DisplayName == "":
		return fmt.Errorf("service profile %s display name is required", profile.ID)
	case profile.TemplateRef == "":
		return fmt.Errorf("service profile %s template ref is required", profile.ID)
	case profile.GoldenTestRef == "":
		return fmt.Errorf("service profile %s golden test ref is required", profile.ID)
	case profile.RollbackStrategy == "":
		return fmt.Errorf("service profile %s rollback strategy is required", profile.ID)
	}
	if profile.ScoreWeights.total() != 100 {
		return fmt.Errorf("service profile %s score weights must total 100", profile.ID)
	}
	return nil
}

func (w ScoreWeights) total() int {
	return w.Latency + w.Throughput + w.Security + w.Stability + w.Compatibility + w.Resource + w.Operations + w.Resilience
}

func cloneProfile(profile ServiceProfile) ServiceProfile {
	profile.Dependencies = append([]string(nil), profile.Dependencies...)
	profile.ClientFormats = append([]string(nil), profile.ClientFormats...)
	return profile
}
