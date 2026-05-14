package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBDD_PermissionPlanMode(t *testing.T) {
	t.Run("Scenario_LLMProposesPlanWithoutExecutingMutation", func(t *testing.T) {
		// Given a session is in plan mode,
		// And the LLM proposes "Bash rm -rf /tmp/x" to clean a workspace,
		// When the harness evaluates,
		// Then the call is recorded as planned (NOT executed) and the
		// LLM can continue planning the next step.
		s := NewPlanSession()
		d, err := EvaluatePermissionInPlanMode(nil, nil, s, "Bash", "rm -rf /tmp/x", "cleanup")
		require.NoError(t, err)
		assert.Equal(t, PlanModePlanned, d)
		assert.Equal(t, 1, s.Size())
	})

	t.Run("Scenario_ReadOnlyToolsExecuteNormallyInPlanMode", func(t *testing.T) {
		// Given the LLM uses document_search to gather context for the plan,
		// When the harness evaluates,
		// Then read-only execution is allowed — the agent can still
		// reason with current data.
		s := NewPlanSession()
		d, _ := EvaluatePermissionInPlanMode(nil, nil, s, "document_search", "Q", "")
		assert.Equal(t, PlanModeAllowReadOnly, d)
		assert.Equal(t, 0, s.Size())
	})

	t.Run("Scenario_DenyRuleStillFiresInPlanMode", func(t *testing.T) {
		// Given the tenant denies Bash entirely,
		// And the LLM proposes Bash in plan mode,
		// When the harness evaluates,
		// Then it returns denied_by_rule — plan mode does NOT relax denies.
		rules := &PermissionRules{Deny: []string{"Bash"}}
		s := NewPlanSession()
		d, _ := EvaluatePermissionInPlanMode(rules, nil, s, "Bash", "ls", "")
		assert.Equal(t, PlanModeDeniedByRule, d)
		assert.Equal(t, 0, s.Size())
	})

	t.Run("Scenario_UserApprovesPlanAsABatch", func(t *testing.T) {
		// Given the LLM has proposed 3 mutating actions,
		// When the user approves the plan,
		// Then the session is sealed approved and no more records
		// can be added.
		s := NewPlanSession()
		for i := 0; i < 3; i++ {
			_, _ = EvaluatePermissionInPlanMode(nil, nil, s, "Bash", "", "")
		}
		require.NoError(t, s.Approve())
		assert.True(t, s.IsApproved())
		_, err := s.AddRecord("Bash", "", "")
		assert.ErrorIs(t, err, ErrPlanSessionSealed)
	})

	t.Run("Scenario_UserRejectsPlanCancelsAllPlannedActions", func(t *testing.T) {
		// Given the LLM proposed a destructive plan,
		// When the user rejects it,
		// Then the session is sealed rejected and the runtime knows
		// not to execute any record.
		s := NewPlanSession()
		_, _ = EvaluatePermissionInPlanMode(nil, nil, s, "Bash", "rm -rf /", "danger")
		require.NoError(t, s.Reject())
		assert.False(t, s.IsApproved())
		assert.True(t, s.IsSealed())
	})

	t.Run("Scenario_ApproveAfterRejectFailsForAuditConsistency", func(t *testing.T) {
		// Given the user rejected the plan,
		// When code tries to flip approval (race / bug),
		// Then the second call fails — audit trail must not flip-flop.
		s := NewPlanSession()
		_ = s.Reject()
		err := s.Approve()
		assert.ErrorIs(t, err, ErrPlanSessionAlreadyRejected)
	})

	t.Run("Scenario_RenderProducesHumanReadableApprovalPrompt", func(t *testing.T) {
		// Given the user needs to read the plan before approving,
		// When Render is called,
		// Then it emits a numbered list of proposed actions with their
		// inputs and rationale.
		s := NewPlanSession()
		_, _ = EvaluatePermissionInPlanMode(nil, nil, s, "Bash", "rm /tmp/x", "cleanup")
		_, _ = EvaluatePermissionInPlanMode(nil, nil, s, "Write", "/tmp/y", "create marker")
		got := s.Render()
		assert.Contains(t, got, "2 action(s)")
		assert.Contains(t, got, "1. Bash")
		assert.Contains(t, got, "cleanup")
		assert.Contains(t, got, "2. Write")
	})

	t.Run("Scenario_CustomClassifierLetsSkillsExtendReadOnlySet", func(t *testing.T) {
		// Given a skill registers `kb_query` as a read-only tool that
		// the default classifier doesn't know about,
		// When the runtime supplies a custom classifier,
		// Then kb_query executes immediately without being planned.
		s := NewPlanSession()
		classifier := func(name, input string) bool { return name == "kb_query" }
		d, _ := EvaluatePermissionInPlanMode(nil, classifier, s, "kb_query", "", "")
		assert.Equal(t, PlanModeAllowReadOnly, d)
	})

	t.Run("Scenario_PlanModeIntegratesWithExistingPermissionMode", func(t *testing.T) {
		// Given the tenant's PermissionRules.Mode could be set to plan,
		// When the runtime detects PermissionModePlan,
		// Then it dispatches to EvaluatePermissionInPlanMode instead of
		// the default evaluator. This BDD demonstrates the dispatch
		// contract: PermissionModePlan is a valid PermissionMode value.
		rules := &PermissionRules{Mode: PermissionModePlan}
		assert.Equal(t, PermissionMode("plan"), rules.Mode)
	})

	t.Run("Scenario_ConcurrentLLMTurnsAccumulateIntoSamePlan", func(t *testing.T) {
		// Given parallel sub-agents propose actions into the same plan
		// session (rare but possible in multi-step orchestration),
		// When 50 goroutines record simultaneously,
		// Then every record lands with a unique sequence number — no
		// data races, no duplicates.
		s := NewPlanSession()
		done := make(chan struct{})
		for i := 0; i < 50; i++ {
			go func() {
				_, _ = s.AddRecord("Bash", "", "")
				done <- struct{}{}
			}()
		}
		for i := 0; i < 50; i++ {
			<-done
		}
		records := s.Records()
		seen := map[int]bool{}
		for _, r := range records {
			assert.False(t, seen[r.Sequence], "duplicate sequence %d", r.Sequence)
			seen[r.Sequence] = true
		}
		assert.Equal(t, 50, len(seen))
	})

	t.Run("Scenario_PlanModeRecordsFeedPERM010Audit", func(t *testing.T) {
		// Given the plan is approved and the runtime executes the
		// planned actions,
		// When each execution completes,
		// Then PERM-010 PermissionAuditEntry records can be derived
		// from PlanRecord (tool name + truncated input).
		// This is a type-level contract: PlanRecord carries the fields
		// PermissionAuditEntry needs.
		s := NewPlanSession()
		_, _ = EvaluatePermissionInPlanMode(nil, nil, s, "Bash", "rm /tmp/x", "cleanup")
		records := s.Records()
		require.Equal(t, 1, len(records))
		// Demonstrate the mapping (no runtime wiring yet, just type compatibility).
		entry := PermissionAuditEntry{
			ToolName:     records[0].ToolName,
			Decision:     AuditDecisionAllow, // approved plan → allow on execution
			InputSnippet: records[0].ToolInput,
			CreatedAt:    records[0].RecordedAt,
		}
		assert.Equal(t, "Bash", entry.ToolName)
		assert.Equal(t, "rm /tmp/x", entry.InputSnippet)
	})
}
