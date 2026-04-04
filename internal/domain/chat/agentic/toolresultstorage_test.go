package agentic_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- ContentReplacementState ---

func TestContentReplacementState_NewIsEmpty(t *testing.T) {
	state := agentic.NewContentReplacementState()
	assert.Equal(t, 0, state.SeenCount())
	assert.Equal(t, 0, state.ReplacementCount())
}

func TestContentReplacementState_MarkSeen(t *testing.T) {
	state := agentic.NewContentReplacementState()
	state.MarkSeen("tc_1")

	assert.True(t, state.IsSeen("tc_1"))
	assert.False(t, state.IsSeen("tc_2"))
	assert.Equal(t, 1, state.SeenCount())
	assert.Equal(t, 0, state.ReplacementCount())
}

func TestContentReplacementState_MarkReplaced(t *testing.T) {
	state := agentic.NewContentReplacementState()
	state.MarkReplaced("tc_1", "preview text")

	assert.True(t, state.IsSeen("tc_1"))
	replacement, ok := state.GetReplacement("tc_1")
	assert.True(t, ok)
	assert.Equal(t, "preview text", replacement)
}

func TestContentReplacementState_GetReplacement_NotFound(t *testing.T) {
	state := agentic.NewContentReplacementState()
	_, ok := state.GetReplacement("nonexistent")
	assert.False(t, ok)
}

func TestContentReplacementState_SeenButNotReplaced(t *testing.T) {
	state := agentic.NewContentReplacementState()
	state.MarkSeen("tc_1")

	_, ok := state.GetReplacement("tc_1")
	assert.False(t, ok, "seen-only IDs should not have replacements")
}

func TestContentReplacementState_Records(t *testing.T) {
	state := agentic.NewContentReplacementState()
	state.MarkReplaced("tc_1", "replacement_1")
	state.MarkReplaced("tc_2", "replacement_2")
	state.MarkSeen("tc_3") // seen-only, no record.

	records := state.Records()
	assert.Len(t, records, 2)
	ids := map[string]string{}
	for _, r := range records {
		assert.Equal(t, "tool-result", r.Kind)
		ids[r.ToolUseID] = r.Replacement
	}
	assert.Equal(t, "replacement_1", ids["tc_1"])
	assert.Equal(t, "replacement_2", ids["tc_2"])
}

func TestContentReplacementState_Clone(t *testing.T) {
	state := agentic.NewContentReplacementState()
	state.MarkSeen("tc_1")
	state.MarkReplaced("tc_2", "preview")

	clone := state.Clone()

	// Clone should have same state.
	assert.True(t, clone.IsSeen("tc_1"))
	r, ok := clone.GetReplacement("tc_2")
	assert.True(t, ok)
	assert.Equal(t, "preview", r)

	// Mutating clone should not affect original.
	clone.MarkReplaced("tc_3", "new")
	assert.False(t, state.IsSeen("tc_3"))
}

// --- ReconstructContentReplacementState ---

func TestReconstructContentReplacementState(t *testing.T) {
	records := []agentic.ContentReplacementRecord{
		{Kind: "tool-result", ToolUseID: "tc_1", Replacement: "preview_1"},
		{Kind: "tool-result", ToolUseID: "tc_2", Replacement: "preview_2"},
	}

	state := agentic.ReconstructContentReplacementState(records)
	assert.Equal(t, 2, state.SeenCount())
	assert.Equal(t, 2, state.ReplacementCount())

	r, ok := state.GetReplacement("tc_1")
	assert.True(t, ok)
	assert.Equal(t, "preview_1", r)
}

func TestReconstructContentReplacementState_EmptyRecords(t *testing.T) {
	state := agentic.ReconstructContentReplacementState(nil)
	assert.Equal(t, 0, state.SeenCount())
}

// --- BuildLargeResultPreview ---

func TestBuildLargeResultPreview(t *testing.T) {
	preview := agentic.BuildLargeResultPreview("tc_1", 100000, "first 2048 chars...")
	assert.Contains(t, preview, "<persisted-output>")
	assert.Contains(t, preview, "100000 chars")
	assert.Contains(t, preview, "tc_1")
	assert.Contains(t, preview, "first 2048 chars...")
	assert.Contains(t, preview, "</persisted-output>")
}

// --- EnforceToolResultBudget ---

func TestEnforceToolResultBudget_NilState(t *testing.T) {
	results := []agentic.ToolExecResult{
		{Output: json.RawMessage(`"hello"`)},
	}
	out := agentic.EnforceToolResultBudget(results, []string{"tc_1"}, nil, 100, 200)
	assert.Equal(t, results, out)
}

