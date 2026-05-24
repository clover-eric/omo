package pairing

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"omo/internal/auth"
	"omo/internal/store"
)

const (
	JobKindPairingAccept = "cascade_pairing_accept"
	JobKindPairingExchange = "cascade_pairing_exchange"
	JobKindCascadeConfigApply = "cascade_config_apply"
	JobKindCascadeHealthSample = "cascade_health_sample"
	StatePairingAccepted = "CASCADE_PAIRING_ACCEPTED"
	StatePairingExchanged = "CASCADE_PAIRING_EXCHANGED"
	StateCascadeConfigPlan = "CASCADE_CONFIG_PLAN"
	StateCascadeConfigApply = "CASCADE_CONFIG_APPLY"
	StateCascadeHealthSample = "CASCADE_HEALTH_SAMPLE"
)

var (
	ErrInvalidInput        = errors.New("invalid cascade pairing input")
	ErrPairingCodeNotFound = errors.New("cascade pairing code not found")
	ErrCascadeNodeNotFound = errors.New("cascade node not found")
	ErrPeerExchangeFailed = errors.New("cascade peer exchange failed")
	ErrCascadePairNotFound = errors.New("cascade pair not found")
	ErrConfirmationRequired = errors.New("cascade configuration confirmation required")
)

type Store interface {
	GetSetting(ctx context.Context, key string) (string, bool, error)
	SetSetting(ctx context.Context, key string, value string) error
	CreatePairingCode(ctx context.Context, nodeID string, nodeName string, domain string, codeHash string, publicKey string, signature string, expiresAt time.Time) (store.PairingCode, error)
	PairingCodeByHash(ctx context.Context, codeHash string, now time.Time) (*store.PairingCode, error)
	AnyPairingCodeByHash(ctx context.Context, codeHash string) (*store.PairingCode, error)
	MarkPairingCodeUsed(ctx context.Context, id string, now time.Time) error
	CreateCascadeNode(ctx context.Context, name string, domain string, status string, role string, fingerprint string) (store.CascadeNode, error)
	ListCascadeNodes(ctx context.Context) ([]store.CascadeNode, error)
	UpdateCascadeNode(ctx context.Context, id string, name string, status string) (*store.CascadeNode, error)
	DeleteCascadeNode(ctx context.Context, id string) (bool, error)
	CreateCascadePair(ctx context.Context, sourceNodeID string, targetNodeID string, status string, configState string) (store.CascadePair, error)
	ListCascadePairs(ctx context.Context) ([]store.CascadePair, error)
	CascadePairByID(ctx context.Context, id string) (*store.CascadePair, error)
	CascadeNodeByID(ctx context.Context, id string) (*store.CascadeNode, error)
	UpdateCascadePairConfigState(ctx context.Context, id string, status string, configState string) (*store.CascadePair, error)
	RecordCascadeHealthSample(ctx context.Context, nodeID string, online bool, latencyMS int, throughputMbps float64, lastError string, sampledAt time.Time) (store.CascadeHealthSample, error)
	CreateJob(ctx context.Context, kind string, state string, status string, progress int, message string) (store.Job, error)
	MarkJobStarted(ctx context.Context, jobID string) error
	UpdateJob(ctx context.Context, jobID string, state string, status string, progress int, message string, errorCode string, finished bool) error
	AppendJobEvent(ctx context.Context, jobID string, kind string, state string, status string, progress int, message string, errorCode string) (store.JobEvent, error)
	LatestJob(ctx context.Context, kind string) (*store.Job, error)
	AppendAuditLog(ctx context.Context, adminID *string, action string, resourceType string, resourceID string, detailsJSON string) error
}

type PeerExchanger interface {
	Exchange(ctx context.Context, domain string, req ExchangeRequest) (ExchangeResult, error)
}

type HealthSampler interface {
	Sample(ctx context.Context, node store.CascadeNode) (store.CascadeHealthSample, error)
}

type Service struct {
	store Store
	exchanger PeerExchanger
	healthSampler HealthSampler
}

type CreateCodeRequest struct {
	NodeName   string `json:"nodeName"`
	Domain     string `json:"domain"`
	TTLMinutes int    `json:"ttlMinutes"`
}

type CodeResult struct {
	Pairing store.PairingCode `json:"pairing"`
	Code    string            `json:"code"`
}

type AcceptRequest struct {
	ExitDomain string `json:"exitDomain"`
	Code       string `json:"code"`
}

type AcceptResult struct {
	Node store.CascadeNode `json:"node"`
	Pair store.CascadePair `json:"pair"`
	Job  store.Job         `json:"job"`
}

type ConfigPlanResult struct {
	Pair store.CascadePair `json:"pair"`
	Plan CascadeConfigPlan `json:"plan"`
}

type ApplyConfigRequest struct {
	Confirm bool `json:"confirm"`
}

type ApplyConfigResult struct {
	Pair store.CascadePair `json:"pair"`
	Plan CascadeConfigPlan `json:"plan"`
	Job  store.Job         `json:"job"`
}

type HealthSampleResult struct {
	Nodes   []store.CascadeNode         `json:"nodes"`
	Pairs   []store.CascadePair         `json:"pairs"`
	Samples []store.CascadeHealthSample `json:"samples"`
	Job     store.Job                   `json:"job"`
}

