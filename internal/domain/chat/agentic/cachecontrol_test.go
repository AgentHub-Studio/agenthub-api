package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// --- GetCacheControl ---

func TestGetCacheControl_ForegroundWithExtendedTTL(t *testing.T) {
	scope := agentic.CacheScopeOrg
	marker := agentic.GetCacheControl(&scope, agentic.SourceMainLoop, true)

	assert.Equal(t, "ephemeral", marker.Type)
	assert.NotNil(t, marker.TTL)
	assert.Equal(t, agentic.CacheTTL1Hour, *marker.TTL)
	// Org scope is NOT propagated (only global is).
	assert.Nil(t, marker.Scope)
}

func TestGetCacheControl_BackgroundNoExtendedTTL(t *testing.T) {
	marker := agentic.GetCacheControl(nil, agentic.SourceCompact, true)

	assert.Equal(t, "ephemeral", marker.Type)
	assert.Nil(t, marker.TTL, "background sources should not get extended TTL")
	assert.Nil(t, marker.Scope)
}

func TestGetCacheControl_GlobalScope(t *testing.T) {
	scope := agentic.CacheScopeGlobal
	marker := agentic.GetCacheControl(&scope, agentic.SourceMainLoop, false)

	assert.Equal(t, "ephemeral", marker.Type)
	assert.NotNil(t, marker.Scope)
	assert.Equal(t, agentic.CacheScopeGlobal, *marker.Scope)
	assert.Nil(t, marker.TTL, "not eligible for extended TTL")
}

func TestGetCacheControl_NotEligible(t *testing.T) {
	marker := agentic.GetCacheControl(nil, agentic.SourceMainLoop, false)

	assert.Equal(t, "ephemeral", marker.Type)
	assert.Nil(t, marker.TTL)
	assert.Nil(t, marker.Scope)
}

// --- SplitSystemPromptBlocks ---

func TestSplitSystemPromptBlocks_Empty(t *testing.T) {
	blocks := agentic.SplitSystemPromptBlocks("", false)
	assert.Nil(t, blocks)
}

func TestSplitSystemPromptBlocks_Default(t *testing.T) {
	blocks := agentic.SplitSystemPromptBlocks("You are a helpful assistant.", false)

	assert.Len(t, blocks, 1)
	assert.Equal(t, "You are a helpful assistant.", blocks[0].Text)
	assert.NotNil(t, blocks[0].CacheScope)
	assert.Equal(t, agentic.CacheScopeOrg, *blocks[0].CacheScope)
}

func TestSplitSystemPromptBlocks_MCPToolsForceOrg(t *testing.T) {
	blocks := agentic.SplitSystemPromptBlocks("prompt text", true)

	assert.Len(t, blocks, 1)
	assert.NotNil(t, blocks[0].CacheScope)
	assert.Equal(t, agentic.CacheScopeOrg, *blocks[0].CacheScope)
}

func TestSplitSystemPromptBlocks_DynamicBoundary(t *testing.T) {
	prompt := "Static identity\n<!-- DYNAMIC_BOUNDARY -->\nDynamic per-user content"
	blocks := agentic.SplitSystemPromptBlocks(prompt, false)

	assert.Len(t, blocks, 2)
	// Static part gets global scope.
	assert.Equal(t, "Static identity", blocks[0].Text)
	assert.NotNil(t, blocks[0].CacheScope)
	assert.Equal(t, agentic.CacheScopeGlobal, *blocks[0].CacheScope)
	// Dynamic part gets no caching.
	assert.Equal(t, "Dynamic per-user content", blocks[1].Text)
	assert.Nil(t, blocks[1].CacheScope)
}

func TestSplitSystemPromptBlocks_DynamicBoundaryOnlyStatic(t *testing.T) {
	prompt := "Static content\n<!-- DYNAMIC_BOUNDARY -->"
	blocks := agentic.SplitSystemPromptBlocks(prompt, false)

	assert.Len(t, blocks, 1)
	assert.Equal(t, "Static content", blocks[0].Text)
	assert.Equal(t, agentic.CacheScopeGlobal, *blocks[0].CacheScope)
}

func TestSplitSystemPromptBlocks_MCPOverridesDynamicBoundary(t *testing.T) {
	prompt := "Static\n<!-- DYNAMIC_BOUNDARY -->\nDynamic"
	blocks := agentic.SplitSystemPromptBlocks(prompt, true)

	// MCP tools present — should NOT split, use org scope for everything.
	assert.Len(t, blocks, 1)
	assert.Equal(t, agentic.CacheScopeOrg, *blocks[0].CacheScope)
}

// --- BuildSystemPromptTextBlocks ---

