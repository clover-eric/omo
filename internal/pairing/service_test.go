package pairing

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"omo/internal/store"
)

func TestCreateAcceptAndListCascadePairing(t *testing.T) {
	ctx := context.Background()
	appStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer appStore.Close()

	service := NewService(appStore)
	codeResult, err := service.CreateCode(ctx, CreateCodeRequest{
		NodeName:   "Exit node",
		Domain:     "exit.example.com",
		TTLMinutes: 10,
	})
	if err != nil {
		t.Fatalf("create code: %v", err)
	}
	if codeResult.Code == "" || codeResult.Pairing.Status != "active" {
		t.Fatalf("expected one-time code, got %#v", codeResult)
	}

	accepted, err := service.Accept(ctx, AcceptRequest{ExitDomain: "exit.example.com", Code: codeResult.Code})
	if err != nil {
		t.Fatalf("accept code: %v", err)
	}
	if accepted.Node.Status != "trusted" || accepted.Pair.ConfigState != "pending_apply" {
		t.Fatalf("expected trusted node with pending config, got %#v", accepted)
	}
	if accepted.Job.Status != "succeeded" {
		t.Fatalf("expected succeeded pairing job, got %#v", accepted.Job)
	}

	if _, err := service.Accept(ctx, AcceptRequest{ExitDomain: "exit.example.com", Code: codeResult.Code}); err != ErrPairingCodeNotFound {
		t.Fatalf("expected one-time code rejection, got %v", err)
	}

	list, err := service.List(ctx)
	if err != nil {
		t.Fatalf("list cascade records: %v", err)
	}
	if len(list.Nodes) != 1 || len(list.Pairs) != 1 {
		t.Fatalf("expected one node and pair, got %#v", list)
	}
}

func TestUpdateAndDeleteCascadeNode(t *testing.T) {
	ctx := context.Background()
	appStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer appStore.Close()

	service := NewService(appStore)
	codeResult, err := service.CreateCode(ctx, CreateCodeRequest{NodeName: "Exit node", Domain: "exit.example.com"})
	if err != nil {
		t.Fatalf("create code: %v", err)
	}
	accepted, err := service.Accept(ctx, AcceptRequest{ExitDomain: "exit.example.com", Code: codeResult.Code})
	if err != nil {
		t.Fatalf("accept code: %v", err)
	}

	updated, err := service.UpdateNode(ctx, accepted.Node.ID, UpdateNodeRequest{Name: "Trusted exit", Status: "disabled"})
	if err != nil {
		t.Fatalf("update node: %v", err)
	}
	if updated.Node.Name != "Trusted exit" || updated.Node.Status != "disabled" {
		t.Fatalf("unexpected updated node %#v", updated.Node)
	}

	deleted, err := service.DeleteNode(ctx, accepted.Node.ID)
	if err != nil {
		t.Fatalf("delete node: %v", err)
	}
	if !deleted.Deleted {
		t.Fatalf("expected deleted result, got %#v", deleted)
	}
}

func TestPlanAndApplyCascadeConfigurationRequiresConfirmation(t *testing.T) {
	ctx := context.Background()
	appStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer appStore.Close()
	configPath := filepath.Join(t.TempDir(), "cascade", "pair.json")
	if err := appStore.SetSetting(ctx, "cascade.config_path", configPath); err != nil {
		t.Fatalf("set config path: %v", err)
	}

	service := NewService(appStore)
	codeResult, err := service.CreateCode(ctx, CreateCodeRequest{NodeName: "Exit node", Domain: "exit.example.com"})
	if err != nil {
		t.Fatalf("create code: %v", err)
	}
	accepted, err := service.Accept(ctx, AcceptRequest{ExitDomain: "exit.example.com", Code: codeResult.Code})
	if err != nil {
		t.Fatalf("accept code: %v", err)
	}

	planned, err := service.PlanConfig(ctx, accepted.Pair.ID)
	if err != nil {
		t.Fatalf("plan config: %v", err)
	}
	if planned.Pair.ConfigState != "planned" || planned.Plan.ConfigPath != configPath {
		t.Fatalf("expected planned config result, got %#v", planned)
	}
	if _, err := service.ApplyConfig(ctx, accepted.Pair.ID, ApplyConfigRequest{}); !errors.Is(err, ErrConfirmationRequired) {
		t.Fatalf("expected confirmation requirement, got %v", err)
	}
	applied, err := service.ApplyConfig(ctx, accepted.Pair.ID, ApplyConfigRequest{Confirm: true})
	if err != nil {
		t.Fatalf("apply config: %v", err)
	}
	if applied.Pair.Status != "active" || applied.Pair.ConfigState != "applied" {
		t.Fatalf("expected applied pair, got %#v", applied.Pair)
	}
	if applied.Job.Status != "succeeded" {
		t.Fatalf("expected succeeded apply job, got %#v", applied.Job)
	}
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read written config: %v", err)
	}
	if !containsAll(string(content), "one-hop-cascade", accepted.Pair.ID, "exit.example.com") {
		t.Fatalf("expected backend-generated cascade config, got %s", string(content))
	}
}

