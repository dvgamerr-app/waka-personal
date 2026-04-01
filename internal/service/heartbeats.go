package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"waka-personal/internal/domain"
	"waka-personal/internal/store"
)

type HeartbeatService struct {
	store *store.Store
}

func NewHeartbeatService(dataStore *store.Store) *HeartbeatService {
	return &HeartbeatService{store: dataStore}
}

func (s *HeartbeatService) Ingest(ctx context.Context, body []byte, machineName string, importBatchID *string) ([]domain.HeartbeatRecord, error) {
	payloads, err := ParseHeartbeatBody(body)
	if err != nil {
		return nil, err
	}

	records := make([]domain.HeartbeatRecord, 0, len(payloads))
	for i := range payloads {
		record, err := NormalizeHeartbeat(&payloads[i], machineName, importBatchID)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	return s.store.UpsertHeartbeats(ctx, records)
}

func ParseHeartbeatBody(body []byte) ([]domain.HeartbeatPayload, error) {
	normalized := bytesTrim(body)
	if len(normalized) == 0 {
		return nil, errors.New("empty body")
	}

	for i := 0; i < 2; i++ {
		if len(normalized) > 0 && normalized[0] == '"' {
			var unwrapped string
			if err := json.Unmarshal(normalized, &unwrapped); err != nil {
				return nil, fmt.Errorf("decode nested json string: %w", err)
			}
			normalized = bytesTrim([]byte(unwrapped))
			continue
		}
		break
	}

	switch normalized[0] {
	case '[':
		var payloads []domain.HeartbeatPayload
		if err := json.Unmarshal(normalized, &payloads); err != nil {
			return nil, fmt.Errorf("decode heartbeat array: %w", err)
		}
		return payloads, nil
	case '{':
		var payload domain.HeartbeatPayload
		if err := json.Unmarshal(normalized, &payload); err != nil {
			return nil, fmt.Errorf("decode heartbeat object: %w", err)
		}
		return []domain.HeartbeatPayload{payload}, nil
	default:
		return nil, fmt.Errorf("unsupported heartbeat payload shape")
	}
}

func NormalizeHeartbeat(payload *domain.HeartbeatPayload, machineName string, importBatchID *string) (domain.HeartbeatRecord, error) {
	if err := validateHeartbeatPayload(payload); err != nil {
		return domain.HeartbeatRecord{}, err
	}

	heartbeatTime := time.Unix(0, int64(payload.Time*float64(time.Second))).UTC()
	sourceCreatedAt, err := parseHeartbeatCreatedAt(payload.CreatedAt)
	if err != nil {
		return domain.HeartbeatRecord{}, err
	}

	normalizedType := defaultType(payload.Type)
	category := normalizeCategory(payload.Category)
	project := resolveHeartbeatProject(payload)
	lines := resolveHeartbeatLines(payload)
	dependencies, err := parseDependencies(payload.Dependencies)
	if err != nil {
		return domain.HeartbeatRecord{}, err
	}
	originPayload, err := json.Marshal(payload)
	if err != nil {
		return domain.HeartbeatRecord{}, fmt.Errorf("marshal origin payload: %w", err)
	}

	id, dedupeHash := domain.BuildDedupeIdentifiers(
		payload.ID,
		heartbeatTime,
		payload.Entity,
		normalizedType,
		category,
		project,
		payload.Branch,
		payload.Language,
		payload.IsWrite,
		payload.Lineno,
		payload.Cursorpos,
		payload.Plugin,
	)

	return domain.HeartbeatRecord{
		ID:                  id,
		SourceHeartbeatID:   payload.ID,
		DedupeHash:          dedupeHash,
		Time:                heartbeatTime,
		SourceCreatedAt:     sourceCreatedAt,
		Entity:              payload.Entity,
		Type:                normalizedType,
		Category:            category,
		Project:             project,
		Branch:              payload.Branch,
		Language:            payload.Language,
		ProjectRootCount:    payload.ProjectRootCount,
		ProjectFolder:       payload.ProjectFolder,
		Lineno:              payload.Lineno,
		Cursorpos:           payload.Cursorpos,
		Lines:               lines,
		IsWrite:             payload.IsWrite,
		IsUnsavedEntity:     payload.IsUnsavedEntity,
		AILineChanges:       payload.AILineChanges,
		HumanLineChanges:    payload.HumanLineChanges,
		MachineName:         machineName,
		SourceMachineNameID: payload.MachineNameID,
		Plugin:              payload.Plugin,
		SourceUserAgentID:   payload.UserAgentID,
		Dependencies:        dependencies,
		ImportBatchID:       importBatchID,
		OriginPayload:       originPayload,
	}, nil
}

func validateHeartbeatPayload(payload *domain.HeartbeatPayload) error {
	if strings.TrimSpace(payload.Entity) == "" {
		return errors.New("heartbeat entity is required")
	}
	if payload.Time <= 0 {
		return errors.New("heartbeat time must be greater than zero")
	}
	return nil
}

func parseHeartbeatCreatedAt(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func resolveHeartbeatLines(payload *domain.HeartbeatPayload) *int {
	if payload.Lines != nil {
		return payload.Lines
	}
	return payload.LinesInFile
}

func resolveHeartbeatProject(payload *domain.HeartbeatPayload) string {
	project := strings.TrimSpace(payload.Project)
	if project != "" {
		return project
	}
	return strings.TrimSpace(payload.AlternateProject)
}

func normalizeCategory(value string) string {
	candidate := strings.TrimSpace(strings.ToLower(value))
	if candidate == "" {
		return "coding"
	}
	return candidate
}

func defaultType(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "file"
	}
	return value
}

func parseDependencies(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 {
		return []string{}, nil
	}

	trimmed := bytesTrim(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return []string{}, nil
	}

	switch trimmed[0] {
	case '[':
		var items []string
		if err := json.Unmarshal(trimmed, &items); err != nil {
			return nil, fmt.Errorf("decode dependencies array: %w", err)
		}
		return items, nil
	case '"':
		var text string
		if err := json.Unmarshal(trimmed, &text); err != nil {
			return nil, fmt.Errorf("decode dependencies string: %w", err)
		}
		if strings.TrimSpace(text) == "" {
			return []string{}, nil
		}
		parts := strings.Split(text, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			trimmedPart := strings.TrimSpace(part)
			if trimmedPart != "" {
				out = append(out, trimmedPart)
			}
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported dependencies payload")
	}
}

func bytesTrim(value []byte) []byte {
	return []byte(strings.TrimSpace(string(value)))
}
