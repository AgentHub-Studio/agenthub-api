package agentic

import (
	"strings"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// CacheScope determines the level at which prompt cache is shared.
// Inspired by Claude Code's CacheScope in utils/api.ts.
type CacheScope string

const (
	// CacheScopeGlobal shares the cache key across all users. Only safe
	// for purely static content (identity text before any user-specific injection).
	CacheScopeGlobal CacheScope = "global"
	// CacheScopeOrg shares the cache key within a single tenant/organization.
	// Appropriate for system prompts that include tenant-level configuration.
	CacheScopeOrg CacheScope = "org"
)

// CacheTTL controls the time-to-live for a cached prompt prefix.
// Inspired by Claude Code's getCacheControl TTL logic.
type CacheTTL string

const (
	// CacheTTL5Min is the default 5-minute ephemeral cache (Anthropic default when
	// no TTL is specified in cache_control).
	CacheTTL5Min CacheTTL = "5m"
	// CacheTTL1Hour is the extended 1-hour cache, available for eligible accounts.
	CacheTTL1Hour CacheTTL = "1h"
)

// CacheTTL5MinMs and CacheTTL1HourMs are the TTL durations in milliseconds,
// used for cache break detection and TTL expiry analysis.
const (
	CacheTTL5MinMs  = 5 * 60 * 1000
	CacheTTL1HourMs = 60 * 60 * 1000
)

// SystemPromptBlock represents a section of the system prompt with an associated
// cache scope. Splitting the prompt into blocks allows different cache sharing
// levels for static vs dynamic content.
//
// Inspired by Claude Code's SystemPromptBlock in utils/api.ts:
// "{ text: string; cacheScope: CacheScope | null }"
type SystemPromptBlock struct {
	// Text is the content of this prompt section.
	Text string
	// CacheScope determines how this block's cache is shared.
	// Nil means no caching for this block (e.g. attribution headers).
	CacheScope *CacheScope
}

// CacheControlMarker is the cache_control annotation attached to API message
// blocks or tool definitions. Mirrors Anthropic's API schema.
type CacheControlMarker struct {
	// Type is always "ephemeral" for Anthropic's prompt caching.
	Type string `json:"type"`
	// Scope optionally narrows the cache key sharing level.
	Scope *CacheScope `json:"scope,omitempty"`
	// TTL optionally extends the cache duration beyond the default 5 minutes.
	TTL *CacheTTL `json:"ttl,omitempty"`
}

// GetCacheControl builds a CacheControlMarker for the given scope and query source.
// Background sources (compact, memory_eval) get the default 5-minute TTL.
// Foreground sources on eligible accounts get the extended 1-hour TTL.
//
// Inspired by Claude Code's getCacheControl() in services/api/claude.ts.
func GetCacheControl(scope *CacheScope, source QuerySource, extendedTTLEligible bool) *CacheControlMarker {
	marker := &CacheControlMarker{Type: "ephemeral"}

	// Extended TTL for foreground sources on eligible accounts.
	if extendedTTLEligible && source.IsForegroundSource() {
		ttl := CacheTTL1Hour
		marker.TTL = &ttl
	}

	// Global scope only applies for explicitly global blocks.
	if scope != nil && *scope == CacheScopeGlobal {
		s := CacheScopeGlobal
		marker.Scope = &s
	}

	return marker
}

// dynamicBoundaryMarker is the sentinel that separates static (cacheable globally)
// from dynamic (per-tenant) content in the system prompt.
const dynamicBoundaryMarker = "<!-- DYNAMIC_BOUNDARY -->"

// SplitSystemPromptBlocks splits a system prompt into blocks with appropriate
// cache scopes. The strategy depends on whether the prompt contains a dynamic
// boundary marker and whether MCP/external tools are present.
//
// Inspired by Claude Code's splitSysPromptPrefix() in utils/api.ts.
//
// Three modes:
//  1. Has MCP/external tools (skipGlobal=true): All blocks get CacheScopeOrg
//  2. Contains dynamic boundary: Static prefix gets CacheScopeGlobal, rest gets nil
//  3. Default: All blocks get CacheScopeOrg
func SplitSystemPromptBlocks(prompt string, hasMCPTools bool) []SystemPromptBlock {
	if prompt == "" {
		return nil
	}

	org := CacheScopeOrg

	// Mode 1: MCP tools present — can't use global cache (tool schemas vary per user).
	if hasMCPTools {
		return []SystemPromptBlock{
			{Text: prompt, CacheScope: &org},
		}
	}

	// Mode 2: Dynamic boundary present — split into global (static) + non-cached (dynamic).
	if idx := strings.Index(prompt, dynamicBoundaryMarker); idx >= 0 {
		global := CacheScopeGlobal
		staticPart := strings.TrimSpace(prompt[:idx])
		dynamicPart := strings.TrimSpace(prompt[idx+len(dynamicBoundaryMarker):])

		var blocks []SystemPromptBlock
		if staticPart != "" {
			blocks = append(blocks, SystemPromptBlock{Text: staticPart, CacheScope: &global})
		}
		if dynamicPart != "" {
			blocks = append(blocks, SystemPromptBlock{Text: dynamicPart, CacheScope: nil})
		}
		return blocks
	}

	// Mode 3: Default — entire prompt cached at org level.
	return []SystemPromptBlock{
		{Text: prompt, CacheScope: &org},
	}
}

// BuildSystemPromptTextBlocks converts SystemPromptBlocks into the text block
// parameters sent to the LLM API. Each block that has a non-nil CacheScope gets
// a cache_control marker attached.
//
// Inspired by Claude Code's buildSystemPromptBlocks() in services/api/claude.ts.
func BuildSystemPromptTextBlocks(
	blocks []SystemPromptBlock,
	enableCaching bool,
	source QuerySource,
	extendedTTLEligible bool,
) []SystemPromptTextBlock {
	result := make([]SystemPromptTextBlock, 0, len(blocks))
	for _, b := range blocks {
		tb := SystemPromptTextBlock{
			Text: b.Text,
		}
		if enableCaching && b.CacheScope != nil {
			tb.CacheControl = GetCacheControl(b.CacheScope, source, extendedTTLEligible)
		}
		result = append(result, tb)
	}
	return result
}

// SystemPromptTextBlock is the wire format for a system prompt block with
// optional cache control. Maps to Anthropic's TextBlockParam.
type SystemPromptTextBlock struct {
	Type         string              `json:"type"`
	Text         string              `json:"text"`
	CacheControl *CacheControlMarker `json:"cache_control,omitempty"`
}

// AddMessageCacheBreakpoints places exactly ONE cache_control marker in the
// message array. The marker goes on the last user message so that the shared
// prefix (system prompt + history up to the latest user turn) is cached.
//
// When skipCacheWrite is true (fire-and-forget forks that won't be resumed),
// the marker shifts to the second-to-last user message to avoid polluting the
// KV cache with ephemeral fork tails.
//
// Inspired by Claude Code's addCacheBreakpoints() in services/api/claude.ts.
func AddMessageCacheBreakpoints(
	messages []ai.Message,
	enableCaching bool,
	source QuerySource,
	extendedTTLEligible bool,
	skipCacheWrite bool,
) []ai.Message {
	if !enableCaching || len(messages) == 0 {
		return messages
	}

	// Find the target message index for the cache marker.
	targetIdx := findLastUserMessageIndex(messages)
	if skipCacheWrite && targetIdx > 0 {
		// Shift to second-to-last user message for fire-and-forget forks.
		targetIdx = findLastUserMessageIndexBefore(messages, targetIdx)
	}

	if targetIdx < 0 {
		return messages
	}

	// Clone messages to avoid mutating the caller's slice.
	result := make([]ai.Message, len(messages))
	copy(result, messages)

	// Attach cache_control marker to the target message's metadata.
	marker := GetCacheControl(nil, source, extendedTTLEligible)
	msg := result[targetIdx]
	if msg.Metadata == nil {
		msg.Metadata = make(map[string]any)
	}
	msg.Metadata["cache_control"] = marker
	result[targetIdx] = msg

	return result
}

// findLastUserMessageIndex returns the index of the last user message, or -1.
func findLastUserMessageIndex(messages []ai.Message) int {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == ai.RoleUser {
			return i
		}
	}
	return -1
}

