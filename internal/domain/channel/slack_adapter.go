package channel

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/AgentHub-Studio/agenthub-api/internal/httputil"
)

// SlackAdapter implements the Adapter interface for Slack.
// Supports:
//   - Signature verification (X-Slack-Signature + X-Slack-Request-Timestamp)
//   - Events API parsing (message events in channels and DMs)
//   - Reply via Slack Web API chat.postMessage
//
// Channel config JSON:
//
//	{
//	  "botToken":      "xoxb-...",
//	  "signingSecret": "abc...",
//	  "channelID":     "C0123ABC456"  // default reply channel
//	}
type SlackAdapter struct{}

// slackConfig is the parsed channel.Config for Slack channels.
type slackConfig struct {
	BotToken      string `json:"botToken"`
	SigningSecret string `json:"signingSecret"`
	ChannelID     string `json:"channelID"`
}

// slackEvent is the envelope for Slack Events API payloads.
type slackEvent struct {
	Type      string      `json:"type"`
	Challenge string      `json:"challenge"` // URL verification
	Event     *slackInner `json:"event,omitempty"`
}

// slackInner is the inner event payload.
type slackInner struct {
	Type    string `json:"type"`
	Channel string `json:"channel"`
	User    string `json:"user"`
	Text    string `json:"text"`
	Ts      string `json:"ts"`
	EventTS string `json:"event_ts"`
	// BotID is set when the message came from a bot; used to skip self-messages.
	BotID string `json:"bot_id,omitempty"`
}

// VerifyRequest validates the Slack request signature.
// See: https://api.slack.com/authentication/verifying-requests-from-slack
func (a *SlackAdapter) VerifyRequest(_ context.Context, ch Channel, headers map[string]string, body []byte) error {
	var cfg slackConfig
	if err := json.Unmarshal(ch.Config, &cfg); err != nil {
		return fmt.Errorf("slack: invalid channel config: %w", err)
	}
	if cfg.SigningSecret == "" {
		// No signing secret configured — allow all (dev/testing only).
		return nil
	}

	sig := headers["X-Slack-Signature"]
	ts := headers["X-Slack-Request-Timestamp"]
	if sig == "" || ts == "" {
		return fmt.Errorf("slack: missing signature headers")
	}

	// Reject requests older than 5 minutes to prevent replay attacks.
	tsInt, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return fmt.Errorf("slack: invalid signature timestamp")
	}
	if time.Now().Unix()-tsInt > 300 {
		return fmt.Errorf("slack: request timestamp too old")
	}

	baseString := "v0:" + ts + ":" + string(body)
	mac := hmac.New(sha256.New, []byte(cfg.SigningSecret))
	mac.Write([]byte(baseString))
	expected := "v0=" + hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return fmt.Errorf("slack: signature mismatch")
	}
	return nil
}

// ParseMessage extracts a normalised InboundMessage from a Slack Events API payload.
// Returns a zero-value InboundMessage with Text="" for non-message events (e.g. url_verification).
func (a *SlackAdapter) ParseMessage(_ context.Context, _ Channel, body []byte) (InboundMessage, error) {
	var env slackEvent
	if err := httputil.DecodeSingleJSON(bytes.NewReader(body), &env); err != nil {
		return InboundMessage{}, fmt.Errorf("slack: parse error: %w", err)
	}

	// URL verification challenge — return Challenge field so the handler can echo it.
	if env.Type == "url_verification" {
		return InboundMessage{Challenge: env.Challenge}, nil
	}

	if env.Event == nil {
		return InboundMessage{}, nil
	}

	ev := env.Event

	// Skip bot messages to prevent infinite loops.
	if ev.BotID != "" {
		return InboundMessage{}, nil
	}

	replyTo := ev.Channel
	raw, _ := json.Marshal(env)

	return InboundMessage{
		ExternalID: ev.EventTS,
		SenderID:   ev.User,
		Text:       strings.TrimSpace(ev.Text),
		ReplyTo:    replyTo,
		Raw:        raw,
	}, nil
}

// SendReply posts a message to the Slack channel using the chat.postMessage Web API.
func (a *SlackAdapter) SendReply(_ context.Context, ch Channel, msg OutboundMessage) error {
	var cfg slackConfig
	if err := json.Unmarshal(ch.Config, &cfg); err != nil {
		return fmt.Errorf("slack: invalid channel config: %w", err)
	}
	if cfg.BotToken == "" {
		return fmt.Errorf("slack: botToken not configured")
	}

	channel := msg.ReplyTo
	if channel == "" {
		channel = cfg.ChannelID
	}

	payload, _ := json.Marshal(map[string]string{
		"channel": channel,
		"text":    msg.Text,
	})

	req, _ := http.NewRequest(http.MethodPost, "https://slack.com/api/chat.postMessage", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.BotToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("slack: send reply: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack: send reply: HTTP %d", resp.StatusCode)
	}
	return nil
}

// Ensure SlackAdapter satisfies the Adapter interface at compile time.
var _ Adapter = (*SlackAdapter)(nil)
