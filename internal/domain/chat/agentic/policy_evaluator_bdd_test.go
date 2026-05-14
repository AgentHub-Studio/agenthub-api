package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBDD_PolicyEvaluator(t *testing.T) {
	t.Run("Scenario_DestructiveShellCommandDeniedByClassifierFallback", func(t *testing.T) {
		// Given declarative ACLs don't have an explicit rule for `shell.rm`,
		// When the classifier evaluates the tool call,
		// Then a deny decision with high confidence is returned.
		c := NewPolicyEvaluatorClassifier()
		require.NoError(t, c.AddRule(validPolicyEvaluatorRule()))
		out := c.Evaluate(PolicyEvaluatorInput{ToolName: "shell.rm -rf /"})
		assert.Equal(t, PolicyEvaluatorDecisionDeny, out.Decision)
		assert.Equal(t, PolicyEvaluatorConfidenceHigh, out.Confidence)
	})

	t.Run("Scenario_UnknownToolAbstainsToLetACLDecide", func(t *testing.T) {
		// Given the classifier has no rule for `internal.api.fetch`,
		// When evaluating,
		// Then it abstains so declarative ACLs upstream can decide.
		c := NewPolicyEvaluatorClassifier()
		require.NoError(t, c.AddRule(validPolicyEvaluatorRule()))
		out := c.Evaluate(PolicyEvaluatorInput{ToolName: "internal.api.fetch"})
		assert.Equal(t, PolicyEvaluatorDecisionAbstain, out.Decision)
		assert.Equal(t, "", out.MatchedRule)
	})

	t.Run("Scenario_PriorityOrderingDeterministic", func(t *testing.T) {
		// Given two rules match the same tool,
		// When the classifier evaluates,
		// Then the lower-priority rule wins (first-match-wins after sort).
		c := NewPolicyEvaluatorClassifier()
		rA := PolicyEvaluatorRule{
			RuleID: "broad-allow", ToolNamePattern: `^shell\..*$`,
			Description: "broad allow for shell tools", Decision: PolicyEvaluatorDecisionAllow,
			Confidence: PolicyEvaluatorConfidenceLow, Priority: 100,
		}
		rB := validPolicyEvaluatorRule() // priority=10, deny shell.rm
		require.NoError(t, c.AddRule(rA))
		require.NoError(t, c.AddRule(rB))
		out := c.Evaluate(PolicyEvaluatorInput{ToolName: "shell.rm"})
		// Lower priority wins → deny-shell-destructive (priority=10).
		assert.Equal(t, "deny-shell-destructive", out.MatchedRule)
		assert.Equal(t, PolicyEvaluatorDecisionDeny, out.Decision)
	})

	t.Run("Scenario_RationaleFromRuleDescriptionForAudit", func(t *testing.T) {
		// Given audit needs human-readable explanation of decisions,
		// When the classifier fires,
		// Then the rule's description is returned as Rationale.
		c := NewPolicyEvaluatorClassifier()
		require.NoError(t, c.AddRule(validPolicyEvaluatorRule()))
		out := c.Evaluate(PolicyEvaluatorInput{ToolName: "shell.drop_table"})
		assert.Contains(t, out.Rationale, "Destructive shell commands")
	})

	t.Run("Scenario_BadRegexRejectedAtRuleRegistration", func(t *testing.T) {
		// Given runtime panics on bad regex would crash the platform,
		// When AddRule is given an unclosed bracket,
		// Then registration fails with ErrPolicyEvaluatorBadPattern.
		c := NewPolicyEvaluatorClassifier()
		bad := validPolicyEvaluatorRule()
		bad.ToolNamePattern = "[oops"
		err := c.AddRule(bad)
		assert.ErrorIs(t, err, ErrPolicyEvaluatorBadPattern)
	})

	t.Run("Scenario_DuplicateRuleIDRejected", func(t *testing.T) {
		// Given RuleIDs must be unique for audit traceability,
		// When the same RuleID is registered twice,
		// Then the second call fails.
		c := NewPolicyEvaluatorClassifier()
		require.NoError(t, c.AddRule(validPolicyEvaluatorRule()))
		err := c.AddRule(validPolicyEvaluatorRule())
		assert.ErrorIs(t, err, ErrPolicyEvaluatorDuplicateRuleID)
	})

	t.Run("Scenario_EmptyClassifierAlwaysAbstains", func(t *testing.T) {
		// Given no rules are registered,
		// When the classifier evaluates anything,
		// Then it abstains with empty MatchedRule.
		c := NewPolicyEvaluatorClassifier()
		out := c.Evaluate(PolicyEvaluatorInput{ToolName: "anything"})
		assert.Equal(t, PolicyEvaluatorDecisionAbstain, out.Decision)
		assert.Equal(t, "", out.MatchedRule)
	})

	t.Run("Scenario_EscalateDecisionRoutesToHuman", func(t *testing.T) {
		// Given some tool calls are ambiguous and need human review,
		// When a rule fires with Decision=escalate,
		// Then the classifier emits escalate (caller routes to GOV-003
		// checkpoint or human-in-the-loop).
		c := NewPolicyEvaluatorClassifier()
		r := validPolicyEvaluatorRule()
		r.RuleID = "escalate-prod-writes"
		r.ToolNamePattern = `^db\.prod\..*$`
		r.Decision = PolicyEvaluatorDecisionEscalate
		r.Confidence = PolicyEvaluatorConfidenceMedium
		r.Description = "Prod DB writes always escalate to human."
		require.NoError(t, c.AddRule(r))
		out := c.Evaluate(PolicyEvaluatorInput{ToolName: "db.prod.delete"})
		assert.Equal(t, PolicyEvaluatorDecisionEscalate, out.Decision)
	})

	t.Run("Scenario_NegativePriorityRejected", func(t *testing.T) {
		// Given priority must be a non-negative ranking,
		// When negative priority is supplied,
		// Then Validate rejects.
		r := validPolicyEvaluatorRule()
		r.Priority = -5
		assert.ErrorIs(t, r.Validate(), ErrPolicyEvaluatorBadPriority)
	})

	t.Run("Scenario_FourDecisionsCoverAllowDenyEscalateAbstain", func(t *testing.T) {
		// Given the classifier's vocabulary must match PERM-006 prefilter,
		// When admin lists supported decisions,
		// Then 4 are bounded.
		assert.Equal(t, 4, len(AllPolicyEvaluatorDecisions()))
	})
}
