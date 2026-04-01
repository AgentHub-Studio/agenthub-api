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
	PipelineID     *uuid.UUID      `json:"pipelineId,omitempty"`
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
	return AgentResponse{
		ID:             a.ID,
		Name:           a.Name,
		Slug:           a.Slug,
		Description:    a.Description,
		Status:         string(a.Status),
		CurrentVersion: a.CurrentVersion,
		PipelineID:     a.PipelineID,
		Config:         config,
		CreatedAt:      a.CreatedAt,
		UpdatedAt:      a.UpdatedAt,
	}
}

// CreateAgentRequest is the JSON body for agent creation.
type CreateAgentRequest struct {
	Name        string          `json:"name"`
	Slug        string          `json:"slug"`
	Description string          `json:"description"`
	PipelineID  *uuid.UUID      `json:"pipelineId,omitempty"`
	Config      json.RawMessage `json:"config,omitempty"`
}

// UpdateAgentRequest is the JSON body for partial agent updates.
type UpdateAgentRequest struct {
	Name        *string         `json:"name,omitempty"`
	Slug        *string         `json:"slug,omitempty"`
	Description *string         `json:"description,omitempty"`
	PipelineID  *uuid.UUID      `json:"pipelineId,omitempty"`
	Config      json.RawMessage `json:"config,omitempty"`
}

// CloneAgentRequest is the JSON body for cloning an agent.
type CloneAgentRequest struct {
	Name string `json:"name"`
}
