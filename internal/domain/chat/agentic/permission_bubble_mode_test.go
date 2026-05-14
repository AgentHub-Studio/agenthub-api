package agentic

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBubble_IsValidOutcome(t *testing.T) {
	for _, o := range allBubbleOutcomes {
		assert.True(t, IsValidBubbleOutcome(o))
	}
	assert.False(t, IsValidBubbleOutcome(BubbleOutcome("nope")))
}

func TestBubble_PermissionModeBubbleRegistered(t *testing.T) {
	assert.Equal(t, PermissionMode("bubble"), PermissionModeBubble)
}

func TestBubble_RequestValidateEmptySubagent(t *testing.T) {
	r := BubbleRequest{ToolName: "Bash"}
	assert.ErrorIs(t, r.Validate(), ErrBubbleRequestEmptySubagent)
}

func TestBubble_RequestValidateEmptyTool(t *testing.T) {
	r := BubbleRequest{SubagentID: "sub-1"}
	assert.ErrorIs(t, r.Validate(), ErrBubbleRequestEmptyTool)
}

func TestBubble_RequestValidateNegativeDepth(t *testing.T) {
	r := BubbleRequest{SubagentID: "sub-1", ToolName: "Bash", Depth: -1}
	assert.ErrorIs(t, r.Validate(), ErrBubbleRequestNegativeDepth)
}

func TestBubble_NewEvaluatorRejectsNegativeMaxDepth(t *testing.T) {
	_, err := NewBubbleModeEvaluator(nil, nil, -1)
	assert.ErrorIs(t, err, ErrBubbleNegativeMaxDepth)
}

func TestBubble_NewEvaluatorRequiresParentWhenMaxDepthPositive(t *testing.T) {
	_, err := NewBubbleModeEvaluator(nil, nil, 1)
	assert.ErrorIs(t, err, ErrBubbleResolverNil)
}

func TestBubble_NewEvaluatorAllowsNilParentWhenMaxDepthZero(t *testing.T) {
	e, err := NewBubbleModeEvaluator(nil, nil, 0)
	require.NoError(t, err)
	assert.NotNil(t, e)
}

func TestBubble_LocalDenyShortCircuits(t *testing.T) {
	parent := StaticParentResolver{Decision: PermissionAllow}
	rules := &PermissionRules{Deny: []string{"Bash"}}
	e, _ := NewBubbleModeEvaluator(rules, parent, 3)
	res, err := e.Evaluate(context.Background(),
		BubbleRequest{SubagentID: "sub-1", ToolName: "Bash"})
	require.NoError(t, err)
	assert.Equal(t, BubbleOutcomeDeniedLocally, res.Outcome)
	assert.Equal(t, PermissionDeny, res.FinalDecision)
	assert.Equal(t, "local", res.ResolvedBy)
}

func TestBubble_LocalAllowShortCircuits(t *testing.T) {
	parent := StaticParentResolver{Decision: PermissionDeny}
	rules := &PermissionRules{Allow: []string{"Read"}}
	e, _ := NewBubbleModeEvaluator(rules, parent, 3)
	res, _ := e.Evaluate(context.Background(),
		BubbleRequest{SubagentID: "sub-1", ToolName: "Read"})
	assert.Equal(t, BubbleOutcomeHandledLocally, res.Outcome)
	assert.Equal(t, PermissionAllow, res.FinalDecision)
	assert.Equal(t, "local", res.ResolvedBy)
}

func TestBubble_ConfirmBubblesToParent(t *testing.T) {
	parent := StaticParentResolver{Decision: PermissionAllow}
	rules := &PermissionRules{Confirm: []string{"Write"}}
	e, _ := NewBubbleModeEvaluator(rules, parent, 3)
	res, err := e.Evaluate(context.Background(), BubbleRequest{
		SubagentID: "sub-1", ParentID: "parent-1",
		ToolName: "Write", Depth: 1,
	})
	require.NoError(t, err)
	assert.Equal(t, BubbleOutcomeBubbledToParent, res.Outcome)
	assert.Equal(t, PermissionAllow, res.FinalDecision)
	assert.Equal(t, "parent-1", res.ResolvedBy)
	assert.Equal(t, PermissionConfirm, res.LocalDecision)
}

func TestBubble_ParentCanStillDenyAfterBubble(t *testing.T) {
	parent := StaticParentResolver{Decision: PermissionDeny}
	rules := &PermissionRules{Confirm: []string{"Write"}}
	e, _ := NewBubbleModeEvaluator(rules, parent, 3)
	res, _ := e.Evaluate(context.Background(), BubbleRequest{
		SubagentID: "sub-1", ParentID: "parent-1",
		ToolName: "Write", Depth: 1,
	})
	assert.Equal(t, BubbleOutcomeBubbledToParent, res.Outcome)
	assert.Equal(t, PermissionDeny, res.FinalDecision)
}

