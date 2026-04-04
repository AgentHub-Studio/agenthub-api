package agentic_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

func TestThroughputTracker_New(t *testing.T) {
	tr := agentic.NewThroughputTracker()
	assert.NotNil(t, tr)
	assert.Equal(t, 0, tr.Count())
}

func TestThroughputTracker_Metrics_Empty(t *testing.T) {
	tr := agentic.NewThroughputTracker()
	assert.Nil(t, tr.Metrics())
}

func TestThroughputTracker_Metrics_SingleEvent(t *testing.T) {
	tr := agentic.NewThroughputTracker()
	tr.Record(10.0)
	// Single event — no time window, returns nil
	assert.Nil(t, tr.Metrics())
}

func TestThroughputTracker_Metrics_MultipleEvents(t *testing.T) {
	tr := agentic.NewThroughputTracker()

	tr.Record(5.0)
	time.Sleep(50 * time.Millisecond)
	tr.Record(10.0)
	time.Sleep(50 * time.Millisecond)
	tr.Record(15.0)

	m := tr.Metrics()
	require.NotNil(t, m)
	assert.Equal(t, 3, m.TotalEvents)
	assert.Greater(t, m.AverageEPS, 0.0)
	assert.Greater(t, m.Low1PctEPS, 0.0)
	assert.Greater(t, m.TotalDurationMs, 0.0)
}

func TestThroughputTracker_Count(t *testing.T) {
	tr := agentic.NewThroughputTracker()

	assert.Equal(t, 0, tr.Count())
	tr.Record(1.0)
	assert.Equal(t, 1, tr.Count())
	tr.Record(2.0)
	assert.Equal(t, 2, tr.Count())
}

func TestThroughputTracker_Reset(t *testing.T) {
	tr := agentic.NewThroughputTracker()

	tr.Record(1.0)
	tr.Record(2.0)
	assert.Equal(t, 2, tr.Count())

	tr.Reset()
	assert.Equal(t, 0, tr.Count())
	assert.Nil(t, tr.Metrics())
}

func TestThroughputTracker_RecordSince(t *testing.T) {
	tr := agentic.NewThroughputTracker()

	start := time.Now()
	time.Sleep(10 * time.Millisecond)
	tr.RecordSince(start)

	assert.Equal(t, 1, tr.Count())
}

func TestThroughputTracker_Low1Pct_WorstCase(t *testing.T) {
	tr := agentic.NewThroughputTracker()

	// Record 8 fast events and 2 slow events (20% slow).
	// With 10 events, top 1% = ceil(10*0.01)=1, so index 0 in descending
	// picks the slowest event.
	for i := 0; i < 8; i++ {
		tr.Record(1.0) // 1ms each — fast
		time.Sleep(2 * time.Millisecond)
	}
	tr.Record(100.0) // 100ms — slow outlier
	time.Sleep(2 * time.Millisecond)
	tr.Record(50.0) // 50ms — another slow event

	m := tr.Metrics()
	require.NotNil(t, m)

	// Low1PctEPS should be based on the slowest event (100ms → 10 eps)
	assert.LessOrEqual(t, m.Low1PctEPS, 1000.0/100.0+1.0, "low 1% should reflect the slowest events")
	assert.Greater(t, m.Low1PctEPS, 0.0, "should not be zero")
}

func TestThroughputTracker_AverageEPS_Reasonable(t *testing.T) {
	tr := agentic.NewThroughputTracker()

	// 10 events over ~100ms = ~100 eps
	for i := 0; i < 10; i++ {
		tr.Record(1.0)
		time.Sleep(10 * time.Millisecond)
	}

	m := tr.Metrics()
	require.NotNil(t, m)
	assert.Equal(t, 10, m.TotalEvents)
	// Should be roughly 100 eps (10 events / 0.1s) ± tolerance
	assert.Greater(t, m.AverageEPS, 30.0, "should have meaningful throughput")
	assert.Less(t, m.AverageEPS, 300.0, "should not be unreasonably high")
}

func TestThroughputTracker_Concurrent(t *testing.T) {
	tr := agentic.NewThroughputTracker()
	done := make(chan struct{})

	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				tr.Record(float64(j))
			}
			done <- struct{}{}
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	assert.Equal(t, 1000, tr.Count())
}
