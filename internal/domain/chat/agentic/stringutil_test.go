package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- EscapeRegExp ---

func TestEscapeRegExp_NoSpecial(t *testing.T) {
	assert.Equal(t, "hello", agentic.EscapeRegExp("hello"))
}

func TestEscapeRegExp_AllSpecial(t *testing.T) {
	result := agentic.EscapeRegExp(`.*+?^${}()|[]\`)
	assert.Contains(t, result, `\.`)
	assert.Contains(t, result, `\*`)
	assert.Contains(t, result, `\+`)
	assert.Contains(t, result, `\?`)
	assert.Contains(t, result, `\^`)
	assert.Contains(t, result, `\$`)
}

func TestEscapeRegExp_Empty(t *testing.T) {
	assert.Equal(t, "", agentic.EscapeRegExp(""))
}

func TestEscapeRegExp_Mixed(t *testing.T) {
	assert.Equal(t, `file\.txt`, agentic.EscapeRegExp("file.txt"))
}

// --- Capitalize ---

func TestCapitalize_Lowercase(t *testing.T) {
	assert.Equal(t, "Hello", agentic.Capitalize("hello"))
}

func TestCapitalize_AlreadyUpper(t *testing.T) {
	assert.Equal(t, "Hello", agentic.Capitalize("Hello"))
}

func TestCapitalize_PreservesRest(t *testing.T) {
	assert.Equal(t, "FooBar", agentic.Capitalize("fooBar"))
}

func TestCapitalize_Empty(t *testing.T) {
	assert.Equal(t, "", agentic.Capitalize(""))
}

func TestCapitalize_SingleChar(t *testing.T) {
	assert.Equal(t, "A", agentic.Capitalize("a"))
}

func TestCapitalize_Unicode(t *testing.T) {
	assert.Equal(t, "Über", agentic.Capitalize("über"))
}

// --- Plural ---

func TestPlural_Singular(t *testing.T) {
	assert.Equal(t, "file", agentic.Plural(1, "file", ""))
}

func TestPlural_DefaultPlural(t *testing.T) {
	assert.Equal(t, "files", agentic.Plural(3, "file", ""))
}

func TestPlural_CustomPlural(t *testing.T) {
	assert.Equal(t, "entries", agentic.Plural(2, "entry", "entries"))
}

func TestPlural_Zero(t *testing.T) {
	assert.Equal(t, "files", agentic.Plural(0, "file", ""))
}

// --- FirstLineOf ---

func TestFirstLineOf_SingleLine(t *testing.T) {
	assert.Equal(t, "hello", agentic.FirstLineOf("hello"))
}

func TestFirstLineOf_MultiLine(t *testing.T) {
	assert.Equal(t, "first", agentic.FirstLineOf("first\nsecond\nthird"))
}

func TestFirstLineOf_Empty(t *testing.T) {
	assert.Equal(t, "", agentic.FirstLineOf(""))
}

func TestFirstLineOf_StartsWithNewline(t *testing.T) {
	assert.Equal(t, "", agentic.FirstLineOf("\nsecond"))
}

// --- CountCharInString ---

func TestCountCharInString_Multiple(t *testing.T) {
	assert.Equal(t, 3, agentic.CountCharInString("a.b.c.d", '.'))
}

func TestCountCharInString_None(t *testing.T) {
	assert.Equal(t, 0, agentic.CountCharInString("hello", '.'))
}

func TestCountCharInString_Empty(t *testing.T) {
	assert.Equal(t, 0, agentic.CountCharInString("", 'x'))
}

func TestCountCharInString_AllSame(t *testing.T) {
	assert.Equal(t, 5, agentic.CountCharInString("aaaaa", 'a'))
}

func TestCountCharInString_Newlines(t *testing.T) {
	assert.Equal(t, 2, agentic.CountCharInString("a\nb\nc", '\n'))
}

// --- NormalizeFullWidthDigits ---

func TestNormalizeFullWidthDigits_FullWidth(t *testing.T) {
	assert.Equal(t, "123", agentic.NormalizeFullWidthDigits("１２３"))
}

func TestNormalizeFullWidthDigits_Mixed(t *testing.T) {
	assert.Equal(t, "page 42", agentic.NormalizeFullWidthDigits("page ４２"))
}

func TestNormalizeFullWidthDigits_NoChange(t *testing.T) {
	assert.Equal(t, "hello", agentic.NormalizeFullWidthDigits("hello"))
}

func TestNormalizeFullWidthDigits_Empty(t *testing.T) {
	assert.Equal(t, "", agentic.NormalizeFullWidthDigits(""))
}

// --- NormalizeFullWidthSpace ---

func TestNormalizeFullWidthSpace_Replace(t *testing.T) {
	assert.Equal(t, "hello world", agentic.NormalizeFullWidthSpace("hello\u3000world"))
}

func TestNormalizeFullWidthSpace_NoChange(t *testing.T) {
	assert.Equal(t, "hello world", agentic.NormalizeFullWidthSpace("hello world"))
}

func TestNormalizeFullWidthSpace_Multiple(t *testing.T) {
	assert.Equal(t, "a b c", agentic.NormalizeFullWidthSpace("a\u3000b\u3000c"))
}
