package agentic

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// PERSIST-005a — Session-level fork.
//
// PDF arXiv:2604.14228v1 §7 (Persistence) — a session-level fork
// branches an existing session at a chosen turn boundary, producing a
// new SessionID with the prior transcript shared up to that point.
// Used for "/branch" workflows where the user wants to explore an
// alternative path without losing the original conversation.
//
// Distinct from existing AgentHub plumbing:
//   - PERSIST-005 (Fork/branch) — sub-agent fork already DONE; that
//     creates a child agent within an active session.
//   - PERSIST-005a (this file) — SESSION-level fork: new SessionID
//     entirely, prior transcript copied/snapshot up to a turn boundary.
//   - PERSIST-001 (Session transcript JSONL) is the WRITE substrate;
//     PERSIST-005a operates above it.
//   - PERSIST-008 (Compact boundary persistence) — forking past a
//     compact boundary requires extra care (snapshot semantics).

// SessionForkStrategy bounded enum declares HOW the fork relates to
// the original session.
type SessionForkStrategy string

const (
	// SessionForkFullCopy — full physical copy of every turn up to
	// fork point. New session is fully independent (mutations to
	// original do not affect the fork). Costliest but safest.
	SessionForkFullCopy SessionForkStrategy = "full_copy"
	// SessionForkBranchPointer — fork stores a pointer to the
	// original session's transcript up to ForkedAtTurn; subsequent
	// turns are stored only in the fork. Cheaper; relies on the
	// original being immutable up to that turn.
	SessionForkBranchPointer SessionForkStrategy = "branch_pointer"
	// SessionForkSnapshotIsolated — like branch_pointer but the
	// fork additionally records a snapshot hash of the shared prefix
	// so divergence can be detected if the original is mutated.
	SessionForkSnapshotIsolated SessionForkStrategy = "snapshot_isolated"
)

var allSessionForkStrategies = []SessionForkStrategy{
	SessionForkFullCopy, SessionForkBranchPointer, SessionForkSnapshotIsolated,
}

// IsValidSessionForkStrategy returns true for the bounded set.
func IsValidSessionForkStrategy(s SessionForkStrategy) bool {
	for _, v := range allSessionForkStrategies {
		if s == v {
			return true
		}
	}
	return false
}

// SessionForkRestriction bounded enum classifies why a fork attempt
// might be refused.
type SessionForkRestriction string

const (
	// SessionForkRestrictionNone — fork is allowed.
	SessionForkRestrictionNone SessionForkRestriction = "none"
	// SessionForkRestrictionPastCurrentTurn — caller requested a turn
	// number higher than the session's current turn.
	SessionForkRestrictionPastCurrentTurn SessionForkRestriction = "past_current_turn"
	// SessionForkRestrictionPastCompactBoundary — caller wants to
	// fork at a turn that's already been compacted; only
	// snapshot_isolated strategy is safe here.
	SessionForkRestrictionPastCompactBoundary SessionForkRestriction = "past_compact_boundary"
	// SessionForkRestrictionSessionSealed — original session was
	// sealed (e.g., terminated, archived). No further forks allowed.
	SessionForkRestrictionSessionSealed SessionForkRestriction = "session_sealed"
)

var allSessionForkRestrictions = []SessionForkRestriction{
	SessionForkRestrictionNone, SessionForkRestrictionPastCurrentTurn,
	SessionForkRestrictionPastCompactBoundary, SessionForkRestrictionSessionSealed,
}

// IsValidSessionForkRestriction returns true for the bounded set.
func IsValidSessionForkRestriction(r SessionForkRestriction) bool {
	for _, v := range allSessionForkRestrictions {
		if r == v {
			return true
		}
	}
	return false
}

// SessionForkRequest is the input the forker evaluates.
type SessionForkRequest struct {
	OriginalSessionID  uuid.UUID
	ForkedAtTurn       int     // 0-based turn index; fork includes turns [0..ForkedAtTurn]
	Strategy           SessionForkStrategy
	Reason             string  // human audit context
	RequestedAt        time.Time
}

// Validate enforces invariants.
func (r SessionForkRequest) Validate() error {
	if r.OriginalSessionID == uuid.Nil {
		return ErrSessionForkEmptyOriginal
	}
	if r.ForkedAtTurn < 0 {
		return ErrSessionForkNegativeTurn
	}
	if !IsValidSessionForkStrategy(r.Strategy) {
		return fmt.Errorf("%w: %q", ErrSessionForkBadStrategy, r.Strategy)
	}
	if strings.TrimSpace(r.Reason) == "" {
		return ErrSessionForkEmptyReason
	}
	return nil
}

// SessionMetadata is the slice of the original session's state the
// forker needs to decide eligibility.
type SessionMetadata struct {
	SessionID       uuid.UUID
	CurrentTurn     int
	CompactedUpTo   int  // turn index of the last compaction boundary; 0 = no compaction
	Sealed          bool
	TranscriptPrefixHashFn func(uptoTurn int) (string, error) // pluggable hash for snapshot isolation
}

// Validate enforces invariants.
func (s SessionMetadata) Validate() error {
	if s.SessionID == uuid.Nil {
		return ErrSessionForkEmptyOriginal
	}
	if s.CurrentTurn < 0 {
		return ErrSessionForkNegativeTurn
	}
	if s.CompactedUpTo < 0 {
		return ErrSessionForkNegativeTurn
	}
	return nil
}

