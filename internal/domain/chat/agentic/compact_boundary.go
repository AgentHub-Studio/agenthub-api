package agentic

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// CTX-011 — Compact boundaries.
//
// PDF arXiv:2604.14228v1 §7.3 (compaction pipeline emits a BOUNDARY
// MARKER annotated with preserved-segment metadata: headUuid +
// anchorUuid + tailUuid; downstream transcript readers use these to
// PROJECT the post-compaction view) + §9.2 (resume-aware boundary
// reconstruction).
//
// Distinct from existing AgentHub plumbing:
//   - context.go ContextManager (CTX-010) = compaction lifecycle counter.
//   - chat.MessageTypeCompactSummary (PERSIST-008) = persists the
//     boundary message.
//   - compactboundary_bdd_test.go = ratifies persistence contract.
//   - compact_boundary.go (this file) = pure-domain BOUNDARY REGISTRY
//     + chain tracking + projection: tells the runtime "given full
//     transcript, here is the post-boundary view" + "what was the
//     previous boundary in this recompaction chain?".

// CompactBoundary records one compaction event. Each carries head/anchor/
// tail UUIDs from the source transcript so the runtime can project a
// boundary-aware view (drop pre-anchor; keep summary message + post-anchor).
type CompactBoundary struct {
	ID                 uuid.UUID `json:"id"`
	SessionID          uuid.UUID `json:"sessionId"`
	// SummaryMessageID is the chat.ChatMessage.ID of the persisted
	// MessageTypeCompactSummary (the boundary marker message itself).
	SummaryMessageID   uuid.UUID `json:"summaryMessageId"`
	// HeadUUID is the FIRST message in the original transcript that this
	// boundary summarizes (oldest summarized message).
	HeadUUID           uuid.UUID `json:"headUuid"`
	// AnchorUUID is the message at which "post-compaction view" begins.
	// AnchorUUID itself is preserved in the post view (boundary-INCLUSIVE).
	AnchorUUID         uuid.UUID `json:"anchorUuid"`
	// TailUUID is the LAST message preserved. Useful for resume to know
	// where the agent left off.
	TailUUID           uuid.UUID `json:"tailUuid"`
	// SummarizedCount is how many messages this boundary summarizes
	// (between HeadUUID and AnchorUUID exclusive).
	SummarizedCount    int       `json:"summarizedCount"`
	OccurredAt         time.Time `json:"occurredAt"`
	// PreviousBoundaryID links to the previous boundary in a recompaction
	// chain (uuid.Nil for the first boundary of the session).
	PreviousBoundaryID uuid.UUID `json:"previousBoundaryId,omitempty"`
	// ChainDepth is 1 for the first boundary, increments by 1 per
	// recompaction. Cached for O(1) lookups.
	ChainDepth         int       `json:"chainDepth"`
	// CumulativeSummarized is the sum of SummarizedCount across this
	// boundary's chain (helps cost analytics: "how many messages compacted total").
	CumulativeSummarized int     `json:"cumulativeSummarized"`
}

// Sentinels.
var (
	ErrCompactBoundaryNotFound       = errors.New("compact boundary: not found")
	ErrCompactBoundarySessionEmpty   = errors.New("compact boundary: session_id required")
	ErrCompactBoundarySummaryEmpty   = errors.New("compact boundary: summary_message_id required")
	ErrCompactBoundaryAnchorEmpty    = errors.New("compact boundary: anchor_uuid required")
	ErrCompactBoundaryHeadEmpty      = errors.New("compact boundary: head_uuid required")
	ErrCompactBoundaryTailEmpty      = errors.New("compact boundary: tail_uuid required")
	ErrCompactBoundaryNegativeCount  = errors.New("compact boundary: summarized_count must be ≥ 1")
	ErrCompactBoundaryChainBroken    = errors.New("compact boundary: chain integrity broken (previous_boundary_id missing)")
	ErrCompactBoundaryAnchorNotFound = errors.New("compact boundary: anchor_uuid not found in transcript")
)

