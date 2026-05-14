package agentic

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// GOV-003 — Human control checkpoints.
//
// PDF arXiv:2604.14228v1 Section 11 — "human-in-the-loop checkpoints
// for destructive operations, large purchases, irreversible actions".
// Section 5.3 — "permission_request hook is one of 5 first-class safety
// hooks". Section 6.1 — explicit user review steps.
//
// Distinction from neighbouring abstractions in the codebase:
//
//   - Elicitation (`elicitation.go`) is the WIRE/QUEUE layer for any
//     async user-facing prompt — it does not classify the prompt.
//   - PermissionConfirm (`permission.go`) is a per-tool runtime ASK
//     decided at evaluation time — it is a yes/no on a single call.
//   - PolicyRequireApproval (`policyengine.go`) is a POLICY-LEVEL
//     requirement — "this class of action needs human approval".
//   - Checkpoint (this file) is the TYPED PAUSE-POINT model: a
//     classified, audit-friendly checkpoint the runner pauses on
//     before executing a class of action. Each checkpoint kind is
//     bounded and stable (analytics aggregate by kind).
//
// A Checkpoint is the bridge between policy ("approval required") and
// elicitation ("ask the user") — runners arm a checkpoint, await the
// human decision, and only then proceed.

// CheckpointKind classifies a checkpoint. The set is BOUNDED and
// STABLE — analytics dashboards / audit reports group by kind so the
// strings cannot drift over time.
type CheckpointKind string

const (
	// CheckpointPreDestructive — about to perform an action that
	// destroys data (DROP, DELETE without WHERE, file removal,
	// repository force-push).
	CheckpointPreDestructive CheckpointKind = "pre_destructive"
	// CheckpointPreIrreversible — about to perform an action that
	// cannot be undone (publish, send notification, deploy).
	CheckpointPreIrreversible CheckpointKind = "pre_irreversible"
	// CheckpointPreExternalSend — about to send data outside the
	// tenant's perimeter (email, webhook, third-party API).
	CheckpointPreExternalSend CheckpointKind = "pre_external_send"
	// CheckpointPrePIIExport — about to export PII data (download,
	// share, embed in external prompt).
	CheckpointPrePIIExport CheckpointKind = "pre_pii_export"
	// CheckpointPreCostThreshold — about to incur cost above a
	// configured threshold (large LLM call, bulk operation).
	CheckpointPreCostThreshold CheckpointKind = "pre_cost_threshold"
	// CheckpointPrePolicyRequiresApproval — generic checkpoint armed
	// when GOV-002 PolicyRequireApproval is returned by the engine.
	CheckpointPrePolicyRequiresApproval CheckpointKind = "pre_policy_requires_approval"
)

// allCheckpointKinds is the closed bounded set.
var allCheckpointKinds = []CheckpointKind{
	CheckpointPreDestructive,
	CheckpointPreIrreversible,
	CheckpointPreExternalSend,
	CheckpointPrePIIExport,
	CheckpointPreCostThreshold,
	CheckpointPrePolicyRequiresApproval,
}

// IsValidCheckpointKind returns true for the closed bounded set.
func IsValidCheckpointKind(k CheckpointKind) bool {
	for _, kk := range allCheckpointKinds {
		if k == kk {
			return true
		}
	}
	return false
}

// AllCheckpointKinds returns a copy of the closed set. Used by audit
// dashboards to render axis labels stably.
func AllCheckpointKinds() []CheckpointKind {
	out := make([]CheckpointKind, len(allCheckpointKinds))
	copy(out, allCheckpointKinds)
	return out
}

// CheckpointDecision is the bounded outcome of an awaited checkpoint.
type CheckpointDecision string

const (
	// CheckpointApproved — human said YES, proceed.
	CheckpointApproved CheckpointDecision = "approved"
	// CheckpointRejected — human said NO, do not proceed.
	CheckpointRejected CheckpointDecision = "rejected"
	// CheckpointCancelled — the checkpoint was cancelled by the
	// runner (e.g. parent run terminated) or by an admin.
	CheckpointCancelled CheckpointDecision = "cancelled"
	// CheckpointTimedOut — no decision arrived within the deadline.
	CheckpointTimedOut CheckpointDecision = "timed_out"
)

