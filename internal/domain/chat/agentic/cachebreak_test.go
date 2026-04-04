package agentic_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- Constants ---

func TestPromptCacheBreakThreshold(t *testing.T) {
	assert.Equal(t, 0.05, agentic.PromptCacheBreakThreshold)
}

// --- PromptCacheBreakChange ---

func TestPromptCacheBreakChange_HasChanges(t *testing.T) {
	assert.False(t, agentic.PromptCacheBreakChange{}.HasChanges())
	assert.True(t, agentic.PromptCacheBreakChange{SystemPromptChanged: true}.HasChanges())
	assert.True(t, agentic.PromptCacheBreakChange{ModelChanged: true}.HasChanges())
	assert.True(t, agentic.PromptCacheBreakChange{AddedTools: []string{"Read"}}.HasChanges())
	assert.True(t, agentic.PromptCacheBreakChange{RemovedTools: []string{"Write"}}.HasChanges())
	assert.True(t, agentic.PromptCacheBreakChange{ChangedToolSchemas: []string{"Bash"}}.HasChanges())
}

func TestPromptCacheBreakChange_Summary_NoChanges(t *testing.T) {
	s := agentic.PromptCacheBreakChange{}.Summary()
	assert.Equal(t, "no changes detected", s)
}

func TestPromptCacheBreakChange_Summary_WithChanges(t *testing.T) {
	c := agentic.PromptCacheBreakChange{
		SystemPromptChanged: true,
		AddedTools:          []string{"Read", "Glob"},
		RemovedTools:        []string{"Write"},
	}
	s := c.Summary()
	assert.Contains(t, s, "system prompt changed")
	assert.Contains(t, s, "2 tools added")
	assert.Contains(t, s, "1 tools removed")
}

// --- DetectPromptChanges ---

func TestDetectPromptChanges_NilInputs(t *testing.T) {
	c := agentic.DetectPromptChanges(nil, nil)
	assert.False(t, c.HasChanges())
}

func TestDetectPromptChanges_SystemPrompt(t *testing.T) {
	prev := &agentic.PromptStateSnapshot{SystemPromptHash: "abc"}
	curr := &agentic.PromptStateSnapshot{SystemPromptHash: "def"}
	c := agentic.DetectPromptChanges(prev, curr)
	assert.True(t, c.SystemPromptChanged)
}

func TestDetectPromptChanges_NoChange(t *testing.T) {
	snap := &agentic.PromptStateSnapshot{
		SystemPromptHash: "same",
		Model:            "claude-3",
		ToolSchemaHashes: map[string]string{"Read": "h1", "Write": "h2"},
	}
	c := agentic.DetectPromptChanges(snap, snap)
	assert.False(t, c.HasChanges())
}

func TestDetectPromptChanges_ToolDiff(t *testing.T) {
	prev := &agentic.PromptStateSnapshot{
		ToolSchemaHashes: map[string]string{"Read": "h1", "Write": "h2", "Old": "h3"},
	}
	curr := &agentic.PromptStateSnapshot{
		ToolSchemaHashes: map[string]string{"Read": "h1", "Write": "h2_changed", "New": "h4"},
	}

	c := agentic.DetectPromptChanges(prev, curr)
	assert.Contains(t, c.AddedTools, "New")
	assert.Contains(t, c.RemovedTools, "Old")
	assert.Contains(t, c.ChangedToolSchemas, "Write")
}

func TestDetectPromptChanges_ModelChanged(t *testing.T) {
	prev := &agentic.PromptStateSnapshot{Model: "claude-3"}
	curr := &agentic.PromptStateSnapshot{Model: "claude-4"}
	c := agentic.DetectPromptChanges(prev, curr)
	assert.True(t, c.ModelChanged)
}

func TestDetectPromptChanges_FastMode(t *testing.T) {
	prev := &agentic.PromptStateSnapshot{FastMode: false}
	curr := &agentic.PromptStateSnapshot{FastMode: true}
	c := agentic.DetectPromptChanges(prev, curr)
	assert.True(t, c.FastModeChanged)
}

