package agentic

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validCP(kind CheckpointKind) Checkpoint {
	return Checkpoint{
		Kind:     kind,
		TenantID: "tenant-x",
		AgentID:  "agent-y",
		RunID:    "run-z",
		Reason:   "test reason",
	}
}

func TestCheckpoint_KindEnumIsBounded(t *testing.T) {
	for _, k := range AllCheckpointKinds() {
		assert.True(t, IsValidCheckpointKind(k),
			"every kind in AllCheckpointKinds must pass IsValidCheckpointKind")
	}
	assert.False(t, IsValidCheckpointKind(CheckpointKind("unknown")))
	assert.False(t, IsValidCheckpointKind(""))
}

func TestCheckpoint_DecisionEnumIsBounded(t *testing.T) {
	for _, d := range []CheckpointDecision{
		CheckpointApproved, CheckpointRejected,
		CheckpointCancelled, CheckpointTimedOut,
	} {
		assert.True(t, IsTerminalCheckpointDecision(d))
	}
	assert.False(t, IsTerminalCheckpointDecision(CheckpointDecision("maybe")))
	assert.False(t, IsTerminalCheckpointDecision(""))
}

func TestCheckpoint_AllCheckpointKindsCount(t *testing.T) {
	// 6 kinds in bounded set.
	assert.Equal(t, 6, len(AllCheckpointKinds()),
		"6 checkpoint kinds expected (refactor must update integration test too)")
}

func TestCheckpoint_Arm_AssignsIDAndArmedAt(t *testing.T) {
	gate := NewInMemoryCheckpointGate()
	armed, err := gate.Arm(context.Background(), validCP(CheckpointPreDestructive))
	require.NoError(t, err)
	assert.NotEqual(t, "", armed.ID.String())
	assert.False(t, armed.ArmedAt.IsZero())
	assert.Equal(t, CheckpointPreDestructive, armed.Kind)
}

func TestCheckpoint_Arm_RejectsInvalidKind(t *testing.T) {
	gate := NewInMemoryCheckpointGate()
	cp := validCP(CheckpointKind("bogus"))
	_, err := gate.Arm(context.Background(), cp)
	assert.True(t, errors.Is(err, ErrInvalidCheckpointKind))
}

func TestCheckpoint_Arm_RejectsEmptyTenantID(t *testing.T) {
	gate := NewInMemoryCheckpointGate()
	cp := Checkpoint{Kind: CheckpointPreDestructive}
	_, err := gate.Arm(context.Background(), cp)
	assert.Error(t, err)
}

func TestCheckpoint_Resolve_FirstDecisionWins(t *testing.T) {
	gate := NewInMemoryCheckpointGate()
	armed, _ := gate.Arm(context.Background(), validCP(CheckpointPreDestructive))

	err := gate.Resolve(context.Background(), CheckpointResolution{
		CheckpointID: armed.ID,
		Decision:     CheckpointApproved,
		DecidedBy:    "alice",
	})
	require.NoError(t, err)

	// Second call must fail (audit guarantee — no flip after decide).
	err2 := gate.Resolve(context.Background(), CheckpointResolution{
		CheckpointID: armed.ID,
		Decision:     CheckpointRejected,
	})
	assert.True(t, errors.Is(err2, ErrCheckpointAlreadyResolved),
		"second resolve must return ErrCheckpointAlreadyResolved")

	_, res, _ := gate.Find(context.Background(), armed.ID)
	require.NotNil(t, res)
	assert.Equal(t, CheckpointApproved, res.Decision,
		"first decision must have won")
}

func TestCheckpoint_Resolve_RejectsInvalidDecision(t *testing.T) {
	gate := NewInMemoryCheckpointGate()
	armed, _ := gate.Arm(context.Background(), validCP(CheckpointPreDestructive))
	err := gate.Resolve(context.Background(), CheckpointResolution{
		CheckpointID: armed.ID,
		Decision:     CheckpointDecision("maybe"),
	})
	assert.Error(t, err)
}

func TestCheckpoint_Resolve_UnknownIDReturnsNotFound(t *testing.T) {
	gate := NewInMemoryCheckpointGate()
	err := gate.Resolve(context.Background(), CheckpointResolution{
		CheckpointID: validCP(CheckpointPreDestructive).ID, // zero UUID
		Decision:     CheckpointApproved,
	})
	assert.True(t, errors.Is(err, ErrCheckpointNotFound))
}

func TestCheckpoint_Await_DeliversResolutionAfterResolve(t *testing.T) {
	gate := NewInMemoryCheckpointGate()
	armed, _ := gate.Arm(context.Background(), validCP(CheckpointPreDestructive))

	done := make(chan CheckpointResolution, 1)
	go func() {
		res, err := gate.Await(context.Background(), armed.ID)
		require.NoError(t, err)
		done <- res
	}()

	// Give the goroutine time to enter Await.
	time.Sleep(20 * time.Millisecond)
	require.NoError(t, gate.Resolve(context.Background(), CheckpointResolution{
		CheckpointID: armed.ID,
		Decision:     CheckpointApproved,
		DecidedBy:    "bob",
	}))

	select {
	case res := <-done:
		assert.Equal(t, CheckpointApproved, res.Decision)
		assert.Equal(t, "bob", res.DecidedBy)
	case <-time.After(time.Second):
		t.Fatal("Await did not deliver resolution within 1s")
	}
}

