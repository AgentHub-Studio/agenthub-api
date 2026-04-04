package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- GenerateContextSuggestions: near capacity ---

func TestContextSuggestions_NearCapacityWithAutocompact(t *testing.T) {
	data := agentic.ContextAnalysisData{
		Percentage:           85,
		RawMaxTokens:         100000,
		IsAutoCompactEnabled: true,
	}
	suggestions := agentic.GenerateContextSuggestions(data)
	require.Len(t, suggestions, 1)
	assert.Equal(t, agentic.CtxSeverityWarning, suggestions[0].Severity)
	assert.Contains(t, suggestions[0].Title, "85%")
	assert.Contains(t, suggestions[0].Detail, "Autocompact will trigger soon")
}

func TestContextSuggestions_NearCapacityWithoutAutocompact(t *testing.T) {
	data := agentic.ContextAnalysisData{
		Percentage:           90,
		RawMaxTokens:         100000,
		IsAutoCompactEnabled: false,
	}
	suggestions := agentic.GenerateContextSuggestions(data)
	require.Len(t, suggestions, 1)
	assert.Contains(t, suggestions[0].Detail, "Autocompact is disabled")
}

func TestContextSuggestions_BelowCapacity(t *testing.T) {
	data := agentic.ContextAnalysisData{
		Percentage:           50,
		RawMaxTokens:         100000,
		IsAutoCompactEnabled: true,
	}
	suggestions := agentic.GenerateContextSuggestions(data)
	assert.Empty(t, suggestions)
}

// --- Large tool results ---

func TestContextSuggestions_LargeBashResults(t *testing.T) {
	data := agentic.ContextAnalysisData{
		Percentage:           40,
		RawMaxTokens:         100000,
		IsAutoCompactEnabled: true,
		MessageBreakdown: &agentic.MessageBreakdown{
			ToolCallsByType: []agentic.ToolUsageStats{
				{Name: "Bash", CallTokens: 1000, ResultTokens: 20000},
			},
		},
	}
	suggestions := agentic.GenerateContextSuggestions(data)
	require.Len(t, suggestions, 1)
	assert.Equal(t, agentic.CtxSeverityWarning, suggestions[0].Severity)
	assert.Contains(t, suggestions[0].Title, "Bash")
	assert.Equal(t, 10500, suggestions[0].SavingsTokens)
}

func TestContextSuggestions_LargeReadResults(t *testing.T) {
	data := agentic.ContextAnalysisData{
		Percentage:           40,
		RawMaxTokens:         100000,
		IsAutoCompactEnabled: true,
		MessageBreakdown: &agentic.MessageBreakdown{
			ToolCallsByType: []agentic.ToolUsageStats{
				{Name: "Read", CallTokens: 1000, ResultTokens: 20000},
			},
		},
	}
	suggestions := agentic.GenerateContextSuggestions(data)
	require.Len(t, suggestions, 1)
	assert.Equal(t, agentic.CtxSeverityInfo, suggestions[0].Severity)
	assert.Contains(t, suggestions[0].Title, "Read")
}

func TestContextSuggestions_LargeGrepResults(t *testing.T) {
	data := agentic.ContextAnalysisData{
		Percentage:           40,
		RawMaxTokens:         100000,
		IsAutoCompactEnabled: true,
		MessageBreakdown: &agentic.MessageBreakdown{
			ToolCallsByType: []agentic.ToolUsageStats{
				{Name: "Grep", CallTokens: 500, ResultTokens: 18000},
			},
		},
	}
	suggestions := agentic.GenerateContextSuggestions(data)
	require.Len(t, suggestions, 1)
	assert.Contains(t, suggestions[0].Title, "Grep")
}

func TestContextSuggestions_LargeWebFetchResults(t *testing.T) {
	data := agentic.ContextAnalysisData{
		Percentage:           40,
		RawMaxTokens:         100000,
		IsAutoCompactEnabled: true,
		MessageBreakdown: &agentic.MessageBreakdown{
			ToolCallsByType: []agentic.ToolUsageStats{
				{Name: "WebFetch", CallTokens: 500, ResultTokens: 25000},
			},
		},
	}
	suggestions := agentic.GenerateContextSuggestions(data)
	require.Len(t, suggestions, 1)
	assert.Contains(t, suggestions[0].Title, "WebFetch")
}

