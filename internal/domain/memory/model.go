// Package memory manages per-agent key/value memory entries.
package memory

import (
	"time"

	"github.com/google/uuid"
)

// AgentMemory represents a persisted memory entry for an agent.
type AgentMemory struct {
	ID        uuid.UUID  `json:"id"`
	AgentID   uuid.UUID  `json:"agentId"`
	UserID    *string    `json:"userId,omitempty"`
	Key       string     `json:"key"`
	Value     []byte     `json:"value"` // raw JSONB
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}
