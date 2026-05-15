package agentic

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validUnderstanding() UnderstandingCheckpoint {
	return UnderstandingCheckpoint{
		TenantID:    "tenant-x",
		AgentID:     "agent-y",
		RunID:       "run-z",
		TaskSummary: "Generate Q4 invoice CSV for finance team",
		KeyAssumptions: []string{
			"Q4 = Oct-Dec 2026",
			"Customer is logged in tenant-x",
		},
		NextActions: []string{
			"Query ah_finance.invoices for Q4 2026",
			"Format as CSV with UTF-8 BOM",
			"Email to finance@tenant.com",
		},
	}
}

func TestUnderstanding_StatusEnumIsBounded(t *testing.T) {
	for _, s := range AllUnderstandingStatuses() {
		assert.True(t, IsValidUnderstandingStatus(s))
	}
	assert.False(t, IsValidUnderstandingStatus(UnderstandingStatus("unknown")))
}

func TestUnderstanding_AllStatusesCount(t *testing.T) {
	// 5 statuses: pending/confirmed/corrected/abandoned/timed_out.
	assert.Equal(t, 5, len(AllUnderstandingStatuses()))
}

func TestUnderstanding_TerminalStatusCheck(t *testing.T) {
	assert.False(t, IsTerminalUnderstandingStatus(UnderstandingStatusPending))
	assert.True(t, IsTerminalUnderstandingStatus(UnderstandingStatusConfirmed))
	assert.True(t, IsTerminalUnderstandingStatus(UnderstandingStatusCorrected))
	assert.True(t, IsTerminalUnderstandingStatus(UnderstandingStatusAbandoned))
	assert.True(t, IsTerminalUnderstandingStatus(UnderstandingStatusTimedOut))
	assert.False(t, IsTerminalUnderstandingStatus(UnderstandingStatus("bogus")))
}

func TestUnderstanding_Arm_AssignsIDAndStatus(t *testing.T) {
	gate := NewInMemoryUnderstandingCheckpointGate()
	armed, err := gate.Arm(context.Background(), validUnderstanding())
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, armed.ID)
	assert.Equal(t, UnderstandingStatusPending, armed.Status)
	assert.False(t, armed.ArmedAt.IsZero())
}

func TestUnderstanding_Arm_RejectsEmptyTenantID(t *testing.T) {
	gate := NewInMemoryUnderstandingCheckpointGate()
	cp := validUnderstanding()
	cp.TenantID = ""
	_, err := gate.Arm(context.Background(), cp)
	assert.Error(t, err)
}

func TestUnderstanding_Arm_RejectsEmptyTaskSummary(t *testing.T) {
	gate := NewInMemoryUnderstandingCheckpointGate()
	cp := validUnderstanding()
	cp.TaskSummary = ""
	_, err := gate.Arm(context.Background(), cp)
	assert.Error(t, err)
}

func TestUnderstanding_Arm_TruncatesLongTaskSummary(t *testing.T) {
	gate := NewInMemoryUnderstandingCheckpointGate()
	cp := validUnderstanding()
	cp.TaskSummary = strings.Repeat("x", 500)
	armed, err := gate.Arm(context.Background(), cp)
	require.NoError(t, err)
	assert.Equal(t, 300, len(armed.TaskSummary))
}

func TestUnderstanding_Respond_FirstWins(t *testing.T) {
	gate := NewInMemoryUnderstandingCheckpointGate()
	armed, _ := gate.Arm(context.Background(), validUnderstanding())

	require.NoError(t, gate.Respond(context.Background(), UnderstandingResponse{
		CheckpointID: armed.ID,
		Status:       UnderstandingStatusConfirmed,
		RespondedBy:  "alice",
	}))

	err := gate.Respond(context.Background(), UnderstandingResponse{
		CheckpointID: armed.ID,
		Status:       UnderstandingStatusAbandoned,
	})
	assert.True(t, errors.Is(err, ErrUnderstandingAlreadyResolved))

	_, res, _ := gate.Find(context.Background(), armed.ID)
	require.NotNil(t, res)
	assert.Equal(t, UnderstandingStatusConfirmed, res.Status)
	assert.Equal(t, "alice", res.RespondedBy)
}

