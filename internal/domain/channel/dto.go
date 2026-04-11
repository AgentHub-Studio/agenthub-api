package channel

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ChannelResponse is the JSON envelope for a Channel.
// The token field is omitted to prevent leaking the inbound secret.
type ChannelResponse struct {
	ID        uuid.UUID       `json:"id"`
	Name      string          `json:"name"`
	Type      ChannelType     `json:"type"`
	AgentID   uuid.UUID       `json:"agentId"`
	Config    json.RawMessage `json:"config,omitempty"`
	Enabled   bool            `json:"enabled"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

// ResponseFrom converts a Channel to ChannelResponse (token omitted).
func ResponseFrom(ch Channel) ChannelResponse {
	cfg := ch.Config
	if len(cfg) == 0 {
		cfg = json.RawMessage(`{}`)
	}
	return ChannelResponse{
		ID:        ch.ID,
		Name:      ch.Name,
		Type:      ch.Type,
		AgentID:   ch.AgentID,
		Config:    cfg,
		Enabled:   ch.Enabled,
		CreatedAt: ch.CreatedAt,
		UpdatedAt: ch.UpdatedAt,
	}
}

// CreateChannelRequest is the JSON body for creating a channel.
type CreateChannelRequest struct {
	Name    string          `json:"name"`
	Type    ChannelType     `json:"type"`
	AgentID uuid.UUID       `json:"agentId"`
	Config  json.RawMessage `json:"config,omitempty"`
	Enabled *bool           `json:"enabled,omitempty"`
}

// UpdateChannelRequest is the JSON body for updating a channel.
type UpdateChannelRequest struct {
	Name    *string         `json:"name,omitempty"`
	AgentID *uuid.UUID      `json:"agentId,omitempty"`
	Config  json.RawMessage `json:"config,omitempty"`
	Enabled *bool           `json:"enabled,omitempty"`
}

// ChannelTokenResponse exposes the inbound token after creation.
// Only returned by the POST /api/channels endpoint.
type ChannelTokenResponse struct {
	ChannelResponse
	Token string `json:"token"`
}

// InboundRequest is the generic inbound message received from any channel.
type InboundRequest struct {
	// EventType is an optional platform event type label.
	EventType string `json:"eventType,omitempty"`
}
