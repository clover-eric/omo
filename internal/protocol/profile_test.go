package protocol

import (
	"errors"
	"testing"
)

func TestDefaultServiceProfiles(t *testing.T) {
	registry, err := DefaultRegistry()
	if err != nil {
		t.Fatalf("default registry: %v", err)
	}

	profiles := registry.List()
	if len(profiles) != 5 {
		t.Fatalf("expected five default profiles, got %d", len(profiles))
	}

	standard, err := registry.Get("standard-secure-access")
	if err != nil {
		t.Fatalf("get standard profile: %v", err)
	}
	if !standard.RequiresDomain || !standard.RequiresTLSCert || standard.RequiresUDP {
		t.Fatalf("unexpected standard profile requirements: %#v", standard)
	}
	if standard.ScoreWeights.total() != 100 {
		t.Fatalf("expected score weights to total 100, got %d", standard.ScoreWeights.total())
	}

	performance, err := registry.Get("high-throughput-access")
	if err != nil {
		t.Fatalf("get high-throughput profile: %v", err)
	}
	if !performance.RequiresUDP {
		t.Fatalf("expected high-throughput profile to require UDP: %#v", performance)
	}

	compatibility, err := registry.Get("broad-compatibility-access")
	if err != nil {
		t.Fatalf("get compatibility profile: %v", err)
	}
	if compatibility.RequiresDomain || compatibility.RequiresTLSCert {
		t.Fatalf("expected broad compatibility profile to work before domain TLS readiness: %#v", compatibility)
	}

	fallback, err := registry.Get("lightweight-fallback-access")
	if err != nil {
		t.Fatalf("get lightweight fallback profile: %v", err)
	}
	if fallback.Category != "fallback" || fallback.RequiresUDP || fallback.ScoreWeights.Resource < 20 {
		t.Fatalf("expected resource-conscious fallback profile, got %#v", fallback)
	}

	mobile, err := registry.Get("mobile-optimized-access")
	if err != nil {
		t.Fatalf("get mobile optimized profile: %v", err)
	}
	if mobile.Category != "mobility" || mobile.ScoreWeights.Resilience < 14 {
		t.Fatalf("expected resilience-oriented mobile profile, got %#v", mobile)
	}
}

func TestRegistryRejectsDuplicateProfileIDs(t *testing.T) {
	profiles := DefaultServiceProfiles()
	profiles = append(profiles, profiles[0])

	_, err := NewRegistry(profiles)
	if !errors.Is(err, ErrDuplicateProfile) {
		t.Fatalf("expected duplicate profile error, got %v", err)
	}
}

func TestRegistryReturnsCopies(t *testing.T) {
	registry, err := DefaultRegistry()
	if err != nil {
		t.Fatalf("default registry: %v", err)
	}

	profiles := registry.List()
	profiles[0].Dependencies[0] = "mutated"

	standard, err := registry.Get("standard-secure-access")
	if err != nil {
		t.Fatalf("get standard profile: %v", err)
	}
	if standard.Dependencies[0] == "mutated" {
		t.Fatal("expected registry to protect profile slice fields from caller mutation")
	}
}
