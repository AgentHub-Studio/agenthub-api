package agentic_test

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- NormalizeLegacyToolName ---

func TestNormalizeLegacyToolName_Known(t *testing.T) {
	assert.Equal(t, "Agent", agentic.NormalizeLegacyToolName("Task"))
}

func TestNormalizeLegacyToolName_Unknown(t *testing.T) {
	assert.Equal(t, "Bash", agentic.NormalizeLegacyToolName("Bash"))
}

func TestNormalizeLegacyToolName_KillShell(t *testing.T) {
	assert.Equal(t, "TaskStop", agentic.NormalizeLegacyToolName("KillShell"))
}

func TestNormalizeLegacyToolName_BashOutputTool(t *testing.T) {
	assert.Equal(t, "TaskOutput", agentic.NormalizeLegacyToolName("BashOutputTool"))
}

// --- GetLegacyToolNames ---

func TestGetLegacyToolNames_TaskOutput(t *testing.T) {
	names := agentic.GetLegacyToolNames("TaskOutput")
	sort.Strings(names)
	assert.Equal(t, []string{"AgentOutputTool", "BashOutputTool"}, names)
}

func TestGetLegacyToolNames_NoLegacy(t *testing.T) {
	names := agentic.GetLegacyToolNames("Bash")
	assert.Empty(t, names)
}

// --- EscapeRuleContent / UnescapeRuleContent ---

func TestEscapeRuleContent_Parentheses(t *testing.T) {
	assert.Equal(t, `psycopg2.connect\(\)`, agentic.EscapeRuleContent("psycopg2.connect()"))
}

func TestEscapeRuleContent_Backslash(t *testing.T) {
	assert.Equal(t, `echo "test\\nvalue"`, agentic.EscapeRuleContent(`echo "test\nvalue"`))
}

func TestEscapeRuleContent_Mixed(t *testing.T) {
	assert.Equal(t, `a\\b\(c\)`, agentic.EscapeRuleContent(`a\b(c)`))
}

func TestEscapeRuleContent_NoSpecial(t *testing.T) {
	assert.Equal(t, "npm install", agentic.EscapeRuleContent("npm install"))
}

func TestUnescapeRuleContent_Parentheses(t *testing.T) {
	assert.Equal(t, "psycopg2.connect()", agentic.UnescapeRuleContent(`psycopg2.connect\(\)`))
}

func TestUnescapeRuleContent_Backslash(t *testing.T) {
	assert.Equal(t, `echo "test\nvalue"`, agentic.UnescapeRuleContent(`echo "test\\nvalue"`))
}

func TestEscapeUnescapeRoundtrip(t *testing.T) {
	original := `print("hello\nworld")`
	assert.Equal(t, original, agentic.UnescapeRuleContent(agentic.EscapeRuleContent(original)))
}

// --- PermissionRuleFromString ---

func TestPermissionRuleFromString_ToolOnly(t *testing.T) {
	rule := agentic.PermissionRuleFromString("Bash")
	assert.Equal(t, "Bash", rule.ToolName)
	assert.Equal(t, "", rule.RuleContent)
}

func TestPermissionRuleFromString_WithContent(t *testing.T) {
	rule := agentic.PermissionRuleFromString("Bash(npm install)")
	assert.Equal(t, "Bash", rule.ToolName)
	assert.Equal(t, "npm install", rule.RuleContent)
}

func TestPermissionRuleFromString_EscapedParens(t *testing.T) {
	rule := agentic.PermissionRuleFromString(`Bash(python -c "print\(1\)")`)
	assert.Equal(t, "Bash", rule.ToolName)
	assert.Equal(t, `python -c "print(1)"`, rule.RuleContent)
}

func TestPermissionRuleFromString_LegacyName(t *testing.T) {
	rule := agentic.PermissionRuleFromString("Task")
	assert.Equal(t, "Agent", rule.ToolName)
}