func TestCheckpoint_Await_AlreadyResolvedReturnsImmediately(t *testing.T) {
	gate := NewInMemoryCheckpointGate()
	armed, _ := gate.Arm(context.Background(), validCP(CheckpointPreDestructive))
	require.NoError(t, gate.Resolve(context.Background(), CheckpointResolution{
		CheckpointID: armed.ID,
		Decision:     CheckpointRejected,
	}))

	res, err := gate.Await(context.Background(), armed.ID)
	require.NoError(t, err)
	assert.Equal(t, CheckpointRejected, res.Decision)
}

func TestCheckpoint_Await_RespectsDeadline(t *testing.T) {
	gate := NewInMemoryCheckpointGate()
	cp := validCP(CheckpointPreCostThreshold)
	cp.Deadline = time.Now().Add(50 * time.Millisecond)
	armed, _ := gate.Arm(context.Background(), cp)

	start := time.Now()
	res, err := gate.Await(context.Background(), armed.ID)
	require.NoError(t, err)
	elapsed := time.Since(start)
	assert.Equal(t, CheckpointTimedOut, res.Decision)
	assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(40),
		"timeout fired around the 50ms deadline")
}

func TestCheckpoint_Await_PastDeadlineReturnsImmediateTimeout(t *testing.T) {
	gate := NewInMemoryCheckpointGate()
	cp := validCP(CheckpointPreDestructive)
	cp.Deadline = time.Now().Add(-1 * time.Second) // already past
	armed, _ := gate.Arm(context.Background(), cp)

	start := time.Now()
	res, err := gate.Await(context.Background(), armed.ID)
	require.NoError(t, err)
	assert.Equal(t, CheckpointTimedOut, res.Decision)
	assert.Less(t, time.Since(start).Milliseconds(), int64(50),
		"past deadline should resolve essentially instantly")
}

func TestCheckpoint_Await_HonorsContextCancellation(t *testing.T) {
	gate := NewInMemoryCheckpointGate()
	armed, _ := gate.Arm(context.Background(), validCP(CheckpointPreDestructive))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := gate.Await(ctx, armed.ID)
	assert.Error(t, err, "cancelled ctx must produce error from Await")
}

func TestCheckpoint_Await_UnknownIDReturnsNotFound(t *testing.T) {
	gate := NewInMemoryCheckpointGate()
	armed := validCP(CheckpointPreDestructive)
	_, err := gate.Await(context.Background(), armed.ID) // zero UUID
	assert.True(t, errors.Is(err, ErrCheckpointNotFound))
}

func TestCheckpoint_Find_BeforeAndAfterResolution(t *testing.T) {
	gate := NewInMemoryCheckpointGate()
	armed, _ := gate.Arm(context.Background(), validCP(CheckpointPreDestructive))

	// Before resolve.
	cp, res, err := gate.Find(context.Background(), armed.ID)
	require.NoError(t, err)
	assert.Equal(t, armed.ID, cp.ID)
	assert.Nil(t, res, "Find before resolution must return nil resolution")

	// After resolve.
	require.NoError(t, gate.Resolve(context.Background(), CheckpointResolution{
		CheckpointID: armed.ID,
		Decision:     CheckpointApproved,
	}))
	cp2, res2, err := gate.Find(context.Background(), armed.ID)
	require.NoError(t, err)
	assert.Equal(t, armed.ID, cp2.ID)
	require.NotNil(t, res2)
	assert.Equal(t, CheckpointApproved, res2.Decision)
}

func TestCheckpoint_ConcurrentArmIsSafe(t *testing.T) {
	gate := NewInMemoryCheckpointGate()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := gate.Arm(context.Background(), validCP(CheckpointPreDestructive))
			assert.NoError(t, err)
		}()
	}
	wg.Wait() // -race must be clean
}

func TestCheckpoint_ConcurrentResolveOnlyOneWins(t *testing.T) {
	gate := NewInMemoryCheckpointGate()
	armed, _ := gate.Arm(context.Background(), validCP(CheckpointPreDestructive))

	const N = 30
	var wg sync.WaitGroup
	successCount := 0
	var successMu sync.Mutex
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := gate.Resolve(context.Background(), CheckpointResolution{
				CheckpointID: armed.ID,
				Decision:     CheckpointApproved,
			})
			if err == nil {
				successMu.Lock()
				successCount++
				successMu.Unlock()
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, 1, successCount,
		"exactly one of %d concurrent Resolve calls must succeed (first-wins)", N)
}
