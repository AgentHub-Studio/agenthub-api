package agentic

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// SessionMemoryConfig holds configuration for session-scoped memory extraction.
// Extraction only fires when BOTH token and tool-call thresholds are met,
// preventing excessive extractions during tool-heavy sequences.
//
// Inspired by Claude Code's sessionMemory.ts threshold configuration.
type SessionMemoryConfig struct {
	// MinimumMessageTokensToInit is the minimum total message tokens before the
	// first extraction is allowed. Default 8000.
	MinimumMessageTokensToInit int `json:"minimumMessageTokensToInit"`
	// MinimumTokensBetweenUpdate is the minimum token growth since the last
	// extraction before the next one is allowed. Default 4000.
	MinimumTokensBetweenUpdate int `json:"minimumTokensBetweenUpdate"`
	// ToolCallsBetweenUpdates is the number of tool calls that must occur
	// between extractions. Default 5.
	ToolCallsBetweenUpdates int `json:"toolCallsBetweenUpdates"`
	// MaxOutputTokens caps the extraction fork's response. Default 1024.
	MaxOutputTokens int `json:"maxOutputTokens"`
	// MaxTurns limits the extraction fork's iterations. Default 3.
	MaxTurns int `json:"maxTurns"`
}

// DefaultSessionMemoryConfig returns sensible defaults.
func DefaultSessionMemoryConfig() SessionMemoryConfig {
	return SessionMemoryConfig{
		MinimumMessageTokensToInit: 8000,
		MinimumTokensBetweenUpdate: 4000,
		ToolCallsBetweenUpdates:    5,
		MaxOutputTokens:            1024,
		MaxTurns:                   3,
	}
}

// SessionMemoryExtractor performs background session memory extraction after
// each main-loop turn that meets the threshold criteria. It uses a forked agent
// to analyze the conversation transcript and extract durable memories.
//
// Inspired by Claude Code's extractSessionMemory in services/SessionMemory/sessionMemory.ts.
type SessionMemoryExtractor struct {
	forkRunner *ForkedAgentRunner
	config     SessionMemoryConfig
	tpl        PromptTemplateResolver

	mu                    sync.Mutex
	lastExtractionTokens  int
	lastExtractionTime    time.Time
	toolCallsSinceLastExt int
	totalExtractions      int
}

// NewSessionMemoryExtractor creates a SessionMemoryExtractor.
// forkRunner may be nil (extraction is silently skipped).
func NewSessionMemoryExtractor(forkRunner *ForkedAgentRunner, config SessionMemoryConfig) *SessionMemoryExtractor {
	return &SessionMemoryExtractor{
		forkRunner: forkRunner,
		config:     config,
	}
}

// WithPromptTemplateResolver attaches an optional resolver for the extraction prompt.
func (e *SessionMemoryExtractor) WithPromptTemplateResolver(resolver PromptTemplateResolver) *SessionMemoryExtractor {
	e.tpl = resolver
	return e
}

// TrackToolCall increments the tool call counter. Called by the runner after
// each tool execution completes.
func (e *SessionMemoryExtractor) TrackToolCall() {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.toolCallsSinceLastExt++
}

