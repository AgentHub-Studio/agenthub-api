package agentic_test

import (
	"fmt"
	"strings"
	"testing"
	"unicode"

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

func TestGenerateArgumentHint_ExtraTypedArgs(t *testing.T) {
	result := agentic.GenerateArgumentHint([]string{"foo", "bar"}, []string{"a", "b", "c"})
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

func FuzzGenerateArgumentHintBounds(f *testing.F) {
	f.Add("alpha beta gamma", "one two")
	f.Add("alpha beta", "one two three")
	f.Add("", "one two")

	f.Fuzz(func(t *testing.T, rawNames string, rawTyped string) {
		names := agentic.ParseArgumentNames(limitArgumentFuzzString(rawNames))
		typed := agentic.ParseArguments(limitArgumentFuzzString(rawTyped))
		got := agentic.GenerateArgumentHint(names, typed)

		if len(typed) >= len(names) {
			if got != "" {
				t.Fatalf("hint for filled arguments = %q, want empty; names=%v typed=%v", got, names, typed)
			}
			return
		}

		for _, name := range names[len(typed):] {
			if !strings.Contains(got, "["+name+"]") {
				t.Fatalf("hint %q does not contain remaining argument %q; names=%v typed=%v", got, name, names, typed)
			}
		}
	})
}

func FuzzSubstituteArgumentsGeneratedPlaceholders(f *testing.F) {
	f.Add("alpha", "beta", "target")
	f.Add("file-name", "tenant_42", "subject")
	f.Add("one.two", "three_four", "arg_name")

	f.Fuzz(func(t *testing.T, rawFirst string, rawSecond string, rawName string) {
		first := safeArgumentToken(rawFirst, "alpha")
		second := safeArgumentToken(rawSecond, "beta")
		name := safeArgumentName(rawName, "target")
		args := first + " " + second

		content := fmt.Sprintf("all=$ARGUMENTS idx0=$ARGUMENTS[0] idx1=$1 named=$%s suffix", name)
		got := agentic.SubstituteArguments(content, args, false, []string{name})
		want := fmt.Sprintf("all=%s idx0=%s idx1=%s named=%s suffix", args, first, second, first)
		if got != want {
			t.Fatalf("substitution = %q, want %q", got, want)
		}

		for _, forbidden := range []string{"$ARGUMENTS", "$1", "$" + name} {
			if strings.Contains(got, forbidden) {
				t.Fatalf("substitution leaked placeholder %q in %q", forbidden, got)
			}
		}
	})
}

func limitArgumentFuzzString(raw string) string {
	if len(raw) > 160 {
		return raw[:160]
	}
	return raw
}

func safeArgumentToken(raw string, fallback string) string {
	var b strings.Builder
	for _, r := range raw {
		if b.Len() >= 32 {
			break
		}
		if r > unicode.MaxASCII {
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '.' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return fallback
	}
	return b.String()
}

func safeArgumentName(raw string, fallback string) string {
	var b strings.Builder
	for _, r := range raw {
		if b.Len() >= 24 {
			break
		}
		if r > unicode.MaxASCII {
			continue
		}
		if b.Len() == 0 {
			if unicode.IsLetter(r) || r == '_' {
				b.WriteRune(unicode.ToLower(r))
			}
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	if b.Len() == 0 {
		return fallback
	}
	name := b.String()
	if name == "arguments" {
		return fallback
	}
	return name
}
