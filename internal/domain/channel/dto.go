package channel

import (
	"encoding/json"
	"strings"
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
// Sensitive keys in config (token, secret, apiKey, password, signingSecret,
// botToken, accessToken, etc.) are masked to "***" — bug 155.
func ResponseFrom(ch Channel) ChannelResponse {
	cfg := ch.Config
	if len(cfg) == 0 {
		cfg = json.RawMessage(`{}`)
	} else {
		cfg = maskSensitiveConfigKeys(cfg)
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

// maskSensitiveConfigKeys returns a JSON copy of config with sensitive keys
// replaced by "***". Channel config is opaque JSONB and platform-specific
// (Slack signing secrets, Telegram bot tokens, custom webhook secrets);
// without masking, GET /api/channels exposes them to anyone with read access.
func maskSensitiveConfigKeys(raw json.RawMessage) json.RawMessage {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return raw
	}
	out, err := json.Marshal(maskSensitiveConfigValue(value))
	if err != nil {
		return raw
	}
	return out
}

func maskSensitiveConfigValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, child := range v {
			if isSensitiveConfigKey(key) {
				out[key] = maskedSensitiveValue(child)
				continue
			}
			out[key] = maskSensitiveConfigValue(child)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, child := range v {
			out[i] = maskSensitiveConfigValue(child)
		}
		return out
	default:
		return value
	}
}

func maskedSensitiveValue(value any) any {
	if s, ok := value.(string); ok && s == "" {
		return s
	}
	if value == nil {
		return nil
	}
	return "***"
}

// isSensitiveConfigKey returns true for config keys that hold credentials
// and must be masked in API responses.
func isSensitiveConfigKey(k string) bool {
	normalized := strings.ToLower(strings.NewReplacer("_", "", "-", "", ".", "").Replace(k))
	switch normalized {
	case "token", "secret", "apikey", "password",
		"signingsecret", "bottoken", "accesstoken",
		"refreshtoken", "clientsecret":
		return true
	default:
		return false
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
