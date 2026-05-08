// Package channel provides channel adapters that bridge external messaging platforms
// (Slack, Telegram, Discord, custom webhooks) to the AgentHub agentic runner.
// Each channel is bound to one agent; incoming messages are routed to that agent.
package channel

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a channel cannot be found.
var ErrNotFound = errors.New("channel: not found")

// ErrTokenNotFound is returned when no channel matches a given inbound token.
var ErrTokenNotFound = errors.New("channel: token not found")

// ErrSlugConflict is returned when a channel name-slug is already in use.
var ErrSlugConflict = errors.New("channel: name conflict")

// ErrValidation é retornado quando CreateChannelRequest falha
// validação server-side (name/type vazios). Sem esse error, esses
// erros caíam em 500 no handler genérico — UX terrível.
var ErrValidation = errors.New("channel: validation failed")

// ChannelType identifies the messaging platform.
type ChannelType string

const (
	ChannelTypeWebhook  ChannelType = "WEBHOOK"
	ChannelTypeSlack    ChannelType = "SLACK"
	ChannelTypeTelegram ChannelType = "TELEGRAM"
	ChannelTypeDiscord  ChannelType = "DISCORD"
	ChannelTypeCustom   ChannelType = "CUSTOM"
)

// Channel represents an inbound messaging channel bound to an agent.
// Stored in ah_{tenantID}.channel — no tenant_id column.
type Channel struct {
	ID        uuid.UUID
	Name      string
	Type      ChannelType
	AgentID   uuid.UUID
	Config    json.RawMessage // platform-specific: tokens, IDs, signing secrets
	Token     string          // auto-generated inbound authentication token
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// InboundMessage is a normalised message received from any channel platform.
type InboundMessage struct {
	// ExternalID is the platform-assigned message ID (for deduplication).
	ExternalID string
	// SenderID is the platform user ID of the message sender.
	SenderID string
	// SenderName is the human-readable display name of the sender (may be empty).
	SenderName string
	// Text is the plain-text body of the message.
	Text string
	// ReplyTo is the platform thread/conversation context for sending the reply back.
	ReplyTo string
	// Raw is the original platform payload for advanced adapters.
	Raw json.RawMessage
	// Challenge is non-empty for platform URL-verification handshakes (e.g. Slack).
	// When set, the inbound handler must echo it back immediately without running the agent.
	Challenge string
}

// OutboundMessage is a reply to be sent back through the channel.
type OutboundMessage struct {
	// ReplyTo mirrors InboundMessage.ReplyTo for routing the reply.
	ReplyTo string
	// Text is the reply body.
	Text string
}
