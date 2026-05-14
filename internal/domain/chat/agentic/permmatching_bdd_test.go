package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD-style scenarios that ratify PERM-002 (Rule matching por tool e input)
// against the Claude Code architecture paper "Dive into Claude Code"
// (arXiv:2604.14228v1), Section 5.1 ("Permission Modes and Rule Evaluation")
// and Section 5.2 ("The Authorization Pipeline").
//
// PERM-001 already ratified the deny-first ORDERING semantics. PERM-002 zooms
// into the MATCHING layer: how a "tool" or "tool(input)" rule string is
// parsed and how it is matched against a runtime tool-call. The PDF requires:
//
//   - Tool-level matching by name (with glob support, e.g. "http-*").
//   - Content-level matching by input pattern within parentheses
//     (e.g. "Bash(prefix:npm)").
//   - Word-boundary matching so "DELETE" doesn't match "deleted" — otherwise
//     "any SQL containing 'delete'" would over-trigger.
//   - Case-insensitive content matching (operators and keywords are not
//     case-sensitive in SQL/HTTP).
//   - Escape handling for parentheses inside content (`\(` `\)`).
//   - Roundtrip: parse(toString(rule)) == rule.
//   - MCP tools matched by their fully qualified name "mcp__server__tool"
//     (PDF Section 5.2: "MCP tools are matched by their fully qualified name").
//
// Implementation under test: matchesPermissionPattern + EvaluatePermission +
// PermissionRuleFromString/ToString in permission.go and permruleparser.go.

