package agentic

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanMode_IsValidDecision(t *testing.T) {
	for _, d := range allPlanModeDecisions {
		assert.True(t, IsValidPlanModeDecision(d))
	}
	assert.False(t, IsValidPlanModeDecision(PlanModeDecision("nope")))
}

func TestPlanMode_PermissionModePlanIsRegistered(t *testing.T) {
	assert.Equal(t, PermissionMode("plan"), PermissionModePlan)
}

func TestPlanRecord_ValidateEmptyTool(t *testing.T) {
	r := PlanRecord{Sequence: 1}
	assert.ErrorIs(t, r.Validate(), ErrPlanRecordEmptyTool)
}

func TestPlanRecord_ValidateBadSequence(t *testing.T) {
	r := PlanRecord{ToolName: "Bash", Sequence: 0}
	assert.ErrorIs(t, r.Validate(), ErrPlanRecordBadSequence)
}

func TestPlanSession_AddRecordReturnsSequence(t *testing.T) {
	s := NewPlanSession()
	seq, err := s.AddRecord("Bash", "rm -rf", "cleanup")
	require.NoError(t, err)
	assert.Equal(t, 1, seq)
	seq2, _ := s.AddRecord("Write", "/tmp/x", "")
	assert.Equal(t, 2, seq2)
}

func TestPlanSession_AddRecordSealedFails(t *testing.T) {
	s := NewPlanSession()
	_ = s.Approve()
	_, err := s.AddRecord("Bash", "", "")
	assert.ErrorIs(t, err, ErrPlanSessionSealed)
}

func TestPlanSession_AddRecordRejectsEmptyTool(t *testing.T) {
	s := NewPlanSession()
	_, err := s.AddRecord("", "x", "")
	assert.ErrorIs(t, err, ErrPlanRecordEmptyTool)
}

func TestPlanSession_RecordsAreDefensiveCopy(t *testing.T) {
	s := NewPlanSession()
	_, _ = s.AddRecord("Bash", "rm", "")
	r1 := s.Records()
	r1[0].ToolName = "tampered"
	r2 := s.Records()
	assert.Equal(t, "Bash", r2[0].ToolName)
}

func TestPlanSession_SizeReflectsRecords(t *testing.T) {
	s := NewPlanSession()
	for i := 0; i < 5; i++ {
		_, _ = s.AddRecord("Bash", "", "")
	}
	assert.Equal(t, 5, s.Size())
}

func TestPlanSession_ApproveSealsSession(t *testing.T) {
	s := NewPlanSession()
	require.NoError(t, s.Approve())
	assert.True(t, s.IsSealed())
	assert.True(t, s.IsApproved())
}

func TestPlanSession_RejectSealsAsNotApproved(t *testing.T) {
	s := NewPlanSession()
	require.NoError(t, s.Reject())
	assert.True(t, s.IsSealed())
	assert.False(t, s.IsApproved())
}

func TestPlanSession_ApproveAfterRejectFails(t *testing.T) {
	s := NewPlanSession()
	_ = s.Reject()
	err := s.Approve()
	assert.ErrorIs(t, err, ErrPlanSessionAlreadyRejected)
}

func TestPlanSession_RejectAfterApproveFails(t *testing.T) {
	s := NewPlanSession()
	_ = s.Approve()
	err := s.Reject()
	assert.ErrorIs(t, err, ErrPlanSessionAlreadyApproved)
}

func TestPlanSession_ApproveIsIdempotentOnSameState(t *testing.T) {
	s := NewPlanSession()
	require.NoError(t, s.Approve())
	require.NoError(t, s.Approve())
}

func TestPlanSession_RejectIsIdempotentOnSameState(t *testing.T) {
	s := NewPlanSession()
	require.NoError(t, s.Reject())
	require.NoError(t, s.Reject())
}

func TestPlanSession_RenderEmpty(t *testing.T) {
	s := NewPlanSession()
	got := s.Render()
	assert.Contains(t, got, "empty")
}

func TestPlanSession_RenderListsActions(t *testing.T) {
	s := NewPlanSession()
	_, _ = s.AddRecord("Bash", "rm -rf /tmp/x", "cleanup")
	_, _ = s.AddRecord("Write", "/tmp/y", "")
	got := s.Render()
	assert.Contains(t, got, "2 action(s)")
	assert.Contains(t, got, "1. Bash")
	assert.Contains(t, got, "rm -rf /tmp/x")
	assert.Contains(t, got, "cleanup")
	assert.Contains(t, got, "2. Write")
}

func TestPlanSession_ClockInjectableForTimestamps(t *testing.T) {
	s := NewPlanSession()
	stamp := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	s.SetClock(func() time.Time { return stamp })
	_, _ = s.AddRecord("Bash", "", "")
	records := s.Records()
	require.Equal(t, 1, len(records))
	assert.Equal(t, stamp, records[0].RecordedAt)
}

