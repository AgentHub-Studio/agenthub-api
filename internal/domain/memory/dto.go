package memory

import "encoding/json"

// UpsertMemoryRequest is the request body for PUT /api/agents/{agentId}/memory/{key}.
type UpsertMemoryRequest struct {
	Value     json.RawMessage `json:"value"`
	UserID    *string         `json:"userId,omitempty"`
	ExpiresAt *string         `json:"expiresAt,omitempty"` // RFC3339 or null
}
