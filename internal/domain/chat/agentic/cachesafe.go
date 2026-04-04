package agentic

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// CacheSafeParams captures the cache-critical parameters that must remain identical
// between a parent agent and its forked/sub-agents to guarantee prompt cache hits.
// When a sub-agent uses the same CacheSafeParams as its parent, the LLM provider
// can reuse the cached prompt prefix, saving 24–90% of input tokens.
//
// Inspired by Claude Code's CacheSafeParams in utils/forkedAgent.ts:
// "Immutable struct capturing all cache-critical parameters. Forked agents
// reuse this struct to guarantee prompt cache hits with the parent."
type CacheSafeParams struct {
	// SystemPrompt is the full system prompt (including tool descriptions, memories, etc.).
	SystemPrompt string
	// Tools is the tool definitions array sent to the LLM.
	Tools []ai.Tool
	// Provider is the LLM provider name (e.g. "anthropic", "openai").
	Provider string
	// Model is the LLM model identifier.
	Model string
	// CacheControl indicates whether cache_control markers should be sent.
	CacheControl bool

	// hash is lazily computed for comparison.
	hash string
}

// NewCacheSafeParams captures the cache-critical parameters at query start.
func NewCacheSafeParams(systemPrompt string, tools []ai.Tool, provider, model string, cacheControl bool) *CacheSafeParams {
	return &CacheSafeParams{
		SystemPrompt: systemPrompt,
		Tools:        tools,
		Provider:     provider,
		Model:        model,
		CacheControl: cacheControl,
	}
}

// Hash returns a stable hash of the cache-critical parameters.
// Two CacheSafeParams with the same hash will share prompt cache.
func (c *CacheSafeParams) Hash() string {
	if c == nil {
		return ""
	}
	if c.hash != "" {
		return c.hash
	}

	h := sha256.New()
	h.Write([]byte(c.SystemPrompt))
	if toolsJSON, err := json.Marshal(c.Tools); err == nil {
		h.Write(toolsJSON)
	}
	h.Write([]byte(c.Provider))
	h.Write([]byte(c.Model))
	if c.CacheControl {
		h.Write([]byte("cache"))
	}

	c.hash = hex.EncodeToString(h.Sum(nil))[:16]
	return c.hash
}

// Matches returns true if two CacheSafeParams would share prompt cache.
func (c *CacheSafeParams) Matches(other *CacheSafeParams) bool {
	if c == nil || other == nil {
		return c == other
	}
	return c.Hash() == other.Hash()
}

// Note: CacheBreakDetector is already implemented in analytics.go with full
// per-source tracking, TTL-aware expiry detection, and compaction awareness.