func TestBuildSystemPromptTextBlocks_CachingEnabled(t *testing.T) {
	org := agentic.CacheScopeOrg
	blocks := []agentic.SystemPromptBlock{
		{Text: "cached block", CacheScope: &org},
		{Text: "uncached block", CacheScope: nil},
	}

	result := agentic.BuildSystemPromptTextBlocks(blocks, true, agentic.SourceMainLoop, false)

	assert.Len(t, result, 2)
	assert.NotNil(t, result[0].CacheControl, "cached block should have marker")
	assert.Equal(t, "ephemeral", result[0].CacheControl.Type)
	assert.Nil(t, result[1].CacheControl, "uncached block should not have marker")
}

func TestBuildSystemPromptTextBlocks_CachingDisabled(t *testing.T) {
	org := agentic.CacheScopeOrg
	blocks := []agentic.SystemPromptBlock{
		{Text: "block", CacheScope: &org},
	}

	result := agentic.BuildSystemPromptTextBlocks(blocks, false, agentic.SourceMainLoop, false)

	assert.Len(t, result, 1)
	assert.Nil(t, result[0].CacheControl, "caching disabled — no markers")
}

// --- AddMessageCacheBreakpoints ---

func TestAddMessageCacheBreakpoints_Disabled(t *testing.T) {
	msgs := []ai.Message{{Role: ai.RoleUser, Content: "hello"}}
	result := agentic.AddMessageCacheBreakpoints(msgs, false, agentic.SourceMainLoop, false, false)
	assert.Equal(t, msgs, result)
}

func TestAddMessageCacheBreakpoints_Empty(t *testing.T) {
	result := agentic.AddMessageCacheBreakpoints(nil, true, agentic.SourceMainLoop, false, false)
	assert.Nil(t, result)
}

func TestAddMessageCacheBreakpoints_MarkerOnLastUser(t *testing.T) {
	msgs := []ai.Message{
		{Role: ai.RoleUser, Content: "first"},
		{Role: ai.RoleAssistant, Content: "reply"},
		{Role: ai.RoleUser, Content: "second"},
	}

	result := agentic.AddMessageCacheBreakpoints(msgs, true, agentic.SourceMainLoop, false, false)

	// Last user message (index 2) should have cache_control metadata.
	assert.NotNil(t, result[2].Metadata)
	assert.Contains(t, result[2].Metadata, "cache_control")
	// Others should not.
	assert.Nil(t, result[0].Metadata)
	assert.Nil(t, result[1].Metadata)
}

func TestAddMessageCacheBreakpoints_SkipCacheWrite(t *testing.T) {
	msgs := []ai.Message{
		{Role: ai.RoleUser, Content: "first"},
		{Role: ai.RoleAssistant, Content: "reply"},
		{Role: ai.RoleUser, Content: "second"},
	}

	result := agentic.AddMessageCacheBreakpoints(msgs, true, agentic.SourceMainLoop, false, true)

	// With skipCacheWrite, marker shifts to second-to-last user message (index 0).
	assert.NotNil(t, result[0].Metadata)
	assert.Contains(t, result[0].Metadata, "cache_control")
	// Last user message should NOT have marker.
	assert.Nil(t, result[2].Metadata)
}

func TestAddMessageCacheBreakpoints_NoUserMessages(t *testing.T) {
	msgs := []ai.Message{{Role: ai.RoleAssistant, Content: "only assistant"}}
	result := agentic.AddMessageCacheBreakpoints(msgs, true, agentic.SourceMainLoop, false, false)
	// No user messages — no marker placed.
	assert.Nil(t, result[0].Metadata)
}

func TestAddMessageCacheBreakpoints_DoesNotMutateOriginal(t *testing.T) {
	msgs := []ai.Message{{Role: ai.RoleUser, Content: "hello"}}
	_ = agentic.AddMessageCacheBreakpoints(msgs, true, agentic.SourceMainLoop, false, false)
	// Original should not be mutated.
	assert.Nil(t, msgs[0].Metadata)
}

// --- DetermineGlobalCacheStrategy ---

func TestDetermineGlobalCacheStrategy_NoMCP(t *testing.T) {
	strategy := agentic.DetermineGlobalCacheStrategy(nil, false)
	assert.Equal(t, agentic.GlobalCacheStrategySystemPrompt, strategy)
}

func TestDetermineGlobalCacheStrategy_WithMCP(t *testing.T) {
	strategy := agentic.DetermineGlobalCacheStrategy(nil, true)
	assert.Equal(t, agentic.GlobalCacheStrategyNone, strategy)
}

// --- TTL constants ---

func TestCacheTTLConstants(t *testing.T) {
	assert.Equal(t, 300000, agentic.CacheTTL5MinMs)
	assert.Equal(t, 3600000, agentic.CacheTTL1HourMs)
}
