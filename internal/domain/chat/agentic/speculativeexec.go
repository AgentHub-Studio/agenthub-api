package agentic

import (
	"fmt"
	"sync"
	"time"
)

// Speculative execution with rollback support.
//
// Inspired by Claude Code's speculation.ts — pre-emptively executes
// predicted user intent to save latency. Uses an overlay approach
// where speculative writes are tracked separately and can be committed
// or discarded.

// SpeculationStatus represents the current speculation state.
type SpeculationStatus string

const (
	SpeculationIdle    SpeculationStatus = "idle"
	SpeculationActive  SpeculationStatus = "active"
	SpeculationDone    SpeculationStatus = "done"
	SpeculationAborted SpeculationStatus = "aborted"
)

// CompletionBoundary identifies why speculation stopped.
type CompletionBoundary string

const (
	BoundaryComplete    CompletionBoundary = "complete"
	BoundaryToolDenied  CompletionBoundary = "denied_tool"
	BoundaryMaxTurns    CompletionBoundary = "max_turns"
	BoundaryMaxMessages CompletionBoundary = "max_messages"
	BoundaryError       CompletionBoundary = "error"
)

// SpeculationMaxTurns is the maximum number of turns per speculation.
const SpeculationMaxTurns = 20

// SpeculationMaxMessages is the maximum messages per speculation.
const SpeculationMaxMessages = 100

// SpeculationResult holds the outcome of a speculative execution.
type SpeculationResult struct {
	// Boundary is why the speculation stopped.
	Boundary CompletionBoundary `json:"boundary"`
	// Messages is the count of messages generated.
	Messages int `json:"messages"`
	// Turns is the count of turns executed.
	Turns int `json:"turns"`
	// WrittenPaths lists files modified during speculation.
	WrittenPaths []string `json:"writtenPaths,omitempty"`
	// TimeSavedMs is the estimated latency savings.
	TimeSavedMs int64 `json:"timeSavedMs"`
	// Duration is how long the speculation ran.
	Duration time.Duration `json:"duration"`
}

// SpeculativeExecution tracks a single speculation session.
type SpeculativeExecution struct {
	mu           sync.Mutex
	id           string
	status       SpeculationStatus
	startTime    time.Time
	turns        int
	messages     int
	writtenPaths map[string]bool
	readSafe     map[string]bool
	boundary     CompletionBoundary
	abort        *AbortController
}

// NewSpeculativeExecution creates a new speculation session.
func NewSpeculativeExecution(id string, readSafeTools []string) *SpeculativeExecution {
	safe := make(map[string]bool)
	for _, t := range readSafeTools {
		safe[t] = true
	}
	return &SpeculativeExecution{
		id:           id,
		status:       SpeculationActive,
		startTime:    time.Now(),
		writtenPaths: make(map[string]bool),
		readSafe:     safe,
		abort:        NewAbortController(),
	}
}

// ID returns the speculation session ID.
func (s *SpeculativeExecution) ID() string {
	return s.id
}

// Status returns the current status.
func (s *SpeculativeExecution) Status() SpeculationStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// IsActive returns true if the speculation is still running.
func (s *SpeculativeExecution) IsActive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status == SpeculationActive
}

// RecordTurn increments the turn counter.
// Returns false if max turns reached, triggering completion.
func (s *SpeculativeExecution) RecordTurn() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status != SpeculationActive {
		return false
	}

	s.turns++
	if s.turns >= SpeculationMaxTurns {
		s.status = SpeculationDone
		s.boundary = BoundaryMaxTurns
		return false
	}
	return true
}

// RecordMessage increments the message counter.
// Returns false if max messages reached.
func (s *SpeculativeExecution) RecordMessage() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status != SpeculationActive {
		return false
	}

	s.messages++
	if s.messages >= SpeculationMaxMessages {
		s.status = SpeculationDone
		s.boundary = BoundaryMaxMessages
		return false
	}
	return true
}

// RecordWrite tracks a file written during speculation.
func (s *SpeculativeExecution) RecordWrite(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writtenPaths[path] = true
}

// IsToolAllowed checks if a tool can be used during speculation.
// Only read-safe tools are allowed.
func (s *SpeculativeExecution) IsToolAllowed(toolName string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readSafe[toolName]
}

// DenyTool marks the speculation as stopped due to a denied tool.
func (s *SpeculativeExecution) DenyTool(toolName string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status == SpeculationActive {
		s.status = SpeculationDone
		s.boundary = BoundaryToolDenied
	}
}

// Complete marks the speculation as successfully finished.
func (s *SpeculativeExecution) Complete() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status == SpeculationActive {
		s.status = SpeculationDone
		s.boundary = BoundaryComplete
	}
}

// Abort cancels the speculation.
func (s *SpeculativeExecution) Abort() {
	s.mu.Lock()
	if s.status != SpeculationActive {
		s.mu.Unlock()
		return
	}
	s.status = SpeculationAborted
	s.boundary = BoundaryError
	s.mu.Unlock()

	s.abort.Abort()
}

// Context returns the abort controller for cancellation monitoring.
func (s *SpeculativeExecution) AbortController() *AbortController {
	return s.abort
}

// Result returns the speculation outcome.
func (s *SpeculativeExecution) Result() SpeculationResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	paths := make([]string, 0, len(s.writtenPaths))
	for p := range s.writtenPaths {
		paths = append(paths, p)
	}

	return SpeculationResult{
		Boundary:     s.boundary,
		Messages:     s.messages,
		Turns:        s.turns,
		WrittenPaths: paths,
		Duration:     time.Since(s.startTime),
		TimeSavedMs:  time.Since(s.startTime).Milliseconds(),
	}
}

// WrittenPaths returns the list of files modified during speculation.
func (s *SpeculativeExecution) WrittenPaths() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	paths := make([]string, 0, len(s.writtenPaths))
	for p := range s.writtenPaths {
		paths = append(paths, p)
	}
	return paths
}

// Summary returns a human-readable summary.
func (s *SpeculativeExecution) Summary() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return fmt.Sprintf("speculation %s: %s (%d turns, %d msgs, %d writes, %s)",
		s.id, s.status, s.turns, s.messages, len(s.writtenPaths),
		time.Since(s.startTime).Round(time.Millisecond))
}
