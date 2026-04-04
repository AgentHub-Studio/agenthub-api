package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- ParseArguments ---

func TestParseArguments_Simple(t *testing.T) {
	assert.Equal(t, []string{"foo", "bar", "baz"}, agentic.ParseArguments("foo bar baz"))
}

func TestParseArguments_DoubleQuotes(t *testing.T) {
	assert.Equal(t, []string{"foo", "hello world", "baz"}, agentic.ParseArguments(`foo "hello world" baz`))
}

func TestParseArguments_SingleQuotes(t *testing.T) {
	assert.Equal(t, []string{"foo", "hello world", "baz"}, agentic.ParseArguments("foo 'hello world' baz"))
}

func TestParseArguments_Empty(t *testing.T) {
	assert.Nil(t, agentic.ParseArguments(""))
}

func TestParseArguments_Whitespace(t *testing.T) {
	assert.Nil(t, agentic.ParseArguments("   "))
}

func TestParseArguments_SingleArg(t *testing.T) {
	assert.Equal(t, []string{"hello"}, agentic.ParseArguments("hello"))
}

func TestParseArguments_ExtraSpaces(t *testing.T) {
	assert.Equal(t, []string{"a", "b", "c"}, agentic.ParseArguments("  a   b   c  "))
}

// --- ParseArgumentNames ---

func TestParseArgumentNames_Simple(t *testing.T) {
	assert.Equal(t, []string{"foo", "bar", "baz"}, agentic.ParseArgumentNames("foo bar baz"))
}

func TestParseArgumentNames_FilterNumeric(t *testing.T) {
	result := agentic.ParseArgumentNames("foo 123 bar")
	assert.Equal(t, []string{"foo", "bar"}, result)
}

func TestParseArgumentNames_Empty(t *testing.T) {
	assert.Nil(t, agentic.ParseArgumentNames(""))
}

// --- GenerateArgumentHint ---

func TestGenerateArgumentHint_Remaining(t *testing.T) {
	result := agentic.GenerateArgumentHint([]string{"foo", "bar", "baz"}, []string{"val1"})
	assert.Equal(t, "[bar] [baz]", result)
}

func TestGenerateArgumentHint_AllFilled(t *testing.T) {
	result := agentic.GenerateArgumentHint([]string{"foo", "bar"}, []string{"a", "b"})
	assert.Equal(t, "", result)
}

func TestGenerateArgumentHint_NoneTyped(t *testing.T) {
	result := agentic.GenerateArgumentHint([]string{"x", "y"}, nil)
	assert.Equal(t, "[x] [y]", result)
}

// --- SubstituteArguments ---

func TestSubstituteArguments_FullArgs(t *testing.T) {
	result := agentic.SubstituteArguments("Run: $ARGUMENTS", "test all", false, nil)
	assert.Equal(t, "Run: test all", result)
}

func TestSubstituteArguments_IndexedArgs(t *testing.T) {
	result := agentic.SubstituteArguments("First: $ARGUMENTS[0], Second: $ARGUMENTS[1]", "foo bar", false, nil)
	assert.Equal(t, "First: foo, Second: bar", result)
}

func TestSubstituteArguments_ShorthandArgs(t *testing.T) {
	result := agentic.SubstituteArguments("First: $0, Second: $1", "foo bar", false, nil)
	assert.Equal(t, "First: foo, Second: bar", result)
}

func TestSubstituteArguments_AppendWhenNoPlaceholder(t *testing.T) {
	result := agentic.SubstituteArguments("Do the thing", "test args", true, nil)
	assert.Contains(t, result, "ARGUMENTS: test args")
}

func TestSubstituteArguments_NoAppendWhenPlaceholderExists(t *testing.T) {
	result := agentic.SubstituteArguments("Run: $ARGUMENTS", "test", true, nil)
	assert.Equal(t, "Run: test", result)
	assert.NotContains(t, result, "ARGUMENTS:")
}

func TestSubstituteArguments_EmptyArgs(t *testing.T) {
	result := agentic.SubstituteArguments("Hello", "", true, nil)
	assert.Equal(t, "Hello", result)
}

func TestSubstituteArguments_NoAppendEmptyArgs(t *testing.T) {
	result := agentic.SubstituteArguments("Hello", "", false, nil)
	assert.Equal(t, "Hello", result)
}

func TestSubstituteArguments_IndexOutOfRange(t *testing.T) {
	result := agentic.SubstituteArguments("Val: $ARGUMENTS[5]", "only one", false, nil)
	assert.Equal(t, "Val: ", result)
}

func TestSubstituteArguments_QuotedArgs(t *testing.T) {
	result := agentic.SubstituteArguments("File: $0", `"hello world"`, false, nil)
	assert.Equal(t, "File: hello world", result)
}

func TestSubstituteArguments_MixedPlaceholders(t *testing.T) {
	content := "All: $ARGUMENTS, First: $0, Second: $ARGUMENTS[1]"
	result := agentic.SubstituteArguments(content, "a b c", false, nil)
	assert.Contains(t, result, "All: a b c")
	assert.Contains(t, result, "First: a")
	assert.Contains(t, result, "Second: b")
}

// --- isNumericOnly (tested via ParseArgumentNames) ---

func TestParseArgumentNames_AllNumeric(t *testing.T) {
	result := agentic.ParseArgumentNames("123 456")
	assert.Nil(t, result)
}
