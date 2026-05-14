package agentic

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD-style scenarios that ratify PERM-001 (Deny-first rule engine) against the
// Claude Code architecture paper "Dive into Claude Code" (arXiv:2604.14228v1),
// Section 5.1 ("Permission Modes and Rule Evaluation") and Table 1 row
// "Deny-first with human escalation".
//
// The PDF specifies:
//   - Deny rules ALWAYS take precedence over allow rules, even when the allow
//     rule is more specific (Section 5.1).
//   - Unrecognized actions are escalated (confirm) rather than allowed silently
//     (Table 1 — "ask-by-default").
//   - Modes form a graduated trust spectrum from plan/default through
//     bypassPermissions; deny rules survive even bypass mode.
//   - Bypass-immune deny rules cannot be overridden by a narrow allow.
//
// These behaviours are unit-level concerns of the permission engine and are
// validated here in the agentic package using Given/When/Then naming, since
// the e2e harness BDD framework targets HTTP endpoints and is not a fit for
// in-process policy evaluation.

func TestBDD_DenyFirstRuleEngine(t *testing.T) {
	t.Run("Scenario_DenyAlwaysWinsOverMoreSpecificAllow", func(t *testing.T) {
		// Given a tenant has configured an allow rule that targets a specific
		//       tool input and a broader deny rule that matches the same call,
		given := &PermissionRules{
			Allow: []string{"execute-sql(SELECT * FROM products)"},
			Deny:  []string{"execute-sql(SELECT)"},
		}

		// When the agent attempts to execute the more specific allowed call,
		when := EvaluatePermission(given, "execute-sql", "SELECT * FROM products")

		// Then the deny rule prevails, matching PDF Section 5.1:
		//      "A broad deny ... cannot be overridden by a narrow allow".
		assert.Equal(t, PermissionDeny, when,
			"deny must override more specific allow — PDF Section 5.1")
	})

	t.Run("Scenario_BypassModeStillRespectsExplicitDeny", func(t *testing.T) {
		// Given the tenant raised the trust level to bypass mode but still
		//       configured a deny rule for destructive shell calls,
		given := &PermissionRules{
			Mode: PermissionModeBypass,
			Deny: []string{"shell(rm -rf)"},
		}

		// When the agent attempts a denied destructive shell call,
		when := EvaluatePermission(given, "shell", "rm -rf /")

		// Then the deny rule still wins — PDF Section 5.1: "deny rules ALWAYS
		//      take precedence" and "safety-critical checks ... bypass-immune".
		assert.Equal(t, PermissionDeny, when,
			"bypass mode must not silently override explicit deny rules")
	})

	t.Run("Scenario_ConfirmRuleEscalatesInsteadOfAllowing", func(t *testing.T) {
		// Given a confirm rule is configured for a sensitive tool
		//       (tool-only rule matches any input),
		given := &PermissionRules{
			Confirm: []string{"http-request"},
		}

		// When the agent attempts the matching call,
		when := EvaluatePermission(given, "http-request", "https://example.com")

		// Then the engine asks for human confirmation rather than allowing
		//      silently — PDF Table 1: "ask-by-default" / human escalation.
		assert.Equal(t, PermissionConfirm, when,
			"confirm rules must escalate, not allow silently")
	})

	t.Run("Scenario_DontAskModeConvertsConfirmIntoDeny", func(t *testing.T) {
		// Given an unattended (dont_ask) tenant where prompting is impossible,
		given := &PermissionRules{
			Mode:    PermissionModeDontAsk,
			Confirm: []string{"http-request"},
		}

		// When the agent attempts a call that would normally require confirm,
		when := EvaluatePermission(given, "http-request", "https://example.com")

		// Then the engine fails closed — PDF Section 5.1: "dontAsk: No
		//      prompting, but deny rules are still enforced".
		assert.Equal(t, PermissionDeny, when,
			"dont_ask mode must fail closed when confirmation is required")
	})

	t.Run("Scenario_DangerousShellInputEscalatesEvenIfAllowed", func(t *testing.T) {
		// Given a permissive ruleset that allows the shell tool,
		given := &PermissionRules{Allow: []string{"shell"}}

		// When the agent attempts a call carrying a dangerous shell pattern,
		when := EvaluatePermissionWithDangerCheck(given, "shell", "sudo rm -rf /", []string{"shell"})

		// Then the engine escalates to confirm — PDF Section 5.1:
		//      "reversibility-weighted risk assessment" and danger detection
		//      complementing allow rules.
		assert.Equal(t, PermissionConfirm, when,
			"dangerous shell patterns must escalate even with broad allow")
	})

	t.Run("Scenario_NoRulesYieldsAllowByDefault", func(t *testing.T) {
		// Given no permission rules are configured for the agent,
		var given *PermissionRules

		// When the agent attempts any tool call,
		when := EvaluatePermission(given, "anything", "any input")

		// Then the engine allows by default — empty rules are not the
		//      production posture but the engine must not crash or block
		//      arbitrarily; the production posture is set by callers.
		assert.Equal(t, PermissionAllow, when,
			"nil rules must not block tool calls inside the engine")
	})

	t.Run("Scenario_AllowRuleFiltersUnlistedTools", func(t *testing.T) {
		// Given an allow-list-only configuration (zero deny / zero confirm),
		given := &PermissionRules{Allow: []string{"document-search"}}

		// When the agent attempts a tool not on the allow list,
		when := EvaluatePermission(given, "execute-sql", "SELECT 1")

		// Then the engine denies — Table 1: "deny-first with human escalation"
		//      manifested as deny when explicit allow list is exhausted.
		assert.Equal(t, PermissionDeny, when,
			"allow-list-only config must deny unlisted tools")
	})

	t.Run("Scenario_LegacyToolNameNormalizesBeforeMatching", func(t *testing.T) {
		// Given a deny rule using the canonical name and a call using the
		//       legacy alias,
		given := PermissionRuleFromString("Task")

		// When normalisation is applied,
		when := given.ToolName

		// Then the engine resolves to the canonical name "Agent" — PDF
		//      Section 5.2: "MCP tools are matched by their fully qualified
		//      name", and rule names must be stable across renames.
		assert.Equal(t, "Agent", when,
			"legacy aliases must normalize to canonical names")
	})

	t.Run("Scenario_RulesRoundtripThroughJSON", func(t *testing.T) {
		// Given a serialized ruleset retrieved from durable storage,
		raw := json.RawMessage(`{"allow":["read-doc"],"deny":["execute-sql(DROP)"],"mode":"default"}`)

		// When the engine parses and evaluates a denied call,
		given := ParsePermissionRules(raw)
		when := EvaluatePermission(given, "execute-sql", "DROP TABLE users")

		// Then the parsed rules behave identically to in-memory rules,
		//      preserving deny-first semantics across persistence boundaries.
		assert.NotNil(t, given, "parsed rules must not be nil for valid JSON")
		assert.Equal(t, PermissionDeny, when,
			"deserialized deny rule must keep precedence")
	})
}
