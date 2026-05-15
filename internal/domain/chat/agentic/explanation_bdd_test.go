package agentic

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GOV-004 — Permission explainability BDD.
//
// PDF arXiv:2604.14228v1 Section 5.3 (decisions audit-observable + UI-
// surfaced); Section 11 (silent denials are the bug class to prevent).
//
// These scenarios validate the unified explanation contract: bounded
// vocabulary, multi-source aggregation, severity ordering, multi-format
// rendering (plain text, markdown, JSON), audit-friendliness.

func TestBDD_PermissionExplainability(t *testing.T) {

	t.Run("Scenario_EveryDecisionCarriesAReason", func(t *testing.T) {
		// Given the contract is "no decision without a stated reason"
		//       (audit guarantee from PDF Section 11),
		// When a runner produces a decision,
		// Then it constructs an explanation with PrimaryReason — the
		//      single-line summary that drops into log lines.
		e := NewPermissionExplanation(PermissionDeny, "shell", "blocked by enterprise policy")
		assert.Equal(t, "blocked by enterprise policy", e.PrimaryReason,
			"PrimaryReason is mandatory — never empty in a real explanation")
		assert.False(t, e.CreatedAt.IsZero(), "timestamp auto-populated")
	})

	t.Run("Scenario_MultipleSourcesContributeToOneDecision", func(t *testing.T) {
		// Given a deny decision emerged from BOTH policy + rule,
		// When the explanation is built,
		// Then both contributions appear, each tagged with its source —
		//      audit can determine which subsystem actually decided.
		e := NewPermissionExplanation(PermissionDeny, "delete-bucket", "destructive op blocked").
			AddRuleMatch("delete-bucket(*)", "deny rule matched").
			AddPolicyDecision("enterprise-opa", PolicyDeny, "compliance class C blocks deletion").
			AddCheckpointDecision("cp-42", "pre_destructive", CheckpointTimedOut, "no human in 5min")

		assert.Len(t, e.Contributions, 3)
		sources := map[ExplanationSource]bool{}
		for _, c := range e.Contributions {
			sources[c.Source] = true
		}
		assert.True(t, sources[ExplanationSourceRuleMatch])
		assert.True(t, sources[ExplanationSourcePolicyEngine])
		assert.True(t, sources[ExplanationSourceCheckpoint])
	})

	t.Run("Scenario_SeverityOrderingPutsCriticalFirstInRenderers", func(t *testing.T) {
		// Given UI/log renderers should lead with the most important
		//       reason (PDF Section 11 — surface critical signals first),
		e := NewPermissionExplanation(PermissionDeny, "x", "blocked")
		e.AddContribution(ExplanationContribution{
			Source: ExplanationSourceRuleMatch, Severity: ExplanationSeverityInfo, Message: "info detail",
		})
		e.AddContribution(ExplanationContribution{
			Source: ExplanationSourcePolicyEngine, Severity: ExplanationSeverityCritical, Message: "policy critical",
		})
		e.SortContributionsBySeverity()

		assert.Equal(t, ExplanationSeverityCritical, e.Contributions[0].Severity,
			"after sort, critical must be first")
	})

	t.Run("Scenario_BoundedSourcesPreventDriftAcrossDeploys", func(t *testing.T) {
		// Given dashboards group contributions by source,
		// When invalid sources reach AddContribution,
		// Then they are SILENTLY DROPPED (no log spam) — caller-bug
		//      detection happens via IsValidExplanationSource in tests.
		e := NewPermissionExplanation(PermissionAllow, "x", "ok")
		e.AddContribution(ExplanationContribution{
			Source: ExplanationSource("typo_source"), Severity: ExplanationSeverityInfo, Message: "x",
		})
		assert.Empty(t, e.Contributions,
			"invalid source dropped; explanation stays clean")
	})

	t.Run("Scenario_LongMessagesAreBoundedToPreventLogSpam", func(t *testing.T) {
		// Given user-controlled inputs may exceed sane log widths,
		long := strings.Repeat("x", 1500)
		e := NewPermissionExplanation(PermissionAllow, "x", "ok").
			AddRuleMatch("x(*)", long)
		assert.Equal(t, 500, len(e.Contributions[0].Message),
			"messages capped at 500 chars with ellipsis")
	})

	t.Run("Scenario_PolicyDenialEscalatesToCritical", func(t *testing.T) {
		// Given GOV-002 PolicyDeny means "do not proceed",
		// When AddPolicyDecision sees that outcome,
		// Then severity is critical — leads renderer ordering.
		e := NewPermissionExplanation(PermissionDeny, "send-email", "blocked").
			AddPolicyDecision("dlp", PolicyDeny, "DLP detected PII")
		assert.Equal(t, ExplanationSeverityCritical, e.Contributions[0].Severity)
		assert.True(t, e.HasCriticalContribution(),
			"HasCriticalContribution flags for audit review")
	})

	t.Run("Scenario_SafetyImmuneOverridesAreFirstClass", func(t *testing.T) {
		// Given safety-immune checks bypass PermissionModeBypass (PDF
		//       Section 5.3 — some checks cannot be silenced),
		// When the explanation records a safety-immune decision,
		// Then the source is ExplanationSourceSafetyImmune (its own
		//      bucket, not lumped under "rule" or "policy").
		e := NewPermissionExplanation(PermissionConfirm, "rm -rf /", "safety check").
			AddSafetyImmune("destructive_path", "deletion of root path is bypass-immune")
		assert.Equal(t, ExplanationSourceSafetyImmune, e.Contributions[0].Source)
		assert.Equal(t, ExplanationSeverityCritical, e.Contributions[0].Severity)
	})

	t.Run("Scenario_DefaultModeFallbackIsExplicit", func(t *testing.T) {
		// Given when no rule/policy/checkpoint matches, the
		//       PermissionMode default decides — but the user/audit
		//       MUST know "this came from the default, no explicit rule",
		e := NewPermissionExplanation(PermissionConfirm, "x", "no specific rule, default mode asks").
			AddDefaultModeFallback(PermissionModeDefault, "no allow/deny/confirm pattern matched")
		assert.Equal(t, ExplanationSourceDefaultMode, e.Contributions[0].Source)
		assert.Equal(t, "default", e.Contributions[0].SourceID,
			"SourceID carries the mode for analytics")
	})

	t.Run("Scenario_MultipleRendererFormatsForDifferentSurfaces", func(t *testing.T) {
		// Given the explanation must surface to: (a) terminal logs,
		//       (b) chat UI, (c) audit JSON,
		e := NewPermissionExplanation(PermissionDeny, "shell", "blocked").
			AddRuleMatch("shell(*)", "deny rule matched")

		plain := e.PlainText()
		md := e.Markdown()
		jsonStr, err := e.JSON()
		require.NoError(t, err)

		// Plain text: single-line, decision tag uppercase.
		assert.Contains(t, plain, "[DENY]")
		assert.True(t, strings.Count(plain, "\n") <= 1, "plain text fits one line")

		// Markdown: multi-line, bullet contributions, tool-name in backticks.
		assert.Contains(t, md, "**DENY**")
		assert.Contains(t, md, "`shell`")
		assert.Contains(t, md, "- **[info]")

		// JSON: stable wire keys.
		assert.Contains(t, jsonStr, `"decision":"deny"`)
		assert.Contains(t, jsonStr, `"toolName":"shell"`)
		assert.Contains(t, jsonStr, `"primaryReason"`)
		assert.Contains(t, jsonStr, `"contributions"`)
	})

	t.Run("Scenario_HistogramBySourceHasStableAxes", func(t *testing.T) {
		// Given dashboards aggregate contributions by source,
		// When CountBySource returns a histogram,
		// Then ALL bounded sources are present (zero defaults included)
		//      — dashboards never need conditional logic.
		e := NewPermissionExplanation(PermissionAllow, "x", "ok").
			AddRuleMatch("x(*)", "matched")
		hist := e.CountBySource()
		for _, s := range AllExplanationSources() {
			_, exists := hist[s]
			assert.True(t, exists, "source %q axis must always exist", s)
		}
		assert.Equal(t, 1, hist[ExplanationSourceRuleMatch])
		assert.Equal(t, 0, hist[ExplanationSourcePolicyEngine])
	})

	t.Run("Scenario_AllSixSourcesHaveStableWireStrings", func(t *testing.T) {
		// Given external systems bind to the wire strings,
		expected := map[string]bool{
			"rule_match":     true,
			"policy_engine":  true,
			"checkpoint":     true,
			"hook":           true,
			"default_mode":   true,
			"safety_immune":  true,
		}
		for _, s := range AllExplanationSources() {
			assert.True(t, expected[string(s)],
				"source %q not in expected wire-stable set", s)
		}
		assert.Equal(t, len(expected), len(AllExplanationSources()))
	})

	t.Run("Scenario_HookBlockingDecisionIsCritical", func(t *testing.T) {
		// Given hooks can block tool execution (PDF Section 5.3 —
		//       PreToolUse hook),
		blocking := NewPermissionExplanation(PermissionDeny, "x", "blocked by hook").
			AddHookDecision("safety-pretooluse-confirm-destructive", "hook blocked", true)
		assert.Equal(t, ExplanationSeverityCritical, blocking.Contributions[0].Severity)

		informative := NewPermissionExplanation(PermissionAllow, "x", "ok").
			AddHookDecision("lifecycle-sessionstart-greeting", "hook noted", false)
		assert.Equal(t, ExplanationSeverityInfo, informative.Contributions[0].Severity)
	})
}