type CascadeConfigPlan struct {
	PairID       string         `json:"pairId"`
	SourceNodeID string         `json:"sourceNodeId"`
	TargetNodeID string         `json:"targetNodeId"`
	Version      string         `json:"version"`
	GeneratedAt  string         `json:"generatedAt"`
	ConfigPath   string         `json:"configPath"`
	BackupPath   string         `json:"backupPath,omitempty"`
	Warnings     []string       `json:"warnings"`
	Summary      string         `json:"summary"`
	Preview      map[string]any `json:"preview"`
}

type PeerNode struct {
	NodeID      string `json:"nodeId"`
	NodeName    string `json:"nodeName"`
	Domain      string `json:"domain"`
	PublicKey   string `json:"publicKey"`
	Fingerprint string `json:"fingerprint"`
}

type ExchangeRequest struct {
	Code      string   `json:"code"`
	EntryNode PeerNode `json:"entryNode"`
}

type ExchangeResult struct {
	ExitNode PeerNode          `json:"exitNode"`
	Pair     store.CascadePair `json:"pair"`
	Job      store.Job         `json:"job"`
}

type ListResult struct {
	Nodes []store.CascadeNode `json:"nodes"`
	Pairs []store.CascadePair `json:"pairs"`
}

type UpdateNodeRequest struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type NodeResult struct {
	Node store.CascadeNode `json:"node"`
}

type DeleteResult struct {
	Deleted bool `json:"deleted"`
}

type codeEnvelope struct {
	Version   int    `json:"version"`
	NodeID    string `json:"nodeId"`
	NodeName  string `json:"nodeName"`
	Domain    string `json:"domain"`
	PublicKey string `json:"publicKey"`
	ExpiresAt string `json:"expiresAt"`
	Signature string `json:"signature"`
	Nonce     string `json:"nonce"`
}

func NewService(appStore Store) *Service {
	client := &http.Client{Timeout: 5 * time.Second}
	return &Service{store: appStore, exchanger: HTTPSPeerExchanger{Client: client}, healthSampler: HTTPSHealthSampler{Client: client}}
}

func NewServiceWithPeerExchanger(appStore Store, exchanger PeerExchanger) *Service {
	client := &http.Client{Timeout: 5 * time.Second}
	return &Service{store: appStore, exchanger: exchanger, healthSampler: HTTPSHealthSampler{Client: client}}
}

func NewServiceWithOptions(appStore Store, exchanger PeerExchanger, sampler HealthSampler) *Service {
	if exchanger == nil {
		exchanger = HTTPSPeerExchanger{Client: &http.Client{Timeout: 5 * time.Second}}
	}
	if sampler == nil {
		sampler = HTTPSHealthSampler{Client: &http.Client{Timeout: 5 * time.Second}}
	}
	return &Service{store: appStore, exchanger: exchanger, healthSampler: sampler}
}

func (s *Service) CreateCode(ctx context.Context, req CreateCodeRequest) (CodeResult, error) {
	if s == nil || s.store == nil {
		return CodeResult{}, errors.New("cascade pairing service is unavailable")
	}
	nodeName := strings.TrimSpace(req.NodeName)
	if nodeName == "" {
		nodeName = "Exit cascade node"
	}
	domain := strings.TrimSpace(req.Domain)
	if !validDomain(domain) || len(nodeName) > 80 {
		return CodeResult{}, ErrInvalidInput
	}
	ttl := req.TTLMinutes
	if ttl == 0 {
		ttl = 15
	}
	if ttl < 5 || ttl > 60 {
		return CodeResult{}, ErrInvalidInput
	}

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return CodeResult{}, err
	}
	nonce, err := auth.GenerateToken(16)
	if err != nil {
		return CodeResult{}, err
	}
	expiresAt := time.Now().UTC().Add(time.Duration(ttl) * time.Minute)
	envelope := codeEnvelope{
		Version:   1,
		NodeID:    "node_" + fingerprint(publicKey)[:16],
		NodeName:  nodeName,
		Domain:    domain,
		PublicKey: base64.RawURLEncoding.EncodeToString(publicKey),
		ExpiresAt: expiresAt.Format(time.RFC3339),
		Nonce:     nonce,
	}
	message := signingMessage(envelope)
	envelope.Signature = base64.RawURLEncoding.EncodeToString(ed25519.Sign(privateKey, []byte(message)))
	payload, err := json.Marshal(envelope)
	if err != nil {
		return CodeResult{}, err
	}
	code := base64.RawURLEncoding.EncodeToString(payload)
	record, err := s.store.CreatePairingCode(ctx, envelope.NodeID, nodeName, domain, auth.HashToken(code), envelope.PublicKey, envelope.Signature, expiresAt)
	if err != nil {
		return CodeResult{}, err
	}
	_ = s.store.AppendAuditLog(ctx, nil, "cascade_pairing_code_created", "pairing_code", record.ID, `{"status":"active"}`)
	return CodeResult{Pairing: record, Code: code}, nil
}