func TestPlanSession_ConcurrentAddIsRaceFree(t *testing.T) {
	s := NewPlanSession()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = s.AddRecord("Bash", "", "")
		}()
	}
	wg.Wait()
	assert.Equal(t, 50, s.Size())
}

func TestDefaultPlanModeClassifier_ReadOnlyTools(t *testing.T) {
	assert.True(t, DefaultPlanModeReadOnlyClassifier("document_search", ""))
	assert.True(t, DefaultPlanModeReadOnlyClassifier("memory_recall", ""))
}

func TestDefaultPlanModeClassifier_MutatingTools(t *testing.T) {
	assert.False(t, DefaultPlanModeReadOnlyClassifier("unknown_tool", ""))
}

func TestEvaluatePlan_NilSessionRejected(t *testing.T) {
	_, err := EvaluatePermissionInPlanMode(nil, nil, nil, "Bash", "", "")
	assert.ErrorIs(t, err, ErrPlanSessionNil)
}

func TestEvaluatePlan_ReadOnlyToolPassesThrough(t *testing.T) {
	s := NewPlanSession()
	d, err := EvaluatePermissionInPlanMode(nil, nil, s, "document_search", "query", "lookup")
	require.NoError(t, err)
	assert.Equal(t, PlanModeAllowReadOnly, d)
	assert.Equal(t, 0, s.Size(), "read-only should not be recorded")
}

func TestEvaluatePlan_MutatingToolRecordedAsPlanned(t *testing.T) {
	s := NewPlanSession()
	d, err := EvaluatePermissionInPlanMode(nil, nil, s, "Bash", "rm -rf /tmp/x", "cleanup")
	require.NoError(t, err)
	assert.Equal(t, PlanModePlanned, d)
	require.Equal(t, 1, s.Size())
	records := s.Records()
	assert.Equal(t, "Bash", records[0].ToolName)
	assert.Equal(t, "rm -rf /tmp/x", records[0].ToolInput)
	assert.Equal(t, "cleanup", records[0].Rationale)
}

func TestEvaluatePlan_DenyRuleStickyEvenInPlanMode(t *testing.T) {
	rules := &PermissionRules{Deny: []string{"Bash"}}
	s := NewPlanSession()
	d, err := EvaluatePermissionInPlanMode(rules, nil, s, "Bash", "ls", "")
	require.NoError(t, err)
	assert.Equal(t, PlanModeDeniedByRule, d)
	assert.Equal(t, 0, s.Size(), "denied tool should not be recorded as planned")
}

func TestEvaluatePlan_CustomClassifierOverridesDefault(t *testing.T) {
	// A skill registers a custom tool as read-only despite default
	// classifier flagging it mutating.
	s := NewPlanSession()
	classifier := func(name, input string) bool {
		return name == "custom_search"
	}
	d, err := EvaluatePermissionInPlanMode(nil, classifier, s, "custom_search", "", "")
	require.NoError(t, err)
	assert.Equal(t, PlanModeAllowReadOnly, d)
}

func TestEvaluatePlan_NilClassifierFallsBackToDefault(t *testing.T) {
	s := NewPlanSession()
	// document_search is read-only per DefaultPlanModeReadOnlyClassifier.
	d, err := EvaluatePermissionInPlanMode(nil, nil, s, "document_search", "", "")
	require.NoError(t, err)
	assert.Equal(t, PlanModeAllowReadOnly, d)
}

func TestEvaluatePlan_RecordIntoSealedSessionFails(t *testing.T) {
	s := NewPlanSession()
	_ = s.Approve()
	_, err := EvaluatePermissionInPlanMode(nil, nil, s, "Bash", "", "")
	assert.ErrorIs(t, err, ErrPlanSessionSealed)
}

func TestSortedRecords_PreservesSequenceOrder(t *testing.T) {
	in := []PlanRecord{
		{ToolName: "C", Sequence: 3},
		{ToolName: "A", Sequence: 1},
		{ToolName: "B", Sequence: 2},
	}
	out := SortedRecords(in)
	assert.Equal(t, "A", out[0].ToolName)
	assert.Equal(t, "B", out[1].ToolName)
	assert.Equal(t, "C", out[2].ToolName)
	// Original untouched.
	assert.Equal(t, "C", in[0].ToolName)
}

func TestPlanSession_RenderFormatStable(t *testing.T) {
	s := NewPlanSession()
	stamp := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	s.SetClock(func() time.Time { return stamp })
	_, _ = s.AddRecord("Bash", "rm", "cleanup")
	r1 := s.Render()
	r2 := s.Render()
	assert.Equal(t, r1, r2)
}

func TestPlanSession_RenderOmitsEmptyInputAndRationale(t *testing.T) {
	s := NewPlanSession()
	_, _ = s.AddRecord("Bash", "", "")
	got := s.Render()
	// Plain tool name, no extra punctuation when input/rationale are empty.
	assert.Contains(t, got, "1. Bash")
	// Must not have stray "()" or " — " markers.
	assert.False(t, strings.Contains(got, "()"))
	assert.False(t, strings.Contains(got, "— "))
}
