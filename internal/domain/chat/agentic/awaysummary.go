package agentic

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// Away summary constants.
//
// Inspired by Claude Code's awaySummary.ts and useAwaySummary.ts.
const (
	// RecentMessageWindow is the number of recent messages used for recap generation.
	// ~15 user-assistant exchanges — sufficient context without triggering prompt_too_long.
	RecentMessageWindow = 30

	// AwayBlurDelay is how long the user must be inactive before generating a recap.
	AwayBlurDelay = 5 * time.Minute
)

// AwaySummaryConfig configures the away summary feature.
type AwaySummaryConfig struct {
	// Enabled controls whether away summaries are generated.
	Enabled bool `json:"enabled"`
	// MessageWindow overrides the default recent message window.
	MessageWindow int `json:"messageWindow,omitempty"`
	// BlurDelay overrides the default inactivity duration before generating.
	BlurDelay time.Duration `json:"blurDelay,omitempty"`
}

// DefaultAwaySummaryConfig returns production defaults.
func DefaultAwaySummaryConfig() AwaySummaryConfig {
	return AwaySummaryConfig{
		Enabled:       true,
		MessageWindow: RecentMessageWindow,
		BlurDelay:     AwayBlurDelay,
	}
}

// AwaySummaryMessage represents a generated session recap.
type AwaySummaryMessage struct {
	Type      string    `json:"type"`      // always "system"
	Subtype   string    `json:"subtype"`   // always "away_summary"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// NewAwaySummaryMessage creates a new away summary message.
func NewAwaySummaryMessage(content string) AwaySummaryMessage {
	return AwaySummaryMessage{
		Type:      "system",
		Subtype:   "away_summary",
		Content:   content,
		Timestamp: time.Now(),
	}
}

// BuildAwaySummaryPrompt constructs the prompt for generating a session recap.
// sessionMemory is optional broader context from the session memory system.
//
// Inspired by Claude Code's buildAwaySummaryPrompt.
func BuildAwaySummaryPrompt(sessionMemory string) string {
	var b strings.Builder

	if sessionMemory != "" {
		b.WriteString("Session memory (broader context):\n")
		b.WriteString(sessionMemory)
		b.WriteString("\n\n")
	}

	b.WriteString("The user stepped away and is coming back. ")
	b.WriteString("Write exactly 1-3 short sentences. ")
	b.WriteString("Start by stating the high-level task — what they are building or debugging, not implementation details. ")
	b.WriteString("Next: the concrete next step. ")
	b.WriteString("Skip status reports and commit recaps.")

	return b.String()
}

// WindowRecentMessages returns the last N messages from a conversation,
// suitable for feeding into the away summary prompt.
func WindowRecentMessages(messages json.RawMessage, windowSize int) (json.RawMessage, error) {
	if len(messages) == 0 {
		return nil, nil
	}

	var parsed []json.RawMessage
	if err := json.Unmarshal(messages, &parsed); err != nil {
		return nil, fmt.Errorf("away summary: unmarshal messages: %w", err)
	}

	if windowSize <= 0 {
		windowSize = RecentMessageWindow
	}

	if len(parsed) <= windowSize {
		return messages, nil
	}

	recent := parsed[len(parsed)-windowSize:]
	return json.Marshal(recent)
}

// HasSummarySinceLastUserTurn checks if an away_summary already exists
// since the last user message. Prevents duplicate summaries.
//
// Inspired by Claude Code's hasSummarySinceLastUserTurn.
func HasSummarySinceLastUserTurn(messages json.RawMessage) bool {
	if len(messages) == 0 {
		return false
	}

	var parsed []map[string]interface{}
	if err := json.Unmarshal(messages, &parsed); err != nil {
		return false
	}

	// Walk backwards from most recent message.
	for i := len(parsed) - 1; i >= 0; i-- {
		msg := parsed[i]
		msgType, _ := msg["type"].(string)
		role, _ := msg["role"].(string)
		subtype, _ := msg["subtype"].(string)

		// If we hit a user message first, no summary exists since then.
		if role == "user" || msgType == "user" {
			return false
		}

		// Found an away_summary before the last user message.
		if subtype == "away_summary" {
			return true
		}
	}
	return false
}

// GenerateAwaySummaryRequest holds the inputs for away summary generation.
type GenerateAwaySummaryRequest struct {
	// Messages is the full conversation history.
	Messages json.RawMessage `json:"messages"`
	// SessionMemory is optional broader context.
	SessionMemory string `json:"sessionMemory,omitempty"`
	// WindowSize overrides the default recent message window.
	WindowSize int `json:"windowSize,omitempty"`
}

// GenerateAwaySummaryResult holds the output of away summary generation.
type GenerateAwaySummaryResult struct {
	// Summary is the generated recap text (1-3 sentences).
	Summary string `json:"summary"`
	// Message is the ready-to-inject AwaySummaryMessage.
	Message AwaySummaryMessage `json:"message"`
	// Skipped indicates the summary was not generated (already exists, empty history, etc.).
	Skipped bool `json:"skipped"`
	// SkipReason explains why generation was skipped.
	SkipReason string `json:"skipReason,omitempty"`
}

// PrepareAwaySummary validates inputs and prepares the prompt for generation.
// Returns the windowed messages and prompt, or a skipped result.
//
// This handles all pre-checks; the actual LLM call is left to the caller
// since it depends on the model provider.
func PrepareAwaySummary(ctx context.Context, req GenerateAwaySummaryRequest) (*GenerateAwaySummaryResult, string, json.RawMessage, error) {
	_ = ctx // reserved for future use

	if len(req.Messages) == 0 {
		return &GenerateAwaySummaryResult{Skipped: true, SkipReason: "empty conversation"}, "", nil, nil
	}

	// Check for existing summary since last user message.
	if HasSummarySinceLastUserTurn(req.Messages) {
		return &GenerateAwaySummaryResult{Skipped: true, SkipReason: "summary already exists since last user message"}, "", nil, nil
	}

	windowSize := req.WindowSize
	if windowSize <= 0 {
		windowSize = RecentMessageWindow
	}

	windowed, err := WindowRecentMessages(req.Messages, windowSize)
	if err != nil {
		return nil, "", nil, fmt.Errorf("away summary: window messages: %w", err)
	}

	if len(windowed) == 0 {
		return &GenerateAwaySummaryResult{Skipped: true, SkipReason: "no messages after windowing"}, "", nil, nil
	}

	prompt := BuildAwaySummaryPrompt(req.SessionMemory)

	slog.Debug("away summary: prepared",
		"windowedMessages", len(windowed),
		"hasMemory", req.SessionMemory != "")

	return nil, prompt, windowed, nil
}

// CompleteAwaySummary wraps a generated summary text into a full result.
func CompleteAwaySummary(summaryText string) GenerateAwaySummaryResult {
	msg := NewAwaySummaryMessage(summaryText)
	return GenerateAwaySummaryResult{
		Summary: summaryText,
		Message: msg,
	}
}