func TestContextSuggestions_LargeUnknownTool(t *testing.T) {
	data := agentic.ContextAnalysisData{
		Percentage:           40,
		RawMaxTokens:         100000,
		IsAutoCompactEnabled: true,
		MessageBreakdown: &agentic.MessageBreakdown{
			ToolCallsByType: []agentic.ToolUsageStats{
				{Name: "CustomTool", CallTokens: 2000, ResultTokens: 22000},
			},
		},
	}
	suggestions := agentic.GenerateContextSuggestions(data)
	require.Len(t, suggestions, 1)
	assert.Contains(t, suggestions[0].Title, "CustomTool")
}

func TestContextSuggestions_SmallToolResults_NoSuggestion(t *testing.T) {
	data := agentic.ContextAnalysisData{
		Percentage:           40,
		RawMaxTokens:         100000,
		IsAutoCompactEnabled: true,
		MessageBreakdown: &agentic.MessageBreakdown{
			ToolCallsByType: []agentic.ToolUsageStats{
				{Name: "Bash", CallTokens: 100, ResultTokens: 500},
			},
		},
	}
	suggestions := agentic.GenerateContextSuggestions(data)
	assert.Empty(t, suggestions)
}

func TestContextSuggestions_UnknownToolBelow20Percent(t *testing.T) {
	data := agentic.ContextAnalysisData{
		Percentage:           40,
		RawMaxTokens:         100000,
		IsAutoCompactEnabled: true,
		MessageBreakdown: &agentic.MessageBreakdown{
			ToolCallsByType: []agentic.ToolUsageStats{
				{Name: "CustomTool", CallTokens: 1000, ResultTokens: 14000},
			},
		},
	}
	suggestions := agentic.GenerateContextSuggestions(data)
	assert.Empty(t, suggestions)
}

// --- Read result bloat ---

func TestContextSuggestions_ReadBloat(t *testing.T) {
	// Read at 8% (above 5% threshold) but below 15% so not caught by large tool check
	data := agentic.ContextAnalysisData{
		Percentage:           40,
		RawMaxTokens:         200000,
		IsAutoCompactEnabled: true,
		MessageBreakdown: &agentic.MessageBreakdown{
			ToolCallsByType: []agentic.ToolUsageStats{
				{Name: "Read", CallTokens: 500, ResultTokens: 16000},
			},
		},
	}
	suggestions := agentic.GenerateContextSuggestions(data)
	require.Len(t, suggestions, 1)
	assert.Contains(t, suggestions[0].Title, "File reads")
}

func TestContextSuggestions_ReadBloatSkippedWhenLargeToolCovers(t *testing.T) {
	// Read at 20% — already caught by checkLargeToolResults
	data := agentic.ContextAnalysisData{
		Percentage:           40,
		RawMaxTokens:         100000,
		IsAutoCompactEnabled: true,
		MessageBreakdown: &agentic.MessageBreakdown{
			ToolCallsByType: []agentic.ToolUsageStats{
				{Name: "Read", CallTokens: 2000, ResultTokens: 20000},
			},
		},
	}
	suggestions := agentic.GenerateContextSuggestions(data)
	// Should only have the large tool suggestion, not the read bloat one
	require.Len(t, suggestions, 1)
	assert.Contains(t, suggestions[0].Title, "Read results")
}

// --- Memory bloat ---

func TestContextSuggestions_MemoryBloat(t *testing.T) {
	data := agentic.ContextAnalysisData{
		Percentage:           40,
		RawMaxTokens:         100000,
		IsAutoCompactEnabled: true,
		MemoryFiles: []agentic.MemoryFileStats{
			{Path: "context.md", Tokens: 3000},
			{Path: "rules.md", Tokens: 2000},
			{Path: "history.md", Tokens: 1500},
			{Path: "tiny.md", Tokens: 100},
		},
	}
	suggestions := agentic.GenerateContextSuggestions(data)
	require.Len(t, suggestions, 1)
	assert.Contains(t, suggestions[0].Title, "Memory files")
	// Top 3 should appear in detail
	assert.Contains(t, suggestions[0].Detail, "context.md")
	assert.Contains(t, suggestions[0].Detail, "rules.md")
	assert.Contains(t, suggestions[0].Detail, "history.md")
}

