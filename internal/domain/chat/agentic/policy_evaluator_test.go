package agentic

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validPolicyEvaluatorRule() PolicyEvaluatorRule {
	return PolicyEvaluatorRule{
		RuleID:          "deny-shell-destructive",
		ToolNamePattern: `^shell\.(rm|drop|truncate).*$`,
		Description:     "Destructive shell commands deny by default at classifier level.",
		Decision:        PolicyEvaluatorDecisionDeny,
		Confidence:      PolicyEvaluatorConfidenceHigh,
		Priority:        10,
	}
}

func TestPolicyEvaluator_IsValidDecision(t *testing.T) {
	for _, d := range allPolicyEvaluatorDecisions {
		assert.True(t, IsValidPolicyEvaluatorDecision(d))
	}
	assert.False(t, IsValidPolicyEvaluatorDecision(PolicyEvaluatorDecision("nope")))
}

func TestPolicyEvaluator_AllDecisionsReturnsCopy(t *testing.T) {
	d := AllPolicyEvaluatorDecisions()
	require.Equal(t, 4, len(d))
	d[0] = "tampered"
	d2 := AllPolicyEvaluatorDecisions()
	assert.Equal(t, PolicyEvaluatorDecisionAllow, d2[0])
}

func TestPolicyEvaluator_IsValidConfidence(t *testing.T) {
	for _, c := range allPolicyEvaluatorConfidences {
		assert.True(t, IsValidPolicyEvaluatorConfidence(c))
	}
	assert.False(t, IsValidPolicyEvaluatorConfidence(PolicyEvaluatorConfidence("nope")))
}

func TestPolicyEvaluator_AllConfidencesReturnsCopy(t *testing.T) {
	c := AllPolicyEvaluatorConfidences()
	require.Equal(t, 3, len(c))
	c[0] = "tampered"
	c2 := AllPolicyEvaluatorConfidences()
	assert.Equal(t, PolicyEvaluatorConfidenceLow, c2[0])
}

func TestPolicyEvaluator_RuleValidateBadRuleID(t *testing.T) {
	r := validPolicyEvaluatorRule()
	r.RuleID = "BAD"
	assert.ErrorIs(t, r.Validate(), ErrPolicyEvaluatorBadRuleID)
}

func TestPolicyEvaluator_RuleValidateEmptyPattern(t *testing.T) {
	r := validPolicyEvaluatorRule()
	r.ToolNamePattern = ""
	assert.ErrorIs(t, r.Validate(), ErrPolicyEvaluatorEmptyPattern)
}

func TestPolicyEvaluator_RuleValidateBadPatternRegex(t *testing.T) {
	r := validPolicyEvaluatorRule()
	r.ToolNamePattern = "[unclosed"
	assert.ErrorIs(t, r.Validate(), ErrPolicyEvaluatorBadPattern)
}

func TestPolicyEvaluator_RuleValidateBadDecision(t *testing.T) {
	r := validPolicyEvaluatorRule()
	r.Decision = "limbo"
	assert.ErrorIs(t, r.Validate(), ErrPolicyEvaluatorBadDecision)
}

func TestPolicyEvaluator_RuleValidateBadConfidence(t *testing.T) {
	r := validPolicyEvaluatorRule()
	r.Confidence = "shaky"
	assert.ErrorIs(t, r.Validate(), ErrPolicyEvaluatorBadConfidence)
}

func TestPolicyEvaluator_RuleValidateBadPriority(t *testing.T) {
	r := validPolicyEvaluatorRule()
	r.Priority = -1
	assert.ErrorIs(t, r.Validate(), ErrPolicyEvaluatorBadPriority)
}

func TestPolicyEvaluator_RuleValidateEmptyDescription(t *testing.T) {
	r := validPolicyEvaluatorRule()
	r.Description = ""
	assert.ErrorIs(t, r.Validate(), ErrPolicyEvaluatorEmptyDescription)
}

func TestPolicyEvaluator_ClassifierAddRuleAndEvaluateMatch(t *testing.T) {
	c := NewPolicyEvaluatorClassifier()
	require.NoError(t, c.AddRule(validPolicyEvaluatorRule()))
	out := c.Evaluate(PolicyEvaluatorInput{ToolName: "shell.rm"})
	assert.Equal(t, PolicyEvaluatorDecisionDeny, out.Decision)
	assert.Equal(t, "deny-shell-destructive", out.MatchedRule)
}

func TestPolicyEvaluator_ClassifierEvaluateAbstainsNoMatch(t *testing.T) {
	c := NewPolicyEvaluatorClassifier()
	require.NoError(t, c.AddRule(validPolicyEvaluatorRule()))
	out := c.Evaluate(PolicyEvaluatorInput{ToolName: "search.lookup"})
	assert.Equal(t, PolicyEvaluatorDecisionAbstain, out.Decision)
	assert.Equal(t, "", out.MatchedRule)
}

