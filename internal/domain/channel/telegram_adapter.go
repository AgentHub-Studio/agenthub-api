package channel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// TelegramAdapter implements the Adapter interface for Telegram Bot API.
// The channel config JSON must contain:
//
//	{
//	  "botToken": "123456:ABC-DEF...",
//	  "allowedChats": ["@mygroup"]  // optional allowlist; empty = allow all
//	}
//
// Inbound authentication: the bot token is embedded in the inbound URL path
// (POST /api/channels/inbound/{token}) — no additional signature header.
// Outbound: sends via https://api.telegram.org/bot{botToken}/sendMessage.
type TelegramAdapter struct{}

// telegramConfig is the parsed channel.Config for Telegram channels.
type telegramConfig struct {
	BotToken     string   `json:"botToken"`
	AllowedChats []string `json:"allowedChats"`
}

// telegramUpdate is the Telegram Bot API update envelope.
type telegramUpdate struct {
	UpdateID int64            `json:"update_id"`
	Message  *telegramMessage `json:"message,omitempty"`
}

type telegramMessage struct {
	MessageID int64         `json:"message_id"`
	From      *telegramUser `json:"from,omitempty"`
	Chat      telegramChat  `json:"chat"`
	Text      string        `json:"text"`
	Date      int64         `json:"date"`
}

type telegramUser struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type telegramChat struct {
	ID       int64  `json:"id"`
	Type     string `json:"type"`
	Username string `json:"username"`
}

// VerifyRequest for Telegram relies solely on the inbound token in the URL.
// Telegram does not send HMAC signatures on webhook payloads.
func (a *TelegramAdapter) VerifyRequest(_ context.Context, _ Channel, _ map[string]string, _ []byte) error {
	return nil
}

// ParseMessage extracts text and routing from a Telegram Update.
func (a *TelegramAdapter) ParseMessage(_ context.Context, ch Channel, body []byte) (InboundMessage, error) {
	var update telegramUpdate
	if err := json.Unmarshal(body, &update); err != nil {
		return InboundMessage{}, fmt.Errorf("telegram: parse error: %w", err)
	}

	msg := update.Message
	if msg == nil || strings.TrimSpace(msg.Text) == "" {
		return InboundMessage{}, nil
	}

	// Optionally filter by allowed chat usernames.
	var cfg telegramConfig
	if len(ch.Config) > 2 {
		_ = json.Unmarshal(ch.Config, &cfg) // best-effort; misconfigured = allow all
	}
	if len(cfg.AllowedChats) > 0 {
		allowed := false
		for _, ac := range cfg.AllowedChats {
			if strings.TrimPrefix(ac, "@") == strings.TrimPrefix(msg.Chat.Username, "@") {
				allowed = true
				break
			}
		}
		if !allowed {
			return InboundMessage{}, nil
		}
	}

	senderID := ""
	senderName := ""
	if msg.From != nil {
		senderID = fmt.Sprintf("%d", msg.From.ID)
		senderName = strings.TrimSpace(msg.From.FirstName + " " + msg.From.LastName)
	}

	raw, _ := json.Marshal(update)
	return InboundMessage{
		ExternalID: fmt.Sprintf("%d", msg.MessageID),
		SenderID:   senderID,
		SenderName: senderName,
		Text:       strings.TrimSpace(msg.Text),
		ReplyTo:    fmt.Sprintf("%d", msg.Chat.ID),
		Raw:        raw,
	}, nil
}

// SendReply sends a text message via the Telegram sendMessage API.
func (a *TelegramAdapter) SendReply(_ context.Context, ch Channel, msg OutboundMessage) error {
	var cfg telegramConfig
	if err := json.Unmarshal(ch.Config, &cfg); err != nil {
		return fmt.Errorf("telegram: invalid channel config: %w", err)
	}
	if cfg.BotToken == "" {
		return fmt.Errorf("telegram: botToken not configured")
	}

	payload, _ := json.Marshal(map[string]string{
		"chat_id":    msg.ReplyTo,
		"text":       msg.Text,
		"parse_mode": "Markdown",
	})

	url := "https://api.telegram.org/bot" + cfg.BotToken + "/sendMessage"
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram: send reply: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram: send reply: HTTP %d", resp.StatusCode)
	}
	return nil
}

// Ensure TelegramAdapter satisfies the Adapter interface at compile time.
var _ Adapter = (*TelegramAdapter)(nil)
