package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
)

// keyPattern restringe memory key a chars seguros: alphanumeric, underscore,
// hyphen, dot, colon. Bug 248: sem este gate, keys com espaços, HTML
// (<script>), path traversal (../), ou Unicode estranho eram aceitas e:
//   - quebravam URL routing (key="x y" precisa double-encoding)
//   - viravam vetor de XSS quando UI renderizava entry.key sem escape
//   - confundiam logs e debugging
// Pattern aceita identificadores de até 255 chars (DB column varchar(255)).
var keyPattern = regexp.MustCompile(`^[A-Za-z0-9_\-.:]{1,255}$`)

// validateKey aplica o pattern e retorna erro descritivo. Usado em Upsert
// e BulkUpsert para garantir consistência.
func validateKey(key string) error {
	if key == "" {
		return fmt.Errorf("memory: key is required")
	}
	if !keyPattern.MatchString(key) {
		return fmt.Errorf("memory: key must match pattern [A-Za-z0-9_\\-.:]{1,255} (got %q)", key)
	}
	return nil
}

// Service handles business logic for agent memory.
type Service struct {
	repo MemoryRepository
}

// NewService creates a new memory Service.
func NewService(repo MemoryRepository) *Service {
	return &Service{repo: repo}
}

// List returns all memory entries for an agent, optionally filtered by userID.
func (s *Service) List(ctx context.Context, agentID uuid.UUID, userID *string) ([]AgentMemory, error) {
	return s.repo.ListByAgent(ctx, agentID, userID)
}

// Upsert creates or updates a memory entry.
func (s *Service) Upsert(ctx context.Context, agentID uuid.UUID, key string, req UpsertMemoryRequest) (AgentMemory, error) {
	// Bug 248: gate restritivo no key (alphanumeric + ._-:). Keys com
	// HTML/espaços/path-traversal eram aceitas silenciosamente.
	if err := validateKey(key); err != nil {
		return AgentMemory{}, err
	}
	if len(req.Value) == 0 || !json.Valid(req.Value) {
		return AgentMemory{}, fmt.Errorf("memory: value must be valid JSON")
	}

	memType := MemoryType(req.MemoryType)
	if memType == "" {
		memType = MemoryTypeGeneral
	}

	scope := MemoryScope(req.Scope)
	if scope == "" {
		scope = MemoryScopeAgent
	}

	m := AgentMemory{
		AgentID:     agentID,
		UserID:      req.UserID,
		Key:         key,
		Value:       req.Value,
		MemoryType:  memType,
		Scope:       scope,
		ExecutionID: req.ExecutionID,
		Embedding:   req.Embedding,
	}

	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			return AgentMemory{}, fmt.Errorf("memory: invalid expiresAt: %w", err)
		}
		m.ExpiresAt = &t
	}

	return s.repo.Upsert(ctx, m)
}

// Recall returns the top-N most semantically similar memory entries for the given embedding,
// with a time-decayed relevance score attached to each result.
func (s *Service) Recall(ctx context.Context, agentID uuid.UUID, req RecallRequest) ([]MemoryRecallResult, error) {
	if len(req.Embedding) == 0 {
		return nil, fmt.Errorf("memory: recall requires a non-empty embedding vector")
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 5
	}

	items, err := s.repo.Recall(ctx, agentID, req.UserID, req.Embedding, limit, req.Scope, req.ExecutionID)
	if err != nil {
		return nil, err
	}

	results := make([]MemoryRecallResult, len(items))
	for i, m := range items {
		results[i] = MemoryRecallResult{
			AgentMemory: m,
			Relevance:   m.RelevanceScore(),
		}
	}
	return results, nil
}

// GetByKey returns a memory entry by key.
func (s *Service) GetByKey(ctx context.Context, agentID uuid.UUID, userID *string, key string) (AgentMemory, error) {
	return s.repo.GetByKey(ctx, agentID, userID, key)
}

// DeleteByKey removes a memory entry.
func (s *Service) DeleteByKey(ctx context.Context, agentID uuid.UUID, userID *string, key string) error {
	return s.repo.DeleteByKey(ctx, agentID, userID, key)
}

// ClearByAgent removes all memory entries for an agent.
func (s *Service) ClearByAgent(ctx context.Context, agentID uuid.UUID) error {
	return s.repo.ClearByAgent(ctx, agentID)
}

// ListByType returns memory entries filtered by memory type.
func (s *Service) ListByType(ctx context.Context, agentID uuid.UUID, userID *string, memoryType MemoryType) ([]AgentMemory, error) {
	return s.repo.ListByAgentAndType(ctx, agentID, userID, memoryType)
}

// SearchByText returns memories matching a text pattern in key or value.
func (s *Service) SearchByText(ctx context.Context, agentID uuid.UUID, query string, limit int) ([]AgentMemory, error) {
	if query == "" {
		return nil, fmt.Errorf("memory: search query is required")
	}
	return s.repo.SearchByText(ctx, agentID, query, limit)
}

// MemoryStats holds aggregated statistics for agent memories.
type MemoryStats struct {
	Total  int            `json:"total"`
	ByType map[string]int `json:"byType"`
}

// Stats returns aggregated memory statistics for an agent.
func (s *Service) Stats(ctx context.Context, agentID uuid.UUID) (MemoryStats, error) {
	counts, err := s.repo.CountByType(ctx, agentID)
	if err != nil {
		return MemoryStats{}, err
	}

	total := 0
	byType := make(map[string]int, len(counts))
	for mt, count := range counts {
		byType[string(mt)] = count
		total += count
	}

	return MemoryStats{Total: total, ByType: byType}, nil
}

// DistillExecutionMemories promotes all execution-scoped memories to workflow scope.
// Call this at the end of a run to make ephemeral knowledge available to future executions.
func (s *Service) DistillExecutionMemories(ctx context.Context, agentID uuid.UUID, executionID uuid.UUID) error {
	return s.repo.DistillExecutionMemories(ctx, agentID, executionID)
}

// BulkUpsert imports multiple memory entries at once.
func (s *Service) BulkUpsert(ctx context.Context, agentID uuid.UUID, entries []BulkMemoryEntry) (int, error) {
	stored := 0
	for _, entry := range entries {
		value := entry.Value
		if len(value) == 0 || !json.Valid(value) {
			continue
		}
		_, err := s.Upsert(ctx, agentID, entry.Key, UpsertMemoryRequest{
			Value:      value,
			MemoryType: entry.MemoryType,
			UserID:     entry.UserID,
		})
		if err != nil {
			continue
		}
		stored++
	}
	return stored, nil
}

// BulkMemoryEntry represents a single entry in a bulk import.
type BulkMemoryEntry struct {
	Key        string          `json:"key"`
	Value      json.RawMessage `json:"value"`
	MemoryType string          `json:"memoryType,omitempty"`
	UserID     *string         `json:"userId,omitempty"`
}
