package agentic

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// ForkedAgentParams configures a forked agent execution.
// Forked agents are background sub-agents that share the parent's prompt cache
// via CacheSafeParams, enabling significant token cost savings.
//
// Inspired by Claude Code's runForkedAgent() in utils/forkedAgent.ts:
// "Background agents that reuse parent's cache-safe params to guarantee
// prompt cache hits on the first turn."
type ForkedAgentParams struct {
	// PromptMessages are the messages to send to the forked agent.
	PromptMessages []ai.Message
	// CacheSafeParams captures the cache-critical parameters from the parent.
	// When non-nil, the fork uses identical system prompt, tools, model, and
	// provider — guaranteeing a cache hit on the shared prefix.
	CacheSafeParams *CacheSafeParams
	// QuerySource identifies this fork for analytics and retry policies.
	QuerySource QuerySource
	// ForkLabel is a human-readable label for logging (e.g. "session_memory", "auto_dream").
	ForkLabel string
	// MaxOutputTokens caps the fork's response length. Zero uses model default.
	MaxOutputTokens int
	// MaxTurns limits the agentic loop iterations. Zero uses config default.
	MaxTurns int
	// SkipCacheWrite, when true, shifts the message-level cache marker to
	// avoid polluting the KV cache with ephemeral fork tails.
	// Use for fire-and-forget forks that won't be resumed.
	SkipCacheWrite bool
	// OnMessage is called for each assistant message produced by the fork.
	// May be nil if the caller doesn't need per-message callbacks.
	OnMessage func(content string)
	// PermissionRules restricts the tools the fork can use.
	PermissionRules *PermissionRules
}

// ForkedAgentResult holds the outcome of a forked agent execution.
type ForkedAgentResult struct {
	// Messages is the full conversation produced by the fork.
	Messages []ai.Message
	// TotalUsage is the accumulated token usage across all turns.
	TotalUsage AccumulatedUsage
	// TotalTurns is the number of LLM calls made.
	TotalTurns int
	// TotalCost is the accumulated cost in USD.
	TotalCost float64
	// Error is non-nil if the fork failed.
	Error error
	// Duration is the wall-clock time of the fork execution.
	Duration time.Duration
}

// AccumulatedUsage tracks token usage accumulated across multiple LLM turns.
// Input tokens are from the LATEST turn (replaced each turn), while output
// and cache tokens are SUMMED across all turns.
//
// Mirrors Claude Code's accumulateUsage() pattern in services/api/claude.ts.
type AccumulatedUsage struct {
	// LatestInputTokens is the prompt token count from the most recent turn.
	LatestInputTokens int `json:"latestInputTokens"`
	// CumulativeOutputTokens is the sum of output tokens across all turns.
	CumulativeOutputTokens int `json:"cumulativeOutputTokens"`
	// CumulativeCacheReadTokens is the sum of cache read tokens across all turns.
	CumulativeCacheReadTokens int `json:"cumulativeCacheReadTokens"`
	// CumulativeCacheCreationTokens is the sum of cache creation tokens across all turns.
	CumulativeCacheCreationTokens int `json:"cumulativeCacheCreationTokens"`
	// TotalTokens is the grand total of all tokens.
	TotalTokens int `json:"totalTokens"`
}

// AccumulateUsage merges a single-turn usage into the accumulated totals.
// Input tokens are REPLACED (latest snapshot), output/cache tokens are SUMMED.
func (a *AccumulatedUsage) AccumulateUsage(turn ai.Usage) {
	a.LatestInputTokens = turn.PromptTokens // REPLACED
	a.CumulativeOutputTokens += turn.CompletionTokens
	a.CumulativeCacheReadTokens += turn.CacheReadTokens
	a.CumulativeCacheCreationTokens += turn.CacheCreationTokens
	a.TotalTokens += turn.TotalTokens
}

// CacheHitRate returns the fraction of input tokens served from cache (0.0–1.0).
// Returns 0 if there were no input tokens.
func (a *AccumulatedUsage) CacheHitRate() float64 {
	total := a.LatestInputTokens + a.CumulativeCacheReadTokens
	if total == 0 {
		return 0
	}
	return float64(a.CumulativeCacheReadTokens) / float64(total)
}

// ForkedAgentRunner executes forked agents with isolated context but shared
// cache parameters. It wraps a RunnerFactory to create child runners with
// the appropriate configuration.
type ForkedAgentRunner struct {
	factory   RunnerFactory
	config    RunConfig
	chatModel ai.ChatModel
}

// NewForkedAgentRunner creates a ForkedAgentRunner.
func NewForkedAgentRunner(factory RunnerFactory, config RunConfig, chatModel ai.ChatModel) *ForkedAgentRunner {
	return &ForkedAgentRunner{
		factory:   factory,
		config:    config,
		chatModel: chatModel,
	}
}