// IsTerminalCheckpointDecision returns true for any of the 4 valid
// terminal decisions.
func IsTerminalCheckpointDecision(d CheckpointDecision) bool {
	switch d {
	case CheckpointApproved, CheckpointRejected, CheckpointCancelled, CheckpointTimedOut:
		return true
	}
	return false
}

// Checkpoint represents an armed pause-point awaiting human decision.
type Checkpoint struct {
	// ID is the storage-layer identifier (assigned at Arm time).
	ID uuid.UUID
	// Kind classifies the checkpoint. Always in the bounded set.
	Kind CheckpointKind
	// TenantID scopes the checkpoint to a tenant. Required.
	TenantID string
	// AgentID identifies the agent armed the checkpoint.
	AgentID string
	// RunID correlates back to the originating run trace.
	RunID string
	// Reason is a short human-readable explanation displayed to the
	// approver (e.g. "DROP TABLE on production database").
	Reason string
	// Context is a free-form map of audit context (tool name, target
	// resource, cost estimate, etc.). Surfaces in the approval UI.
	Context map[string]string
	// ArmedAt is when the checkpoint was created.
	ArmedAt time.Time
	// Deadline is when the checkpoint will time out if no decision
	// arrives. Zero = no deadline (infinite wait — discouraged).
	Deadline time.Time
}

// CheckpointResolution wraps the human decision plus audit context.
type CheckpointResolution struct {
	CheckpointID uuid.UUID
	Decision     CheckpointDecision
	// DecidedBy identifies the user who resolved the checkpoint.
	// Empty when the resolution is system-driven (cancel, timeout).
	DecidedBy string
	// Reason — required for Rejected, optional for Approved/Cancelled.
	// Surfaces in the audit log so future-them know WHY the rejection.
	Reason string
	// DecidedAt is the wall-clock timestamp.
	DecidedAt time.Time
}

// ErrCheckpointNotFound — sentinel for unknown checkpoint IDs.
var ErrCheckpointNotFound = errors.New("checkpoint not found")

// ErrInvalidCheckpointKind — sentinel for kinds outside the bounded set.
var ErrInvalidCheckpointKind = errors.New("invalid checkpoint kind")

// ErrCheckpointAlreadyResolved — sentinel for double-resolve attempts.
var ErrCheckpointAlreadyResolved = errors.New("checkpoint already resolved")

// CheckpointGate is the interface the runner uses to arm and await
// human-control checkpoints.
//
// Implementations MUST:
//   - Be concurrent-safe (multiple runners may arm checkpoints simultaneously).
//   - Reject invalid CheckpointKind (defensive against typos).
//   - Reject empty TenantID (multi-tenant isolation).
//   - Treat Resolve as IDEMPOTENT-after-once — first decision wins;
//     duplicate resolves return ErrCheckpointAlreadyResolved.
//   - Honor ctx cancellation in Await.
//   - Time out per Checkpoint.Deadline if no resolution arrives.
type CheckpointGate interface {
	// Arm creates a Checkpoint, returns the populated record (with ID
	// + ArmedAt set). Caller passes the kind, reason, deadline, etc.
	Arm(ctx context.Context, cp Checkpoint) (Checkpoint, error)

	// Await blocks until the checkpoint is resolved (approved /
	// rejected / cancelled / timed out) OR ctx is cancelled.
	Await(ctx context.Context, id uuid.UUID) (CheckpointResolution, error)

	// Resolve records a human decision for an armed checkpoint.
	// First call wins; subsequent calls return ErrCheckpointAlreadyResolved.
	Resolve(ctx context.Context, res CheckpointResolution) error

	// Find returns the current state of a checkpoint without blocking.
	// Returns ErrCheckpointNotFound when the ID is unknown.
	Find(ctx context.Context, id uuid.UUID) (Checkpoint, *CheckpointResolution, error)
}

// --- InMemoryCheckpointGate ---

type checkpointEntry struct {
	cp         Checkpoint
	resolution *CheckpointResolution
	resolveCh  chan CheckpointResolution
	resolved   bool
}

// InMemoryCheckpointGate is a concurrent-safe in-memory implementation
// suitable for tests, dev, and single-process deployments. Production
// should use a SQL-backed gate (separate file, same interface).
type InMemoryCheckpointGate struct {
	mu      sync.Mutex
	entries map[uuid.UUID]*checkpointEntry
}

// NewInMemoryCheckpointGate creates an empty in-memory gate.
func NewInMemoryCheckpointGate() *InMemoryCheckpointGate {
	return &InMemoryCheckpointGate{entries: map[uuid.UUID]*checkpointEntry{}}
}

