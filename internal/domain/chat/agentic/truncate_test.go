package agentic

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Tests for truncateToolResult (ACT-F2-27: structured truncation marker).

func TestTruncate_BelowMax_NoMarker(t *testing.T) {
	result := ToolExecResult{Output: json.RawMessage(`{"data":"small"}`)}
	out := truncateToolResult(result, 1000)
	assert.Equal(t, `{"data":"small"}`, string(out.Output))
	assert.NotContains(t, string(out.Output), "TRUNCATED")
}

func TestTruncate_AtMaxChars_NoMarker(t *testing.T) {
	data := strings.Repeat("x", 100)
	result := ToolExecResult{Output: json.RawMessage(data)}
	out := truncateToolResult(result, 100)
	assert.Equal(t, data, string(out.Output))
	assert.NotContains(t, string(out.Output), "TRUNCATED")
}

func TestTruncate_AddsMarker(t *testing.T) {
	data := strings.Repeat("x", 500)
	result := ToolExecResult{Output: json.RawMessage(data)}
	out := truncateToolResult(result, 200)

	s := string(out.Output)
	assert.Contains(t, s, "[TRUNCATED:")
	assert.Contains(t, s, "200")  // shown size
	assert.Contains(t, s, "500")  // original size
	assert.Contains(t, s, "full result available on request")
}

func TestTruncate_ZeroMax_NoTruncation(t *testing.T) {
	data := strings.Repeat("x", 10000)
	result := ToolExecResult{Output: json.RawMessage(data)}
	out := truncateToolResult(result, 0)
	assert.Equal(t, data, string(out.Output))
}

func TestTruncate_OutputLengthWithinBounds(t *testing.T) {
	data := strings.Repeat("y", 1000)
	result := ToolExecResult{Output: json.RawMessage(data)}
	out := truncateToolResult(result, 100)

	// Output length should not exceed maxChars + marker length
	marker := "\n[TRUNCATED: showing first 100 of 1000 chars — full result available on request]"
	assert.LessOrEqual(t, len(out.Output), 100+len(marker))
}

func TestTruncate_VerySmallMax_NoNegativeSlice(t *testing.T) {
	// maxChars smaller than marker length — should not panic
	data := strings.Repeat("z", 200)
	result := ToolExecResult{Output: json.RawMessage(data)}
	assert.NotPanics(t, func() {
		out := truncateToolResult(result, 5)
		assert.Contains(t, string(out.Output), "TRUNCATED")
	})
}
