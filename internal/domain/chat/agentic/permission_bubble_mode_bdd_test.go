package agentic

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_PermissionBubbleMode(t *testing.T) {
	t.Run("Scenario_SubagentConfirmEscalatesToParentNotUser", func(t *testing.T) {
		// Given a subagent at depth 1 with a Confirm rule for Write,
		// And the parent agent is online,
		// When the subagent tries to Write,
		// Then the request bubbles to parent (not the user) and parent
		// decides.
		parent := StaticParentResolver{Decision: PermissionAllow}
		rules := &PermissionRules{Confirm: []string{"Write"}}
		e, _ := NewBubbleModeEvaluator(rules, parent, 3)
		res, _ := e.Evaluate(context.Background(), BubbleRequest{
			SubagentID: "doc-gen", ParentID: "orchestrator",
			ToolName: "Write", Depth: 1,
		})
		assert.Equal(t, BubbleOutcomeBubbledToParent, res.Outcome)
		assert.Equal(t, "orchestrator", res.ResolvedBy)
	})

	t.Run("Scenario_LocalDenyNeverBubblesPreservingRefusal", func(t *testing.T) {
		// Given the subagent has a local deny for Bash,
		// When it tries Bash,
		// Then the deny fires locally — never escalated (parent cannot
		// silently grant what the subagent's own rules refuse).
		parent := StaticParentResolver{Decision: PermissionAllow}
		rules := &PermissionRules{Deny: []string{"Bash"}}
		e, _ := NewBubbleModeEvaluator(rules, parent, 3)
		res, _ := e.Evaluate(context.Background(), BubbleRequest{
			SubagentID: "sub-1", ParentID: "parent-1",
			ToolName: "Bash", Depth: 1,
		})
		assert.Equal(t, BubbleOutcomeDeniedLocally, res.Outcome)
		assert.Equal(t, "local", res.ResolvedBy)
	})

	t.Run("Scenario_LocalAllowDoesNotBubbleAvoidsParentNoise", func(t *testing.T) {
		// Given the subagent's rules already allow Read,
		// When it uses Read,
		// Then no bubble — parent is not bothered with routine calls.
		parent := StaticParentResolver{Decision: PermissionDeny}
		rules := &PermissionRules{Allow: []string{"Read"}}
		e, _ := NewBubbleModeEvaluator(rules, parent, 3)
		res, _ := e.Evaluate(context.Background(), BubbleRequest{
			SubagentID: "sub-1", ToolName: "Read", Depth: 1,
		})
		assert.Equal(t, BubbleOutcomeHandledLocally, res.Outcome)
		assert.Equal(t, PermissionAllow, res.FinalDecision)
	})

	t.Run("Scenario_MaxDepthGuardAgainstEscalationLoops", func(t *testing.T) {
		// Given the platform sets max bubble depth to 2,
		// And a deeply-nested sub-sub-subagent at depth 3 needs confirm,
		// When it tries to bubble,
		// Then it's auto-denied — runaway escalation impossible.
		parent := StaticParentResolver{Decision: PermissionAllow}
		rules := &PermissionRules{Confirm: []string{"Write"}}
		e, _ := NewBubbleModeEvaluator(rules, parent, 2)
		res, _ := e.Evaluate(context.Background(), BubbleRequest{
			SubagentID: "very-deep", ToolName: "Write", Depth: 3,
		})
		assert.Equal(t, BubbleOutcomeMaxDepthReached, res.Outcome)
		assert.Equal(t, PermissionDeny, res.FinalDecision)
	})

	t.Run("Scenario_ParentCanDenyAfterBubbleRespectingParentPolicy", func(t *testing.T) {
		// Given the subagent bubbles a Write request,
		// And the parent agent's own posture denies Write at runtime,
		// When the parent responds Deny,
		// Then the subagent inherits the deny — parent's authority wins.
		parent := StaticParentResolver{Decision: PermissionDeny}
		rules := &PermissionRules{Confirm: []string{"Write"}}
		e, _ := NewBubbleModeEvaluator(rules, parent, 3)
		res, _ := e.Evaluate(context.Background(), BubbleRequest{
			SubagentID: "sub-1", ParentID: "parent-1",
			ToolName: "Write", Depth: 1,
		})
		assert.Equal(t, PermissionDeny, res.FinalDecision)
		assert.Equal(t, BubbleOutcomeBubbledToParent, res.Outcome)
	})

	t.Run("Scenario_ParentErrorPropagatesUpstreamForOpsToHandle", func(t *testing.T) {
		// Given the parent resolver is temporarily offline,
		// When a subagent bubbles a request,
		// Then the error propagates so the caller (ops/runtime) can
		// decide whether to retry or fail-closed.
		parent := StaticParentResolver{
			Err: assertBubbleErr("parent unreachable"),
		}
		rules := &PermissionRules{Confirm: []string{"Write"}}
		e, _ := NewBubbleModeEvaluator(rules, parent, 3)
		_, err := e.Evaluate(context.Background(), BubbleRequest{
			SubagentID: "sub-1", ParentID: "parent-1",
			ToolName: "Write", Depth: 1,
		})
		assert.Error(t, err)
	})

	t.Run("Scenario_BubbleHistoryRecordedForAudit", func(t *testing.T) {
		// Given compliance asks "what did the parent see during this
		// subagent's confirm decisions?",
		// When the parent uses ChainedParentResolver,
		// Then HistorySnapshot returns every bubbled request.
		parent := &ChainedParentResolver{
			Inner: StaticParentResolver{Decision: PermissionAllow},
		}
		rules := &PermissionRules{Confirm: []string{"Write"}}
		e, _ := NewBubbleModeEvaluator(rules, parent, 3)
		for i := 0; i < 3; i++ {
			_, _ = e.Evaluate(context.Background(), BubbleRequest{
				SubagentID: "sub-1", ParentID: "parent-1",
				ToolName: "Write", Depth: 1,
			})
		}
		history := parent.HistorySnapshot()
		assert.Equal(t, 3, len(history))
	})

	t.Run("Scenario_ConfigurationWithoutParentDisablesBubbling", func(t *testing.T) {
		// Given a leaf agent has maxDepth=0 (no parents to bubble to),
		// When it receives a confirm-tier request,
		// Then it auto-denies (max_depth_reached) — never silently approves.
		rules := &PermissionRules{Confirm: []string{"Write"}}
		e, _ := NewBubbleModeEvaluator(rules, nil, 0)
		res, _ := e.Evaluate(context.Background(), BubbleRequest{
			SubagentID: "leaf", ToolName: "Write",
		})
		assert.Equal(t, BubbleOutcomeMaxDepthReached, res.Outcome)
		assert.Equal(t, PermissionDeny, res.FinalDecision)
	})

	t.Run("Scenario_AuditRetainsBothLocalAndFinalDecision", func(t *testing.T) {
		// Given an auditor wants "what would the subagent have decided
		// without bubble?" AND "what was the final outcome?",
		// When the resolution is captured,
		// Then both LocalDecision (Confirm) and FinalDecision (Allow)
		// are present.
		parent := StaticParentResolver{Decision: PermissionAllow}
		rules := &PermissionRules{Confirm: []string{"Write"}}
		e, _ := NewBubbleModeEvaluator(rules, parent, 3)
		res, _ := e.Evaluate(context.Background(), BubbleRequest{
			SubagentID: "sub-1", ParentID: "parent-1",
			ToolName: "Write", Depth: 1,
		})
		assert.Equal(t, PermissionConfirm, res.LocalDecision)
		assert.Equal(t, PermissionAllow, res.FinalDecision)
	})

	t.Run("Scenario_MultipleSubagentsBubbleIndependently", func(t *testing.T) {
		// Given parallel subagents both bubble at the same time,
		// When evaluations run concurrently,
		// Then each request is independently recorded — no cross-talk,
		// no race.
		parent := &ChainedParentResolver{
			Inner: StaticParentResolver{Decision: PermissionAllow},
		}
		rules := &PermissionRules{Confirm: []string{"Write"}}
		e, _ := NewBubbleModeEvaluator(rules, parent, 3)
		done := make(chan struct{})
		for i := 0; i < 20; i++ {
			go func(id int) {
				_, _ = e.Evaluate(context.Background(), BubbleRequest{
					SubagentID: "sub", ParentID: "parent", ToolName: "Write", Depth: 1,
				})
				done <- struct{}{}
			}(i)
		}
		for i := 0; i < 20; i++ {
			<-done
		}
		assert.Equal(t, 20, len(parent.HistorySnapshot()))
	})

	t.Run("Scenario_PermissionModeBubbleIsValidSixthMode", func(t *testing.T) {
		// Given PermissionMode has 5 existing values, plus plan and now bubble,
		// When the runtime detects PermissionModeBubble,
		// Then it dispatches to the bubble evaluator. Type-level contract.
		rules := &PermissionRules{Mode: PermissionModeBubble}
		assert.Equal(t, PermissionMode("bubble"), rules.Mode)
	})
}

type bubbleAssertErr string

func (e bubbleAssertErr) Error() string  { return string(e) }
func assertBubbleErr(s string) error     { return bubbleAssertErr(s) }