func TestPolicyEvaluator_ClassifierEvaluateEmptyAbstains(t *testing.T) {
	c := NewPolicyEvaluatorClassifier()
	out := c.Evaluate(PolicyEvaluatorInput{ToolName: "anything"})
	assert.Equal(t, PolicyEvaluatorDecisionAbstain, out.Decision)
}

func TestPolicyEvaluator_ClassifierRejectsBadRule(t *testing.T) {
	c := NewPolicyEvaluatorClassifier()
	bad := validPolicyEvaluatorRule()
	bad.RuleID = "BAD"
	assert.ErrorIs(t, c.AddRule(bad), ErrPolicyEvaluatorBadRuleID)
}

func TestPolicyEvaluator_ClassifierRejectsDuplicateRuleID(t *testing.T) {
	c := NewPolicyEvaluatorClassifier()
	require.NoError(t, c.AddRule(validPolicyEvaluatorRule()))
	err := c.AddRule(validPolicyEvaluatorRule())
	assert.ErrorIs(t, err, ErrPolicyEvaluatorDuplicateRuleID)
}

func TestPolicyEvaluator_PriorityOrderingFirstMatchWins(t *testing.T) {
	c := NewPolicyEvaluatorClassifier()
	// Lower priority fires first. Rule A (priority=5) matches anything;
	// Rule B (priority=10) matches shell.*.
	ruleA := PolicyEvaluatorRule{
		RuleID:          "allow-all-low-priority",
		ToolNamePattern: `^.*$`,
		Description:     "fallback allow at priority 5",
		Decision:        PolicyEvaluatorDecisionAllow,
		Confidence:      PolicyEvaluatorConfidenceLow,
		Priority:        5,
	}
	ruleB := validPolicyEvaluatorRule() // priority 10, deny shell
	require.NoError(t, c.AddRule(ruleA))
	require.NoError(t, c.AddRule(ruleB))
	// shell.rm matches both, but ruleA (priority=5) wins.
	out := c.Evaluate(PolicyEvaluatorInput{ToolName: "shell.rm"})
	assert.Equal(t, PolicyEvaluatorDecisionAllow, out.Decision)
	assert.Equal(t, "allow-all-low-priority", out.MatchedRule)
}

func TestPolicyEvaluator_TieBreakerOnEqualPriorityByRuleID(t *testing.T) {
	c := NewPolicyEvaluatorClassifier()
	rA := validPolicyEvaluatorRule()
	rA.RuleID = "alpha-rule"
	rB := validPolicyEvaluatorRule()
	rB.RuleID = "beta-rule"
	require.NoError(t, c.AddRule(rA))
	require.NoError(t, c.AddRule(rB))
	list := c.ListRules()
	require.Equal(t, 2, len(list))
	// Priority equal → sort by RuleID asc.
	assert.Equal(t, "alpha-rule", list[0].RuleID)
}

func TestPolicyEvaluator_ListRulesReturnsCopy(t *testing.T) {
	c := NewPolicyEvaluatorClassifier()
	require.NoError(t, c.AddRule(validPolicyEvaluatorRule()))
	list := c.ListRules()
	list[0].RuleID = "tampered"
	list2 := c.ListRules()
	assert.Equal(t, "deny-shell-destructive", list2[0].RuleID)
}

func TestPolicyEvaluator_Size(t *testing.T) {
	c := NewPolicyEvaluatorClassifier()
	assert.Equal(t, 0, c.Size())
	_ = c.AddRule(validPolicyEvaluatorRule())
	assert.Equal(t, 1, c.Size())
}

func TestPolicyEvaluator_ConcurrentAddRuleSafe(t *testing.T) {
	c := NewPolicyEvaluatorClassifier()
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r := validPolicyEvaluatorRule()
			r.RuleID = "concurrent-" + string(rune('a'+i%26)) + string(rune('0'+i/26))
			_ = c.AddRule(r)
		}(i)
	}
	wg.Wait()
	assert.Equal(t, 30, c.Size())
}

func TestPolicyEvaluator_FourDecisionsCover(t *testing.T) {
	expected := map[PolicyEvaluatorDecision]bool{
		PolicyEvaluatorDecisionAllow:    true,
		PolicyEvaluatorDecisionDeny:     true,
		PolicyEvaluatorDecisionEscalate: true,
		PolicyEvaluatorDecisionAbstain:  true,
	}
	assert.Equal(t, 4, len(expected))
	for _, d := range allPolicyEvaluatorDecisions {
		assert.True(t, expected[d])
	}
}

func TestPolicyEvaluator_ThreeConfidencesCover(t *testing.T) {
	expected := map[PolicyEvaluatorConfidence]bool{
		PolicyEvaluatorConfidenceLow:    true,
		PolicyEvaluatorConfidenceMedium: true,
		PolicyEvaluatorConfidenceHigh:   true,
	}
	assert.Equal(t, 3, len(expected))
	for _, c := range allPolicyEvaluatorConfidences {
		assert.True(t, expected[c])
	}
}