func TestDetectPromptChanges_Effort(t *testing.T) {
	prev := &agentic.PromptStateSnapshot{EffortValue: "low"}
	curr := &agentic.PromptStateSnapshot{EffortValue: "high"}
	c := agentic.DetectPromptChanges(prev, curr)
	assert.True(t, c.EffortChanged)
}

// --- PromptCacheDiagnostics ---

func TestPromptCacheDiagnostics_New(t *testing.T) {
	d := agentic.NewPromptCacheDiagnostics(10)
	assert.Equal(t, 0, d.SourceCount())
}

func TestPromptCacheDiagnostics_RecordPreCall(t *testing.T) {
	d := agentic.NewPromptCacheDiagnostics(10)
	d.RecordPreCall("agent-1", agentic.PromptStateSnapshot{
		SystemPromptHash: "abc",
		Model:            "claude-3",
	})
	assert.Equal(t, 1, d.SourceCount())
}

func TestPromptCacheDiagnostics_NoBreak(t *testing.T) {
	d := agentic.NewPromptCacheDiagnostics(10)

	d.RecordPreCall("agent-1", agentic.PromptStateSnapshot{Model: "claude-3"})
	event := d.RecordPostCall("agent-1", 1000)
	assert.Nil(t, event)

	d.RecordPreCall("agent-1", agentic.PromptStateSnapshot{Model: "claude-3"})
	event = d.RecordPostCall("agent-1", 960) // 4% drop, below 5% threshold
	assert.Nil(t, event)
}

func TestPromptCacheDiagnostics_DetectsBreak(t *testing.T) {
	d := agentic.NewPromptCacheDiagnostics(10)

	d.RecordPreCall("agent-1", agentic.PromptStateSnapshot{Model: "claude-3"})
	d.RecordPostCall("agent-1", 1000)

	d.RecordPreCall("agent-1", agentic.PromptStateSnapshot{Model: "claude-4"})
	event := d.RecordPostCall("agent-1", 100) // 90% drop

	require.NotNil(t, event)
	assert.Equal(t, "agent-1", event.Source)
	assert.Equal(t, int64(1000), event.PreviousTokens)
	assert.Equal(t, int64(100), event.CurrentTokens)
	assert.Greater(t, event.DropPercent, 5.0)
}

func TestPromptCacheDiagnostics_Events(t *testing.T) {
	d := agentic.NewPromptCacheDiagnostics(10)

	d.RecordPreCall("a", agentic.PromptStateSnapshot{})
	d.RecordPostCall("a", 1000)
	d.RecordPreCall("a", agentic.PromptStateSnapshot{})
	d.RecordPostCall("a", 0) // 100% drop

	events := d.Events()
	assert.Len(t, events, 1)

	d.ClearEvents()
	assert.Empty(t, d.Events())
}

func TestPromptCacheDiagnostics_EvictsOldest(t *testing.T) {
	d := agentic.NewPromptCacheDiagnostics(2)

	d.RecordPreCall("s1", agentic.PromptStateSnapshot{Timestamp: time.Now().Add(-2 * time.Hour)})
	d.RecordPreCall("s2", agentic.PromptStateSnapshot{Timestamp: time.Now().Add(-1 * time.Hour)})
	assert.Equal(t, 2, d.SourceCount())

	d.RecordPreCall("s3", agentic.PromptStateSnapshot{Timestamp: time.Now()})
	assert.Equal(t, 2, d.SourceCount())
}

// --- HashJSONContent ---

func TestHashJSONContent(t *testing.T) {
	h1 := agentic.HashJSONContent("hello")
	h2 := agentic.HashJSONContent("hello")
	h3 := agentic.HashJSONContent("world")

	assert.Equal(t, h1, h2, "same input should produce same hash")
	assert.NotEqual(t, h1, h3, "different inputs should produce different hashes")
	assert.Len(t, h1, 64, "SHA-256 hex is 64 chars")
}

func TestHashJSONContent_Struct(t *testing.T) {
	h := agentic.HashJSONContent(map[string]int{"a": 1, "b": 2})
	assert.NotEmpty(t, h)
}
