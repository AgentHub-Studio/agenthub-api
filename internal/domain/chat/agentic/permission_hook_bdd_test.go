package agentic

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBDD_PermissionHook(t *testing.T) {
	t.Run("Scenario_BeforeHookGrantsTemporaryEscalationForOncall", func(t *testing.T) {
		// Given the tenant policy denies "execute-sql" generally,
		// And user "alice" is currently on-call,
		// When alice's request reaches the chain,
		// Then a Before hook grants Allow without consulting the engine.
		c := NewPermissionHookChain(&PermissionRules{Deny: []string{"execute-sql"}})
		c.Register(PermissionHook{
			Name: "oncall-allow", Phase: PermissionHookBeforeEvaluate, Enabled: true,
			Handle: func(ctx context.Context, r PermissionHookRequest, e PermissionDecision) (PermissionHookOutcome, string) {
				if r.UserID == "alice" {
					return PermissionHookOverrideAllow, "on-call escalation"
				}
				return PermissionHookContinue, ""
			},
		})
		eval := c.Evaluate(context.Background(), PermissionHookRequest{
			ToolName: "execute-sql", UserID: "alice",
		})
		assert.Equal(t, PermissionAllow, eval.FinalDecision)
		assert.True(t, eval.ShortCircuited)
	})

	t.Run("Scenario_AfterHookEscalatesAllowToConfirmOnFlaggedInput", func(t *testing.T) {
		// Given the engine says Allow for "execute-sql" with arbitrary input,
		// And a customer-data heuristic flags the input as sensitive,
		// When the After hook runs,
		// Then the final decision is escalated to Confirm.
		c := NewPermissionHookChain(&PermissionRules{Allow: []string{"execute-sql"}})
		c.Register(PermissionHook{
			Name: "pii-escalate", Phase: PermissionHookAfterEvaluate, Enabled: true,
			Handle: func(ctx context.Context, r PermissionHookRequest, e PermissionDecision) (PermissionHookOutcome, string) {
				if containsCustomerData(r.ToolInput) {
					return PermissionHookOverrideConfirm, "PII-touching query"
				}
				return PermissionHookContinue, ""
			},
		})
		eval := c.Evaluate(context.Background(), PermissionHookRequest{
			ToolName: "execute-sql", ToolInput: "SELECT * FROM customer_data",
		})
		assert.Equal(t, PermissionAllow, eval.EngineDecision)
		assert.Equal(t, PermissionConfirm, eval.FinalDecision)
	})

	t.Run("Scenario_DisabledHookDoesNotRun", func(t *testing.T) {
		// Given an emergency procedure registered a "deny-all" hook,
		// And on-call later disabled it during a controlled deploy,
		// When the chain evaluates,
		// Then the engine decision passes through unchanged.
		c := NewPermissionHookChain(nil)
		c.Register(PermissionHook{
			Name: "deny-all", Phase: PermissionHookBeforeEvaluate, Enabled: true,
			Handle: overrideDenyHandle,
		})
		c.Disable("deny-all", PermissionHookBeforeEvaluate)
		eval := c.Evaluate(context.Background(), PermissionHookRequest{ToolName: "Read"})
		assert.Equal(t, PermissionAllow, eval.FinalDecision)
	})

	t.Run("Scenario_HookDecisionsCapturedForCompliance", func(t *testing.T) {
		// Given a compliance review asks "why was tool X denied at 14:32?",
		// When the chain emits its PermissionEvaluation,
		// Then every hook decision (name+phase+outcome+reason+timestamp)
		// is present in the audit record.
		c := NewPermissionHookChain(&PermissionRules{Deny: []string{"Bash"}})
		c.Register(PermissionHook{
			Name: "trace", Phase: PermissionHookAfterEvaluate, Enabled: true,
			Handle: continueHandle("just observing"),
		})
		eval := c.Evaluate(context.Background(), PermissionHookRequest{ToolName: "Bash"})
		require.Equal(t, 1, len(eval.HookDecisions))
		assert.Equal(t, "trace", eval.HookDecisions[0].HookName)
		assert.Equal(t, "just observing", eval.HookDecisions[0].Reason)
	})

	t.Run("Scenario_PriorityLadderRunsHighPriorityFirst", func(t *testing.T) {
		// Given two Before hooks: low-priority "block-all" and high-priority
		// "emergency-allow",
		// When the chain evaluates,
		// Then the high-priority hook runs first and short-circuits the
		// chain — low-priority block never executes.
		c := NewPermissionHookChain(nil)
		blockCalled := false
		c.Register(PermissionHook{
			Name: "emergency-allow", Phase: PermissionHookBeforeEvaluate,
			Priority: 1, Enabled: true,
			Handle: overrideAllowHandle,
		})
		c.Register(PermissionHook{
			Name: "block-all", Phase: PermissionHookBeforeEvaluate,
			Priority: 100, Enabled: true,
			Handle: func(ctx context.Context, r PermissionHookRequest, e PermissionDecision) (PermissionHookOutcome, string) {
				blockCalled = true
				return PermissionHookOverrideDeny, ""
			},
		})
		eval := c.Evaluate(context.Background(), PermissionHookRequest{ToolName: "Bash"})
		assert.Equal(t, PermissionAllow, eval.FinalDecision)
		assert.False(t, blockCalled)
	})

	t.Run("Scenario_AfterChainAllRunsLastOverrideWins", func(t *testing.T) {
		// Given two After hooks that both want to override,
		// When the chain runs,
		// Then both record their decisions but the LAST override wins —
		// the more recent rule has the final say.
		c := NewPermissionHookChain(&PermissionRules{Allow: []string{"Read"}})
		c.Register(PermissionHook{
			Name: "first", Phase: PermissionHookAfterEvaluate, Priority: 10,
			Enabled: true, Handle: overrideConfirmHandle,
		})
		c.Register(PermissionHook{
			Name: "second", Phase: PermissionHookAfterEvaluate, Priority: 20,
			Enabled: true, Handle: overrideDenyHandle,
		})
		eval := c.Evaluate(context.Background(), PermissionHookRequest{ToolName: "Read"})
		assert.Equal(t, PermissionDeny, eval.FinalDecision)
		assert.Equal(t, 2, len(eval.HookDecisions))
	})

	t.Run("Scenario_RulesCanBeReplacedAtRuntime", func(t *testing.T) {
		// Given a hot reload of permission rules (admin pushed an
		// updated deny list mid-session),
		// When the chain re-evaluates,
		// Then the new rules apply immediately — no chain restart.
		c := NewPermissionHookChain(nil)
		eval1 := c.Evaluate(context.Background(), PermissionHookRequest{ToolName: "Bash"})
		assert.Equal(t, PermissionAllow, eval1.FinalDecision)
		c.SetRules(&PermissionRules{Deny: []string{"Bash"}})
		eval2 := c.Evaluate(context.Background(), PermissionHookRequest{ToolName: "Bash"})
		assert.Equal(t, PermissionDeny, eval2.FinalDecision)
	})

	t.Run("Scenario_SameNameDifferentPhasesAreIndependentHooks", func(t *testing.T) {
		// Given a "audit" hook is wanted both as Before observer and
		// After observer,
		// When both are registered with the same name in different phases,
		// Then both register cleanly (composite key is name+phase).
		c := NewPermissionHookChain(nil)
		require.NoError(t, c.Register(PermissionHook{
			Name: "audit", Phase: PermissionHookBeforeEvaluate, Enabled: true,
			Handle: noopHookHandle,
		}))
		require.NoError(t, c.Register(PermissionHook{
			Name: "audit", Phase: PermissionHookAfterEvaluate, Enabled: true,
			Handle: noopHookHandle,
		}))
		assert.Equal(t, 2, c.HookCount())
	})

	t.Run("Scenario_TimestampPreservedFromInjectedClock", func(t *testing.T) {
		// Given a deterministic test needs reproducible audit timestamps,
		// When the chain uses an injected clock,
		// Then EvaluatedAt and each hook decision share the stamp.
		c := NewPermissionHookChain(nil)
		stamp := time.Date(2026, 5, 11, 23, 0, 0, 0, time.UTC)
		c.SetClock(func() time.Time { return stamp })
		c.Register(PermissionHook{
			Name: "x", Phase: PermissionHookBeforeEvaluate, Enabled: true,
			Handle: noopHookHandle,
		})
		eval := c.Evaluate(context.Background(), PermissionHookRequest{ToolName: "Read"})
		assert.Equal(t, stamp, eval.EvaluatedAt)
		assert.Equal(t, stamp, eval.HookDecisions[0].At)
	})

	t.Run("Scenario_RemoveHookRetainsRestOfChain", func(t *testing.T) {
		// Given the chain has 3 hooks and ops decides one is buggy,
		// When that hook is removed,
		// Then the other 2 still run normally.
		c := NewPermissionHookChain(nil)
		c.Register(PermissionHook{Name: "a", Phase: PermissionHookBeforeEvaluate, Enabled: true, Handle: noopHookHandle})
		c.Register(PermissionHook{Name: "b", Phase: PermissionHookBeforeEvaluate, Enabled: true, Handle: noopHookHandle})
		c.Register(PermissionHook{Name: "c", Phase: PermissionHookBeforeEvaluate, Enabled: true, Handle: noopHookHandle})
		require.NoError(t, c.Remove("b", PermissionHookBeforeEvaluate))
		assert.Equal(t, 2, c.HookCount())
		eval := c.Evaluate(context.Background(), PermissionHookRequest{ToolName: "Read"})
		assert.Equal(t, 2, len(eval.HookDecisions))
	})
}

// containsCustomerData is a tiny BDD helper, not production.
func containsCustomerData(s string) bool {
	return len(s) >= 13 && containsLower(s, "customer_data")
}

func containsLower(s, sub string) bool {
	if len(s) < len(sub) {
		return false
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
