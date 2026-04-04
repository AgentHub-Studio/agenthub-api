package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- ExtractFrontmatter ---

func TestExtractFrontmatter_WithFrontmatter(t *testing.T) {
	md := "---\ntitle: Hello\ndescription: World\n---\n# Content"
	result := agentic.ExtractFrontmatter(md)
	assert.Equal(t, "Hello", result.Fields["title"])
	assert.Equal(t, "World", result.Fields["description"])
	assert.Equal(t, "# Content", result.Content)
}

func TestExtractFrontmatter_NoFrontmatter(t *testing.T) {
	md := "# Just content"
	result := agentic.ExtractFrontmatter(md)
	assert.Empty(t, result.RawYAML)
	assert.Equal(t, "# Just content", result.Content)
}

func TestExtractFrontmatter_EmptyFrontmatter(t *testing.T) {
	md := "---\n---\n# Content"
	result := agentic.ExtractFrontmatter(md)
	assert.Empty(t, result.Fields)
	assert.Equal(t, "# Content", result.Content)
}

func TestExtractFrontmatter_QuotedValues(t *testing.T) {
	md := "---\ntitle: \"Hello World\"\n---\nBody"
	result := agentic.ExtractFrontmatter(md)
	assert.Equal(t, "Hello World", result.Fields["title"])
}

func TestExtractFrontmatter_SingleQuotedValues(t *testing.T) {
	md := "---\ntitle: 'Hello World'\n---\nBody"
	result := agentic.ExtractFrontmatter(md)
	assert.Equal(t, "Hello World", result.Fields["title"])
}

func TestExtractFrontmatter_Comments(t *testing.T) {
	md := "---\n# comment\ntitle: Hello\n---\nBody"
	result := agentic.ExtractFrontmatter(md)
	assert.Equal(t, "Hello", result.Fields["title"])
	_, hasComment := result.Fields["# comment"]
	assert.False(t, hasComment)
}

func TestExtractFrontmatter_Empty(t *testing.T) {
	result := agentic.ExtractFrontmatter("")
	assert.Empty(t, result.Fields)
	assert.Equal(t, "", result.Content)
}

// --- QuoteYAMLSpecialValues ---

func TestQuoteYAMLSpecialValues_NoSpecial(t *testing.T) {
	input := "title: hello"
	assert.Equal(t, "title: hello", agentic.QuoteYAMLSpecialValues(input))
}

func TestQuoteYAMLSpecialValues_BraceGlob(t *testing.T) {
	input := "paths: src/*.{ts,tsx}"
	result := agentic.QuoteYAMLSpecialValues(input)
	assert.Contains(t, result, `"src/*.{ts,tsx}"`)
}

func TestQuoteYAMLSpecialValues_AlreadyQuoted(t *testing.T) {
	input := `paths: "already quoted"`
	assert.Equal(t, `paths: "already quoted"`, agentic.QuoteYAMLSpecialValues(input))
}

func TestQuoteYAMLSpecialValues_AnchorChar(t *testing.T) {
	input := "value: test & more"
	result := agentic.QuoteYAMLSpecialValues(input)
	assert.Contains(t, result, `"test & more"`)
}

// --- SplitPathInFrontmatter ---

func TestSplitPathInFrontmatter_Simple(t *testing.T) {
	result := agentic.SplitPathInFrontmatter("a, b, c")
	assert.Equal(t, []string{"a", "b", "c"}, result)
}

func TestSplitPathInFrontmatter_BraceExpansion(t *testing.T) {
	result := agentic.SplitPathInFrontmatter("src/*.{ts,tsx}")
	assert.Equal(t, []string{"src/*.ts", "src/*.tsx"}, result)
}

func TestSplitPathInFrontmatter_CommaInsideBraces(t *testing.T) {
	result := agentic.SplitPathInFrontmatter("a, src/*.{ts,tsx}, b")
	assert.Equal(t, []string{"a", "src/*.ts", "src/*.tsx", "b"}, result)
}

func TestSplitPathInFrontmatter_Empty(t *testing.T) {
	result := agentic.SplitPathInFrontmatter("")
	assert.Nil(t, result)
}

func TestSplitPathInFrontmatter_NoComma(t *testing.T) {
	result := agentic.SplitPathInFrontmatter("src/**/*.go")
	assert.Equal(t, []string{"src/**/*.go"}, result)
}

// --- ExpandBraces ---

func TestExpandBraces_Simple(t *testing.T) {
	result := agentic.ExpandBraces("*.{ts,tsx}")
	assert.Equal(t, []string{"*.ts", "*.tsx"}, result)
}

func TestExpandBraces_CartesianProduct(t *testing.T) {
	result := agentic.ExpandBraces("{a,b}/{c,d}")
	assert.Equal(t, []string{"a/c", "a/d", "b/c", "b/d"}, result)
}

func TestExpandBraces_NoBraces(t *testing.T) {
	result := agentic.ExpandBraces("plain.txt")
	assert.Equal(t, []string{"plain.txt"}, result)
}

func TestExpandBraces_ThreeAlternatives(t *testing.T) {
	result := agentic.ExpandBraces("file.{js,ts,go}")
	assert.Equal(t, []string{"file.js", "file.ts", "file.go"}, result)
}

func TestExpandBraces_PrefixAndSuffix(t *testing.T) {
	result := agentic.ExpandBraces("src/{a,b}/index.ts")
	assert.Equal(t, []string{"src/a/index.ts", "src/b/index.ts"}, result)
}

// --- ParseBooleanFrontmatter ---

func TestParseBooleanFrontmatter_True(t *testing.T) {
	assert.True(t, agentic.ParseBooleanFrontmatter("true"))
}

func TestParseBooleanFrontmatter_False(t *testing.T) {
	assert.False(t, agentic.ParseBooleanFrontmatter("false"))
}

func TestParseBooleanFrontmatter_Empty(t *testing.T) {
	assert.False(t, agentic.ParseBooleanFrontmatter(""))
}

func TestParseBooleanFrontmatter_Other(t *testing.T) {
	assert.False(t, agentic.ParseBooleanFrontmatter("yes"))
}

// --- ParsePositiveIntFrontmatter ---

func TestParsePositiveIntFrontmatter_Valid(t *testing.T) {
	assert.Equal(t, 42, agentic.ParsePositiveIntFrontmatter("42"))
}

func TestParsePositiveIntFrontmatter_Zero(t *testing.T) {
	assert.Equal(t, 0, agentic.ParsePositiveIntFrontmatter("0"))
}

func TestParsePositiveIntFrontmatter_Negative(t *testing.T) {
	assert.Equal(t, 0, agentic.ParsePositiveIntFrontmatter("-5"))
}

func TestParsePositiveIntFrontmatter_NonNumeric(t *testing.T) {
	assert.Equal(t, 0, agentic.ParsePositiveIntFrontmatter("abc"))
}

func TestParsePositiveIntFrontmatter_Empty(t *testing.T) {
	assert.Equal(t, 0, agentic.ParsePositiveIntFrontmatter(""))
}

func TestParsePositiveIntFrontmatter_Whitespace(t *testing.T) {
	assert.Equal(t, 10, agentic.ParsePositiveIntFrontmatter("  10  "))
}
