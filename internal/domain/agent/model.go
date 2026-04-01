// Package agent implements the agent CRUD and versioning domain.
package agent

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// AgentStatus represents the lifecycle status of an Agent.
type AgentStatus string

const (
	StatusDraft     AgentStatus = "DRAFT"
	StatusPublished AgentStatus = "PUBLISHED"
	StatusArchived  AgentStatus = "ARCHIVED"
)

// Agent is the domain entity for a tenant-scoped agent.
// Stored in ah_{tenantID}.agent — no tenant_id column.
type Agent struct {
	ID             uuid.UUID
	Name           string
	Slug           string
	Description    string
	Status         AgentStatus
	CurrentVersion int
	PipelineID     *uuid.UUID
	Config         json.RawMessage
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