// Arm creates a Checkpoint and registers it. ID and ArmedAt are
// assigned by the gate; caller-provided values are overwritten.
func (g *InMemoryCheckpointGate) Arm(ctx context.Context, cp Checkpoint) (Checkpoint, error) {
	if err := ctx.Err(); err != nil {
		return Checkpoint{}, fmt.Errorf("checkpoint: ctx cancelled: %w", err)
	}
	if !IsValidCheckpointKind(cp.Kind) {
		return Checkpoint{}, fmt.Errorf("%w: %q", ErrInvalidCheckpointKind, cp.Kind)
	}
	if cp.TenantID == "" {
		return Checkpoint{}, fmt.Errorf("checkpoint: TenantID required")
	}
	cp.ID = uuid.New()
	cp.ArmedAt = time.Now()

	entry := &checkpointEntry{
		cp:        cp,
		resolveCh: make(chan CheckpointResolution, 1),
	}

	g.mu.Lock()
	g.entries[cp.ID] = entry
	g.mu.Unlock()
	return cp, nil
}

// Await blocks until the checkpoint resolves, ctx cancels, or the
// deadline elapses. The deadline path produces a CheckpointTimedOut
// resolution that is also persisted (so subsequent Find calls see it).
func (g *InMemoryCheckpointGate) Await(ctx context.Context, id uuid.UUID) (CheckpointResolution, error) {
	g.mu.Lock()
	entry, ok := g.entries[id]
	g.mu.Unlock()
	if !ok {
		return CheckpointResolution{}, ErrCheckpointNotFound
	}

	// Already resolved? Return the recorded resolution immediately.
	g.mu.Lock()
	if entry.resolved && entry.resolution != nil {
		res := *entry.resolution
		g.mu.Unlock()
		return res, nil
	}
	deadline := entry.cp.Deadline
	g.mu.Unlock()

	var deadlineCh <-chan time.Time
	if !deadline.IsZero() {
		d := time.Until(deadline)
		if d <= 0 {
			// Deadline already past. Synthesize a timeout.
			res := CheckpointResolution{
				CheckpointID: id,
				Decision:     CheckpointTimedOut,
				DecidedAt:    time.Now(),
			}
			_ = g.Resolve(ctx, res) // best-effort persist
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
		res := CheckpointResolution{
			CheckpointID: id,
			Decision:     CheckpointTimedOut,
			DecidedAt:    time.Now(),
		}
		_ = g.Resolve(ctx, res)
		return res, nil
	case <-ctx.Done():
		return CheckpointResolution{}, fmt.Errorf("checkpoint: await ctx cancelled: %w", ctx.Err())
	}
}

// Resolve records a decision. First-write-wins; subsequent calls return
// ErrCheckpointAlreadyResolved (so retries don't accidentally flip
// approve→reject after the user already decided).
func (g *InMemoryCheckpointGate) Resolve(ctx context.Context, res CheckpointResolution) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("checkpoint: resolve ctx cancelled: %w", err)
	}
	if !IsTerminalCheckpointDecision(res.Decision) {
		return fmt.Errorf("checkpoint: invalid decision %q", res.Decision)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	entry, ok := g.entries[res.CheckpointID]
	if !ok {
		return ErrCheckpointNotFound
	}
	if entry.resolved {
		return ErrCheckpointAlreadyResolved
	}
	if res.DecidedAt.IsZero() {
		res.DecidedAt = time.Now()
	}
	entry.resolved = true
	resCopy := res
	entry.resolution = &resCopy

	// Send to any waiter (non-blocking — buffer of 1).
	select {
	case entry.resolveCh <- res:
	default:
	}
	return nil
}

// Find returns the checkpoint and (if resolved) its resolution.
func (g *InMemoryCheckpointGate) Find(ctx context.Context, id uuid.UUID) (Checkpoint, *CheckpointResolution, error) {
	if err := ctx.Err(); err != nil {
		return Checkpoint{}, nil, fmt.Errorf("checkpoint: find ctx cancelled: %w", err)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	entry, ok := g.entries[id]
	if !ok {
		return Checkpoint{}, nil, ErrCheckpointNotFound
	}
	if entry.resolved {
		resCopy := *entry.resolution
		return entry.cp, &resCopy, nil
	}
	return entry.cp, nil, nil
}