func TestUnderstanding_Respond_RejectsNonTerminalStatus(t *testing.T) {
	gate := NewInMemoryUnderstandingCheckpointGate()
	armed, _ := gate.Arm(context.Background(), validUnderstanding())
	err := gate.Respond(context.Background(), UnderstandingResponse{
		CheckpointID: armed.ID,
		Status:       UnderstandingStatusPending,
	})
	assert.True(t, errors.Is(err, ErrUnderstandingResponseStatusNotTerminal),
		"pending is not a valid response status")
}

func TestUnderstanding_Respond_RejectsInvalidStatus(t *testing.T) {
	gate := NewInMemoryUnderstandingCheckpointGate()
	armed, _ := gate.Arm(context.Background(), validUnderstanding())
	err := gate.Respond(context.Background(), UnderstandingResponse{
		CheckpointID: armed.ID,
		Status:       UnderstandingStatus("bogus"),
	})
	assert.True(t, errors.Is(err, ErrInvalidUnderstandingStatus))
}

func TestUnderstanding_Respond_UnknownIDReturnsNotFound(t *testing.T) {
	gate := NewInMemoryUnderstandingCheckpointGate()
	err := gate.Respond(context.Background(), UnderstandingResponse{
		CheckpointID: uuid.New(),
		Status:       UnderstandingStatusConfirmed,
	})
	assert.True(t, errors.Is(err, ErrUnderstandingCheckpointNotFound))
}

func TestUnderstanding_Await_DeliversAfterRespond(t *testing.T) {
	gate := NewInMemoryUnderstandingCheckpointGate()
	armed, _ := gate.Arm(context.Background(), validUnderstanding())

	done := make(chan UnderstandingResponse, 1)
	go func() {
		res, err := gate.Await(context.Background(), armed.ID)
		require.NoError(t, err)
		done <- res
	}()
	time.Sleep(15 * time.Millisecond)
	require.NoError(t, gate.Respond(context.Background(), UnderstandingResponse{
		CheckpointID: armed.ID, Status: UnderstandingStatusCorrected,
		Corrections: []string{"Q4 = previous fiscal year not calendar"},
	}))
	select {
	case res := <-done:
		assert.Equal(t, UnderstandingStatusCorrected, res.Status)
		assert.Len(t, res.Corrections, 1)
	case <-time.After(time.Second):
		t.Fatal("Await did not deliver within 1s")
	}
}

func TestUnderstanding_Await_AlreadyResolvedReturnsImmediately(t *testing.T) {
	gate := NewInMemoryUnderstandingCheckpointGate()
	armed, _ := gate.Arm(context.Background(), validUnderstanding())
	require.NoError(t, gate.Respond(context.Background(), UnderstandingResponse{
		CheckpointID: armed.ID, Status: UnderstandingStatusConfirmed,
	}))
	res, err := gate.Await(context.Background(), armed.ID)
	require.NoError(t, err)
	assert.Equal(t, UnderstandingStatusConfirmed, res.Status)
}

func TestUnderstanding_Await_DeadlineFiresTimeout(t *testing.T) {
	gate := NewInMemoryUnderstandingCheckpointGate()
	cp := validUnderstanding()
	cp.ConfirmationDeadline = time.Now().Add(50 * time.Millisecond)
	armed, _ := gate.Arm(context.Background(), cp)
	res, err := gate.Await(context.Background(), armed.ID)
	require.NoError(t, err)
	assert.Equal(t, UnderstandingStatusTimedOut, res.Status)
}

func TestUnderstanding_Await_PastDeadlineImmediateTimeout(t *testing.T) {
	gate := NewInMemoryUnderstandingCheckpointGate()
	cp := validUnderstanding()
	cp.ConfirmationDeadline = time.Now().Add(-time.Second)
	armed, _ := gate.Arm(context.Background(), cp)
	start := time.Now()
	res, err := gate.Await(context.Background(), armed.ID)
	require.NoError(t, err)
	assert.Equal(t, UnderstandingStatusTimedOut, res.Status)
	assert.Less(t, time.Since(start).Milliseconds(), int64(50))
}

func TestUnderstanding_Await_HonorsContextCancel(t *testing.T) {
	gate := NewInMemoryUnderstandingCheckpointGate()
	armed, _ := gate.Arm(context.Background(), validUnderstanding())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := gate.Await(ctx, armed.ID)
	assert.Error(t, err)
}

