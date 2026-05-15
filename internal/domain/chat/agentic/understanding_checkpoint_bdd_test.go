package agentic

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// HUMAN-004 — Understanding checkpoints BDD.
//
// PDF arXiv:2604.14228v1 §11 (when ambiguous/expensive, agent should
// PAUSE and present its understanding for human confirmation BEFORE acting).
//
// Distinguishes from GOV-003 (policy-driven approval): this is comprehension-
// driven verification, not gating.

func TestBDD_UnderstandingCheckpoints(t *testing.T) {

	t.Run("Scenario_AgentPausesToConfirmInterpretationBeforeExpensiveAction", func(t *testing.T) {
		// Given a user asks "send Q4 invoices to finance" — ambiguous
		//       (which Q4? which finance? which format?),
		// When the agent arms an understanding checkpoint,
		// Then status=pending + task summary + assumptions + next actions
		//      reach the human BEFORE any tool runs.
		gate := NewInMemoryUnderstandingCheckpointGate()
		armed, err := gate.Arm(context.Background(), UnderstandingCheckpoint{
			TenantID:    "tenant-acme", AgentID: "agent-finance", RunID: "run-7",
			TaskSummary: "Generate Q4 2026 invoice CSV for finance@tenant-acme.com",
			KeyAssumptions: []string{
				"Q4 = Oct-Dec 2026 (calendar year)",
				"Currency = EUR (tenant default)",
				"Include only paid invoices",
			},
			NextActions: []string{
				"Query ah_finance.invoices WHERE status='paid' AND quarter='Q4-2026'",
				"Format as CSV (UTF-8 BOM, semicolon-delimited for Excel)",
				"Email to finance@tenant-acme.com",
			},
		})
		require.NoError(t, err)
		assert.Equal(t, UnderstandingStatusPending, armed.Status)
		assert.Len(t, armed.KeyAssumptions, 3)
		assert.Len(t, armed.NextActions, 3)
	})

	t.Run("Scenario_HumanCanCorrectMisunderstandingBeforeAgentActs", func(t *testing.T) {
		// Given the agent's interpretation is wrong (Q4 actually means
		//       fiscal year Q4 not calendar),
		// When the human responds with corrections,
		// Then status=corrected + corrections list reach the agent
		//      so it can re-plan.
		gate := NewInMemoryUnderstandingCheckpointGate()
		armed, _ := gate.Arm(context.Background(), validUnderstanding())
		require.NoError(t, gate.Respond(context.Background(), UnderstandingResponse{
			CheckpointID: armed.ID,
			Status:       UnderstandingStatusCorrected,
			Corrections: []string{
				"Q4 means fiscal year (Jul-Sep), not calendar year",
				"Use USD not EUR for this customer",
			},
			RespondedBy: "alice@tenant.com",
		}))

		_, res, _ := gate.Find(context.Background(), armed.ID)
		require.NotNil(t, res)
		assert.Equal(t, UnderstandingStatusCorrected, res.Status)
		assert.Len(t, res.Corrections, 2)
	})

	t.Run("Scenario_HumanCanAbandonAmbiguousTaskEntirely", func(t *testing.T) {
		// Given the agent's interpretation makes the human realize
		//       the task itself is wrong,
		gate := NewInMemoryUnderstandingCheckpointGate()
		armed, _ := gate.Arm(context.Background(), validUnderstanding())
		require.NoError(t, gate.Respond(context.Background(), UnderstandingResponse{
			CheckpointID: armed.ID,
			Status:       UnderstandingStatusAbandoned,
			Notes:        "Reconsidering whether to do this at all",
		}))
		_, res, _ := gate.Find(context.Background(), armed.ID)
		require.NotNil(t, res)
		assert.Equal(t, UnderstandingStatusAbandoned, res.Status)
	})

	t.Run("Scenario_FirstResponseWinsAuditGuarantee", func(t *testing.T) {
		// Given audit requires "once human said X, can't be re-flipped",
		gate := NewInMemoryUnderstandingCheckpointGate()
		armed, _ := gate.Arm(context.Background(), validUnderstanding())
		require.NoError(t, gate.Respond(context.Background(), UnderstandingResponse{
			CheckpointID: armed.ID, Status: UnderstandingStatusConfirmed,
		}))
		err := gate.Respond(context.Background(), UnderstandingResponse{
			CheckpointID: armed.ID, Status: UnderstandingStatusAbandoned,
		})
		assert.Error(t, err)
	})

	t.Run("Scenario_DeadlineDrivenTimeoutPreventsZombieRuns", func(t *testing.T) {
		// Given the human is unreachable for 50ms (test scale),
		gate := NewInMemoryUnderstandingCheckpointGate()
		cp := validUnderstanding()
		cp.ConfirmationDeadline = time.Now().Add(50 * time.Millisecond)
		armed, _ := gate.Arm(context.Background(), cp)
		res, err := gate.Await(context.Background(), armed.ID)
		require.NoError(t, err)
		assert.Equal(t, UnderstandingStatusTimedOut, res.Status,
			"agent unblocks via timeout — never zombies")
	})

	t.Run("Scenario_TimeoutResolutionIsPersistedForAudit", func(t *testing.T) {
		// Given a timed-out checkpoint,
		gate := NewInMemoryUnderstandingCheckpointGate()
		cp := validUnderstanding()
		cp.ConfirmationDeadline = time.Now().Add(20 * time.Millisecond)
		armed, _ := gate.Arm(context.Background(), cp)
		_, _ = gate.Await(context.Background(), armed.ID)

		_, res, _ := gate.Find(context.Background(), armed.ID)
		require.NotNil(t, res)
		assert.Equal(t, UnderstandingStatusTimedOut, res.Status,
			"timeout persists — dashboards count it")
	})

	t.Run("Scenario_FiveStatusesCoverPendingAndFourTerminals", func(t *testing.T) {
		// Given audit dashboards aggregate by status,
		expected := map[string]bool{
			"pending": true, "confirmed": true, "corrected": true,
			"abandoned": true, "timed_out": true,
		}
		for _, s := range AllUnderstandingStatuses() {
			assert.True(t, expected[string(s)])
		}
		assert.Equal(t, 5, len(AllUnderstandingStatuses()))
	})

	t.Run("Scenario_PendingIsTheOnlyNonTerminalStatus", func(t *testing.T) {
		// Given response status MUST be terminal (pending isn't a reply),
		assert.False(t, IsTerminalUnderstandingStatus(UnderstandingStatusPending))
		for _, s := range []UnderstandingStatus{
			UnderstandingStatusConfirmed, UnderstandingStatusCorrected,
			UnderstandingStatusAbandoned, UnderstandingStatusTimedOut,
		} {
			assert.True(t, IsTerminalUnderstandingStatus(s))
		}
	})

	t.Run("Scenario_RespondRejectsPendingAsResponseStatus", func(t *testing.T) {
		// Given pending isn't a valid reply — that would loop forever,
		gate := NewInMemoryUnderstandingCheckpointGate()
		armed, _ := gate.Arm(context.Background(), validUnderstanding())
		err := gate.Respond(context.Background(), UnderstandingResponse{
			CheckpointID: armed.ID, Status: UnderstandingStatusPending,
		})
		assert.Error(t, err, "pending response is a contract violation")
	})

	t.Run("Scenario_RequiredFieldsEnforceMeaningfulCheckpoint", func(t *testing.T) {
		// Given empty TenantID would breach isolation; empty TaskSummary
		//       would mean "what am I confirming?",
		gate := NewInMemoryUnderstandingCheckpointGate()

		noTenant := validUnderstanding()
		noTenant.TenantID = ""
		_, err := gate.Arm(context.Background(), noTenant)
		assert.Error(t, err)

		noSummary := validUnderstanding()
		noSummary.TaskSummary = ""
		_, err = gate.Arm(context.Background(), noSummary)
		assert.Error(t, err)
	})

	t.Run("Scenario_LongTaskSummariesAreBoundedToPreventLogSpam", func(t *testing.T) {
		gate := NewInMemoryUnderstandingCheckpointGate()
		cp := validUnderstanding()
		cp.TaskSummary = strings.Repeat("x", 500)
		armed, _ := gate.Arm(context.Background(), cp)
		assert.Equal(t, 300, len(armed.TaskSummary))
	})

	t.Run("Scenario_MarkdownShowsCallToActionOnlyWhilePending", func(t *testing.T) {
		// Given UI renders different states differently,
		pending := validUnderstanding()
		pending.Status = UnderstandingStatusPending
		assert.Contains(t, pending.Markdown(), "Please confirm")

		confirmed := validUnderstanding()
		confirmed.Status = UnderstandingStatusConfirmed
		assert.NotContains(t, confirmed.Markdown(), "Please confirm",
			"resolved checkpoints don't beg for action")
	})

	t.Run("Scenario_HistogramByStatusEnablesDashboards", func(t *testing.T) {
		gate := NewInMemoryUnderstandingCheckpointGate()
		armed1, _ := gate.Arm(context.Background(), validUnderstanding())
		_ = gate.Respond(context.Background(), UnderstandingResponse{
			CheckpointID: armed1.ID, Status: UnderstandingStatusConfirmed,
		})
		armed2, _ := gate.Arm(context.Background(), validUnderstanding())
		_ = gate.Respond(context.Background(), UnderstandingResponse{
			CheckpointID: armed2.ID, Status: UnderstandingStatusCorrected,
		})

		hist, _ := gate.CountByStatus(context.Background(), "tenant-x")
		// Stable axes including unused statuses with 0:
		assert.Equal(t, 1, hist[UnderstandingStatusConfirmed])
		assert.Equal(t, 1, hist[UnderstandingStatusCorrected])
		assert.Equal(t, 0, hist[UnderstandingStatusAbandoned])
		assert.Equal(t, 0, hist[UnderstandingStatusTimedOut])
	})

	t.Run("Scenario_ConcurrentRespondsResolveExactlyOneWinner", func(t *testing.T) {
		// Given 30 simulated approval clicks racing for the same
		//       checkpoint (browser tab dedup edge case),
		gate := NewInMemoryUnderstandingCheckpointGate()
		armed, _ := gate.Arm(context.Background(), validUnderstanding())

		const N = 30
		var success int
		var mu sync.Mutex
		var wg sync.WaitGroup
		for i := 0; i < N; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := gate.Respond(context.Background(), UnderstandingResponse{
					CheckpointID: armed.ID, Status: UnderstandingStatusConfirmed,
				})
				if err == nil {
					mu.Lock()
					success++
					mu.Unlock()
				}
			}()
		}
		wg.Wait()
		assert.Equal(t, 1, success, "exactly one wins under race")
	})
}
