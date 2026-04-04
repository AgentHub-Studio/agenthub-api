package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- TruncatePathMiddle ---

func TestTruncatePathMiddle_NoTruncation(t *testing.T) {
	assert.Equal(t, "src/main.go", agentic.TruncatePathMiddle("src/main.go", 20))
}

func TestTruncatePathMiddle_MiddleTruncation(t *testing.T) {
	path := "src/components/deeply/nested/folder/MyComponent.tsx"
	result := agentic.TruncatePathMiddle(path, 30)
	assert.Contains(t, result, "…")
	assert.Contains(t, result, "/MyComponent.tsx") // filename preserved
	assert.LessOrEqual(t, len([]rune(result)), 30)
}

func TestTruncatePathMiddle_ShortMaxLen(t *testing.T) {
	result := agentic.TruncatePathMiddle("very/long/path.txt", 3)
	runes := []rune(result)
	assert.LessOrEqual(t, len(runes), 3)
	assert.Contains(t, result, "…")
}

func TestTruncatePathMiddle_ZeroMaxLen(t *testing.T) {
	assert.Equal(t, "…", agentic.TruncatePathMiddle("anything", 0))
}

func TestTruncatePathMiddle_NoSlash(t *testing.T) {
	result := agentic.TruncatePathMiddle("verylongfilename.tsx", 10)
	assert.LessOrEqual(t, len([]rune(result)), 10)
}

func TestTruncatePathMiddle_ExactFit(t *testing.T) {
	path := "src/main.go"
	assert.Equal(t, path, agentic.TruncatePathMiddle(path, len(path)))
}

func TestTruncatePathMiddle_LongFilename(t *testing.T) {
	path := "dir/ThisIsAVeryLongComponentFileName.tsx"
	result := agentic.TruncatePathMiddle(path, 15)
	assert.LessOrEqual(t, len([]rune(result)), 15)
	assert.Contains(t, result, "…")
}

// --- TruncateEnd ---

func TestTruncateEnd_NoTruncation(t *testing.T) {
	assert.Equal(t, "hello", agentic.TruncateEnd("hello", 10))
}

func TestTruncateEnd_Truncated(t *testing.T) {
	result := agentic.TruncateEnd("hello world", 8)
	assert.Equal(t, "hello w…", result)
}

func TestTruncateEnd_ExactFit(t *testing.T) {
	assert.Equal(t, "hello", agentic.TruncateEnd("hello", 5))
}

func TestTruncateEnd_MinLen(t *testing.T) {
	assert.Equal(t, "…", agentic.TruncateEnd("hello", 1))
}

func TestTruncateEnd_Unicode(t *testing.T) {
	result := agentic.TruncateEnd("日本語テスト", 4)
	assert.Equal(t, "日本語…", result)
}

// --- TruncateStart ---

func TestTruncateStart_NoTruncation(t *testing.T) {
	assert.Equal(t, "hello", agentic.TruncateStart("hello", 10))
}

func TestTruncateStart_Truncated(t *testing.T) {
	result := agentic.TruncateStart("hello world", 8)
	assert.Equal(t, "…o world", result) // keeps 7 runes + ellipsis
}

func TestTruncateStart_ExactFit(t *testing.T) {
	assert.Equal(t, "hello", agentic.TruncateStart("hello", 5))
}

func TestTruncateStart_MinLen(t *testing.T) {
	assert.Equal(t, "…", agentic.TruncateStart("hello", 1))
}

func TestTruncateStart_Unicode(t *testing.T) {
	result := agentic.TruncateStart("日本語テスト", 4)
	assert.Equal(t, "…テスト", result)
}