func (s *Service) Accept(ctx context.Context, req AcceptRequest) (AcceptResult, error) {
	if s == nil || s.store == nil {
		return AcceptResult{}, errors.New("cascade pairing service is unavailable")
	}
	exitDomain := strings.TrimSpace(req.ExitDomain)
	code := strings.TrimSpace(req.Code)
	if !validDomain(exitDomain) || code == "" {
		return AcceptResult{}, ErrInvalidInput
	}
	envelope, err := parseCode(code)
	if err != nil {
		return AcceptResult{}, ErrInvalidInput
	}
	if !strings.EqualFold(envelope.Domain, exitDomain) {
		return AcceptResult{}, ErrInvalidInput
	}
	publicKey, err := base64.RawURLEncoding.DecodeString(envelope.PublicKey)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return AcceptResult{}, ErrInvalidInput
	}
	signature, err := base64.RawURLEncoding.DecodeString(envelope.Signature)
	if err != nil || !ed25519.Verify(ed25519.PublicKey(publicKey), []byte(signingMessage(envelope)), signature) {
		return AcceptResult{}, ErrInvalidInput
	}

	now := time.Now().UTC()
	expiresAt, err := time.Parse(time.RFC3339, envelope.ExpiresAt)
	if err != nil || !now.Before(expiresAt) {
		return AcceptResult{}, ErrPairingCodeNotFound
	}
	record, err := s.store.PairingCodeByHash(ctx, auth.HashToken(code), now)
	if err != nil {
		return AcceptResult{}, err
	}
	if record == nil {
		localRecord, err := s.store.AnyPairingCodeByHash(ctx, auth.HashToken(code))
		if err != nil {
			return AcceptResult{}, err
		}
		if localRecord != nil {
			return AcceptResult{}, ErrPairingCodeNotFound
		}
		return s.acceptRemote(ctx, exitDomain, code, envelope, publicKey)
	}

	return s.acceptVerified(ctx, envelope, record, publicKey, "local")
}

func (s *Service) acceptRemote(ctx context.Context, exitDomain string, code string, envelope codeEnvelope, exitPublicKey []byte) (AcceptResult, error) {
	if s.exchanger == nil {
		return AcceptResult{}, ErrPairingCodeNotFound
	}
	identity, err := s.localIdentity(ctx)
	if err != nil {
		return AcceptResult{}, err
	}
	result, err := s.exchanger.Exchange(ctx, exitDomain, ExchangeRequest{
		Code: code,
		EntryNode: PeerNode{
			NodeID:      identity.NodeID,
			NodeName:    identity.NodeName,
			Domain:      identity.Domain,
			PublicKey:   identity.PublicKey,
			Fingerprint: identity.Fingerprint,
		},
	})
	if err != nil {
		return AcceptResult{}, ErrPeerExchangeFailed
	}
	if result.ExitNode.NodeID != envelope.NodeID ||
		!strings.EqualFold(result.ExitNode.Domain, envelope.Domain) ||
		result.ExitNode.PublicKey != envelope.PublicKey ||
		result.ExitNode.Fingerprint != fingerprint(exitPublicKey) {
		return AcceptResult{}, ErrInvalidInput
	}

	job, err := s.store.CreateJob(ctx, JobKindPairingAccept, StatePairingAccepted, "queued", 0, "Remote cascade pairing exchange started.")
	if err != nil {
		return AcceptResult{}, err
	}
	_, _ = s.store.AppendJobEvent(ctx, job.ID, JobKindPairingAccept, StatePairingAccepted, "queued", 0, "Remote cascade pairing exchange started.", "")
	if err := s.store.MarkJobStarted(ctx, job.ID); err != nil {
		return AcceptResult{}, err
	}

	localNode, err := s.localEntryNode(ctx)
	if err != nil {
		_ = s.store.UpdateJob(ctx, job.ID, StatePairingAccepted, "failed", 100, "Local cascade node record could not be prepared.", "CASCADE_LOCAL_NODE_FAILED", true)
		return AcceptResult{}, err
	}
	node, err := s.store.CreateCascadeNode(ctx, envelope.NodeName, envelope.Domain, "trusted", "exit", result.ExitNode.Fingerprint)
	if err != nil {
		_ = s.store.UpdateJob(ctx, job.ID, StatePairingAccepted, "failed", 100, "Remote cascade node trust record could not be saved.", "CASCADE_NODE_SAVE_FAILED", true)
		return AcceptResult{}, err
	}
	pair, err := s.store.CreateCascadePair(ctx, localNode.ID, node.ID, "pending", "pending_apply")
	if err != nil {
		_ = s.store.UpdateJob(ctx, job.ID, StatePairingAccepted, "failed", 100, "Cascade link record could not be saved.", "CASCADE_PAIR_SAVE_FAILED", true)
		return AcceptResult{}, err
	}
	_ = s.store.AppendAuditLog(ctx, nil, "cascade_pairing_remote_accepted", "cascade_node", node.ID, `{"configState":"pending_apply"}`)
	if err := s.store.UpdateJob(ctx, job.ID, StatePairingAccepted, "succeeded", 100, "Remote cascade trust relation created and awaiting configuration apply.", "", true); err != nil {
		return AcceptResult{}, err
	}
	latest, err := s.store.LatestJob(ctx, JobKindPairingAccept)
	if err != nil {
		return AcceptResult{}, err
	}
	return AcceptResult{Node: node, Pair: pair, Job: *latest}, nil
}