func TestUnderstanding_Find_BeforeAndAfterResponse(t *testing.T) {
	gate := NewInMemoryUnderstandingCheckpointGate()
	armed, _ := gate.Arm(context.Background(), validUnderstanding())

	cp, res, err := gate.Find(context.Background(), armed.ID)
	require.NoError(t, err)
	assert.Nil(t, res)
	assert.Equal(t, UnderstandingStatusPending, cp.Status)

	require.NoError(t, gate.Respond(context.Background(), UnderstandingResponse{
		CheckpointID: armed.ID, Status: UnderstandingStatusConfirmed,
	}))
	cp2, res2, err := gate.Find(context.Background(), armed.ID)
	require.NoError(t, err)
	require.NotNil(t, res2)
	assert.Equal(t, UnderstandingStatusConfirmed, cp2.Status)
}

func TestUnderstanding_CountByStatus_AlwaysAllStatuses(t *testing.T) {
	gate := NewInMemoryUnderstandingCheckpointGate()
	armed1, _ := gate.Arm(context.Background(), validUnderstanding())
	_ = gate.Respond(context.Background(), UnderstandingResponse{
		CheckpointID: armed1.ID, Status: UnderstandingStatusConfirmed,
	})
	armed2, _ := gate.Arm(context.Background(), validUnderstanding())
	_ = gate.Respond(context.Background(), UnderstandingResponse{
		CheckpointID: armed2.ID, Status: UnderstandingStatusAbandoned,
	})

	hist, err := gate.CountByStatus(context.Background(), "tenant-x")
	require.NoError(t, err)
	for _, s := range AllUnderstandingStatuses() {
		_, ok := hist[s]
		assert.True(t, ok)
	}
	assert.Equal(t, 1, hist[UnderstandingStatusConfirmed])
	assert.Equal(t, 1, hist[UnderstandingStatusAbandoned])
	assert.Equal(t, 0, hist[UnderstandingStatusCorrected])
}

func TestUnderstanding_PlainText_Format(t *testing.T) {
	cp := validUnderstanding()
	cp.Status = UnderstandingStatusPending
	out := cp.PlainText()
	assert.Contains(t, out, "[UNDERSTAND pending]")
	assert.Contains(t, out, "2 assumptions")
	assert.Contains(t, out, "3 next actions")
}

func TestUnderstanding_Markdown_PendingShowsCallToAction(t *testing.T) {
	cp := validUnderstanding()
	cp.Status = UnderstandingStatusPending
	out := cp.Markdown()
	assert.Contains(t, out, "## Understanding Checkpoint — pending")
	assert.Contains(t, out, "Q4 = Oct-Dec 2026")
	assert.Contains(t, out, "1. Query ah_finance.invoices")
	assert.Contains(t, out, "Please confirm, correct, or abandon")
}

func TestUnderstanding_Markdown_ResolvedHidesCallToAction(t *testing.T) {
	cp := validUnderstanding()
	cp.Status = UnderstandingStatusConfirmed
	out := cp.Markdown()
	assert.NotContains(t, out, "Please confirm, correct, or abandon",
		"resolved checkpoints don't show pending CTA")
}

func TestUnderstanding_ContextCancelledOperationsError(t *testing.T) {
	gate := NewInMemoryUnderstandingCheckpointGate()
	armed, _ := gate.Arm(context.Background(), validUnderstanding())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := gate.Arm(ctx, validUnderstanding())
	assert.Error(t, err)

	err = gate.Respond(ctx, UnderstandingResponse{
		CheckpointID: armed.ID, Status: UnderstandingStatusConfirmed,
	})
	assert.Error(t, err)

	_, _, err = gate.Find(ctx, armed.ID)
	assert.Error(t, err)

	_, err = gate.CountByStatus(ctx, "x")
	assert.Error(t, err)
}

func TestUnderstanding_ConcurrentArmIsSafe(t *testing.T) {
	gate := NewInMemoryUnderstandingCheckpointGate()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := gate.Arm(context.Background(), validUnderstanding())
			assert.NoError(t, err)
		}()
	}
	wg.Wait()
}

func TestUnderstanding_ConcurrentRespondOnlyOneWins(t *testing.T) {
	gate := NewInMemoryUnderstandingCheckpointGate()
	armed, _ := gate.Arm(context.Background(), validUnderstanding())

	const N = 30
	successCount := 0
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
				successCount++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, 1, successCount)
}
