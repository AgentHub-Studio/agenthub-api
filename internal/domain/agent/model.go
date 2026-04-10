// Package agent implements the agent CRUD and versioning domain.
package agent

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// sensitiveModelConfigKeys lists model config keys that must not be returned in API
// responses or tool results. P-C211-1: apiKey must not appear in any response body.
var sensitiveModelConfigKeys = []string{"apiKey", "api_key", "apiSecret", "api_secret"}

// SanitizeModelConfig removes credential keys from a raw model config JSON blob.
// Returns the sanitized JSON; on parse error returns an empty JSON object.
// Safe to call on nil or empty input.
func SanitizeModelConfig(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return raw
	}
	for _, k := range sensitiveModelConfigKeys {
		delete(m, k)
	}
	sanitized, err := json.Marshal(m)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return sanitized
}

// AgentManageResponse is the redacted DTO exposed by the agenthub_manage tool.
// It intentionally omits systemPrompt and permissionRules to prevent the LLM from
// reading sensitive configuration via the management interface.
// P-C208-1: agenthub_manage must not expose systemPrompt to the LLM.
// P-C211-1: modelConfig is sanitized to remove credential fields.
type AgentManageResponse struct {
	ID               uuid.UUID       `json:"id"`
	Name             string          `json:"name"`
	Slug             string          `json:"slug"`
	Description      string          `json:"description"`
	Status           AgentStatus     `json:"status"`
	CurrentVersion   int             `json:"currentVersion"`
	ModelConfig      json.RawMessage `json:"modelConfig,omitempty"` // credentials redacted
	EnableManagement bool            `json:"enableManagement"`
	CreatedAt        time.Time       `json:"createdAt"`
	UpdatedAt        time.Time       `json:"updatedAt"`
	// SystemPrompt and PermissionRules are intentionally absent.
}

// ManageResponseFrom creates a redacted AgentManageResponse from an Agent entity.
// Omits systemPrompt, permissionRules; sanitizes modelConfig to remove credentials.
func ManageResponseFrom(a Agent) AgentManageResponse {
	return AgentManageResponse{
		ID:               a.ID,
		Name:             a.Name,
		Slug:             a.Slug,
		Description:      a.Description,
		Status:           a.Status,
		CurrentVersion:   a.CurrentVersion,
		ModelConfig:      SanitizeModelConfig(a.ModelConfig),
		EnableManagement: a.EnableManagement,
		CreatedAt:        a.CreatedAt,
		UpdatedAt:        a.UpdatedAt,
	}
}

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
	ID               uuid.UUID
	Name             string
	Slug             string
	Description      string
	Status           AgentStatus
	CurrentVersion   int
	SystemPrompt     *string
	ModelConfig      json.RawMessage
	PermissionRules  json.RawMessage
	Config           json.RawMessage
	// EnableManagement controls whether the agenthub_manage builtin tool is included
	// in this agent's toolset. Default false — requires explicit opt-in.
	// P-C184-2: prevents agents from managing other agents without explicit authorization.
	EnableManagement bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
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