// ShouldExtract evaluates the dual threshold gates:
//  1. Token threshold: enough new tokens since last extraction
//  2. Tool threshold: enough tool calls since last extraction OR the current
//     turn has no tool calls (safe extraction point)
//
// Both must be satisfied for extraction to proceed.
//
// Inspired by Claude Code's shouldExtractMemory() in sessionMemory.ts.
func (e *SessionMemoryExtractor) ShouldExtract(currentTokens int, turnHasToolCalls bool) bool {
	if e == nil || e.forkRunner == nil {
		return false
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	// Token threshold: first extraction requires init threshold, subsequent
	// extractions require growth threshold.
	if e.totalExtractions == 0 {
		if currentTokens < e.config.MinimumMessageTokensToInit {
			return false
		}
	} else {
		tokenGrowth := currentTokens - e.lastExtractionTokens
		if tokenGrowth < e.config.MinimumTokensBetweenUpdate {
			return false
		}
	}

	// Tool threshold: either enough tool calls have accumulated, or this is
	// a non-tool turn (safe extraction point without interrupting tool sequences).
	toolThreshold := e.toolCallsSinceLastExt >= e.config.ToolCallsBetweenUpdates
	noToolsThisTurn := !turnHasToolCalls

	return toolThreshold || noToolsThisTurn
}

// Extract runs the session memory extraction as a fire-and-forget forked agent.
// It captures the cache-safe params from the parent to maximize cache hits.
// This method is non-blocking — the extraction runs in a background goroutine.
//
// Inspired by Claude Code's extractSessionMemory sequential hook.
func (e *SessionMemoryExtractor) Extract(
	ctx context.Context,
	agentID uuid.UUID,
	cacheSafeParams *CacheSafeParams,
	currentTokens int,
	permissionRules *PermissionRules,
) {
	if e == nil || e.forkRunner == nil {
		return
	}

	// Update state.
	e.mu.Lock()
	e.lastExtractionTokens = currentTokens
	e.lastExtractionTime = time.Now()
	e.toolCallsSinceLastExt = 0
	e.totalExtractions++
	extractionNum := e.totalExtractions
	e.mu.Unlock()

	baseCtx := detachedContext(ctx)

	// Run extraction in background (fire-and-forget).
	go func() {
		// Preserve request values while detaching from request cancellation.
		extractCtx, cancel := context.WithTimeout(baseCtx, 60*time.Second)
		defer cancel()

		result := e.forkRunner.Run(extractCtx, ForkedAgentParams{
			PromptMessages: []ai.Message{
				{
					Role:    ai.RoleUser,
					Content: e.resolveExtractionPrompt(extractCtx, agentID),
				},
			},
			CacheSafeParams: cacheSafeParams,
			QuerySource:     SourceMemoryEval,
			ForkLabel:       "session_memory",
			MaxOutputTokens: e.config.MaxOutputTokens,
			MaxTurns:        e.config.MaxTurns,
			SkipCacheWrite:  true, // Fire-and-forget — don't pollute KV cache.
			PermissionRules: restrictToMemoryPermissions(permissionRules),
		})

		if result.Error != nil {
			slog.Warn("session memory extraction failed",
				"extraction", extractionNum,
				"error", result.Error,
			)
			return
		}

		slog.Info("session memory extraction completed",
			"extraction", extractionNum,
			"turns", result.TotalTurns,
			"tokens", result.TotalUsage.TotalTokens,
			"cacheHitRate", result.TotalUsage.CacheHitRate(),
			"duration", result.Duration,
		)
	}()
}

func detachedContext(parent context.Context) context.Context {
	if parent == nil {
		return context.Background()
	}
	return context.WithoutCancel(parent)
}

func (e *SessionMemoryExtractor) resolveExtractionPrompt(ctx context.Context, agentID uuid.UUID) string {
	if e == nil {
		return sessionMemoryExtractionPrompt
	}
	return resolvePromptTemplateOrFallback(
		ctx,
		e.tpl,
		agentID,
		promptTemplateSlugSessionMemoryExtractionPrompt,
		sessionMemoryExtractionPrompt,
	)
}

// Stats returns the current extraction statistics.
func (e *SessionMemoryExtractor) Stats() SessionMemoryStats {
	if e == nil {
		return SessionMemoryStats{}
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return SessionMemoryStats{
		TotalExtractions:      e.totalExtractions,
		LastExtractionTokens:  e.lastExtractionTokens,
		LastExtractionTime:    e.lastExtractionTime,
		ToolCallsSinceLastExt: e.toolCallsSinceLastExt,
	}
}

// SessionMemoryStats holds observable extraction state.
type SessionMemoryStats struct {
	TotalExtractions      int       `json:"totalExtractions"`
	LastExtractionTokens  int       `json:"lastExtractionTokens"`
	LastExtractionTime    time.Time `json:"lastExtractionTime,omitempty"`
	ToolCallsSinceLastExt int       `json:"toolCallsSinceLastExt"`
}

// restrictToMemoryPermissions creates permission rules that only allow:
// - Read-only operations (file read, search)
// - Write operations restricted to memory paths
// This prevents the extraction fork from making arbitrary changes.
//
// Inspired by Claude Code's createAutoMemCanUseTool() in extractMemories.ts.
func restrictToMemoryPermissions(parent *PermissionRules) *PermissionRules {
	// Memory extraction forks are restricted to read-only tools plus memory_store.
	// We use the Allow/Deny list model from PermissionRules.
	return &PermissionRules{
		Allow: []string{"document_search", "memory_store"},
		Mode:  PermissionModeDontAsk,
	}
}

// sessionMemoryExtractionPrompt is the prompt sent to the forked agent to
// extract durable memories from the conversation transcript.
const sessionMemoryExtractionPrompt = `Review the conversation above and extract any information that should be remembered for future sessions.

Focus on:
- User preferences and corrections (feedback type)
- User role, expertise, and goals (user type)
- Project decisions, deadlines, and context (project type)
- External resource pointers mentioned (reference type)

For each memory, use the memory_store tool with:
- key: short snake_case identifier (e.g., "preferred_language", "project_deadline")
- value: concise description of the information
- memoryType: one of user, feedback, project, reference, general

Only store information that is:
- Durable (useful beyond the current session)
- Non-obvious (not derivable from code or git history)
- Actionable (helps tailor future assistance)

If there is nothing worth remembering, do not call memory_store.`
