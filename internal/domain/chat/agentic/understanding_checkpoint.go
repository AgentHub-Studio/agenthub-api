package agentic

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// HUMAN-004 — Understanding checkpoints.
//
// PDF arXiv:2604.14228v1 §11 (when a multi-step task is ambiguous or
// expensive, the agent should PAUSE and present its understanding for
// human confirmation BEFORE acting); §6.1 (explicit user review steps).
//
// Distinction from neighbouring abstractions:
//   - GOV-003 Checkpoint   — "you NEED approval per policy/safety to do this".
//   - HUMAN-004 Understanding Checkpoint — "I want to verify I understood
//     correctly before acting". Different intent: comprehension, not gating.
//   - HUMAN-001 ReviewGuidance — directs human attention POST-artifact.
//   - HUMAN-002 ExplainedDiff — annotates POST-change.
//   - HUMAN-003 DecisionRecord — records WHY past decisions were made.
//
// An UnderstandingCheckpoint asks the human BEFORE the work:
//   "Here's my interpretation of your task + key assumptions + next
//    actions I'm about to take. Confirm or correct."
//
// The human responds:
//   - confirmed: "yes, proceed" → agent runs
//   - corrected: "no, fix these assumptions" → agent re-plans
//   - abandoned: "stop entirely"
//
// Used for ambiguous prompts, multi-step plans, expensive operations,
// or when the agent's confidence is low.

// UnderstandingStatus bounded enum.
type UnderstandingStatus string

const (
	// UnderstandingStatusPending — awaiting human response.
	UnderstandingStatusPending UnderstandingStatus = "pending"
	// UnderstandingStatusConfirmed — human said "proceed".
	UnderstandingStatusConfirmed UnderstandingStatus = "confirmed"
	// UnderstandingStatusCorrected — human supplied corrections; agent re-plans.
	UnderstandingStatusCorrected UnderstandingStatus = "corrected"
	// UnderstandingStatusAbandoned — human said "stop entirely".
	UnderstandingStatusAbandoned UnderstandingStatus = "abandoned"
	// UnderstandingStatusTimedOut — deadline elapsed without response.
	UnderstandingStatusTimedOut UnderstandingStatus = "timed_out"
)

// allUnderstandingStatuses is the closed bounded set.
var allUnderstandingStatuses = []UnderstandingStatus{
	UnderstandingStatusPending,
	UnderstandingStatusConfirmed,
	UnderstandingStatusCorrected,
	UnderstandingStatusAbandoned,
	UnderstandingStatusTimedOut,
}

// IsValidUnderstandingStatus returns true for the bounded set.
func IsValidUnderstandingStatus(s UnderstandingStatus) bool {
	for _, v := range allUnderstandingStatuses {
		if s == v {
			return true
		}
	}
	return false
}

// AllUnderstandingStatuses returns a copy of the bounded set.
func AllUnderstandingStatuses() []UnderstandingStatus {
	out := make([]UnderstandingStatus, len(allUnderstandingStatuses))
	copy(out, allUnderstandingStatuses)
	return out
}

// IsTerminalUnderstandingStatus returns true for confirmed/corrected/
// abandoned/timed_out. Pending is the only non-terminal.
func IsTerminalUnderstandingStatus(s UnderstandingStatus) bool {
	return s != UnderstandingStatusPending && IsValidUnderstandingStatus(s)
}

// UnderstandingCheckpoint captures an agent's interpretation of a task
// and pauses for human confirmation.
type UnderstandingCheckpoint struct {
	// ID is assigned at Arm time.
	ID uuid.UUID `json:"id"`
	// TenantID scopes the checkpoint. Required.
	TenantID string `json:"tenantId"`
	// AgentID identifies the agent.
	AgentID string `json:"agentId"`
	// RunID correlates back to the run trace.
	RunID string `json:"runId"`
	// TaskSummary is the agent's one-line summary of what it
	// interpreted the task to be (≤ 300 chars). Required.
	TaskSummary string `json:"taskSummary"`
	// KeyAssumptions is the list of assumptions the agent is making
	// (e.g. "user wants invoices for current FY", "data is in postgres").
	// The human reads these to confirm/correct.
	KeyAssumptions []string `json:"keyAssumptions,omitempty"`
	// NextActions is the list of concrete steps the agent will take
	// once confirmed (e.g. "1. query invoice table", "2. format CSV").
	NextActions []string `json:"nextActions,omitempty"`
	// ConfirmationDeadline is when this checkpoint will time out.
	// Zero = no deadline (discouraged).
	ConfirmationDeadline time.Time `json:"confirmationDeadline,omitempty"`
	// ArmedAt is when the checkpoint was created.
	ArmedAt time.Time `json:"armedAt"`
	// Status tracks lifecycle.
	Status UnderstandingStatus `json:"status"`
}

