package agentic

import (
	"sync"
	"time"
)

// StallDetector monitors a tool execution and emits ToolStateStalled events
// when the tool hasn't produced output within the configured threshold.
// It runs a background goroutine that periodically checks for staleness.
type StallDetector struct {
	interval  time.Duration
	threshold time.Duration
}

// NewStallDetector creates a StallDetector with the given check interval and
// stall threshold. If either is zero, defaults are used (15s interval, 45s threshold).
func NewStallDetector(interval, threshold time.Duration) *StallDetector {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	if threshold <= 0 {
		threshold = 45 * time.Second
	}
	return &StallDetector{
		interval:  interval,
		threshold: threshold,
	}
}

// StallMonitor tracks the activity state of a single tool execution.
type StallMonitor struct {
	mu          sync.Mutex
	toolID      string
	toolName    string
	lastActive  time.Time
	stalled     bool
	stopped     bool
	stopCh      chan struct{}
	onStall     func(toolID, toolName string)
	onRecover   func(toolID, toolName string)
	detector    *StallDetector
}

// Monitor starts monitoring a tool execution and returns a StallMonitor.
// The onStall callback is called when the tool becomes stalled.
// The onRecover callback is called when activity resumes after a stall.
// Call Stop() when the tool execution completes.
func (d *StallDetector) Monitor(
	toolID, toolName string,
	onStall func(toolID, toolName string),
	onRecover func(toolID, toolName string),
) *StallMonitor {
	m := &StallMonitor{
		toolID:    toolID,
		toolName:  toolName,
		lastActive: time.Now(),
		stopCh:    make(chan struct{}),
		onStall:   onStall,
		onRecover: onRecover,
		detector:  d,
	}

	go m.run()
	return m
}

// RecordActivity signals that the tool is still active (e.g., produced output).
// If the tool was stalled, it transitions back to executing.
func (m *StallMonitor) RecordActivity() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastActive = time.Now()
	if m.stalled {
		m.stalled = false
		if m.onRecover != nil {
			m.onRecover(m.toolID, m.toolName)
		}
	}
}

// IsStalled returns whether the tool is currently considered stalled.
func (m *StallMonitor) IsStalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stalled
}

// Stop terminates the stall monitoring goroutine.
func (m *StallMonitor) Stop() {
	m.mu.Lock()
	if m.stopped {
		m.mu.Unlock()
		return
	}
	m.stopped = true
	m.mu.Unlock()
	close(m.stopCh)
}

func (m *StallMonitor) run() {
	ticker := time.NewTicker(m.detector.interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.check()
		}
	}
}

func (m *StallMonitor) check() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.stalled {
		return // already stalled, waiting for RecordActivity
	}

	elapsed := time.Since(m.lastActive)
	if elapsed >= m.detector.threshold {
		m.stalled = true
		if m.onStall != nil {
			m.onStall(m.toolID, m.toolName)
		}
	}
}
