package audit

import (
	"context"
	"encoding/json"

	"omo/internal/store"
)

type Store interface {
	ListAuditLogs(ctx context.Context, limit int) ([]store.AuditLog, error)
}

type Service struct {
	store Store
}

type LogEntry struct {
	ID           string         `json:"id"`
	ActorAdminID string         `json:"actorAdminId,omitempty"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resourceType"`
	ResourceID   string         `json:"resourceId,omitempty"`
	Details      map[string]any `json:"details"`
	CreatedAt    string         `json:"createdAt"`
}

type ListResult struct {
	Logs []LogEntry `json:"logs"`
}

func NewService(appStore Store) *Service {
	return &Service{store: appStore}
}

func (s *Service) List(ctx context.Context, limit int) (ListResult, error) {
	records, err := s.store.ListAuditLogs(ctx, limit)
	if err != nil {
		return ListResult{}, err
	}
	logs := make([]LogEntry, 0, len(records))
	for _, record := range records {
		logs = append(logs, LogEntry{
			ID:           record.ID,
			ActorAdminID: record.ActorAdminID,
			Action:       record.Action,
			ResourceType: record.ResourceType,
			ResourceID:   record.ResourceID,
			Details:      detailsMap(record.DetailsJSON),
			CreatedAt:    record.CreatedAt.Format("2006-01-02T15:04:05.999999999Z07:00"),
		})
	}
	return ListResult{Logs: logs}, nil
}

func detailsMap(raw string) map[string]any {
	if raw == "" {
		return map[string]any{}
	}
	var details map[string]any
	if err := json.Unmarshal([]byte(raw), &details); err == nil && details != nil {
		return details
	}
	return map[string]any{"raw": raw}
}