func TestPermissionRuleFromString_LegacyNameWithContent(t *testing.T) {
	rule := agentic.PermissionRuleFromString("Task(something)")
	assert.Equal(t, "Agent", rule.ToolName)
	assert.Equal(t, "something", rule.RuleContent)
}

func TestPermissionRuleFromString_EmptyContent(t *testing.T) {
	rule := agentic.PermissionRuleFromString("Bash()")
	assert.Equal(t, "Bash", rule.ToolName)
	assert.Equal(t, "", rule.RuleContent)
}

func TestPermissionRuleFromString_WildcardContent(t *testing.T) {
	rule := agentic.PermissionRuleFromString("Bash(*)")
	assert.Equal(t, "Bash", rule.ToolName)
	assert.Equal(t, "", rule.RuleContent)
}

func TestPermissionRuleFromString_MissingToolName(t *testing.T) {
	rule := agentic.PermissionRuleFromString("(foo)")
	assert.Equal(t, "(foo)", rule.ToolName)
	assert.Equal(t, "", rule.RuleContent)
}

func TestPermissionRuleFromString_ContentAfterParen(t *testing.T) {
	rule := agentic.PermissionRuleFromString("Bash(foo)bar")
	assert.Equal(t, "Bash(foo)bar", rule.ToolName)
	assert.Equal(t, "", rule.RuleContent)
}

func TestPermissionRuleFromString_NoClosingParen(t *testing.T) {
	rule := agentic.PermissionRuleFromString("Bash(foo")
	assert.Equal(t, "Bash(foo", rule.ToolName)
	assert.Equal(t, "", rule.RuleContent)
}

// --- PermissionRuleToString ---

func TestPermissionRuleToString_ToolOnly(t *testing.T) {
	s := agentic.PermissionRuleToString(agentic.PermissionRuleValue{ToolName: "Bash"})
	assert.Equal(t, "Bash", s)
}

func TestPermissionRuleToString_WithContent(t *testing.T) {
	s := agentic.PermissionRuleToString(agentic.PermissionRuleValue{
		ToolName:    "Bash",
		RuleContent: "npm install",
	})
	assert.Equal(t, "Bash(npm install)", s)
}

func TestPermissionRuleToString_EscapesParens(t *testing.T) {
	s := agentic.PermissionRuleToString(agentic.PermissionRuleValue{
		ToolName:    "Bash",
		RuleContent: `python -c "print(1)"`,
	})
	assert.Equal(t, `Bash(python -c "print\(1\)")`, s)
}

func TestPermissionRuleRoundtrip(t *testing.T) {
	original := agentic.PermissionRuleValue{
		ToolName:    "Bash",
		RuleContent: `test\path(arg)`,
	}
	s := agentic.PermissionRuleToString(original)
	parsed := agentic.PermissionRuleFromString(s)
	assert.Equal(t, original.ToolName, parsed.ToolName)
	assert.Equal(t, original.RuleContent, parsed.RuleContent)
}

// --- findFirstUnescapedChar / findLastUnescapedChar (via PermissionRuleFromString) ---

func TestPermissionRuleFromString_EscapedOpenParen(t *testing.T) {
	// "\(" should not be treated as content delimiter
	rule := agentic.PermissionRuleFromString(`Bash\(not-content)`)
	// The \( is escaped, so there's no valid open paren → tool name only
	// Actually: findFirstUnescapedChar finds ')' ... let's see what happens
	// The first unescaped '(' is not found → tool only
	assert.Equal(t, `Bash\(not-content)`, rule.ToolName)
}

func TestPermissionRuleFromString_DoubleEscapedBackslash(t *testing.T) {
	// \\( means escaped backslash + unescaped paren
	rule := agentic.PermissionRuleFromString(`Bash\\(content)`)
	// The \\ is an escaped backslash, so ( is unescaped.
	// Tool name is the raw substring before the unescaped '(' = "Bash\\"
	assert.Equal(t, `Bash\\`, rule.ToolName)
	assert.Equal(t, "content", rule.RuleContent)
}