// CompactBoundaryRegistry is the persistence interface.
type CompactBoundaryRegistry interface {
	Record(ctx context.Context, b CompactBoundary) (CompactBoundary, error)
	Find(ctx context.Context, id uuid.UUID) (CompactBoundary, error)
	FindLatestForSession(ctx context.Context, sessionID uuid.UUID) (CompactBoundary, error)
	ListChain(ctx context.Context, sessionID uuid.UUID) ([]CompactBoundary, error)
	VerifyChain(ctx context.Context, sessionID uuid.UUID) error
	DropBefore(ctx context.Context, sessionID uuid.UUID, before time.Time) (int, error)
}

// validateBoundary checks structural invariants pre-record.
func validateBoundary(b CompactBoundary) error {
	if b.SessionID == uuid.Nil {
		return ErrCompactBoundarySessionEmpty
	}
	if b.SummaryMessageID == uuid.Nil {
		return ErrCompactBoundarySummaryEmpty
	}
	if b.AnchorUUID == uuid.Nil {
		return ErrCompactBoundaryAnchorEmpty
	}
	if b.HeadUUID == uuid.Nil {
		return ErrCompactBoundaryHeadEmpty
	}
	if b.TailUUID == uuid.Nil {
		return ErrCompactBoundaryTailEmpty
	}
	if b.SummarizedCount < 1 {
		return ErrCompactBoundaryNegativeCount
	}
	return nil
}

// --- InMemoryCompactBoundaryRegistry ---

type InMemoryCompactBoundaryRegistry struct {
	mu         sync.Mutex
	boundaries map[uuid.UUID]CompactBoundary
	now        func() time.Time
}

// NewInMemoryCompactBoundaryRegistry returns a concurrent-safe registry.
func NewInMemoryCompactBoundaryRegistry() *InMemoryCompactBoundaryRegistry {
	return &InMemoryCompactBoundaryRegistry{
		boundaries: map[uuid.UUID]CompactBoundary{},
		now:        time.Now,
	}
}

// SetClock allows tests to inject a deterministic clock.
func (r *InMemoryCompactBoundaryRegistry) SetClock(clock func() time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.now = clock
}

