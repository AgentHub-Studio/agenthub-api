package agent

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// AgentResponse is the JSON response envelope for an Agent.
type AgentResponse struct {
	ID             uuid.UUID       `json:"id"`
	Name           string          `json:"name"`
	Slug           string          `json:"slug"`
	Description    string          `json:"description"`
	Status         string          `json:"status"`
	CurrentVersion int             `json:"currentVersion"`
	SystemPrompt   *string         `json:"systemPrompt,omitempty"`
	ModelConfig    json.RawMessage `json:"modelConfig,omitempty"`
	Config         json.RawMessage `json:"config"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

// ResponseFrom converts an Agent entity to AgentResponse.
func ResponseFrom(a Agent) AgentResponse {
	config := a.Config
	if len(config) == 0 {
		config = json.RawMessage(`{}`)
	}
	var modelConfig json.RawMessage
	if len(a.ModelConfig) > 0 {
		modelConfig = a.ModelConfig
	}
	return AgentResponse{
		ID:             a.ID,
		Name:           a.Name,
		Slug:           a.Slug,
		Description:    a.Description,
		Status:         string(a.Status),
		CurrentVersion: a.CurrentVersion,
		SystemPrompt:   a.SystemPrompt,
		ModelConfig:    modelConfig,
		Config:         config,
		CreatedAt:      a.CreatedAt,
		UpdatedAt:      a.UpdatedAt,
	}
}

// CreateAgentRequest is the JSON body for agent creation.
type CreateAgentRequest struct {
	Name         string          `json:"name"`
	Slug         string          `json:"slug"`
	Description  string          `json:"description"`
	SystemPrompt *string         `json:"systemPrompt,omitempty"`
	ModelConfig  json.RawMessage `json:"modelConfig,omitempty"`
	Config       json.RawMessage `json:"config,omitempty"`
}

// UpdateAgentRequest is the JSON body for partial agent updates.
type UpdateAgentRequest struct {
	Name         *string         `json:"name,omitempty"`
	Slug         *string         `json:"slug,omitempty"`
	Description  *string         `json:"description,omitempty"`
	SystemPrompt *string         `json:"systemPrompt,omitempty"`
	ModelConfig  json.RawMessage `json:"modelConfig,omitempty"`
	Config       json.RawMessage `json:"config,omitempty"`
}

// CloneAgentRequest is the JSON body for cloning an agent.
type CloneAgentRequest struct {
	Name string `json:"name"`
}

// AgentVersionResponse is the JSON response for an AgentVersion.
type AgentVersionResponse struct {
	ID             uuid.UUID       `json:"id"`
	AgentID        uuid.UUID       `json:"agentId"`
	VersionNumber  int             `json:"versionNumber"`
	Status         string          `json:"status"`
	Description    string          `json:"description"`
	DefinitionJSON json.RawMessage `json:"definitionJson,omitempty"`
	ConfigJSON     json.RawMessage `json:"configJson,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
	PublishedAt    *time.Time      `json:"publishedAt,omitempty"`
}

// VersionResponseFrom converts an AgentVersion entity to AgentVersionResponse.
func VersionResponseFrom(v AgentVersion) AgentVersionResponse {
	def := v.DefinitionJSON
	if len(def) == 0 {
		def = json.RawMessage(`{}`)
	}
	cfg := v.ConfigJSON
	if len(cfg) == 0 {
		cfg = json.RawMessage(`{}`)
	}
	return AgentVersionResponse{
		ID:             v.ID,
		AgentID:        v.AgentID,
		VersionNumber:  v.VersionNumber,
		Status:         string(v.Status),
		Description:    v.Description,
		DefinitionJSON: def,
		ConfigJSON:     cfg,
		CreatedAt:      v.CreatedAt,
		UpdatedAt:      v.UpdatedAt,
		PublishedAt:    v.PublishedAt,
	}
}

// CreateAgentVersionRequest is the JSON body for creating a draft version.
type CreateAgentVersionRequest struct {
	Description    string          `json:"description"`
	DefinitionJSON json.RawMessage `json:"definitionJson,omitempty"`
	ConfigJSON     json.RawMessage `json:"configJson,omitempty"`
}

// UpdateAgentVersionRequest is the JSON body for updating a draft version.
type UpdateAgentVersionRequest struct {
	Description    *string         `json:"description,omitempty"`
	DefinitionJSON json.RawMessage `json:"definitionJson,omitempty"`
	ConfigJSON     json.RawMessage `json:"configJson,omitempty"`
}