// UnderstandingResponse captures the human reply.
type UnderstandingResponse struct {
	CheckpointID uuid.UUID
	Status       UnderstandingStatus
	// Corrections is the list of corrections supplied (when Status=corrected).
	Corrections []string
	// RespondedBy identifies the human.
	RespondedBy string
	// Notes is freeform feedback.
	Notes string
	// RespondedAt is the wall-clock timestamp.
	RespondedAt time.Time
}

// Sentinels.
var (
	ErrUnderstandingCheckpointNotFound      = errors.New("understanding: not found")
	ErrUnderstandingAlreadyResolved         = errors.New("understanding: already resolved")
	ErrInvalidUnderstandingStatus           = errors.New("understanding: invalid status")
	ErrUnderstandingResponseStatusNotTerminal = errors.New("understanding: response status must be terminal")
)

// validateCheckpoint checks required fields.
func validateCheckpoint(cp UnderstandingCheckpoint) error {
	if cp.TenantID == "" {
		return errors.New("understanding: tenantID required")
	}
	if cp.TaskSummary == "" {
		return errors.New("understanding: taskSummary required")
	}
	return nil
}

// UnderstandingCheckpointGate is the persistence + signaling interface.
// Implementations must be concurrent-safe and honor first-resolve-wins.
type UnderstandingCheckpointGate interface {
	// Arm creates a checkpoint with status=pending. ID + ArmedAt assigned.
	Arm(ctx context.Context, cp UnderstandingCheckpoint) (UnderstandingCheckpoint, error)
	// Await blocks until checkpoint resolves OR ctx cancels OR deadline elapses.
	Await(ctx context.Context, id uuid.UUID) (UnderstandingResponse, error)
	// Respond records the human reply. First-respond-wins.
	Respond(ctx context.Context, res UnderstandingResponse) error
	// Find returns the current state without blocking.
	Find(ctx context.Context, id uuid.UUID) (UnderstandingCheckpoint, *UnderstandingResponse, error)
	// CountByStatus returns histogram for a tenant.
	CountByStatus(ctx context.Context, tenantID string) (map[UnderstandingStatus]int, error)
}

// --- InMemoryUnderstandingCheckpointGate ---

type understandingEntry struct {
	cp        UnderstandingCheckpoint
	response  *UnderstandingResponse
	resolveCh chan UnderstandingResponse
	resolved  bool
}

// InMemoryUnderstandingCheckpointGate is the default in-memory gate.
type InMemoryUnderstandingCheckpointGate struct {
	mu      sync.Mutex
	entries map[uuid.UUID]*understandingEntry
}

// NewInMemoryUnderstandingCheckpointGate creates an empty gate.
func NewInMemoryUnderstandingCheckpointGate() *InMemoryUnderstandingCheckpointGate {
	return &InMemoryUnderstandingCheckpointGate{
		entries: map[uuid.UUID]*understandingEntry{},
	}
}

// Arm creates and registers a checkpoint.
func (g *InMemoryUnderstandingCheckpointGate) Arm(ctx context.Context, cp UnderstandingCheckpoint) (UnderstandingCheckpoint, error) {
	if err := ctx.Err(); err != nil {
		return UnderstandingCheckpoint{}, err
	}
	if err := validateCheckpoint(cp); err != nil {
		return UnderstandingCheckpoint{}, err
	}
	if len(cp.TaskSummary) > 300 {
		cp.TaskSummary = cp.TaskSummary[:297] + "..."
	}
	cp.ID = uuid.New()
	cp.ArmedAt = time.Now()
	cp.Status = UnderstandingStatusPending

	entry := &understandingEntry{
		cp:        cp,
		resolveCh: make(chan UnderstandingResponse, 1),
	}
	g.mu.Lock()
	g.entries[cp.ID] = entry
	g.mu.Unlock()
	return cp, nil
}