func TestRemoteCascadePairingExchange(t *testing.T) {
	ctx := context.Background()
	exitStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "exit.db"))
	if err != nil {
		t.Fatalf("open exit store: %v", err)
	}
	defer exitStore.Close()
	entryStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "entry.db"))
	if err != nil {
		t.Fatalf("open entry store: %v", err)
	}
	defer entryStore.Close()
	if err := entryStore.SetSetting(ctx, "bootstrap.domain", "entry.example.com"); err != nil {
		t.Fatalf("set entry domain: %v", err)
	}

	exitService := NewService(exitStore)
	entryService := NewServiceWithPeerExchanger(entryStore, fakePeerExchanger{service: exitService})
	codeResult, err := exitService.CreateCode(ctx, CreateCodeRequest{
		NodeName:   "Remote exit",
		Domain:     "exit.example.com",
		TTLMinutes: 10,
	})
	if err != nil {
		t.Fatalf("create exit code: %v", err)
	}

	accepted, err := entryService.Accept(ctx, AcceptRequest{ExitDomain: "exit.example.com", Code: codeResult.Code})
	if err != nil {
		t.Fatalf("accept remote code: %v", err)
	}
	if accepted.Node.Domain != "exit.example.com" || accepted.Node.Status != "trusted" {
		t.Fatalf("expected trusted remote exit node, got %#v", accepted.Node)
	}
	if accepted.Pair.ConfigState != "pending_apply" {
		t.Fatalf("expected pending config pair, got %#v", accepted.Pair)
	}
	if _, err := entryService.Accept(ctx, AcceptRequest{ExitDomain: "exit.example.com", Code: codeResult.Code}); err != ErrPeerExchangeFailed {
		t.Fatalf("expected remote one-time code rejection, got %v", err)
	}

	exitList, err := exitService.List(ctx)
	if err != nil {
		t.Fatalf("list exit records: %v", err)
	}
	if !hasCascadeNodeDomain(exitList.Nodes, "entry.example.com") {
		t.Fatalf("expected entry trust record on exit side, got %#v", exitList)
	}
}

func TestSampleCascadeHealthUpdatesNodes(t *testing.T) {
	ctx := context.Background()
	appStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer appStore.Close()

	service := NewServiceWithOptions(appStore, nil, fakeHealthSampler{})
	codeResult, err := service.CreateCode(ctx, CreateCodeRequest{NodeName: "Exit node", Domain: "exit.example.com"})
	if err != nil {
		t.Fatalf("create code: %v", err)
	}
	accepted, err := service.Accept(ctx, AcceptRequest{ExitDomain: "exit.example.com", Code: codeResult.Code})
	if err != nil {
		t.Fatalf("accept code: %v", err)
	}

	result, err := service.SampleHealth(ctx)
	if err != nil {
		t.Fatalf("sample health: %v", err)
	}
	if result.Job.Status != "succeeded" || len(result.Samples) == 0 {
		t.Fatalf("expected succeeded health job with samples, got %#v", result)
	}
	found := false
	for _, node := range result.Nodes {
		if node.ID == accepted.Node.ID {
			found = node.Online && node.LatencyMS == 24 && node.ThroughputMbps == 2.5
		}
	}
	if !found {
		t.Fatalf("expected sampled node health in result, got %#v", result.Nodes)
	}
}

func containsAll(value string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(value, part) {
			return false
		}
	}
	return true
}

func hasCascadeNodeDomain(nodes []store.CascadeNode, domain string) bool {
	for _, node := range nodes {
		if node.Domain == domain {
			return true
		}
	}
	return false
}

type fakePeerExchanger struct {
	service *Service
}

func (f fakePeerExchanger) Exchange(ctx context.Context, _ string, req ExchangeRequest) (ExchangeResult, error) {
	return f.service.Exchange(ctx, req)
}

type fakeHealthSampler struct{}

func (fakeHealthSampler) Sample(_ context.Context, node store.CascadeNode) (store.CascadeHealthSample, error) {
	return store.CascadeHealthSample{
		NodeID:         node.ID,
		Status:         "online",
		Online:         true,
		LatencyMS:      24,
		ThroughputMbps: 2.5,
		SampledAt:      time.Now().UTC(),
	}, nil
}