func (s *Service) acceptVerified(ctx context.Context, envelope codeEnvelope, record *store.PairingCode, publicKey []byte, mode string) (AcceptResult, error) {
	now := time.Now().UTC()
	jobMessage := "Cascade pairing acceptance started."
	if mode == "local" {
		jobMessage = "Local cascade pairing acceptance started."
	}
	job, err := s.store.CreateJob(ctx, JobKindPairingAccept, StatePairingAccepted, "queued", 0, jobMessage)
	if err != nil {
		return AcceptResult{}, err
	}
	_, _ = s.store.AppendJobEvent(ctx, job.ID, JobKindPairingAccept, StatePairingAccepted, "queued", 0, jobMessage, "")
	if err := s.store.MarkJobStarted(ctx, job.ID); err != nil {
		return AcceptResult{}, err
	}

	localNode, err := s.localEntryNode(ctx)
	if err != nil {
		_ = s.store.UpdateJob(ctx, job.ID, StatePairingAccepted, "failed", 100, "Local cascade node record could not be prepared.", "CASCADE_LOCAL_NODE_FAILED", true)
		return AcceptResult{}, err
	}
	node, err := s.store.CreateCascadeNode(ctx, envelope.NodeName, envelope.Domain, "trusted", "exit", fingerprint(publicKey))
	if err != nil {
		_ = s.store.UpdateJob(ctx, job.ID, StatePairingAccepted, "failed", 100, "Cascade node trust record could not be saved.", "CASCADE_NODE_SAVE_FAILED", true)
		return AcceptResult{}, err
	}
	pair, err := s.store.CreateCascadePair(ctx, localNode.ID, node.ID, "pending", "pending_apply")
	if err != nil {
		_ = s.store.UpdateJob(ctx, job.ID, StatePairingAccepted, "failed", 100, "Cascade link record could not be saved.", "CASCADE_PAIR_SAVE_FAILED", true)
		return AcceptResult{}, err
	}
	if err := s.store.MarkPairingCodeUsed(ctx, record.ID, now); err != nil {
		_ = s.store.UpdateJob(ctx, job.ID, StatePairingAccepted, "failed", 100, "Pairing code could not be marked as used.", "PAIRING_CODE_UPDATE_FAILED", true)
		return AcceptResult{}, err
	}
	_ = s.store.AppendAuditLog(ctx, nil, "cascade_pairing_accepted", "cascade_node", node.ID, `{"configState":"pending_apply"}`)
	if err := s.store.UpdateJob(ctx, job.ID, StatePairingAccepted, "succeeded", 100, "Cascade trust relation created and awaiting configuration apply.", "", true); err != nil {
		return AcceptResult{}, err
	}
	latest, err := s.store.LatestJob(ctx, JobKindPairingAccept)
	if err != nil {
		return AcceptResult{}, err
	}
	return AcceptResult{Node: node, Pair: pair, Job: *latest}, nil
}

func (s *Service) Exchange(ctx context.Context, req ExchangeRequest) (ExchangeResult, error) {
	if s == nil || s.store == nil {
		return ExchangeResult{}, errors.New("cascade pairing service is unavailable")
	}
	code := strings.TrimSpace(req.Code)
	if code == "" || !validPeerNode(req.EntryNode) {
		return ExchangeResult{}, ErrInvalidInput
	}
	envelope, err := parseAndVerifyCode(code)
	if err != nil {
		return ExchangeResult{}, ErrInvalidInput
	}
	now := time.Now().UTC()
	expiresAt, err := time.Parse(time.RFC3339, envelope.ExpiresAt)
	if err != nil || !now.Before(expiresAt) {
		return ExchangeResult{}, ErrPairingCodeNotFound
	}
	record, err := s.store.PairingCodeByHash(ctx, auth.HashToken(code), now)
	if err != nil {
		return ExchangeResult{}, err
	}
	if record == nil {
		return ExchangeResult{}, ErrPairingCodeNotFound
	}

	job, err := s.store.CreateJob(ctx, JobKindPairingExchange, StatePairingExchanged, "queued", 0, "Peer cascade pairing exchange started.")
	if err != nil {
		return ExchangeResult{}, err
	}
	_, _ = s.store.AppendJobEvent(ctx, job.ID, JobKindPairingExchange, StatePairingExchanged, "queued", 0, "Peer cascade pairing exchange started.", "")
	if err := s.store.MarkJobStarted(ctx, job.ID); err != nil {
		return ExchangeResult{}, err
	}
	entryNode, err := s.store.CreateCascadeNode(ctx, req.EntryNode.NodeName, req.EntryNode.Domain, "trusted", "entry", req.EntryNode.Fingerprint)
	if err != nil {
		_ = s.store.UpdateJob(ctx, job.ID, StatePairingExchanged, "failed", 100, "Entry cascade node trust record could not be saved.", "CASCADE_NODE_SAVE_FAILED", true)
		return ExchangeResult{}, err
	}
	localNode, err := s.localExitNode(ctx, envelope)
	if err != nil {
		_ = s.store.UpdateJob(ctx, job.ID, StatePairingExchanged, "failed", 100, "Local cascade node record could not be prepared.", "CASCADE_LOCAL_NODE_FAILED", true)
		return ExchangeResult{}, err
	}
	pair, err := s.store.CreateCascadePair(ctx, entryNode.ID, localNode.ID, "pending", "pending_apply")
	if err != nil {
		_ = s.store.UpdateJob(ctx, job.ID, StatePairingExchanged, "failed", 100, "Cascade link record could not be saved.", "CASCADE_PAIR_SAVE_FAILED", true)
		return ExchangeResult{}, err
	}
	if err := s.store.MarkPairingCodeUsed(ctx, record.ID, now); err != nil {
		_ = s.store.UpdateJob(ctx, job.ID, StatePairingExchanged, "failed", 100, "Pairing code could not be marked as used.", "PAIRING_CODE_UPDATE_FAILED", true)
		return ExchangeResult{}, err
	}
	_ = s.store.AppendAuditLog(ctx, nil, "cascade_pairing_exchanged", "cascade_node", entryNode.ID, `{"configState":"pending_apply"}`)
	if err := s.store.UpdateJob(ctx, job.ID, StatePairingExchanged, "succeeded", 100, "Peer cascade trust relation created and awaiting configuration apply.", "", true); err != nil {
		return ExchangeResult{}, err
	}
	latest, err := s.store.LatestJob(ctx, JobKindPairingExchange)
	if err != nil {
		return ExchangeResult{}, err
	}
	return ExchangeResult{
		ExitNode: PeerNode{
			NodeID:      envelope.NodeID,
			NodeName:    envelope.NodeName,
			Domain:      envelope.Domain,
			PublicKey:   envelope.PublicKey,
			Fingerprint: recordFingerprint(record),
		},
		Pair: pair,
		Job:  *latest,
	}, nil
}

