package channel

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// DiscordAdapter implements the Adapter interface for Discord slash commands / interaction webhooks.
// The channel config JSON must contain:
//
//	{
//	  "publicKey":    "hexEncodedEd25519PublicKey",
//	  "botToken":     "Bot YOUR_TOKEN",    // for sending replies
//	  "applicationID": "1234567890"        // Discord application ID
//	}
//
// Inbound authentication: Ed25519 signature over (timestamp + body) in
// X-Signature-Ed25519 / X-Signature-Timestamp headers.
// Outbound: sends via https://discord.com/api/v10/channels/{channel.id}/messages
type DiscordAdapter struct{}

// discordConfig is the parsed channel.Config for Discord channels.
type discordConfig struct {
	PublicKey     string `json:"publicKey"`
	BotToken      string `json:"botToken"`
	ApplicationID string `json:"applicationID"`
}

// discordInteraction is the top-level payload sent by Discord to the webhook.
type discordInteraction struct {
	// Type: 1 = PING, 2 = APPLICATION_COMMAND, 3 = MESSAGE_COMPONENT, etc.
	Type      int                      `json:"type"`
	ID        string                   `json:"id"`
	Token     string                   `json:"token"` // interaction token for follow-up
	ChannelID string                   `json:"channel_id"`
	Data      *discordInteractionData  `json:"data,omitempty"`
	Member    *discordMember           `json:"member,omitempty"`
	User      *discordUser             `json:"user,omitempty"` // DMs
}

// discordInteractionData holds the command payload.
type discordInteractionData struct {
	Name    string                    `json:"name"`
	Options []discordCommandOption    `json:"options,omitempty"`
}

// discordCommandOption holds a slash command option (e.g. value of a text parameter).
type discordCommandOption struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type discordMember struct {
	User *discordUser `json:"user,omitempty"`
	Nick string       `json:"nick,omitempty"`
}

type discordUser struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	GlobalName    string `json:"global_name,omitempty"`
}

// VerifyRequest validates the Discord Ed25519 signature.
// Discord requires the webhook to respond with HTTP 401 on invalid signatures.
// See: https://discord.com/developers/docs/interactions/receiving-and-responding#security-and-authorization
func (a *DiscordAdapter) VerifyRequest(_ context.Context, ch Channel, headers map[string]string, body []byte) error {
	var cfg discordConfig
	if err := json.Unmarshal(ch.Config, &cfg); err != nil {
		return fmt.Errorf("discord: invalid channel config: %w", err)
	}
	if cfg.PublicKey == "" {
		// No public key configured — allow all (dev/testing only).
		return nil
	}

	sigHex := headers["X-Signature-Ed25519"]
	timestamp := headers["X-Signature-Timestamp"]
	if sigHex == "" || timestamp == "" {
		return fmt.Errorf("discord: missing signature headers")
	}

	pubKeyBytes, err := hex.DecodeString(cfg.PublicKey)
	if err != nil {
		return fmt.Errorf("discord: invalid public key encoding: %w", err)
	}
	if len(pubKeyBytes) != ed25519.PublicKeySize {
		return fmt.Errorf("discord: public key must be %d bytes", ed25519.PublicKeySize)
	}

	sigBytes, err := hex.DecodeString(sigHex)
	if err != nil {
		return fmt.Errorf("discord: invalid signature encoding: %w", err)
	}

	// Discord signs: timestamp + body (concatenated as raw bytes).
	message := append([]byte(timestamp), body...)
	if !ed25519.Verify(ed25519.PublicKey(pubKeyBytes), message, sigBytes) {
		return fmt.Errorf("discord: signature verification failed")
	}
	return nil
}

// ParseMessage extracts a normalised InboundMessage from a Discord Interaction payload.
// PING interactions (type 1) return a Challenge response so the handler can echo {"type":1}.
// Application commands (type 2) extract the first option value as message text.
func (a *DiscordAdapter) ParseMessage(_ context.Context, _ Channel, body []byte) (InboundMessage, error) {
	var interaction discordInteraction
	if err := json.Unmarshal(body, &interaction); err != nil {
		return InboundMessage{}, fmt.Errorf("discord: parse error: %w", err)
	}

	// Type 1 = PING — Discord sends this to verify the endpoint.
	// The handler will echo {"type":1} back to Discord.
	if interaction.Type == 1 {
		return InboundMessage{Challenge: "pong"}, nil
	}

	// Only handle application commands (type 2) and message components (type 3).
	if interaction.Type != 2 && interaction.Type != 3 {
		return InboundMessage{}, nil
	}

	// Resolve sender.
	senderID := ""
	senderName := ""
	if interaction.Member != nil && interaction.Member.User != nil {
		senderID = interaction.Member.User.ID
		senderName = displayName(interaction.Member.User, interaction.Member.Nick)
	} else if interaction.User != nil {
		senderID = interaction.User.ID
		senderName = displayName(interaction.User, "")
	}

	// Build text from the command name + options.
	text := commandText(interaction.Data)
	if strings.TrimSpace(text) == "" {
		return InboundMessage{}, nil
	}

	raw, _ := json.Marshal(interaction)
	return InboundMessage{
		ExternalID: interaction.ID,
		SenderID:   senderID,
		SenderName: senderName,
		Text:       text,
		// ReplyTo carries the channel_id so SendReply can post there,
		// plus the interaction token for deferred response support.
		ReplyTo: interaction.ChannelID + ":" + interaction.Token,
		Raw:     raw,
	}, nil
}

// SendReply sends a message to the Discord channel.
// msg.ReplyTo is expected to be "channelID:interactionToken" (as set by ParseMessage).
// The reply is posted via the REST Messages API using the bot token.
func (a *DiscordAdapter) SendReply(_ context.Context, ch Channel, msg OutboundMessage) error {
	var cfg discordConfig
	if err := json.Unmarshal(ch.Config, &cfg); err != nil {
		return fmt.Errorf("discord: invalid channel config: %w", err)
	}
	if cfg.BotToken == "" {
		return fmt.Errorf("discord: botToken not configured")
	}

	// Extract channelID from replyTo ("channelID:token" or bare channelID).
	channelID := msg.ReplyTo
	if idx := strings.Index(msg.ReplyTo, ":"); idx > 0 {
		channelID = msg.ReplyTo[:idx]
	}
	if channelID == "" {
		return fmt.Errorf("discord: no channel ID in ReplyTo")
	}

	payload, _ := json.Marshal(map[string]string{
		"content": msg.Text,
	})

	url := "https://discord.com/api/v10/channels/" + channelID + "/messages"
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bot "+strings.TrimPrefix(cfg.BotToken, "Bot "))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("discord: send reply: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("discord: send reply: HTTP %d", resp.StatusCode)
	}
	return nil
}

// displayName returns the best available name for a Discord user.
func displayName(u *discordUser, nick string) string {
	if nick != "" {
		return nick
	}
	if u.GlobalName != "" {
		return u.GlobalName
	}
	return u.Username
}

// commandText assembles a human-readable string from the slash command data.
// For a command /ask question:"hello world" it returns "ask: hello world".
func commandText(data *discordInteractionData) string {
	if data == nil {
		return ""
	}
	if len(data.Options) == 0 {
		return data.Name
	}
	// Return the value of the first string option as the message text.
	return strings.TrimSpace(data.Options[0].Value)
}

// Ensure DiscordAdapter satisfies the Adapter interface at compile time.
var _ Adapter = (*DiscordAdapter)(nil)
