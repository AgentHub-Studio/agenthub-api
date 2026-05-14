package agentic

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GOV-003 — Human control checkpoints BDD.
//
// PDF arXiv:2604.14228v1 Section 11 — "human-in-the-loop checkpoints
// for destructive operations, large purchases, irreversible actions".
// Section 5.3 — permission_request first-class safety hook.
//
// These scenarios validate the typed checkpoint contract: bounded kinds,
// first-write-wins resolution, deadline-driven timeouts, audit-friendly
// records, multi-tenant isolation enforced at Arm time.

func TestBDD_HumanControlCheckpoints(t *testing.T) {

	t.Run("Scenario_RunnerArmsCheckpointBeforeDestructiveOperation", func(t *testing.T) {
		// Given the agent is about to DROP TABLE,
		// When the runner arms a checkpoint,
		// Then the gate accepts it and assigns ID + ArmedAt — the
		//      audit trail begins immediately.
		gate := NewInMemoryCheckpointGate()
		armed, err := gate.Arm(context.Background(), Checkpoint{
			Kind:     CheckpointPreDestructive,
			TenantID: "tenant-prod",
			AgentID:  "ops-agent",
			RunID:    "run-42",
			Reason:   "DROP TABLE users",
			Context:  map[string]string{"sql": "DROP TABLE users"},
		})
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, armed.ID,
			"Arm must assign a stable ID for audit correlation")
		assert.False(t, armed.ArmedAt.IsZero())
		assert.Equal(t, "DROP TABLE users", armed.Reason)
	})

	t.Run("Scenario_BoundedKindsPreventTypoDriftAcrossDeploys", func(t *testing.T) {
		// Given analytics aggregate by kind (PDF Section 11 — stable
		//       audit categories),
		// When a typo'd kind is supplied at arm time,
		// Then the gate REJECTS — caller bug surfaces immediately, not
		//      6 months later when the dashboard breaks.
		gate := NewInMemoryCheckpointGate()
		_, err := gate.Arm(context.Background(), Checkpoint{
			Kind:     CheckpointKind("pre-destruct"), // hyphen, not underscore
			TenantID: "t",
		})
		assert.Error(t, err, "typo'd kind must be rejected at arm time")
	})

	t.Run("Scenario_TenantIDIsRequiredAtArmTime", func(t *testing.T) {
		// Given multi-tenancy is the platform's primary security
		//       boundary (CLAUDE.md),
		// When the runner forgets to set TenantID,
		// Then Arm REJECTS — silent global checkpoint would breach.
		gate := NewInMemoryCheckpointGate()
		_, err := gate.Arm(context.Background(), Checkpoint{
			Kind: CheckpointPreDestructive,
		})
		assert.Error(t, err)
	})

	t.Run("Scenario_FirstDecisionWinsIsAuditGuarantee", func(t *testing.T) {
		// Given audit requires that once a human says "approved", a
		//       later "rejected" cannot rewrite history,
		gate := NewInMemoryCheckpointGate()
		armed, _ := gate.Arm(context.Background(), Checkpoint{
			Kind: CheckpointPreDestructive, TenantID: "t",
		})
		require.NoError(t, gate.Resolve(context.Background(), CheckpointResolution{
			CheckpointID: armed.ID,
			Decision:     CheckpointApproved,
			DecidedBy:    "alice",
		}))

		// When a second resolve attempts to flip,
		err := gate.Resolve(context.Background(), CheckpointResolution{
			CheckpointID: armed.ID,
			Decision:     CheckpointRejected,
			DecidedBy:    "mallory",
		})

		// Then it errors — no rewrite.
		assert.Error(t, err, "second resolve must error — first wins")

		_, res, _ := gate.Find(context.Background(), armed.ID)
		require.NotNil(t, res)
		assert.Equal(t, "alice", res.DecidedBy,
			"audit shows the human who actually decided")
	})

	t.Run("Scenario_AwaitBlocksUntilHumanApproves", func(t *testing.T) {
		// Given the runner is paused at a checkpoint,
		gate := NewInMemoryCheckpointGate()
		armed, _ := gate.Arm(context.Background(), Checkpoint{
			Kind: CheckpointPreIrreversible, TenantID: "t", RunID: "r",
		})

		approved := make(chan CheckpointResolution, 1)
		go func() {
			res, err := gate.Await(context.Background(), armed.ID)
			if err == nil {
				approved <- res
			}
		}()

		// When the human resolves a moment later,
		time.Sleep(15 * time.Millisecond)
		require.NoError(t, gate.Resolve(context.Background(), CheckpointResolution{
			CheckpointID: armed.ID,
			Decision:     CheckpointApproved,
			DecidedBy:    "ops-lead",
		}))

		// Then the awaiting runner unblocks with the approval.
		select {
		case res := <-approved:
			assert.Equal(t, CheckpointApproved, res.Decision)
			assert.Equal(t, "ops-lead", res.DecidedBy)
		case <-time.After(time.Second):
			t.Fatal("Await did not return after Resolve within 1s")
		}
	})

	t.Run("Scenario_DeadlineDrivenTimeoutPreventsZombieRuns", func(t *testing.T) {
		// Given a checkpoint with a 50ms deadline (no human responds),
		gate := NewInMemoryCheckpointGate()
		cp := Checkpoint{
			Kind:     CheckpointPreCostThreshold,
			TenantID: "t",
			Deadline: time.Now().Add(50 * time.Millisecond),
		}
		armed, _ := gate.Arm(context.Background(), cp)

		// When the runner Awaits,
		res, err := gate.Await(context.Background(), armed.ID)
		require.NoError(t, err)

		// Then it times out — no zombie run waiting forever.
		assert.Equal(t, CheckpointTimedOut, res.Decision)
	})

	t.Run("Scenario_TimeoutResolutionIsPersistedForAudit", func(t *testing.T) {
		// Given a timed-out checkpoint,
		gate := NewInMemoryCheckpointGate()
		cp := Checkpoint{
			Kind:     CheckpointPreDestructive,
			TenantID: "t",
			Deadline: time.Now().Add(20 * time.Millisecond),
		}
		armed, _ := gate.Arm(context.Background(), cp)
		_, _ = gate.Await(context.Background(), armed.ID)

		// When Find is called after the timeout,
		_, res, err := gate.Find(context.Background(), armed.ID)
		require.NoError(t, err)
		require.NotNil(t, res)

		// Then the timed-out decision is recorded permanently.
		assert.Equal(t, CheckpointTimedOut, res.Decision,
			"timeout must persist so audit dashboards can count it")
	})

	t.Run("Scenario_RunnerCancellationFreesAwaiterWithoutResolving", func(t *testing.T) {
		// Given the parent run is cancelled,
		gate := NewInMemoryCheckpointGate()
		armed, _ := gate.Arm(context.Background(), Checkpoint{
			Kind: CheckpointPreDestructive, TenantID: "t",
		})

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() {
			_, err := gate.Await(ctx, armed.ID)
			done <- err
		}()

		// When the runner cancels,
		time.Sleep(10 * time.Millisecond)
		cancel()

		// Then Await returns an error AND the checkpoint stays
		//      unresolved — a separate Resolve can still arrive later.
		select {
		case err := <-done:
			assert.Error(t, err, "Await must error on cancellation")
		case <-time.After(time.Second):
			t.Fatal("Await did not unblock after cancel")
		}
		_, res, _ := gate.Find(context.Background(), armed.ID)
		assert.Nil(t, res, "checkpoint must remain unresolved after Await cancel")
	})

	t.Run("Scenario_AllSixCheckpointKindsHaveStableStrings", func(t *testing.T) {
		// Given dashboards / audit reports bind to kind strings,
		// When the bounded set is inspected,
		// Then the 6 expected values are present and round-trip stable.
		expected := map[string]bool{
			"pre_destructive":               true,
			"pre_irreversible":              true,
			"pre_external_send":             true,
			"pre_pii_export":                true,
			"pre_cost_threshold":            true,
			"pre_policy_requires_approval":  true,
		}
		for _, k := range AllCheckpointKinds() {
			assert.True(t, expected[string(k)],
				"kind %q not in expected stable set — wire contract changed", k)
		}
		assert.Equal(t, len(expected), len(AllCheckpointKinds()))
	})

	t.Run("Scenario_CheckpointEnvelopeIsJSONStableForUIWire", func(t *testing.T) {
		// Given the approval UI consumes Checkpoint JSON,
		gate := NewInMemoryCheckpointGate()
		armed, _ := gate.Arm(context.Background(), Checkpoint{
			Kind:     CheckpointPrePIIExport,
			TenantID: "t",
			AgentID:  "a",
			RunID:    "r",
			Reason:   "exporting customer table",
			Context:  map[string]string{"row_count": "10000"},
		})
		raw, err := json.Marshal(armed)
		require.NoError(t, err)
		s := string(raw)
		// Field names are the wire contract.
		assert.Contains(t, s, `"ID"`)
		assert.Contains(t, s, `"Kind"`)
		assert.Contains(t, s, `"TenantID"`)
		assert.Contains(t, s, `"AgentID"`)
		assert.Contains(t, s, `"RunID"`)
		assert.Contains(t, s, `"Reason"`)
		assert.Contains(t, s, `"Context"`)
		assert.Contains(t, s, `"ArmedAt"`)
	})

	t.Run("Scenario_DecisionEnumIsBoundedFor4Outcomes", func(t *testing.T) {
		// Given audit / metrics consume bounded decision values,
		valid := []CheckpointDecision{
			CheckpointApproved, CheckpointRejected,
			CheckpointCancelled, CheckpointTimedOut,
		}
		for _, d := range valid {
			assert.True(t, IsTerminalCheckpointDecision(d))
		}
		assert.False(t, IsTerminalCheckpointDecision(CheckpointDecision("partial")))
	})

	t.Run("Scenario_BridgesPolicyRequireApprovalToCheckpointWorkflow", func(t *testing.T) {
		// Given GOV-002 returned PolicyRequireApproval (this is the
		//       conceptual handoff between policy and checkpoint),
		// When the runner translates to a checkpoint,
		// Then it uses CheckpointPrePolicyRequiresApproval kind. This
		//      makes the dashboard "policy-driven approvals" filterable
		//      from "destructive" / "PII" / etc.
		assert.True(t, IsValidCheckpointKind(CheckpointPrePolicyRequiresApproval),
			"the bridging kind must exist so policy approvals classify cleanly")
	})

	t.Run("Scenario_AuditableContextSurvivesArmToFind", func(t *testing.T) {
		// Given the audit log needs the context map at decision time,
		gate := NewInMemoryCheckpointGate()
		armed, _ := gate.Arm(context.Background(), Checkpoint{
			Kind:     CheckpointPreExternalSend,
			TenantID: "t",
			Context:  map[string]string{"recipient": "ops@external.com"},
		})

		cp, _, err := gate.Find(context.Background(), armed.ID)
		require.NoError(t, err)
		assert.Equal(t, "ops@external.com", cp.Context["recipient"],
			"context map must round-trip exact for audit clarity")
	})
}