func (s *Service) localEntryNode(ctx context.Context) (store.CascadeNode, error) {
	nodes, err := s.store.ListCascadeNodes(ctx)
	if err != nil {
		return store.CascadeNode{}, err
	}
	for _, node := range nodes {
		if node.Role == "entry" && node.Domain == "local.omo" {
			return node, nil
		}
	}
	return s.store.CreateCascadeNode(ctx, "Local entry node", "local.omo", "trusted", "entry", "")
}

func (s *Service) localExitNode(ctx context.Context, envelope codeEnvelope) (store.CascadeNode, error) {
	nodes, err := s.store.ListCascadeNodes(ctx)
	if err != nil {
		return store.CascadeNode{}, err
	}
	for _, node := range nodes {
		if node.Role == "exit" && strings.EqualFold(node.Domain, envelope.Domain) && node.TrustKeyFingerprint == recordFingerprintFromEnvelope(envelope) {
			return node, nil
		}
	}
	return s.store.CreateCascadeNode(ctx, envelope.NodeName, envelope.Domain, "trusted", "exit", recordFingerprintFromEnvelope(envelope))
}

type localIdentity struct {
	NodeID      string
	NodeName    string
	Domain      string
	PublicKey   string
	Fingerprint string
}

func (s *Service) localIdentity(ctx context.Context) (localIdentity, error) {
	publicKeyText, publicOK, err := s.store.GetSetting(ctx, "cascade.local_public_key")
	if err != nil {
		return localIdentity{}, err
	}
	if !publicOK || publicKeyText == "" {
		publicKey, _, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return localIdentity{}, err
		}
		publicKeyText = base64.RawURLEncoding.EncodeToString(publicKey)
		if err := s.store.SetSetting(ctx, "cascade.local_public_key", publicKeyText); err != nil {
			return localIdentity{}, err
		}
	}
	publicKey, err := base64.RawURLEncoding.DecodeString(publicKeyText)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return localIdentity{}, ErrInvalidInput
	}
	domain, ok, err := s.store.GetSetting(ctx, "bootstrap.domain")
	if err != nil {
		return localIdentity{}, err
	}
	domain = strings.TrimSpace(domain)
	if !ok || domain == "" {
		domain = "local.omo"
	}
	return localIdentity{
		NodeID:      "node_" + fingerprint(publicKey)[:16],
		NodeName:    "Entry cascade node",
		Domain:      domain,
		PublicKey:   publicKeyText,
		Fingerprint: fingerprint(publicKey),
	}, nil
}

func (s *Service) List(ctx context.Context) (ListResult, error) {
	nodes, err := s.store.ListCascadeNodes(ctx)
	if err != nil {
		return ListResult{}, err
	}
	nodes = publicNodes(nodes)
	pairs, err := s.store.ListCascadePairs(ctx)
	if err != nil {
		return ListResult{}, err
	}
	return ListResult{Nodes: nodes, Pairs: pairs}, nil
}

func (s *Service) UpdateNode(ctx context.Context, id string, req UpdateNodeRequest) (NodeResult, error) {
	name := strings.TrimSpace(req.Name)
	status := strings.TrimSpace(req.Status)
	if name != "" && len(name) > 80 {
		return NodeResult{}, ErrInvalidInput
	}
	if status != "" && status != "pending" && status != "trusted" && status != "disabled" {
		return NodeResult{}, ErrInvalidInput
	}
	node, err := s.store.UpdateCascadeNode(ctx, strings.TrimSpace(id), name, status)
	if err != nil {
		return NodeResult{}, err
	}
	if node == nil {
		return NodeResult{}, ErrCascadeNodeNotFound
	}
	_ = s.store.AppendAuditLog(ctx, nil, "cascade_node_updated", "cascade_node", node.ID, `{"status":"`+node.Status+`"}`)
	return NodeResult{Node: *node}, nil
}

func (s *Service) DeleteNode(ctx context.Context, id string) (DeleteResult, error) {
	deleted, err := s.store.DeleteCascadeNode(ctx, strings.TrimSpace(id))
	if err != nil {
		return DeleteResult{}, err
	}
	if !deleted {
		return DeleteResult{}, ErrCascadeNodeNotFound
	}
	_ = s.store.AppendAuditLog(ctx, nil, "cascade_node_deleted", "cascade_node", strings.TrimSpace(id), `{"deleted":true}`)
	return DeleteResult{Deleted: true}, nil
}

func (s *Service) PlanConfig(ctx context.Context, pairID string) (ConfigPlanResult, error) {
	pair, source, target, err := s.pairContext(ctx, pairID)
	if err != nil {
		return ConfigPlanResult{}, err
	}
	plan, err := s.renderCascadePlan(ctx, *pair, *source, *target)
	if err != nil {
		return ConfigPlanResult{}, err
	}
	if pair.ConfigState == "pending_apply" {
		updated, err := s.store.UpdateCascadePairConfigState(ctx, pair.ID, pair.Status, "planned")
		if err != nil {
			return ConfigPlanResult{}, err
		}
		pair = updated
	}
	_ = s.store.AppendAuditLog(ctx, nil, "cascade_config_planned", "cascade_pair", pair.ID, `{"configState":"`+pair.ConfigState+`"}`)
	return ConfigPlanResult{Pair: *pair, Plan: plan}, nil
}