func TestBubble_MaxDepthReachedAutoDenies(t *testing.T) {
	parent := StaticParentResolver{Decision: PermissionAllow}
	rules := &PermissionRules{Confirm: []string{"Write"}}
	e, _ := NewBubbleModeEvaluator(rules, parent, 2)
	res, _ := e.Evaluate(context.Background(), BubbleRequest{
		SubagentID: "deep", ToolName: "Write", Depth: 2,
	})
	assert.Equal(t, BubbleOutcomeMaxDepthReached, res.Outcome)
	assert.Equal(t, PermissionDeny, res.FinalDecision)
	assert.Equal(t, "max_depth", res.ResolvedBy)
}

func TestBubble_MaxDepthZeroNeverBubbles(t *testing.T) {
	rules := &PermissionRules{Confirm: []string{"Write"}}
	e, _ := NewBubbleModeEvaluator(rules, nil, 0)
	res, _ := e.Evaluate(context.Background(), BubbleRequest{
		SubagentID: "sub-1", ToolName: "Write",
	})
	assert.Equal(t, BubbleOutcomeMaxDepthReached, res.Outcome)
	assert.Equal(t, PermissionDeny, res.FinalDecision)
}

func TestBubble_ParentErrorPropagates(t *testing.T) {
	parent := StaticParentResolver{Err: errors.New("parent offline")}
	rules := &PermissionRules{Confirm: []string{"Write"}}
	e, _ := NewBubbleModeEvaluator(rules, parent, 3)
	_, err := e.Evaluate(context.Background(), BubbleRequest{
		SubagentID: "sub-1", ParentID: "parent-1",
		ToolName: "Write", Depth: 1,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parent offline")
}

func TestBubble_DefaultAllowWhenNoRulesShortCircuits(t *testing.T) {
	// EvaluatePermission returns Allow when rules is nil. Bubble mode
	// then handles locally without invoking parent.
	parent := StaticParentResolver{Decision: PermissionDeny}
	e, _ := NewBubbleModeEvaluator(nil, parent, 3)
	res, _ := e.Evaluate(context.Background(), BubbleRequest{
		SubagentID: "sub-1", ParentID: "parent-1",
		ToolName: "anything", Depth: 1,
	})
	assert.Equal(t, BubbleOutcomeHandledLocally, res.Outcome)
	assert.Equal(t, PermissionAllow, res.FinalDecision)
}

func TestBubble_RecordsBubbleChain(t *testing.T) {
	parent := &ChainedParentResolver{
		Inner: StaticParentResolver{Decision: PermissionAllow},
	}
	rules := &PermissionRules{Confirm: []string{"Write"}}
	e, _ := NewBubbleModeEvaluator(rules, parent, 3)
	_, _ = e.Evaluate(context.Background(), BubbleRequest{
		SubagentID: "sub-1", ParentID: "parent-1",
		ToolName: "Write", Depth: 1,
	})
	history := parent.HistorySnapshot()
	require.Equal(t, 1, len(history))
	assert.Equal(t, "sub-1", history[0].SubagentID)
}

func TestBubble_HistorySnapshotIsDefensiveCopy(t *testing.T) {
	parent := &ChainedParentResolver{}
	_, _ = parent.Resolve(context.Background(), BubbleRequest{
		SubagentID: "sub-1", ToolName: "Bash",
	})
	h1 := parent.HistorySnapshot()
	h1[0].SubagentID = "tampered"
	h2 := parent.HistorySnapshot()
	assert.Equal(t, "sub-1", h2[0].SubagentID)
}

func TestBubble_BubbledAtFilledFromClockWhenZero(t *testing.T) {
	stamp := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	parent := StaticParentResolver{Decision: PermissionAllow}
	rules := &PermissionRules{Confirm: []string{"Write"}}
	e, _ := NewBubbleModeEvaluator(rules, parent, 3)
	e.SetClock(func() time.Time { return stamp })
	res, _ := e.Evaluate(context.Background(), BubbleRequest{
		SubagentID: "sub-1", ParentID: "parent-1",
		ToolName: "Write", Depth: 1,
	})
	assert.Equal(t, stamp, res.Request.BubbledAt)
}

func TestBubble_BubbledAtPreservedIfSet(t *testing.T) {
	provided := time.Date(2026, 5, 12, 8, 0, 0, 0, time.UTC)
	stamp := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	parent := StaticParentResolver{Decision: PermissionAllow}
	rules := &PermissionRules{Confirm: []string{"Write"}}
	e, _ := NewBubbleModeEvaluator(rules, parent, 3)
	e.SetClock(func() time.Time { return stamp })
	res, _ := e.Evaluate(context.Background(), BubbleRequest{
		SubagentID: "sub-1", ParentID: "parent-1",
		ToolName: "Write", Depth: 1, BubbledAt: provided,
	})
	assert.Equal(t, provided, res.Request.BubbledAt)
}

func TestBubble_ResolutionCarriesBubbledThroughChain(t *testing.T) {
	parent := StaticParentResolver{Decision: PermissionAllow}
	rules := &PermissionRules{Confirm: []string{"Write"}}
	e, _ := NewBubbleModeEvaluator(rules, parent, 3)
	res, _ := e.Evaluate(context.Background(), BubbleRequest{
		SubagentID: "sub-1", ParentID: "parent-1",
		ToolName: "Write", Depth: 1,
	})
	assert.Equal(t, []string{"parent-1"}, res.BubbledThrough)
}

func TestBubble_SortedResolutionsByMultipleKeys(t *testing.T) {
	t1 := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)
	in := []BubbleResolution{
		{Request: BubbleRequest{SubagentID: "z", ToolName: "B", BubbledAt: t2}},
		{Request: BubbleRequest{SubagentID: "a", ToolName: "B", BubbledAt: t1}},
		{Request: BubbleRequest{SubagentID: "a", ToolName: "A", BubbledAt: t2}},
	}
	out := SortedBubbleResolutions(in)
	assert.Equal(t, "a", out[0].Request.SubagentID)
	assert.Equal(t, "A", out[0].Request.ToolName)
	assert.Equal(t, "a", out[1].Request.SubagentID)
	assert.Equal(t, "B", out[1].Request.ToolName)
	assert.Equal(t, "z", out[2].Request.SubagentID)
}

func TestBubble_SortedResolutionsDoesNotMutateInput(t *testing.T) {
	in := []BubbleResolution{
		{Request: BubbleRequest{SubagentID: "z", ToolName: "B"}},
		{Request: BubbleRequest{SubagentID: "a", ToolName: "A"}},
	}
	_ = SortedBubbleResolutions(in)
	assert.Equal(t, "z", in[0].Request.SubagentID)
}

func TestBubble_ConcurrentEvaluateIsRaceFree(t *testing.T) {
	parent := &ChainedParentResolver{
		Inner: StaticParentResolver{Decision: PermissionAllow},
	}
	rules := &PermissionRules{Confirm: []string{"Write"}}
	e, _ := NewBubbleModeEvaluator(rules, parent, 3)
	done := make(chan struct{})
	for i := 0; i < 50; i++ {
		go func() {
			_, _ = e.Evaluate(context.Background(), BubbleRequest{
				SubagentID: "sub-1", ParentID: "parent-1",
				ToolName: "Write", Depth: 1,
			})
			done <- struct{}{}
		}()
	}
	for i := 0; i < 50; i++ {
		<-done
	}
	assert.Equal(t, 50, len(parent.HistorySnapshot()))
}

func TestBubble_ResolutionReasonPopulated(t *testing.T) {
	parent := StaticParentResolver{Decision: PermissionAllow}
	rules := &PermissionRules{Confirm: []string{"Write"}}
	e, _ := NewBubbleModeEvaluator(rules, parent, 3)
	res, _ := e.Evaluate(context.Background(), BubbleRequest{
		SubagentID: "sub-1", ParentID: "parent-1",
		ToolName: "Write", Depth: 1,
	})
	assert.NotEmpty(t, res.Reason)
	assert.Contains(t, res.Reason, "parent-1")
}

func TestBubble_LocalDecisionPreservedInResolution(t *testing.T) {
	parent := StaticParentResolver{Decision: PermissionAllow}
	rules := &PermissionRules{Confirm: []string{"Write"}}
	e, _ := NewBubbleModeEvaluator(rules, parent, 3)
	res, _ := e.Evaluate(context.Background(), BubbleRequest{
		SubagentID: "sub-1", ParentID: "parent-1",
		ToolName: "Write", Depth: 1,
	})
	// LocalDecision captures the engine's first-pass before bubble
	// (audit: "what would have happened without bubble?").
	assert.Equal(t, PermissionConfirm, res.LocalDecision)
	assert.Equal(t, PermissionAllow, res.FinalDecision)
}
