// Package memory manages per-agent key/value memory entries with semantic recall.
package memory

import (
	"math"
	"time"

	"github.com/google/uuid"
)

// MemoryType categorizes a memory entry for structured selection.
type MemoryType string

const (
	MemoryTypeUser      MemoryType = "user"
	MemoryTypeFeedback  MemoryType = "feedback"
	MemoryTypeProject   MemoryType = "project"
	MemoryTypeReference MemoryType = "reference"
	MemoryTypeGeneral   MemoryType = "general"
)

// MemoryScope controls the lifetime and visibility of a memory entry.
type MemoryScope string

const (
	// MemoryScopeAgent is the default scope — persists across all sessions for the agent.
	MemoryScopeAgent MemoryScope = "agent"
	// MemoryScopeWorkflow is shared across all executions of a given workflow.
	MemoryScopeWorkflow MemoryScope = "workflow"
	// MemoryScopeExecution is ephemeral — tied to a single execution run.
	MemoryScopeExecution MemoryScope = "execution"
)

const (
	// relevanceDecayLambda controls how fast relevance decays over time.
	// With lambda=0.01, relevance halves in ~69 hours (~3 days).
	relevanceDecayLambda = 0.01
)

// AgentMemory represents a persisted memory entry for an agent.
type AgentMemory struct {
	ID             uuid.UUID   `json:"id"`
	AgentID        uuid.UUID   `json:"agentId"`
	UserID         *string     `json:"userId,omitempty"`
	Key            string      `json:"key"`
	Value          []byte      `json:"value"` // raw JSONB
	MemoryType     MemoryType  `json:"memoryType"`
	Scope          MemoryScope `json:"scope"`
	ExecutionID    *uuid.UUID  `json:"executionId,omitempty"`
	Embedding      []float32   `json:"embedding,omitempty"`
	LastAccessedAt time.Time   `json:"lastAccessedAt"`
	ExpiresAt      *time.Time  `json:"expiresAt,omitempty"`
	CreatedAt      time.Time   `json:"createdAt"`
	UpdatedAt      time.Time   `json:"updatedAt"`
}

// RelevanceScore computes a time-decayed relevance score.
// Formula: exp(-lambda * hours_since_last_access)
// Returns 1.0 for very recent entries, approaching 0 for old ones.
func (m AgentMemory) RelevanceScore() float64 {
	hoursSince := time.Since(m.LastAccessedAt).Hours()
	return math.Exp(-relevanceDecayLambda * hoursSince)
}

// MemoryRecallResult wraps a recalled memory entry with its relevance score.
type MemoryRecallResult struct {
	AgentMemory
	Relevance float64 `json:"relevance"`
}

