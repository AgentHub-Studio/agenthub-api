package memory

import "encoding/json"

// UpsertMemoryRequest is the request body for PUT /api/agents/{agentId}/memory/{key}.
type UpsertMemoryRequest struct {
	Value      json.RawMessage `json:"value"`
	MemoryType string          `json:"memoryType,omitempty"` // user|feedback|project|reference|general
	Embedding  []float32       `json:"embedding,omitempty"`
	UserID     *string         `json:"userId,omitempty"`
	ExpiresAt  *string         `json:"expiresAt,omitempty"` // RFC3339 or null
}

// RecallRequest is the payload for POST /api/agents/{agentId}/memory/recall.
type RecallRequest struct {
	Embedding  []float32 `json:"embedding"`
	Limit      int       `json:"limit"`
	MemoryType string    `json:"memoryType,omitempty"` // optional filter by type
	UserID     *string   `json:"userId,omitempty"`
}