// Record persists a new boundary. Auto-fills ID, OccurredAt, ChainDepth,
// and CumulativeSummarized by walking PreviousBoundaryID chain.
func (r *InMemoryCompactBoundaryRegistry) Record(ctx context.Context, b CompactBoundary) (CompactBoundary, error) {
	if err := ctx.Err(); err != nil {
		return CompactBoundary{}, err
	}
	if err := validateBoundary(b); err != nil {
		return CompactBoundary{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	if b.OccurredAt.IsZero() {
		b.OccurredAt = r.now()
	}

	// Derive chain depth + cumulative if PreviousBoundaryID set.
	if b.PreviousBoundaryID != uuid.Nil {
		prev, ok := r.boundaries[b.PreviousBoundaryID]
		if !ok {
			return CompactBoundary{}, fmt.Errorf("%w: prev=%s", ErrCompactBoundaryChainBroken, b.PreviousBoundaryID)
		}
		b.ChainDepth = prev.ChainDepth + 1
		b.CumulativeSummarized = prev.CumulativeSummarized + b.SummarizedCount
	} else {
		b.ChainDepth = 1
		b.CumulativeSummarized = b.SummarizedCount
	}

	r.boundaries[b.ID] = b
	return b, nil
}

// Find returns one boundary by ID.
func (r *InMemoryCompactBoundaryRegistry) Find(ctx context.Context, id uuid.UUID) (CompactBoundary, error) {
	if err := ctx.Err(); err != nil {
		return CompactBoundary{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.boundaries[id]
	if !ok {
		return CompactBoundary{}, ErrCompactBoundaryNotFound
	}
	return b, nil
}

// FindLatestForSession returns the most recent boundary (highest
// OccurredAt, tie-break by ChainDepth descending).
func (r *InMemoryCompactBoundaryRegistry) FindLatestForSession(ctx context.Context, sessionID uuid.UUID) (CompactBoundary, error) {
	if err := ctx.Err(); err != nil {
		return CompactBoundary{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	var latest CompactBoundary
	found := false
	for _, b := range r.boundaries {
		if b.SessionID != sessionID {
			continue
		}
		if !found ||
			b.OccurredAt.After(latest.OccurredAt) ||
			(b.OccurredAt.Equal(latest.OccurredAt) && b.ChainDepth > latest.ChainDepth) {
			latest = b
			found = true
		}
	}
	if !found {
		return CompactBoundary{}, ErrCompactBoundaryNotFound
	}
	return latest, nil
}

// ListChain returns boundaries for a session in CHAIN ORDER (oldest
// first → newest last). Walks ChainDepth ascending.
func (r *InMemoryCompactBoundaryRegistry) ListChain(ctx context.Context, sessionID uuid.UUID) ([]CompactBoundary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []CompactBoundary{}
	for _, b := range r.boundaries {
		if b.SessionID == sessionID {
			out = append(out, b)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ChainDepth < out[j].ChainDepth })
	return out, nil
}

// VerifyChain walks the chain and ensures every PreviousBoundaryID
// points to an actual boundary (no broken links). Returns nil if clean,
// ErrCompactBoundaryChainBroken on first orphan.
func (r *InMemoryCompactBoundaryRegistry) VerifyChain(ctx context.Context, sessionID uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	chain, err := r.ListChain(ctx, sessionID)
	if err != nil {
		return err
	}
	if len(chain) == 0 {
		return nil // empty chain is valid
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	for _, b := range chain {
		if b.PreviousBoundaryID == uuid.Nil {
			continue // chain root
		}
		if _, ok := r.boundaries[b.PreviousBoundaryID]; !ok {
			return fmt.Errorf("%w: boundary=%s prev=%s",
				ErrCompactBoundaryChainBroken, b.ID, b.PreviousBoundaryID)
		}
	}
	return nil
}

// DropBefore removes boundaries occurring strictly before the cutoff.
// Returns count removed. Does NOT preserve chain integrity if intermediate
// boundaries are dropped — caller should drop oldest-first only.
func (r *InMemoryCompactBoundaryRegistry) DropBefore(ctx context.Context, sessionID uuid.UUID, before time.Time) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	removed := 0
	for id, b := range r.boundaries {
		if b.SessionID != sessionID {
			continue
		}
		if b.OccurredAt.Before(before) {
			delete(r.boundaries, id)
			removed++
		}
	}
	return removed, nil
}

// --- BoundaryAwareTranscriptView ---
//
// Pure projection helper: given a transcript (slice of message UUIDs in
// arrival order) + the latest boundary, returns the post-boundary view
// (anchor + tail). Pre-anchor messages are dropped.

// ProjectionResult is the output of ProjectTranscript.
type ProjectionResult struct {
	// SummaryMessageID is the boundary's summary message — emit FIRST.
	SummaryMessageID  uuid.UUID   `json:"summaryMessageId"`
	// PreservedMessageIDs are the post-anchor messages from the original
	// transcript, in arrival order, INCLUDING the anchor.
	PreservedMessageIDs []uuid.UUID `json:"preservedMessageIds"`
	// DroppedCount is how many pre-anchor messages were dropped.
	DroppedCount      int         `json:"droppedCount"`
}

// ProjectTranscript builds the post-boundary view from a transcript and
// boundary. Returns ErrCompactBoundaryAnchorNotFound if anchor isn't in
// the transcript (transcript out-of-sync with boundary metadata).
func ProjectTranscript(transcript []uuid.UUID, boundary CompactBoundary) (ProjectionResult, error) {
	if err := validateBoundary(boundary); err != nil {
		return ProjectionResult{}, err
	}
	anchorIdx := -1
	for i, id := range transcript {
		if id == boundary.AnchorUUID {
			anchorIdx = i
			break
		}
	}
	if anchorIdx < 0 {
		return ProjectionResult{}, fmt.Errorf("%w: anchor=%s",
			ErrCompactBoundaryAnchorNotFound, boundary.AnchorUUID)
	}
	preserved := make([]uuid.UUID, len(transcript)-anchorIdx)
	copy(preserved, transcript[anchorIdx:])
	return ProjectionResult{
		SummaryMessageID:    boundary.SummaryMessageID,
		PreservedMessageIDs: preserved,
		DroppedCount:        anchorIdx,
	}, nil
}

// IsRootBoundary returns true when the boundary is the first in its session.
func (b CompactBoundary) IsRootBoundary() bool {
	return b.PreviousBoundaryID == uuid.Nil
}

// IsRecompaction returns true when the boundary follows a previous one.
func (b CompactBoundary) IsRecompaction() bool {
	return b.PreviousBoundaryID != uuid.Nil
}
