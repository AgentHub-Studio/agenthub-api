package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

func TestSanitizeUnicode_PlainASCII(t *testing.T) {
	result, err := agentic.SanitizeUnicode("hello world")
	require.NoError(t, err)
	assert.Equal(t, "hello world", result)
}

func TestSanitizeUnicode_Empty(t *testing.T) {
	result, err := agentic.SanitizeUnicode("")
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestSanitizeUnicode_ZeroWidthSpaces(t *testing.T) {
	// U+200B zero-width space
	input := "hello\u200Bworld"
	result, err := agentic.SanitizeUnicode(input)
	require.NoError(t, err)
	assert.Equal(t, "helloworld", result)
}

func TestSanitizeUnicode_DirectionalMarks(t *testing.T) {
	// U+200E LTR mark, U+200F RTL mark
	input := "hello\u200E\u200Fworld"
	result, err := agentic.SanitizeUnicode(input)
	require.NoError(t, err)
	assert.Equal(t, "helloworld", result)
}

func TestSanitizeUnicode_DirectionalFormatting(t *testing.T) {
	// U+202A LRE, U+202B RLE, U+202C PDF, U+202D LRO, U+202E RLO
	input := "a\u202Ab\u202Bc\u202Cd\u202De\u202Ef"
	result, err := agentic.SanitizeUnicode(input)
	require.NoError(t, err)
	assert.Equal(t, "abcdef", result)
}

func TestSanitizeUnicode_DirectionalIsolates(t *testing.T) {
	// U+2066 LRI, U+2067 RLI, U+2068 FSI, U+2069 PDI
	input := "a\u2066b\u2067c\u2068d\u2069e"
	result, err := agentic.SanitizeUnicode(input)
	require.NoError(t, err)
	assert.Equal(t, "abcde", result)
}

func TestSanitizeUnicode_BOM(t *testing.T) {
	// U+FEFF byte order mark
	input := "\uFEFFhello"
	result, err := agentic.SanitizeUnicode(input)
	require.NoError(t, err)
	assert.Equal(t, "hello", result)
}

func TestSanitizeUnicode_PrivateUseArea(t *testing.T) {
	// U+E000 is in BMP private use area
	input := "hello\uE000world"
	result, err := agentic.SanitizeUnicode(input)
	require.NoError(t, err)
	assert.Equal(t, "helloworld", result)
}

func TestSanitizeUnicode_PreservesLegitimateUnicode(t *testing.T) {
	// Chinese, Japanese, Korean, emoji, accented Latin
	input := "日本語 한국어 café résumé"
	result, err := agentic.SanitizeUnicode(input)
	require.NoError(t, err)
	assert.Equal(t, input, result)
}

func TestSanitizeUnicode_PreservesEmoji(t *testing.T) {
	input := "hello 👋 world 🌍"
	result, err := agentic.SanitizeUnicode(input)
	require.NoError(t, err)
	assert.Equal(t, input, result)
}

func TestSanitizeUnicode_PreservesNewlines(t *testing.T) {
	input := "line1\nline2\ttab"
	result, err := agentic.SanitizeUnicode(input)
	require.NoError(t, err)
	assert.Equal(t, input, result)
}

func TestSanitizeUnicode_NormalizesNFKC(t *testing.T) {
	// The ligature ﬁ (U+FB01) should be normalized to "fi" by NFKC
	input := "ﬁnd"
	result, err := agentic.SanitizeUnicode(input)
	require.NoError(t, err)
	assert.Equal(t, "find", result)
}

func TestSanitizeUnicode_MixedDangerous(t *testing.T) {
	// Mix zero-width, directional, and private use
	input := "safe\u200B\u202A\uE001text"
	result, err := agentic.SanitizeUnicode(input)
	require.NoError(t, err)
	assert.Equal(t, "safetext", result)
}

func TestMustSanitizeUnicode_Normal(t *testing.T) {
	result := agentic.MustSanitizeUnicode("hello\u200Bworld")
	assert.Equal(t, "helloworld", result)
}

func TestMustSanitizeUnicode_Clean(t *testing.T) {
	result := agentic.MustSanitizeUnicode("clean text")
	assert.Equal(t, "clean text", result)
}

func TestIsDangerousUnicode_Clean(t *testing.T) {
	assert.False(t, agentic.IsDangerousUnicode("hello world"))
}

func TestIsDangerousUnicode_WithZeroWidth(t *testing.T) {
	assert.True(t, agentic.IsDangerousUnicode("hello\u200Bworld"))
}

func TestIsDangerousUnicode_WithBOM(t *testing.T) {
	assert.True(t, agentic.IsDangerousUnicode("\uFEFFhello"))
}

func TestIsDangerousUnicode_Empty(t *testing.T) {
	assert.False(t, agentic.IsDangerousUnicode(""))
}

func TestSanitizeUnicode_SoftHyphen(t *testing.T) {
	// U+00AD soft hyphen — Cf category
	input := "auto\u00ADmatic"
	result, err := agentic.SanitizeUnicode(input)
	require.NoError(t, err)
	assert.Equal(t, "automatic", result)
}

func TestSanitizeUnicode_TagCharacters(t *testing.T) {
	// U+E0001 language tag — in supplementary private use or tag range
	// Tags range U+E0000-U+E007F are Cf in Unicode
	input := "text" + string(rune(0xE0001)) + "more"
	result, err := agentic.SanitizeUnicode(input)
	require.NoError(t, err)
	assert.Equal(t, "textmore", result)
}
