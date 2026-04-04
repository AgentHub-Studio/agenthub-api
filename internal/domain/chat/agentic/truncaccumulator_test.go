package agentic_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- TruncAccumulator ---

func TestTruncAccumulator_New(t *testing.T) {
	a := agentic.NewTruncAccumulator(0)
	assert.Equal(t, 0, a.Len())
	assert.False(t, a.IsTruncated())
	assert.Equal(t, 0, a.TotalBytes())
}

func TestTruncAccumulator_Append(t *testing.T) {
	a := agentic.NewTruncAccumulator(100)
	a.Append("hello")
	assert.Equal(t, 5, a.Len())
	assert.Equal(t, "hello", a.String())
}

func TestTruncAccumulator_AppendMultiple(t *testing.T) {
	a := agentic.NewTruncAccumulator(100)
	a.Append("hello ")
	a.Append("world")
	assert.Equal(t, 11, a.Len())
	assert.Equal(t, "hello world", a.String())
}

func TestTruncAccumulator_Truncation(t *testing.T) {
	a := agentic.NewTruncAccumulator(10)
	a.Append("12345")
	a.Append("67890")  // fills to exactly 10
	a.Append("EXCESS") // this should be truncated

	assert.True(t, a.IsTruncated())
	assert.Equal(t, 10, a.Len())
	assert.Equal(t, 16, a.TotalBytes())
	assert.Contains(t, a.String(), "1234567890")
	assert.Contains(t, a.String(), "truncated")
}

func TestTruncAccumulator_PartialTruncation(t *testing.T) {
	a := agentic.NewTruncAccumulator(8)
	a.Append("12345")
	a.Append("67890") // only 3 more chars fit

	assert.True(t, a.IsTruncated())
	assert.Equal(t, 8, a.Len())
	assert.Contains(t, a.String(), "12345678")
}

func TestTruncAccumulator_NoTruncation(t *testing.T) {
	a := agentic.NewTruncAccumulator(100)
	a.Append("short text")
	assert.False(t, a.IsTruncated())
	assert.Equal(t, "short text", a.String())
}

func TestTruncAccumulator_Clear(t *testing.T) {
	a := agentic.NewTruncAccumulator(10)
	a.Append("12345678901234567890")
	assert.True(t, a.IsTruncated())

	a.Clear()
	assert.False(t, a.IsTruncated())
	assert.Equal(t, 0, a.Len())
	assert.Equal(t, 0, a.TotalBytes())
	assert.Equal(t, "", a.String())
}

func TestTruncAccumulator_AfterTruncation_IgnoresMore(t *testing.T) {
	a := agentic.NewTruncAccumulator(5)
	a.Append("12345")
	a.Append("MORE") // capacity full
	a.Append("EVEN MORE")

	assert.True(t, a.IsTruncated())
	assert.Equal(t, 5, a.Len())
	assert.Equal(t, 18, a.TotalBytes())
}

func TestTruncAccumulator_Concurrent(t *testing.T) {
	a := agentic.NewTruncAccumulator(10000)
	done := make(chan struct{})

	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				a.Append("x")
			}
			done <- struct{}{}
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	assert.Equal(t, 1000, a.Len())
}

// --- SafeJoinLines ---

func TestSafeJoinLines_Normal(t *testing.T) {
	result := agentic.SafeJoinLines([]string{"a", "b", "c"}, ",", 100)
	assert.Equal(t, "a,b,c", result)
}

func TestSafeJoinLines_Truncated(t *testing.T) {
	result := agentic.SafeJoinLines([]string{"hello", "world", "foo"}, ",", 12)
	assert.Contains(t, result, "truncated")
	// Result = content up to maxSize + marker. Marker is "...[truncated]" (14 chars)
	assert.LessOrEqual(t, len(result), 12+14+1)
}

func TestSafeJoinLines_Empty(t *testing.T) {
	result := agentic.SafeJoinLines(nil, ",", 100)
	assert.Equal(t, "", result)
}

func TestSafeJoinLines_SingleItem(t *testing.T) {
	result := agentic.SafeJoinLines([]string{"hello"}, ",", 100)
	assert.Equal(t, "hello", result)
}

func TestSafeJoinLines_ExactFit(t *testing.T) {
	result := agentic.SafeJoinLines([]string{"ab", "cd"}, ",", 5)
	assert.Equal(t, "ab,cd", result)
}

// --- TruncateToLines ---

func TestTruncateToLines_NoTruncation(t *testing.T) {
	text := "line1\nline2\nline3"
	result := agentic.TruncateToLines(text, 5)
	assert.Equal(t, text, result)
}

func TestTruncateToLines_Truncated(t *testing.T) {
	text := "line1\nline2\nline3\nline4\nline5"
	result := agentic.TruncateToLines(text, 3)
	assert.Equal(t, "line1\nline2\nline3…", result)
}

func TestTruncateToLines_SingleLine(t *testing.T) {
	result := agentic.TruncateToLines("hello", 1)
	assert.Equal(t, "hello", result)
}

func TestTruncateToLines_ExactLines(t *testing.T) {
	text := "a\nb\nc"
	result := agentic.TruncateToLines(text, 3)
	assert.Equal(t, text, result)
}

func TestTruncateToLines_LargeInput(t *testing.T) {
	lines := make([]string, 1000)
	for i := range lines {
		lines[i] = "line"
	}
	text := strings.Join(lines, "\n")
	result := agentic.TruncateToLines(text, 10)
	require.Contains(t, result, "…")
	assert.Equal(t, 10, strings.Count(result[:len(result)-len("…")], "\n")+1)
}