// SessionForkResult is the audit-shaped output of a fork decision.
type SessionForkResult struct {
	Request                   SessionForkRequest
	NewSessionID              uuid.UUID
	Restriction               SessionForkRestriction
	Allowed                   bool
	SharedTranscriptPrefixHash string  // populated for snapshot_isolated
	DecidedAt                 time.Time
	Reason                    string
}

// SessionForker computes the fork eligibility + new session ID. Pure
// (the actual transcript copy/snapshot work is done downstream by the
// persistence adapter).
type SessionForker struct {
	mu  sync.RWMutex
	now func() time.Time
	gen func() uuid.UUID // for deterministic tests
}

// NewSessionForker builds a forker with default time.Now + uuid.New.
func NewSessionForker() *SessionForker {
	return &SessionForker{now: time.Now, gen: uuid.New}
}

// SetClock injects a clock for deterministic tests. nil is a defensive
// no-op.
func (f *SessionForker) SetClock(fn func() time.Time) {
	if fn == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = fn
}

// SetIDGenerator injects a UUID generator for deterministic tests. nil
// is a defensive no-op.
func (f *SessionForker) SetIDGenerator(fn func() uuid.UUID) {
	if fn == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gen = fn
}

// Evaluate decides whether the requested fork is allowed and computes
// the resolution. Pure function modulo the injected clock/generator.
func (f *SessionForker) Evaluate(req SessionForkRequest, meta SessionMetadata) (SessionForkResult, error) {
	if err := req.Validate(); err != nil {
		return SessionForkResult{}, err
	}
	if err := meta.Validate(); err != nil {
		return SessionForkResult{}, err
	}
	if req.OriginalSessionID != meta.SessionID {
		return SessionForkResult{}, fmt.Errorf("%w: request original %s != metadata session %s",
			ErrSessionForkMetadataMismatch, req.OriginalSessionID, meta.SessionID)
	}

	f.mu.RLock()
	now := f.now
	gen := f.gen
	f.mu.RUnlock()

	res := SessionForkResult{
		Request:   req,
		DecidedAt: now(),
	}

	// 1. Sealed sessions cannot be forked.
	if meta.Sealed {
		res.Restriction = SessionForkRestrictionSessionSealed
		res.Reason = "original session is sealed (archived/terminated)"
		return res, nil
	}

	// 2. Cannot fork at a turn beyond current.
	if req.ForkedAtTurn > meta.CurrentTurn {
		res.Restriction = SessionForkRestrictionPastCurrentTurn
		res.Reason = fmt.Sprintf("requested turn %d > current turn %d",
			req.ForkedAtTurn, meta.CurrentTurn)
		return res, nil
	}

	// 3. Fork past compact boundary requires snapshot_isolated.
	if meta.CompactedUpTo > 0 && req.ForkedAtTurn <= meta.CompactedUpTo &&
		req.Strategy != SessionForkSnapshotIsolated {
		res.Restriction = SessionForkRestrictionPastCompactBoundary
		res.Reason = fmt.Sprintf("turn %d is within compacted prefix (up to %d); only snapshot_isolated is safe",
			req.ForkedAtTurn, meta.CompactedUpTo)
		return res, nil
	}

	// Allowed. Compute the new session ID and (if needed) hash.
	res.Restriction = SessionForkRestrictionNone
	res.Allowed = true
	res.NewSessionID = gen()

	if req.Strategy == SessionForkSnapshotIsolated && meta.TranscriptPrefixHashFn != nil {
		hash, err := meta.TranscriptPrefixHashFn(req.ForkedAtTurn)
		if err != nil {
			return SessionForkResult{}, fmt.Errorf("%w: %v", ErrSessionForkHashFailed, err)
		}
		res.SharedTranscriptPrefixHash = hash
	}
	res.Reason = fmt.Sprintf("fork allowed at turn %d with strategy %q", req.ForkedAtTurn, req.Strategy)
	return res, nil
}

// HashTranscriptPrefix is a convenience helper computing SHA-256 of
// concatenated turn bodies up to (and including) uptoTurn. Callers
// can inject custom hash functions via SessionMetadata.TranscriptPrefixHashFn.
func HashTranscriptPrefix(turnBodies []string, uptoTurn int) string {
	if uptoTurn < 0 {
		return ""
	}
	end := uptoTurn + 1
	if end > len(turnBodies) {
		end = len(turnBodies)
	}
	h := sha256.New()
	for i := 0; i < end; i++ {
		h.Write([]byte(turnBodies[i]))
		h.Write([]byte{0}) // separator to avoid concat ambiguity
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Sentinel errors.
var (
	ErrSessionForkEmptyOriginal    = errors.New("session fork: original session id required")
	ErrSessionForkNegativeTurn     = errors.New("session fork: turn index must be >= 0")
	ErrSessionForkBadStrategy      = errors.New("session fork: invalid strategy")
	ErrSessionForkEmptyReason      = errors.New("session fork: reason required for audit trail")
	ErrSessionForkMetadataMismatch = errors.New("session fork: request original id != metadata session id")
	ErrSessionForkHashFailed       = errors.New("session fork: transcript prefix hash failed")
)
