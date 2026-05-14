package agentic

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBDD_SessionFork(t *testing.T) {
	t.Run("Scenario_UserBranchesConversationToExploreAlternative", func(t *testing.T) {
		// Given a chat session at turn 5,
		// And the user wants to explore an alternative path from turn 3,
		// When the forker evaluates with full_copy strategy,
		// Then a new SessionID is allocated and the prior 4 turns are
		// shared with the original.
		sid := uuid.New()
		f := NewSessionForker()
		req := SessionForkRequest{
			OriginalSessionID: sid, ForkedAtTurn: 3,
			Strategy: SessionForkFullCopy, Reason: "explore alternative reply",
		}
		meta := SessionMetadata{SessionID: sid, CurrentTurn: 5}
		res, err := f.Evaluate(req, meta)
		require.NoError(t, err)
		assert.True(t, res.Allowed)
		assert.NotEqual(t, uuid.Nil, res.NewSessionID)
		assert.NotEqual(t, sid, res.NewSessionID)
	})

	t.Run("Scenario_CannotForkSealedSession", func(t *testing.T) {
		// Given the original session was archived/terminated,
		// When the user tries to fork,
		// Then the request is denied with session_sealed restriction.
		sid := uuid.New()
		f := NewSessionForker()
		req := SessionForkRequest{
			OriginalSessionID: sid, ForkedAtTurn: 1,
			Strategy: SessionForkFullCopy, Reason: "x",
		}
		meta := SessionMetadata{SessionID: sid, CurrentTurn: 5, Sealed: true}
		res, _ := f.Evaluate(req, meta)
		assert.False(t, res.Allowed)
		assert.Equal(t, SessionForkRestrictionSessionSealed, res.Restriction)
	})

	t.Run("Scenario_CannotForkAtFutureTurn", func(t *testing.T) {
		// Given the original session has 5 turns,
		// When the user requests a fork at turn 10,
		// Then the request is denied with past_current_turn restriction.
		sid := uuid.New()
		f := NewSessionForker()
		req := SessionForkRequest{
			OriginalSessionID: sid, ForkedAtTurn: 10,
			Strategy: SessionForkFullCopy, Reason: "x",
		}
		meta := SessionMetadata{SessionID: sid, CurrentTurn: 5}
		res, _ := f.Evaluate(req, meta)
		assert.False(t, res.Allowed)
		assert.Equal(t, SessionForkRestrictionPastCurrentTurn, res.Restriction)
	})

	t.Run("Scenario_ForkingInsideCompactedPrefixRequiresSnapshotIsolated", func(t *testing.T) {
		// Given the session was compacted up to turn 3,
		// And the user wants to fork at turn 2 (inside compacted prefix),
		// When the strategy is full_copy,
		// Then the request is denied — compacted turns no longer exist
		// in raw form.
		sid := uuid.New()
		f := NewSessionForker()
		req := SessionForkRequest{
			OriginalSessionID: sid, ForkedAtTurn: 2,
			Strategy: SessionForkFullCopy, Reason: "x",
		}
		meta := SessionMetadata{SessionID: sid, CurrentTurn: 5, CompactedUpTo: 3}
		res, _ := f.Evaluate(req, meta)
		assert.False(t, res.Allowed)
		assert.Equal(t, SessionForkRestrictionPastCompactBoundary, res.Restriction)
	})

	t.Run("Scenario_SnapshotIsolatedAllowsForkInsideCompactedPrefix", func(t *testing.T) {
		// Given the same compacted state,
		// When strategy is snapshot_isolated AND a hash function is provided,
		// Then the fork is allowed and the shared prefix hash is recorded.
		sid := uuid.New()
		f := NewSessionForker()
		req := SessionForkRequest{
			OriginalSessionID: sid, ForkedAtTurn: 2,
			Strategy: SessionForkSnapshotIsolated, Reason: "x",
		}
		meta := SessionMetadata{
			SessionID: sid, CurrentTurn: 5, CompactedUpTo: 3,
			TranscriptPrefixHashFn: func(upto int) (string, error) {
				return "snapshot-hash-abc", nil
			},
		}
		res, err := f.Evaluate(req, meta)
		require.NoError(t, err)
		assert.True(t, res.Allowed)
		assert.Equal(t, "snapshot-hash-abc", res.SharedTranscriptPrefixHash)
	})

	t.Run("Scenario_AuditReasonRequiredForCompliance", func(t *testing.T) {
		// Given audit-strict tenants need a Reason on every fork,
		// When the request has no reason,
		// Then validation rejects upfront.
		sid := uuid.New()
		f := NewSessionForker()
		req := SessionForkRequest{
			OriginalSessionID: sid, ForkedAtTurn: 1,
			Strategy: SessionForkFullCopy,
		}
		meta := SessionMetadata{SessionID: sid, CurrentTurn: 5}
		_, err := f.Evaluate(req, meta)
		assert.ErrorIs(t, err, ErrSessionForkEmptyReason)
	})

	t.Run("Scenario_MetadataMismatchPreventsCrossSessionFork", func(t *testing.T) {
		// Given the request and metadata point at different sessions
		// (caller bug or replay attack),
		// When the forker validates,
		// Then it rejects to prevent silent cross-session fork.
		f := NewSessionForker()
		req := SessionForkRequest{
			OriginalSessionID: uuid.New(), ForkedAtTurn: 1,
			Strategy: SessionForkFullCopy, Reason: "x",
		}
		meta := SessionMetadata{SessionID: uuid.New(), CurrentTurn: 5}
		_, err := f.Evaluate(req, meta)
		assert.ErrorIs(t, err, ErrSessionForkMetadataMismatch)
	})

	t.Run("Scenario_BranchPointerSkipsHashOverhead", func(t *testing.T) {
		// Given branch_pointer assumes the original is immutable,
		// When the forker evaluates,
		// Then no transcript prefix hash is computed — cheaper than
		// snapshot_isolated.
		sid := uuid.New()
		f := NewSessionForker()
		hashCalls := 0
		req := SessionForkRequest{
			OriginalSessionID: sid, ForkedAtTurn: 3,
			Strategy: SessionForkBranchPointer, Reason: "x",
		}
		meta := SessionMetadata{
			SessionID: sid, CurrentTurn: 5,
			TranscriptPrefixHashFn: func(upto int) (string, error) {
				hashCalls++
				return "", nil
			},
		}
		res, _ := f.Evaluate(req, meta)
		assert.True(t, res.Allowed)
		assert.Equal(t, 0, hashCalls)
	})

	t.Run("Scenario_HashFunctionFailureFailsClosed", func(t *testing.T) {
		// Given snapshot_isolated needs a hash and the hash impl errors,
		// When the forker evaluates,
		// Then the error propagates — fork is NOT silently created
		// without the audit hash.
		sid := uuid.New()
		f := NewSessionForker()
		req := SessionForkRequest{
			OriginalSessionID: sid, ForkedAtTurn: 1,
			Strategy: SessionForkSnapshotIsolated, Reason: "x",
		}
		meta := SessionMetadata{
			SessionID: sid, CurrentTurn: 5,
			TranscriptPrefixHashFn: func(upto int) (string, error) {
				return "", assertErrSF("hash backend down")
			},
		}
		_, err := f.Evaluate(req, meta)
		assert.ErrorIs(t, err, ErrSessionForkHashFailed)
	})

	t.Run("Scenario_HashTranscriptPrefixHelperRoundTrips", func(t *testing.T) {
		// Given the convenience helper computes a stable hash,
		// When two callers hash the same prefix,
		// Then they get identical 64-char hex strings — audit trails
		// from different agents can be cross-referenced.
		turns := []string{"hello", "world", "again"}
		h1 := HashTranscriptPrefix(turns, 2)
		h2 := HashTranscriptPrefix(turns, 2)
		assert.Equal(t, h1, h2)
		assert.Equal(t, 64, len(h1))
	})
}

type sfAssertErr string

func (e sfAssertErr) Error() string { return string(e) }
func assertErrSF(s string) error    { return sfAssertErr(s) }