// Await blocks until the checkpoint resolves, ctx cancels, or deadline elapses.
func (g *InMemoryUnderstandingCheckpointGate) Await(ctx context.Context, id uuid.UUID) (UnderstandingResponse, error) {
	g.mu.Lock()
	entry, ok := g.entries[id]
	g.mu.Unlock()
	if !ok {
		return UnderstandingResponse{}, ErrUnderstandingCheckpointNotFound
	}

	g.mu.Lock()
	if entry.resolved && entry.response != nil {
		res := *entry.response
		g.mu.Unlock()
		return res, nil
	}
	deadline := entry.cp.ConfirmationDeadline
	g.mu.Unlock()

	var deadlineCh <-chan time.Time
	if !deadline.IsZero() {
		d := time.Until(deadline)
		if d <= 0 {
			res := UnderstandingResponse{
				CheckpointID: id,
				Status:       UnderstandingStatusTimedOut,
				RespondedAt:  time.Now(),
			}
			_ = g.Respond(ctx, res)
			return res, nil
		}
		t := time.NewTimer(d)
		defer t.Stop()
		deadlineCh = t.C
	}

	select {
	case res := <-entry.resolveCh:
		return res, nil
	case <-deadlineCh:
		res := UnderstandingResponse{
			CheckpointID: id,
			Status:       UnderstandingStatusTimedOut,
			RespondedAt:  time.Now(),
		}
		_ = g.Respond(ctx, res)
		return res, nil
	case <-ctx.Done():
		return UnderstandingResponse{}, fmt.Errorf("understanding: await ctx cancelled: %w", ctx.Err())
	}
}

// Respond records a reply. First-respond-wins.
func (g *InMemoryUnderstandingCheckpointGate) Respond(ctx context.Context, res UnderstandingResponse) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !IsValidUnderstandingStatus(res.Status) {
		return fmt.Errorf("%w: %q", ErrInvalidUnderstandingStatus, res.Status)
	}
	if !IsTerminalUnderstandingStatus(res.Status) {
		return fmt.Errorf("%w: %q", ErrUnderstandingResponseStatusNotTerminal, res.Status)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	entry, ok := g.entries[res.CheckpointID]
	if !ok {
		return ErrUnderstandingCheckpointNotFound
	}
	if entry.resolved {
		return ErrUnderstandingAlreadyResolved
	}
	if res.RespondedAt.IsZero() {
		res.RespondedAt = time.Now()
	}
	entry.cp.Status = res.Status
	entry.resolved = true
	resCopy := res
	entry.response = &resCopy
	select {
	case entry.resolveCh <- res:
	default:
	}
	return nil
}

// Find returns a snapshot.
func (g *InMemoryUnderstandingCheckpointGate) Find(ctx context.Context, id uuid.UUID) (UnderstandingCheckpoint, *UnderstandingResponse, error) {
	if err := ctx.Err(); err != nil {
		return UnderstandingCheckpoint{}, nil, err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	entry, ok := g.entries[id]
	if !ok {
		return UnderstandingCheckpoint{}, nil, ErrUnderstandingCheckpointNotFound
	}
	if entry.resolved {
		resCopy := *entry.response
		return entry.cp, &resCopy, nil
	}
	return entry.cp, nil, nil
}

// CountByStatus returns histogram. Always 5 keys.
func (g *InMemoryUnderstandingCheckpointGate) CountByStatus(ctx context.Context, tenantID string) (map[UnderstandingStatus]int, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	hist := map[UnderstandingStatus]int{}
	for _, s := range allUnderstandingStatuses {
		hist[s] = 0
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, e := range g.entries {
		if e.cp.TenantID == tenantID {
			hist[e.cp.Status]++
		}
	}
	return hist, nil
}

// --- Renderers ---

// PlainText renders a single-line summary for terminals.
func (cp UnderstandingCheckpoint) PlainText() string {
	return fmt.Sprintf("[UNDERSTAND %s] %s — %d assumptions, %d next actions",
		cp.Status, cp.TaskSummary, len(cp.KeyAssumptions), len(cp.NextActions))
}

// Markdown renders a multi-line block for chat UI / approval surface.
func (cp UnderstandingCheckpoint) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Understanding Checkpoint — %s\n\n", cp.Status)
	fmt.Fprintf(&b, "**Task:** %s\n\n", cp.TaskSummary)
	if len(cp.KeyAssumptions) > 0 {
		b.WriteString("### Key Assumptions\n")
		for _, a := range cp.KeyAssumptions {
			fmt.Fprintf(&b, "- %s\n", a)
		}
		b.WriteString("\n")
	}
	if len(cp.NextActions) > 0 {
		b.WriteString("### Next Actions\n")
		for i, a := range cp.NextActions {
			fmt.Fprintf(&b, "%d. %s\n", i+1, a)
		}
		b.WriteString("\n")
	}
	if cp.Status == UnderstandingStatusPending {
		b.WriteString("**Please confirm, correct, or abandon.**\n")
	}
	return b.String()
}