func (s *Service) ApplyConfig(ctx context.Context, pairID string, req ApplyConfigRequest) (ApplyConfigResult, error) {
	if !req.Confirm {
		return ApplyConfigResult{}, ErrConfirmationRequired
	}
	pair, source, target, err := s.pairContext(ctx, pairID)
	if err != nil {
		return ApplyConfigResult{}, err
	}
	plan, err := s.renderCascadePlan(ctx, *pair, *source, *target)
	if err != nil {
		return ApplyConfigResult{}, err
	}

	job, err := s.store.CreateJob(ctx, JobKindCascadeConfigApply, StateCascadeConfigPlan, "queued", 0, "Cascade configuration apply job created.")
	if err != nil {
		return ApplyConfigResult{}, err
	}
	_, _ = s.store.AppendJobEvent(ctx, job.ID, JobKindCascadeConfigApply, StateCascadeConfigPlan, "queued", 0, "Cascade configuration apply job created.", "")
	if err := s.store.MarkJobStarted(ctx, job.ID); err != nil {
		return ApplyConfigResult{}, err
	}
	_, _ = s.store.AppendJobEvent(ctx, job.ID, JobKindCascadeConfigApply, StateCascadeConfigPlan, "running", 35, "Rendering backend-owned cascade configuration.", "")
	backupPath, err := s.writeCascadeConfig(plan)
	if err != nil {
		_ = s.store.UpdateJob(ctx, job.ID, StateCascadeConfigApply, "failed", 100, "Cascade configuration could not be written.", "CASCADE_CONFIG_WRITE_FAILED", true)
		return ApplyConfigResult{}, err
	}
	plan.BackupPath = backupPath
	updated, err := s.store.UpdateCascadePairConfigState(ctx, pair.ID, "active", "applied")
	if err != nil {
		_ = s.store.UpdateJob(ctx, job.ID, StateCascadeConfigApply, "failed", 100, "Cascade pair state could not be updated.", "CASCADE_PAIR_UPDATE_FAILED", true)
		return ApplyConfigResult{}, err
	}
	_ = s.store.AppendAuditLog(ctx, nil, "cascade_config_applied", "cascade_pair", pair.ID, `{"configState":"applied"}`)
	if err := s.store.UpdateJob(ctx, job.ID, StateCascadeConfigApply, "succeeded", 100, "Cascade configuration applied after operator confirmation.", "", true); err != nil {
		return ApplyConfigResult{}, err
	}
	_, _ = s.store.AppendJobEvent(ctx, job.ID, JobKindCascadeConfigApply, StateCascadeConfigApply, "succeeded", 100, "Cascade configuration applied after operator confirmation.", "")
	latest, err := s.store.LatestJob(ctx, JobKindCascadeConfigApply)
	if err != nil {
		return ApplyConfigResult{}, err
	}
	return ApplyConfigResult{Pair: *updated, Plan: plan, Job: *latest}, nil
}

func (s *Service) SampleHealth(ctx context.Context) (HealthSampleResult, error) {
	if s == nil || s.store == nil {
		return HealthSampleResult{}, errors.New("cascade health sampling is unavailable")
	}
	sampler := s.healthSampler
	if sampler == nil {
		sampler = HTTPSHealthSampler{Client: &http.Client{Timeout: 5 * time.Second}}
	}
	job, err := s.store.CreateJob(ctx, JobKindCascadeHealthSample, StateCascadeHealthSample, "queued", 0, "Cascade health sampling job created.")
	if err != nil {
		return HealthSampleResult{}, err
	}
	_, _ = s.store.AppendJobEvent(ctx, job.ID, JobKindCascadeHealthSample, StateCascadeHealthSample, "queued", 0, "Cascade health sampling job created.", "")
	if err := s.store.MarkJobStarted(ctx, job.ID); err != nil {
		return HealthSampleResult{}, err
	}
	nodes, err := s.store.ListCascadeNodes(ctx)
	if err != nil {
		_ = s.store.UpdateJob(ctx, job.ID, StateCascadeHealthSample, "failed", 100, "Cascade nodes could not be read.", "CASCADE_HEALTH_NODE_READ_FAILED", true)
		return HealthSampleResult{}, err
	}
	samples := make([]store.CascadeHealthSample, 0, len(nodes))
	now := time.Now().UTC()
	for _, node := range nodes {
		if node.Role == "entry" && node.Domain == "local.omo" {
			continue
		}
		if node.Status == "disabled" {
			sample, err := s.store.RecordCascadeHealthSample(ctx, node.ID, false, 0, 0, "node disabled", now)
			if err != nil {
				return HealthSampleResult{}, err
			}
			samples = append(samples, sample)
			continue
		}
		sample, err := sampler.Sample(ctx, node)
		if err != nil {
			sample = store.CascadeHealthSample{
				NodeID:         node.ID,
				Status:         "offline",
				Online:         false,
				LatencyMS:      0,
				ThroughputMbps: 0,
				LastError:      err.Error(),
				SampledAt:      time.Now().UTC(),
			}
		}
		recorded, err := s.store.RecordCascadeHealthSample(ctx, node.ID, sample.Online, sample.LatencyMS, sample.ThroughputMbps, sample.LastError, sample.SampledAt)
		if err != nil {
			_ = s.store.UpdateJob(ctx, job.ID, StateCascadeHealthSample, "failed", 100, "Cascade health sample could not be saved.", "CASCADE_HEALTH_SAMPLE_SAVE_FAILED", true)
			return HealthSampleResult{}, err
		}
		samples = append(samples, recorded)
	}
	_ = s.store.AppendAuditLog(ctx, nil, "cascade_health_sampled", "cascade_node", "all", fmt.Sprintf(`{"samples":%d}`, len(samples)))
	if err := s.store.UpdateJob(ctx, job.ID, StateCascadeHealthSample, "succeeded", 100, "Cascade health sampling completed.", "", true); err != nil {
		return HealthSampleResult{}, err
	}
	_, _ = s.store.AppendJobEvent(ctx, job.ID, JobKindCascadeHealthSample, StateCascadeHealthSample, "succeeded", 100, "Cascade health sampling completed.", "")
	latest, err := s.store.LatestJob(ctx, JobKindCascadeHealthSample)
	if err != nil {
		return HealthSampleResult{}, err
	}
	list, err := s.List(ctx)
	if err != nil {
		return HealthSampleResult{}, err
	}
	return HealthSampleResult{Nodes: list.Nodes, Pairs: list.Pairs, Samples: samples, Job: *latest}, nil
}

