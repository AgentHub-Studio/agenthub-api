package agentic

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// ProgressSummarizer periodically generates brief progress descriptions for
// long-running sub-agents. It runs a background goroutine with a ticker that
// asks the LLM for a 3-5 word present-tense summary of what the sub-agent is
// doing. The summary shares the parent's system prompt and tools to maximise
// prompt cache hits (no extra cache write cost).
//
// Inspired by Claude Code's agentSummary.ts.
type ProgressSummarizer struct {
	chatModel    ai.ChatModel
	systemPrompt string
	tools        []ai.Tool
	model        string
	interval     time.Duration

	mu              sync.Mutex
	previousSummary string
	messagesSoFar   []ai.Message
}

// ProgressSummarizerConfig holds configuration for the progress summarizer.
type ProgressSummarizerConfig struct {
	// ChatModel is the LLM used for summary generation.
	ChatModel ai.ChatModel
	// SystemPrompt is the parent's system prompt (shared for cache efficiency).
	SystemPrompt string
	// Tools are the parent's tool schemas (shared for cache efficiency).
	Tools []ai.Tool
	// Model is the model identifier.
	Model string
	// Interval is how often to generate a summary. Default 30s.
	Interval time.Duration
}

// NewProgressSummarizer creates a summarizer with the given config.
func NewProgressSummarizer(cfg ProgressSummarizerConfig) *ProgressSummarizer {
	interval := cfg.Interval
	if interval == 0 {
		interval = 30 * time.Second
	}
	return &ProgressSummarizer{
		chatModel:    cfg.ChatModel,
		systemPrompt: cfg.SystemPrompt,
		tools:        cfg.Tools,
		model:        cfg.Model,
		interval:     interval,
	}
}

// Run starts the periodic summarisation goroutine. It emits SubtaskProgress
// events to the provided channel. The goroutine stops when ctx is cancelled.
// subtaskID is included in emitted events. messages is the conversation
// history that the summarizer reads (shared with the runner — read-only).
//
// Call this in a separate goroutine:
//
//	go summarizer.Run(ctx, ch, subtaskID)
func (ps *ProgressSummarizer) Run(ctx context.Context, ch chan<- RunEvent, subtaskID string) {
	ticker := time.NewTicker(ps.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			summary := ps.generateSummary(ctx)
			if summary != "" {
				ch <- NewRunEvent(EventSubtaskProgress, SubtaskProgressData{
					ID:      subtaskID,
					Summary: summary,
				})
			}
			// Timer resets on completion (not initiation) to prevent overlapping.
			ticker.Reset(ps.interval)
		}
	}
}

// UpdateMessages updates the conversation snapshot for the next summary.
// The caller should invoke this after each turn.
func (ps *ProgressSummarizer) UpdateMessages(messages []ai.Message) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	// Copy to avoid races with the runner.
	ps.messagesSoFar = make([]ai.Message, len(messages))
	copy(ps.messagesSoFar, messages)
}

// generateSummary calls the LLM to produce a brief progress description.
// Returns empty string on failure (non-fatal).
func (ps *ProgressSummarizer) generateSummary(ctx context.Context) string {
	ps.mu.Lock()
	msgs := make([]ai.Message, len(ps.messagesSoFar))
	copy(msgs, ps.messagesSoFar)
	prevSummary := ps.previousSummary
	ps.mu.Unlock()

	if len(msgs) == 0 {
		return ""
	}

	// Build summary request. Use the same system prompt and tools as the parent
	// for cache efficiency (identical prefix = cache hit).
	prompt := "Describe what you are currently doing in 3-5 words, present tense, naming the file or function. Do not repeat the previous summary."
	if prevSummary != "" {
		prompt += "\nPrevious summary: " + prevSummary
	}
	prompt += "\nSay something NEW each time."

	summaryMsgs := append(msgs, ai.Message{
		Role:    ai.RoleUser,
		Content: prompt,
	})

	// Use a short context timeout — this is a low-priority background call.
	summaryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := ps.chatModel.Chat(summaryCtx, summaryMsgs, ai.ChatOptions{
		Model:     ps.model,
		MaxTokens: 50,
		SystemMsg: ps.systemPrompt,
		Tools:     ps.tools,
		// Deny tool use by not forcing any tools — the model shouldn't call tools
		// for a summary. We keep the tools in the request to preserve the cache prefix.
		ToolChoice: &ai.ToolChoice{Type: ai.ToolChoiceNone},
	})
	if err != nil {
		slog.Debug("progress summary generation failed", "error", err)
		return ""
	}

	summary := resp.Content
	if summary == "" {
		return ""
	}

	ps.mu.Lock()
	ps.previousSummary = summary
	ps.mu.Unlock()

	return summary
}
