// Package agent implements the agent CRUD and versioning domain.
package agent

import (
	"encoding/json"
	"fmt"
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

// VersionStatus represents the lifecycle of an AgentVersion.
type VersionStatus string

const (
	VersionStatusDraft     VersionStatus = "DRAFT"
	VersionStatusPublished VersionStatus = "PUBLISHED"
)

// AgentVersion is an immutable snapshot of an agent's configuration at a version number.
// Stored in ah_{tenantID}.agent_version — no tenant_id column.
type AgentVersion struct {
	ID            uuid.UUID
	AgentID       uuid.UUID
	VersionNumber int
	Status        VersionStatus
	Description   string
	DefinitionJSON json.RawMessage // pipeline graph definition
	ConfigJSON     json.RawMessage // model/tool config
	CreatedAt     time.Time
	UpdatedAt     time.Time
	PublishedAt   *time.Time
}

// Sentinel errors for version operations.
var (
	ErrVersionNotFound    = fmt.Errorf("agent version not found")
	ErrDraftAlreadyExists = fmt.Errorf("agent already has an active draft version")
	ErrVersionImmutable   = fmt.Errorf("published version is immutable")
)