func TestContextSuggestions_MemoryBelowThreshold(t *testing.T) {
	data := agentic.ContextAnalysisData{
		Percentage:           40,
		RawMaxTokens:         100000,
		IsAutoCompactEnabled: true,
		MemoryFiles: []agentic.MemoryFileStats{
			{Path: "small.md", Tokens: 500},
		},
	}
	suggestions := agentic.GenerateContextSuggestions(data)
	assert.Empty(t, suggestions)
}

// --- Autocompact disabled ---

func TestContextSuggestions_AutocompactDisabledMidRange(t *testing.T) {
	data := agentic.ContextAnalysisData{
		Percentage:           60,
		RawMaxTokens:         100000,
		IsAutoCompactEnabled: false,
	}
	suggestions := agentic.GenerateContextSuggestions(data)
	require.Len(t, suggestions, 1)
	assert.Equal(t, "Autocompact is disabled", suggestions[0].Title)
}

func TestContextSuggestions_AutocompactDisabledLowUsage(t *testing.T) {
	data := agentic.ContextAnalysisData{
		Percentage:           30,
		RawMaxTokens:         100000,
		IsAutoCompactEnabled: false,
	}
	suggestions := agentic.GenerateContextSuggestions(data)
	assert.Empty(t, suggestions)
}

func TestContextSuggestions_AutocompactDisabledHighUsage(t *testing.T) {
	// At 80%+ the near-capacity warning fires, not the autocompact-disabled info
	data := agentic.ContextAnalysisData{
		Percentage:           85,
		RawMaxTokens:         100000,
		IsAutoCompactEnabled: false,
	}
	suggestions := agentic.GenerateContextSuggestions(data)
	require.Len(t, suggestions, 1)
	assert.Equal(t, agentic.CtxSeverityWarning, suggestions[0].Severity)
}

// --- Sorting ---

func TestContextSuggestions_WarningsFirst(t *testing.T) {
	data := agentic.ContextAnalysisData{
		Percentage:           85,
		RawMaxTokens:         100000,
		IsAutoCompactEnabled: true,
		MessageBreakdown: &agentic.MessageBreakdown{
			ToolCallsByType: []agentic.ToolUsageStats{
				{Name: "Read", CallTokens: 1000, ResultTokens: 20000},
			},
		},
		MemoryFiles: []agentic.MemoryFileStats{
			{Path: "big.md", Tokens: 6000},
		},
	}
	suggestions := agentic.GenerateContextSuggestions(data)
	require.GreaterOrEqual(t, len(suggestions), 2)
	// First suggestion should be a warning
	assert.Equal(t, agentic.CtxSeverityWarning, suggestions[0].Severity)
}

// --- Edge cases ---

func TestContextSuggestions_NilBreakdown(t *testing.T) {
	data := agentic.ContextAnalysisData{
		Percentage:           40,
		RawMaxTokens:         100000,
		IsAutoCompactEnabled: true,
	}
	suggestions := agentic.GenerateContextSuggestions(data)
	assert.Empty(t, suggestions)
}

func TestContextSuggestions_ZeroMaxTokens(t *testing.T) {
	data := agentic.ContextAnalysisData{
		Percentage:           0,
		RawMaxTokens:         0,
		IsAutoCompactEnabled: true,
		MessageBreakdown: &agentic.MessageBreakdown{
			ToolCallsByType: []agentic.ToolUsageStats{
				{Name: "Bash", CallTokens: 100, ResultTokens: 500},
			},
		},
	}
	// Should not panic on division by zero
	suggestions := agentic.GenerateContextSuggestions(data)
	assert.Empty(t, suggestions)
}

func TestContextSuggestions_EmptyMemoryFiles(t *testing.T) {
	data := agentic.ContextAnalysisData{
		Percentage:           40,
		RawMaxTokens:         100000,
		IsAutoCompactEnabled: true,
		MemoryFiles:          []agentic.MemoryFileStats{},
	}
	suggestions := agentic.GenerateContextSuggestions(data)
	assert.Empty(t, suggestions)
}