func (s *Service) pairContext(ctx context.Context, pairID string) (*store.CascadePair, *store.CascadeNode, *store.CascadeNode, error) {
	pairID = strings.TrimSpace(pairID)
	if pairID == "" {
		return nil, nil, nil, ErrInvalidInput
	}
	pair, err := s.store.CascadePairByID(ctx, pairID)
	if err != nil {
		return nil, nil, nil, err
	}
	if pair == nil {
		return nil, nil, nil, ErrCascadePairNotFound
	}
	source, err := s.store.CascadeNodeByID(ctx, pair.SourceNodeID)
	if err != nil {
		return nil, nil, nil, err
	}
	target, err := s.store.CascadeNodeByID(ctx, pair.TargetNodeID)
	if err != nil {
		return nil, nil, nil, err
	}
	if source == nil || target == nil {
		return nil, nil, nil, ErrCascadeNodeNotFound
	}
	if source.Status == "disabled" || target.Status == "disabled" {
		return nil, nil, nil, ErrInvalidInput
	}
	return pair, source, target, nil
}

func (s *Service) renderCascadePlan(ctx context.Context, pair store.CascadePair, source store.CascadeNode, target store.CascadeNode) (CascadeConfigPlan, error) {
	version := time.Now().UTC().Format("20060102150405")
	configPath, ok, err := s.store.GetSetting(ctx, "cascade.config_path")
	if err != nil {
		return CascadeConfigPlan{}, err
	}
	configPath = strings.TrimSpace(configPath)
	if !ok || configPath == "" {
		configPath = filepath.Join("data", "sing-box", "cascade", pair.ID+".json")
	}
	warnings := []string{}
	if pair.ConfigState != "pending_apply" && pair.ConfigState != "planned" {
		warnings = append(warnings, "This cascade link already has configuration state "+pair.ConfigState+".")
	}
	if source.TrustKeyFingerprint == "" || target.TrustKeyFingerprint == "" {
		warnings = append(warnings, "One side has no recorded trust fingerprint; confirm the pairing records before applying.")
	}
	preview := map[string]any{
		"managedBy": "omo",
		"kind":      "one-hop-cascade",
		"pairId":    pair.ID,
		"source": map[string]any{
			"id":     source.ID,
			"name":   source.Name,
			"domain": source.Domain,
			"role":   source.Role,
		},
		"target": map[string]any{
			"id":                  target.ID,
			"name":                target.Name,
			"domain":              target.Domain,
			"role":                target.Role,
			"trustKeyFingerprint": target.TrustKeyFingerprint,
		},
		"route": map[string]any{
			"type":     "one-hop",
			"mode":     "authorized-boundary-access",
			"finalTag": "omo-cascade-" + pair.ID,
		},
	}
	return CascadeConfigPlan{
		PairID:       pair.ID,
		SourceNodeID: pair.SourceNodeID,
		TargetNodeID: pair.TargetNodeID,
		Version:      version,
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
		ConfigPath:   configPath,
		Warnings:     warnings,
		Summary:      "Backend-generated one-hop cascade configuration is ready for operator confirmation.",
		Preview:      preview,
	}, nil
}

func (s *Service) writeCascadeConfig(plan CascadeConfigPlan) (string, error) {
	if strings.TrimSpace(plan.ConfigPath) == "" {
		return "", ErrInvalidInput
	}
	if err := os.MkdirAll(filepath.Dir(plan.ConfigPath), 0o755); err != nil {
		return "", err
	}
	payload, err := json.MarshalIndent(plan.Preview, "", "  ")
	if err != nil {
		return "", err
	}
	payload = append(payload, '\n')
	backupPath := ""
	if _, err := os.Stat(plan.ConfigPath); err == nil {
		backupPath = plan.ConfigPath + ".previous"
		if err := copyFile(plan.ConfigPath, backupPath); err != nil {
			return "", err
		}
	}
	tmpPath := plan.ConfigPath + ".tmp-" + plan.Version
	if err := os.WriteFile(tmpPath, payload, 0o600); err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(tmpPath) }()
	var decoded any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return "", err
	}
	return backupPath, os.Rename(tmpPath, plan.ConfigPath)
}

