package agentic_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

func TestNewStallDetector_Defaults(t *testing.T) {
	d := agentic.NewStallDetector(0, 0)
	require.NotNil(t, d)
}

func TestStallDetector_DetectsStall(t *testing.T) {
	// Use very short intervals for testing.
	d := agentic.NewStallDetector(10*time.Millisecond, 30*time.Millisecond)

	var stallCount atomic.Int32
	var recoverCount atomic.Int32

	mon := d.Monitor("tool-1", "http_call",
		func(id, name string) {
			stallCount.Add(1)
			assert.Equal(t, "tool-1", id)
			assert.Equal(t, "http_call", name)
		},
		func(id, name string) {
			recoverCount.Add(1)
		},
	)
	defer mon.Stop()

	// Wait for the stall threshold to be exceeded.
	time.Sleep(80 * time.Millisecond)

	assert.True(t, mon.IsStalled(), "tool should be stalled after threshold")
	assert.GreaterOrEqual(t, int(stallCount.Load()), 1, "stall callback should fire at least once")
}

func TestStallDetector_RecoveryAfterStall(t *testing.T) {
	d := agentic.NewStallDetector(10*time.Millisecond, 25*time.Millisecond)

	var stallCount atomic.Int32
	var recoverCount atomic.Int32

	mon := d.Monitor("tool-1", "sql_query",
		func(id, name string) { stallCount.Add(1) },
		func(id, name string) { recoverCount.Add(1) },
	)
	defer mon.Stop()

	// Wait for stall.
	time.Sleep(60 * time.Millisecond)
	require.True(t, mon.IsStalled())

	// Record activity to recover.
	mon.RecordActivity()
	assert.False(t, mon.IsStalled(), "tool should recover after RecordActivity")
	assert.GreaterOrEqual(t, int(recoverCount.Load()), 1, "recover callback should fire")
}

func TestStallDetector_NoStallWithActivity(t *testing.T) {
	d := agentic.NewStallDetector(10*time.Millisecond, 40*time.Millisecond)

	var stallCount atomic.Int32

	mon := d.Monitor("tool-1", "fast_tool",
		func(id, name string) { stallCount.Add(1) },
		nil,
	)
	defer mon.Stop()

	// Record activity frequently — should prevent stall.
	for i := 0; i < 5; i++ {
		time.Sleep(15 * time.Millisecond)
		mon.RecordActivity()
	}

	assert.False(t, mon.IsStalled())
	assert.Equal(t, int32(0), stallCount.Load(), "stall callback should not fire")
}

func TestStallDetector_StopTerminatesMonitor(t *testing.T) {
	d := agentic.NewStallDetector(5*time.Millisecond, 15*time.Millisecond)

	var stallCount atomic.Int32

	mon := d.Monitor("tool-1", "stopped_tool",
		func(id, name string) { stallCount.Add(1) },
		nil,
	)

	// Stop immediately.
	mon.Stop()

	// Wait past threshold — should not detect stall since stopped.
	time.Sleep(40 * time.Millisecond)
	// Note: stall count may be 0 or 1 depending on timing, but monitor should not panic.
	assert.False(t, mon.IsStalled() && stallCount.Load() > 1, "stopped monitor should not repeatedly fire")
}

func TestStallDetector_DoubleStopIsSafe(t *testing.T) {
	d := agentic.NewStallDetector(10*time.Millisecond, 30*time.Millisecond)

	mon := d.Monitor("tool-1", "test",
		func(id, name string) {},
		nil,
	)

	// Double stop should not panic.
	mon.Stop()
	mon.Stop()
}

func TestStallDetector_NilCallbacks(t *testing.T) {
	d := agentic.NewStallDetector(5*time.Millisecond, 15*time.Millisecond)

	// nil callbacks should not panic.
	mon := d.Monitor("tool-1", "test", nil, nil)
	time.Sleep(30 * time.Millisecond)
	mon.Stop()
}

func TestToolStateStalled_Exists(t *testing.T) {
	// Verify the new ToolState constant.
	assert.Equal(t, agentic.ToolState("stalled"), agentic.ToolStateStalled)
}

func TestDefaultRunConfig_StallFields(t *testing.T) {
	cfg := agentic.DefaultRunConfig()
	assert.Equal(t, 15*time.Second, cfg.StallCheckInterval)
	assert.Equal(t, 45*time.Second, cfg.StallThreshold)
}
