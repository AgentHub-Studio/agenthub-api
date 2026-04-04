package agentic

import (
	"sort"
	"sync"
	"time"
)

// Throughput tracker for measuring event delivery performance.
//
// Inspired by Claude Code's FpsTracker — records event processing
// durations, computes average throughput (events/second) and the
// low-1% throughput (derived from p99 processing time). Useful for
// monitoring SSE streaming performance, tool execution rates, and
// any event-driven pipeline where sustained throughput matters.

// ThroughputMetrics summarizes measured throughput.
type ThroughputMetrics struct {
	// AverageEPS is the average events per second over the observation window.
	AverageEPS float64
	// Low1PctEPS is events/second derived from the p99 event duration.
	// Represents worst-case sustained throughput.
	Low1PctEPS float64
	// TotalEvents is the total number of events recorded.
	TotalEvents int
	// TotalDurationMs is the wall-clock duration from first to last event.
	TotalDurationMs float64
}

// ThroughputTracker records event durations and computes throughput metrics.
type ThroughputTracker struct {
	mu             sync.Mutex
	durations      []float64 // individual event durations in ms
	firstEventTime time.Time
	lastEventTime  time.Time
}

// NewThroughputTracker creates a new tracker.
func NewThroughputTracker() *ThroughputTracker {
	return &ThroughputTracker{}
}

// Record adds an event processing duration (in milliseconds).
func (t *ThroughputTracker) Record(durationMs float64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	if t.firstEventTime.IsZero() {
		t.firstEventTime = now
	}
	t.lastEventTime = now
	t.durations = append(t.durations, durationMs)
}

// RecordSince records the duration since the given start time.
func (t *ThroughputTracker) RecordSince(start time.Time) {
	dur := time.Since(start).Seconds() * 1000 // to ms
	t.Record(dur)
}

// Metrics computes throughput metrics. Returns nil if no events recorded
// or if the observation window is zero.
func (t *ThroughputTracker) Metrics() *ThroughputMetrics {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.durations) == 0 || t.firstEventTime.IsZero() || t.lastEventTime.IsZero() {
		return nil
	}

	totalMs := t.lastEventTime.Sub(t.firstEventTime).Seconds() * 1000
	if totalMs <= 0 {
		return nil
	}

	totalEvents := len(t.durations)
	averageEPS := float64(totalEvents) / (totalMs / 1000)

	// p99 frame time → low 1% throughput
	// Sort descending to find the top 1% slowest durations.
	sorted := make([]float64, len(t.durations))
	copy(sorted, t.durations)
	sort.Sort(sort.Reverse(sort.Float64Slice(sorted)))

	// Index into descending-sorted array: ceil(len * 0.01) - 1
	// gives the boundary of the top 1% slowest events.
	p99Idx := max(0, ceilInt(len(sorted), 100)-1)
	if p99Idx >= len(sorted) {
		p99Idx = len(sorted) - 1
	}
	p99DurationMs := sorted[p99Idx]

	var low1PctEPS float64
	if p99DurationMs > 0 {
		low1PctEPS = 1000 / p99DurationMs
	}

	return &ThroughputMetrics{
		AverageEPS:      roundTo2(averageEPS),
		Low1PctEPS:      roundTo2(low1PctEPS),
		TotalEvents:     totalEvents,
		TotalDurationMs: roundTo2(totalMs),
	}
}

// Count returns the total number of recorded events.
func (t *ThroughputTracker) Count() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.durations)
}

// Reset clears all recorded data.
func (t *ThroughputTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.durations = nil
	t.firstEventTime = time.Time{}
	t.lastEventTime = time.Time{}
}

func roundTo2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

// ceilInt returns ceil(n / d) for positive integers.
func ceilInt(n, d int) int {
	return (n + d - 1) / d
}