func TestEnforceToolResultBudget_EmptyResults(t *testing.T) {
	state := agentic.NewContentReplacementState()
	out := agentic.EnforceToolResultBudget(nil, nil, state, 100, 200)
	assert.Nil(t, out)
}

func TestEnforceToolResultBudget_UnderPerToolLimit(t *testing.T) {
	state := agentic.NewContentReplacementState()
	output := json.RawMessage(`"small output"`)
	results := []agentic.ToolExecResult{
		{Output: output},
	}
	out := agentic.EnforceToolResultBudget(results, []string{"tc_1"}, state, 1000, 10000)

	// Should keep original output unchanged.
	assert.Equal(t, string(output), string(out[0].Output))
	assert.True(t, state.IsSeen("tc_1"))
}

func TestEnforceToolResultBudget_ExceedsPerToolLimit(t *testing.T) {
	state := agentic.NewContentReplacementState()
	largeOutput := json.RawMessage(`"` + strings.Repeat("x", 200) + `"`)
	results := []agentic.ToolExecResult{
		{Output: largeOutput},
	}
	out := agentic.EnforceToolResultBudget(results, []string{"tc_1"}, state, 50, 10000)

	// Should be replaced with a preview.
	assert.Contains(t, string(out[0].Output), "<persisted-output>")
	assert.Contains(t, string(out[0].Output), "tc_1")
	// Should be recorded as replaced.
	_, ok := state.GetReplacement("tc_1")
	assert.True(t, ok)
}

func TestEnforceToolResultBudget_ReappliesCachedReplacement(t *testing.T) {
	state := agentic.NewContentReplacementState()
	state.MarkReplaced("tc_1", "cached_preview")

	results := []agentic.ToolExecResult{
		{Output: json.RawMessage(`"new output"`)},
	}
	out := agentic.EnforceToolResultBudget(results, []string{"tc_1"}, state, 1000, 10000)

	// Should use the cached replacement, not the new output.
	assert.Equal(t, "cached_preview", string(out[0].Output))
}

func TestEnforceToolResultBudget_FrozenNotReplaced(t *testing.T) {
	state := agentic.NewContentReplacementState()
	state.MarkSeen("tc_1") // Previously seen, not replaced → frozen.

	results := []agentic.ToolExecResult{
		{Output: json.RawMessage(`"original"`)},
	}
	out := agentic.EnforceToolResultBudget(results, []string{"tc_1"}, state, 5, 10000)

	// Should keep original even though it exceeds per-tool limit — frozen.
	assert.Equal(t, `"original"`, string(out[0].Output))
}

func TestEnforceToolResultBudget_MessageBudgetTriggersLargestFirst(t *testing.T) {
	state := agentic.NewContentReplacementState()
	small := json.RawMessage(`"small"`)
	// Large enough that the replacement (preview ~2KB + headers) is much smaller.
	large := json.RawMessage(`"` + strings.Repeat("L", 10000) + `"`)

	results := []agentic.ToolExecResult{
		{Output: small},
		{Output: large},
	}
	ids := []string{"tc_1", "tc_2"}

	// Per-tool limit high enough for both, but message budget forces the large
	// one to be replaced. The replacement preview (~2KB + headers) + small (7 chars) < 5000.
	out := agentic.EnforceToolResultBudget(results, ids, state, 20000, 5000)

	// Large should be replaced.
	assert.Contains(t, string(out[1].Output), "<persisted-output>")
	// Small should remain untouched.
	assert.Equal(t, string(small), string(out[0].Output))
}

func TestEnforceToolResultBudget_ErrorResultsSkipped(t *testing.T) {
	state := agentic.NewContentReplacementState()
	errMsg := "something failed"
	results := []agentic.ToolExecResult{
		{Error: &errMsg},
	}
	out := agentic.EnforceToolResultBudget(results, []string{"tc_1"}, state, 10, 10)

	// Error results should pass through unchanged.
	require.NotNil(t, out[0].Error)
	assert.Equal(t, "something failed", *out[0].Error)
}

func TestEnforceToolResultBudget_NoIDSkipped(t *testing.T) {
	state := agentic.NewContentReplacementState()
	results := []agentic.ToolExecResult{
		{Output: json.RawMessage(`"data"`)},
	}
	// Empty ID list — should skip enforcement.
	out := agentic.EnforceToolResultBudget(results, nil, state, 1, 1)
	assert.Equal(t, `"data"`, string(out[0].Output))
}