func TestBDD_PermissionRuleMatching(t *testing.T) {
	t.Run("Scenario_ToolOnlyRuleMatchesAnyInput", func(t *testing.T) {
		// Given a rule with no parenthesised content (PDF: tool-level rule),
		given := &PermissionRules{Allow: []string{"document-search"}}

		// When the agent calls the tool with arbitrary input,
		when := EvaluatePermission(given, "document-search", "completely irrelevant payload")

		// Then the rule matches — content is unconstrained at tool-only scope.
		assert.Equal(t, PermissionAllow, when,
			"tool-only rule must match any input (PDF Section 5.1 pattern syntax)")
	})

	t.Run("Scenario_GlobToolPatternMatchesPrefixFamilies", func(t *testing.T) {
		// Given a glob rule covering an entire family of tools (PDF Section 5.1
		//       example "Bash(prefix:npm)"-style patterns and Section 5.2:
		//       "server-prefix rules like mcp__server strip all tools from
		//       that server"),
		given := &PermissionRules{Deny: []string{"http-*"}}

		// When the agent attempts a tool inside that family,
		when1 := EvaluatePermission(given, "http-get", "https://example.com")
		when2 := EvaluatePermission(given, "http-post", `{"body":1}`)
		when3 := EvaluatePermission(given, "execute-sql", "SELECT 1")

		// Then matching tools are denied and unrelated tools are not affected.
		assert.Equal(t, PermissionDeny, when1, "http-get must match http-*")
		assert.Equal(t, PermissionDeny, when2, "http-post must match http-*")
		assert.Equal(t, PermissionAllow, when3,
			"execute-sql must NOT match http-* — glob is anchored to tool name")
	})

	t.Run("Scenario_ContentMatchUsesWordBoundary", func(t *testing.T) {
		// Given a content-level rule for the SQL keyword DELETE (PDF: rules
		//       must not silently over-match on substring),
		given := &PermissionRules{Deny: []string{"execute-sql(DELETE)"}}

		// When the agent submits SQL where "delete" appears only inside an
		//      identifier ("is_deleted", "deletedAt"),
		when1 := EvaluatePermission(given, "execute-sql",
			"SELECT id FROM users WHERE is_deleted = false")
		when2 := EvaluatePermission(given, "execute-sql",
			"SELECT name, deletedAt FROM products")

		// Then the rule does NOT match — preventing false positives that would
		//      block read queries on tables with soft-delete columns.
		assert.Equal(t, PermissionAllow, when1,
			"is_deleted must not trigger DELETE rule (word boundary required)")
		assert.Equal(t, PermissionAllow, when2,
			"deletedAt identifier must not trigger DELETE rule")

		// And the actual destructive statement DOES match.
		when3 := EvaluatePermission(given, "execute-sql",
			"DELETE FROM users WHERE id = 1")
		assert.Equal(t, PermissionDeny, when3,
			"actual DELETE statement must match the content rule")
	})

	t.Run("Scenario_ContentMatchIsCaseInsensitive", func(t *testing.T) {
		// Given a rule on a lowercase pattern,
		given := &PermissionRules{Deny: []string{"execute-sql(drop)"}}

		// When the agent emits SQL with the keyword in a different case,
		when := EvaluatePermission(given, "execute-sql", "DROP TABLE customers")

		// Then matching is case-insensitive — SQL keywords don't care about
		//      case, and rule authors must not be forced to enumerate every
		//      casing variation.
		assert.Equal(t, PermissionDeny, when,
			"content matching must be case-insensitive")
	})

	t.Run("Scenario_EscapedParenthesesAreLiterals", func(t *testing.T) {
		// Given a rule whose content itself contains parentheses, escaped
		//       per the parser convention,
		given := PermissionRuleFromString(`exec(echo \(hello\))`)

		// When the parser splits tool from content,
		// Then the tool name is correct and the content is unescaped to a
		//      literal — the parens are payload, not delimiters.
		assert.Equal(t, "exec", given.ToolName,
			"tool name must stop at the first unescaped paren")
		assert.Equal(t, "echo (hello)", given.RuleContent,
			"escaped parens must be unescaped in the parsed content")
	})

	t.Run("Scenario_RuleRoundtripPreservesContent", func(t *testing.T) {
		// Given a rule with content containing special characters,
		original := PermissionRuleValue{
			ToolName:    "shell",
			RuleContent: "rm -rf (logs)",
		}

		// When the rule is serialised then parsed back,
		serialised := PermissionRuleToString(original)
		roundtripped := PermissionRuleFromString(serialised)

		// Then the value is identical — durable storage is safe and an
		//      equality check is reliable.
		assert.Equal(t, original.ToolName, roundtripped.ToolName,
			"roundtrip must preserve tool name")
		assert.Equal(t, original.RuleContent, roundtripped.RuleContent,
			"roundtrip must preserve content with parens")
	})

	t.Run("Scenario_ToolNameMustMatchBeforeContentIsConsidered", func(t *testing.T) {
		// Given a content-bearing rule,
		given := &PermissionRules{Deny: []string{"execute-sql(DROP)"}}

		// When a different tool emits a payload that matches the content,
		when := EvaluatePermission(given, "memo-write", "DROP this note")

		// Then the rule does not fire — the tool gate is the precondition.
		//      Otherwise content rules would leak across tools.
		assert.Equal(t, PermissionAllow, when,
			"content match is irrelevant when tool name does not match")
	})

	t.Run("Scenario_MCPFullyQualifiedNameMatchedAsLiteral", func(t *testing.T) {
		// Given an MCP tool denied by its fully qualified name (PDF Section
		//       5.2: "MCP tools are matched by their fully qualified
		//       mcp__server__tool name, and server-level rules match all
		//       tools from that server"),
		given := &PermissionRules{Deny: []string{"mcp__github__create_issue"}}

		// When that exact MCP tool is invoked,
		when := EvaluatePermission(given, "mcp__github__create_issue",
			`{"repo":"foo","title":"bar"}`)

		// Then it is denied — the fully qualified naming convention round-trips
		//      through the matcher unchanged.
		assert.Equal(t, PermissionDeny, when,
			"fully qualified MCP name must match a literal MCP rule")
	})

	t.Run("Scenario_MCPServerPrefixGlobMatchesAllToolsFromThatServer", func(t *testing.T) {
		// Given a server-level deny on every tool from one MCP server (PDF
		//       Section 5.2: "server-prefix rules like mcp__server strip all
		//       tools from that server"),
		given := &PermissionRules{Deny: []string{"mcp__github__*"}}

		// When tools from that server are invoked,
		when1 := EvaluatePermission(given, "mcp__github__create_issue", `{}`)
		when2 := EvaluatePermission(given, "mcp__github__list_pulls", `{}`)
		// And a tool from a different server is invoked,
		when3 := EvaluatePermission(given, "mcp__linear__list_issues", `{}`)

		// Then only the targeted server's tools are denied.
		assert.Equal(t, PermissionDeny, when1, "server prefix must deny tool 1")
		assert.Equal(t, PermissionDeny, when2, "server prefix must deny tool 2")
		assert.Equal(t, PermissionAllow, when3,
			"server prefix must NOT cross over to other MCP servers")
	})

	t.Run("Scenario_LegacyAliasResolvesBeforeMatching", func(t *testing.T) {
		// Given a rule using the canonical name and a call coming in with the
		//       legacy alias (e.g. "Task" → "Agent"),
		given := PermissionRuleFromString("Task")

		// When the parser normalises it,
		// Then the rule's tool name is the canonical "Agent" — preventing
		//      stale durable rules from drifting after a tool rename.
		assert.Equal(t, "Agent", given.ToolName,
			"legacy alias must normalise to canonical name in rule storage")
	})
}
