package memory

import (
	"encoding/json"

	"github.com/google/uuid"
)

// UpsertMemoryRequest is the request body for PUT /api/agents/{agentId}/memory/{key}.
type UpsertMemoryRequest struct {
	Value       json.RawMessage `json:"value"`
	MemoryType  string          `json:"memoryType,omitempty"`  // user|feedback|project|reference|general
	Scope       string          `json:"scope,omitempty"`       // agent|workflow|execution (default: agent)
	ExecutionID *uuid.UUID      `json:"executionId,omitempty"` // set when scope=execution
	Embedding   []float32       `json:"embedding,omitempty"`
	UserID      *string         `json:"userId,omitempty"`
	ExpiresAt   *string         `json:"expiresAt,omitempty"` // RFC3339 or null
}

// MemoryResponse is the safe API-facing shape of AgentMemory.
// It intentionally omits the embedding vector (1024 floats ≈ 5 KB per entry)
// which is internal infrastructure and must never be sent to clients.
type MemoryResponse struct {
	ID             string      `json:"id"`
	AgentID        string      `json:"agentId"`
	UserID         *string     `json:"userId,omitempty"`
	Key            string      `json:"key"`
	Value          []byte      `json:"value"`
	MemoryType     MemoryType  `json:"memoryType"`
	Scope          MemoryScope `json:"scope"`
	ExecutionID    *string     `json:"executionId,omitempty"`
	LastAccessedAt string      `json:"lastAccessedAt"`
	ExpiresAt      *string     `json:"expiresAt,omitempty"`
	CreatedAt      string      `json:"createdAt"`
	UpdatedAt      string      `json:"updatedAt"`
}

// MemoryResponseFrom converts an AgentMemory to a MemoryResponse, stripping the embedding.
func MemoryResponseFrom(m AgentMemory) MemoryResponse {
	r := MemoryResponse{
		ID:             m.ID.String(),
		AgentID:        m.AgentID.String(),
		UserID:         m.UserID,
		Key:            m.Key,
		Value:          m.Value,
		MemoryType:     m.MemoryType,
		Scope:          m.Scope,
		LastAccessedAt: m.LastAccessedAt.Format("2006-01-02T15:04:05.999999999Z"),
		CreatedAt:      m.CreatedAt.Format("2006-01-02T15:04:05.999999999Z"),
		UpdatedAt:      m.UpdatedAt.Format("2006-01-02T15:04:05.999999999Z"),
	}
	if m.ExecutionID != nil {
		s := m.ExecutionID.String()
		r.ExecutionID = &s
	}
	if m.ExpiresAt != nil {
		s := m.ExpiresAt.Format("2006-01-02T15:04:05.999999999Z")
		r.ExpiresAt = &s
	}
	return r
}

// MemoryRecallResponse wraps MemoryResponse with its relevance score.
type MemoryRecallResponse struct {
	MemoryResponse
	Relevance float64 `json:"relevance"`
}

// RecallRequest is the payload for POST /api/agents/{agentId}/memory/recall.
type RecallRequest struct {
	Embedding   []float32  `json:"embedding"`
	Limit       int        `json:"limit"`
	MemoryType  string     `json:"memoryType,omitempty"`  // optional filter by type
	Scope       string     `json:"scope,omitempty"`       // optional filter by scope
	ExecutionID *uuid.UUID `json:"executionId,omitempty"` // when scope=execution, filter to this run
	UserID      *string    `json:"userId,omitempty"`
}