// Run executes a forked agent with the given parameters.
// The fork runs in an isolated context: it shares the parent's cache-safe
// parameters (system prompt, tools, model) for prompt cache hits, but has
// its own message history and event channel.
//
// Inspired by Claude Code's runForkedAgent() in utils/forkedAgent.ts.
func (f *ForkedAgentRunner) Run(ctx context.Context, params ForkedAgentParams) ForkedAgentResult {
	start := time.Now()

	// Configure child with fork-appropriate settings.
	childConfig := f.config
	if params.MaxTurns > 0 {
		childConfig.MaxIterations = params.MaxTurns
	}
	if params.MaxOutputTokens > 0 {
		childConfig.MaxTokensPerCall = params.MaxOutputTokens
	}

	// Use CacheSafeParams to override model/provider if available.
	if params.CacheSafeParams != nil {
		if params.CacheSafeParams.Model != "" {
			childConfig.Model = params.CacheSafeParams.Model
		}
		if params.CacheSafeParams.Provider != "" {
			childConfig.Provider = params.CacheSafeParams.Provider
		}
	}

	childRunner := f.factory.NewRunner(childConfig)

	// Create ephemeral session for the fork.
	forkSessionID := uuid.New()

	// Build system prompt from cache-safe params or fall back to empty.
	systemPrompt := ""
	if params.CacheSafeParams != nil {
		systemPrompt = params.CacheSafeParams.SystemPrompt
	}

	childInput := RunInput{
		SessionID:       forkSessionID,
		AgentID:         uuid.Nil, // Forks don't belong to a specific agent.
		UserMessage:     extractUserMessage(params.PromptMessages),
		SystemPrompt:    systemPrompt,
		PermissionRules: params.PermissionRules,
	}

	// Run the forked agent.
	childCh := childRunner.Run(ctx, childInput)

	// Consume events and accumulate usage.
	var result ForkedAgentResult
	for ev := range childCh {
		switch ev.Type {
		case EventTextDelta:
			if params.OnMessage != nil {
				var td TextDeltaData
				if err := json.Unmarshal(ev.Data, &td); err == nil {
					params.OnMessage(td.Content)
				}
			}

		case EventTurnComplete:
			var tc TurnCompleteData
			if err := json.Unmarshal(ev.Data, &tc); err == nil {
				result.TotalUsage.AccumulateUsage(ai.Usage{
					PromptTokens:     tc.TokenUsage.PromptTokens,
					CompletionTokens: tc.TokenUsage.CompletionTokens,
					TotalTokens:      tc.TokenUsage.TotalTokens,
					CacheReadTokens:  tc.TokenUsage.CacheReadTokens,
					CacheCreationTokens: tc.TokenUsage.CacheCreationTokens,
				})
				result.TotalTurns++
			}

		case EventRunComplete:
			var rc RunCompleteData
			if err := json.Unmarshal(ev.Data, &rc); err == nil {
				result.TotalCost = rc.TotalCost
			}

		case EventError:
			var ed ErrorData
			if err := json.Unmarshal(ev.Data, &ed); err == nil {
				result.Error = fmt.Errorf("forked agent %s: %s", params.ForkLabel, ed.Message)
			}
		}
	}

	result.Duration = time.Since(start)

	slog.Info("forked agent completed",
		"label", params.ForkLabel,
		"source", params.QuerySource,
		"turns", result.TotalTurns,
		"totalTokens", result.TotalUsage.TotalTokens,
		"cacheHitRate", fmt.Sprintf("%.1f%%", result.TotalUsage.CacheHitRate()*100),
		"cost", result.TotalCost,
		"duration", result.Duration,
		"skipCacheWrite", params.SkipCacheWrite,
	)

	return result
}

// extractUserMessage gets the content of the first user message from a slice.
func extractUserMessage(messages []ai.Message) string {
	for _, m := range messages {
		if m.Role == ai.RoleUser {
			return m.Content
		}
	}
	return ""
}

// CacheSafeParamsSnapshot is a thread-safe holder for the latest CacheSafeParams.
// The parent saves its params after each turn so that post-turn forks (session
// memory, auto-dream, prompt suggestion) can share the cache prefix.
//
// Inspired by Claude Code's saveCacheSafeParams/getLastCacheSafeParams in forkedAgent.ts.
type CacheSafeParamsSnapshot struct {
	mu     sync.RWMutex
	params *CacheSafeParams
}

// Save stores the latest CacheSafeParams snapshot (thread-safe).
func (s *CacheSafeParamsSnapshot) Save(params *CacheSafeParams) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.params = params
}

// Get retrieves the latest CacheSafeParams snapshot (thread-safe).
// Returns nil if no params have been saved yet.
func (s *CacheSafeParamsSnapshot) Get() *CacheSafeParams {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.params
}

// Clear removes the stored CacheSafeParams.
func (s *CacheSafeParamsSnapshot) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.params = nil
}