func copyFile(src string, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func parseCode(code string) (codeEnvelope, error) {
	payload, err := base64.RawURLEncoding.DecodeString(code)
	if err != nil {
		return codeEnvelope{}, err
	}
	var envelope codeEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return codeEnvelope{}, err
	}
	return envelope, nil
}

func parseAndVerifyCode(code string) (codeEnvelope, error) {
	envelope, err := parseCode(code)
	if err != nil {
		return codeEnvelope{}, err
	}
	publicKey, err := base64.RawURLEncoding.DecodeString(envelope.PublicKey)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return codeEnvelope{}, ErrInvalidInput
	}
	signature, err := base64.RawURLEncoding.DecodeString(envelope.Signature)
	if err != nil || !ed25519.Verify(ed25519.PublicKey(publicKey), []byte(signingMessage(envelope)), signature) {
		return codeEnvelope{}, ErrInvalidInput
	}
	return envelope, nil
}

func signingMessage(envelope codeEnvelope) string {
	return strings.Join([]string{
		"omo-cascade-v1",
		envelope.NodeID,
		envelope.NodeName,
		envelope.Domain,
		envelope.PublicKey,
		envelope.ExpiresAt,
		envelope.Nonce,
	}, "\n")
}

func fingerprint(publicKey []byte) string {
	sum := sha256.Sum256(publicKey)
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func publicNodes(nodes []store.CascadeNode) []store.CascadeNode {
	public := make([]store.CascadeNode, 0, len(nodes))
	for _, node := range nodes {
		if node.Role == "entry" && node.Domain == "local.omo" {
			continue
		}
		public = append(public, node)
	}
	return public
}

func validPeerNode(node PeerNode) bool {
	if strings.TrimSpace(node.NodeID) == "" || len(strings.TrimSpace(node.NodeID)) > 80 {
		return false
	}
	if strings.TrimSpace(node.NodeName) == "" || len(strings.TrimSpace(node.NodeName)) > 80 {
		return false
	}
	if !validDomain(node.Domain) {
		return false
	}
	publicKey, err := base64.RawURLEncoding.DecodeString(node.PublicKey)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return false
	}
	return node.Fingerprint == fingerprint(publicKey)
}

func recordFingerprint(record *store.PairingCode) string {
	if record == nil {
		return ""
	}
	publicKey, err := base64.RawURLEncoding.DecodeString(record.PublicKey)
	if err != nil {
		return ""
	}
	return fingerprint(publicKey)
}

func recordFingerprintFromEnvelope(envelope codeEnvelope) string {
	publicKey, err := base64.RawURLEncoding.DecodeString(envelope.PublicKey)
	if err != nil {
		return ""
	}
	return fingerprint(publicKey)
}

func validDomain(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 253 || strings.Contains(value, "://") {
		return false
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	if net.ParseIP(value) != nil {
		return true
	}
	parts := strings.Split(value, ".")
	if len(parts) < 2 {
		return false
	}
	for _, part := range parts {
		if part == "" || len(part) > 63 {
			return false
		}
	}
	return true
}

type HTTPSPeerExchanger struct {
	Client *http.Client
}

func (e HTTPSPeerExchanger) Exchange(ctx context.Context, domain string, req ExchangeRequest) (ExchangeResult, error) {
	domain = strings.TrimSpace(domain)
	if !validDomain(domain) {
		return ExchangeResult{}, ErrInvalidInput
	}
	client := e.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	endpoint := url.URL{Scheme: "https", Host: domain, Path: "/api/pairing/exchange"}
	body, err := json.Marshal(req)
	if err != nil {
		return ExchangeResult{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return ExchangeResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", "omo-cascade-peer/0.1")
	resp, err := client.Do(httpReq)
	if err != nil {
		return ExchangeResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return ExchangeResult{}, fmt.Errorf("%w: peer returned %s", ErrPeerExchangeFailed, resp.Status)
	}
	var envelope struct {
		Success bool           `json:"success"`
		Data    ExchangeResult `json:"data"`
		Error   any            `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&envelope); err != nil {
		return ExchangeResult{}, err
	}
	if !envelope.Success {
		return ExchangeResult{}, ErrPeerExchangeFailed
	}
	return envelope.Data, nil
}

type HTTPSHealthSampler struct {
	Client *http.Client
}

func (s HTTPSHealthSampler) Sample(ctx context.Context, node store.CascadeNode) (store.CascadeHealthSample, error) {
	if !validDomain(node.Domain) {
		return store.CascadeHealthSample{}, ErrInvalidInput
	}
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	endpoint := url.URL{Scheme: "https", Host: node.Domain, Path: "/api/system/health"}
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return store.CascadeHealthSample{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "omo-cascade-health/0.1")
	resp, err := client.Do(req)
	latency := time.Since(start)
	sample := store.CascadeHealthSample{
		NodeID:         node.ID,
		Status:         "offline",
		Online:         false,
		LatencyMS:      int(latency.Milliseconds()),
		ThroughputMbps: 0,
		SampledAt:      time.Now().UTC(),
	}
	if err != nil {
		sample.LastError = err.Error()
		return sample, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		sample.LastError = err.Error()
		return sample, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		sample.LastError = resp.Status
		return sample, fmt.Errorf("remote health returned %s", resp.Status)
	}
	sample.Status = "online"
	sample.Online = true
	if sample.LatencyMS == 0 {
		sample.LatencyMS = 1
	}
	seconds := latency.Seconds()
	if seconds > 0 {
		sample.ThroughputMbps = float64(len(body)*8) / seconds / 1_000_000
	}
	return sample, nil
}
