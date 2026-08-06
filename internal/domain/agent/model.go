// Package agent implements the agent CRUD and versioning domain.
package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/evals"
)

// sensitiveModelConfigKeys lists normalized model config keys that must not be
// returned in API responses or tool results. P-C211-1: apiKey must not appear
// in any response body.
var sensitiveModelConfigKeys = map[string]bool{
	"apikey":             true,
	"apisecret":          true,
	"clientsecret":       true,
	"secret":             true,
	"password":           true,
	"authtoken":          true,
	"accesstoken":        true,
	"refreshtoken":       true,
	"bearertoken":        true,
	"authorization":      true,
	"proxyauthorization": true,
	"xapikey":            true,
	"xapitoken":          true,
	"xauthtoken":         true,
	"xaccesstoken":       true,
	"xsecret":            true,
}

// SanitizeModelConfig removes credential keys from a raw model config JSON blob.
// Returns the sanitized JSON; on parse error returns an empty JSON object.
// Safe to call on nil or empty input.
func SanitizeModelConfig(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return raw
	}
	sanitized, err := json.Marshal(sanitizeModelConfigValue(value))
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return sanitized
}

func sanitizeModelConfigValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, child := range v {
			if isSensitiveModelConfigKey(key) {
				continue
			}
			out[key] = sanitizeModelConfigValue(child)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, child := range v {
			out[i] = sanitizeModelConfigValue(child)
		}
		return out
	default:
		return value
	}
}

func isSensitiveModelConfigKey(key string) bool {
	normalized := strings.ToLower(strings.NewReplacer("_", "", "-", "", ".", "").Replace(key))
	return sensitiveModelConfigKeys[normalized]
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
	EvalConfig       evals.EvalConfig
	InputProcessors  []string
	OutputProcessors []string
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
	ID             uuid.UUID
	AgentID        uuid.UUID
	VersionNumber  int
	Status         VersionStatus
	Description    string
	DefinitionJSON json.RawMessage // pipeline graph definition
	ConfigJSON     json.RawMessage // model/tool config
	CreatedAt      time.Time
	UpdatedAt      time.Time
	PublishedAt    *time.Time
}

// Sentinel errors for version operations.
var (
	ErrVersionNotFound        = fmt.Errorf("agent version not found")
	ErrDraftAlreadyExists     = fmt.Errorf("agent already has an active draft version")
	ErrVersionImmutable       = fmt.Errorf("published version is immutable")
	ErrRollbackBlockedByDraft = fmt.Errorf("cannot rollback while a draft version exists; publish or discard the draft first")
)

// ReadinessLevel classifies the quality of an agent configuration.
type ReadinessLevel string

const (
	ReadinessIncomplete ReadinessLevel = "INCOMPLETE" // 0–39
	ReadinessBasic      ReadinessLevel = "BASIC"      // 40–59
	ReadinessStandard   ReadinessLevel = "STANDARD"   // 60–79
	ReadinessProduction ReadinessLevel = "PRODUCTION" // 80–100
)

// ReadinessCheck is a single pass/fail criterion in the readiness evaluation.
type ReadinessCheck struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Weight  int    `json:"-"` // points awarded when Passed
	Message string `json:"message,omitempty"`
}

// ReadinessScore is the computed quality assessment of an agent.
type ReadinessScore struct {
	Score  int              `json:"score"` // 0–100
	Level  ReadinessLevel   `json:"level"`
	Checks []ReadinessCheck `json:"checks"`
}

// defaultReadinessMessage returns a human-readable hint for a failed check.
func defaultReadinessMessage(name string) string {
	switch name {
	case "has_name":
		return "Agent must have a name"
	case "has_description":
		return "Add a description to help users understand what this agent does"
	case "has_system_prompt":
		return "A system prompt defines the agent's behaviour and persona"
	case "has_provider":
		return "Configure an LLM provider (e.g. openai, anthropic) in model settings"
	case "has_skills":
		return "Bind at least one skill so the agent can use tools"
	case "has_active_tools":
		return "Each bound skill must have at least one active tool"
	case "has_instructions":
		return "Add instructions to a skill or set a system prompt"
	default:
		return ""
	}
}

// ComputeReadiness evaluates an agent's configuration completeness.
// boundSkillCount is the number of skills linked to the agent.
// activeToolCount is the total count of active tools across all bound skills.
func ComputeReadiness(a Agent, boundSkillCount int, activeToolCount int) ReadinessScore {
	hasProvider := func() bool {
		if len(a.ModelConfig) == 0 {
			return false
		}
		var mc struct {
			Provider string `json:"provider"`
		}
		return json.Unmarshal(a.ModelConfig, &mc) == nil && mc.Provider != ""
	}

	hasInstructions := boundSkillCount > 0 || (a.SystemPrompt != nil && *a.SystemPrompt != "")

	checks := []ReadinessCheck{
		// Identity — 30 pts
		{Name: "has_name", Passed: a.Name != "", Weight: 10},
		{Name: "has_description", Passed: a.Description != "", Weight: 10},
		{Name: "has_system_prompt", Passed: a.SystemPrompt != nil && *a.SystemPrompt != "", Weight: 10},
		// Provider — 20 pts
		{Name: "has_provider", Passed: hasProvider(), Weight: 20},
		// Skills & Tools — 30 pts
		{Name: "has_skills", Passed: boundSkillCount > 0, Weight: 15},
		{Name: "has_active_tools", Passed: activeToolCount > 0, Weight: 15},
		// Instructions — 20 pts
		{Name: "has_instructions", Passed: hasInstructions, Weight: 20},
	}

	score := 0
	for i, c := range checks {
		if c.Passed {
			score += c.Weight
		} else {
			checks[i].Message = defaultReadinessMessage(c.Name)
		}
	}

	return ReadinessScore{
		Score:  score,
		Level:  readinessLevel(score),
		Checks: checks,
	}
}

func readinessLevel(score int) ReadinessLevel {
	switch {
	case score >= 80:
		return ReadinessProduction
	case score >= 60:
		return ReadinessStandard
	case score >= 40:
		return ReadinessBasic
	default:
		return ReadinessIncomplete
	}
}