// findLastUserMessageIndexBefore returns the index of the last user message
// before the given index, or -1.
func findLastUserMessageIndexBefore(messages []ai.Message, before int) int {
	for i := before - 1; i >= 0; i-- {
		if messages[i].Role == ai.RoleUser {
			return i
		}
	}
	return -1
}

// GlobalCacheStrategy describes how global cache scope is applied to the request.
// Inspired by Claude Code's GlobalCacheStrategy type.
type GlobalCacheStrategy string

const (
	// GlobalCacheStrategySystemPrompt applies global cache to system prompt blocks.
	GlobalCacheStrategySystemPrompt GlobalCacheStrategy = "system_prompt"
	// GlobalCacheStrategyNone disables global caching (MCP tools or feature disabled).
	GlobalCacheStrategyNone GlobalCacheStrategy = "none"
)

// DetermineGlobalCacheStrategy decides whether global cache scope can be used.
// Global cache is disabled when external/MCP tools are present (their schemas
// vary per user, breaking the global cache key).
//
// Inspired by Claude Code's globalCacheStrategy logic in services/api/claude.ts.
func DetermineGlobalCacheStrategy(tools []ai.Tool, hasMCPTools bool) GlobalCacheStrategy {
	if hasMCPTools {
		return GlobalCacheStrategyNone
	}
	return GlobalCacheStrategySystemPrompt
}
