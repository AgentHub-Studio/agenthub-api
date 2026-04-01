package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

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
	if len(req.Value) == 0 || !json.Valid(req.Value) {
		return AgentMemory{}, fmt.Errorf("memory: value must be valid JSON")
	}

	m := AgentMemory{
		AgentID:   agentID,
		UserID:    req.UserID,
		Key:       key,
		Value:     req.Value,
		Embedding: req.Embedding,
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

	items, err := s.repo.Recall(ctx, agentID, req.UserID, req.Embedding, limit)
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
