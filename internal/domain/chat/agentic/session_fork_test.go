package agentic

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionFork_IsValidStrategy(t *testing.T) {
	for _, s := range allSessionForkStrategies {
		assert.True(t, IsValidSessionForkStrategy(s))
	}
	assert.False(t, IsValidSessionForkStrategy(SessionForkStrategy("nope")))
}

func TestSessionFork_IsValidRestriction(t *testing.T) {
	for _, r := range allSessionForkRestrictions {
		assert.True(t, IsValidSessionForkRestriction(r))
	}
	assert.False(t, IsValidSessionForkRestriction(SessionForkRestriction("nope")))
}

func TestSessionFork_RequestValidateEmptyOriginal(t *testing.T) {
	r := SessionForkRequest{Strategy: SessionForkFullCopy, Reason: "x"}
	assert.ErrorIs(t, r.Validate(), ErrSessionForkEmptyOriginal)
}

func TestSessionFork_RequestValidateNegativeTurn(t *testing.T) {
	r := SessionForkRequest{
		OriginalSessionID: uuid.New(),
		Strategy:          SessionForkFullCopy,
		ForkedAtTurn:      -1,
		Reason:            "x",
	}
	assert.ErrorIs(t, r.Validate(), ErrSessionForkNegativeTurn)
}

func TestSessionFork_RequestValidateBadStrategy(t *testing.T) {
	r := SessionForkRequest{
		OriginalSessionID: uuid.New(),
		Strategy:          SessionForkStrategy("nope"),
		Reason:            "x",
	}
	assert.ErrorIs(t, r.Validate(), ErrSessionForkBadStrategy)
}

func TestSessionFork_RequestValidateEmptyReason(t *testing.T) {
	r := SessionForkRequest{
		OriginalSessionID: uuid.New(),
		Strategy:          SessionForkFullCopy,
	}
	assert.ErrorIs(t, r.Validate(), ErrSessionForkEmptyReason)
}

func TestSessionFork_MetadataValidateEmptySession(t *testing.T) {
	m := SessionMetadata{}
	assert.ErrorIs(t, m.Validate(), ErrSessionForkEmptyOriginal)
}

func TestSessionFork_MetadataValidateNegativeTurn(t *testing.T) {
	m := SessionMetadata{SessionID: uuid.New(), CurrentTurn: -1}
	assert.ErrorIs(t, m.Validate(), ErrSessionForkNegativeTurn)
}

func TestSessionFork_EvaluateMetadataMismatch(t *testing.T) {
	f := NewSessionForker()
	req := SessionForkRequest{
		OriginalSessionID: uuid.New(),
		Strategy: SessionForkFullCopy, Reason: "x",
	}
	meta := SessionMetadata{SessionID: uuid.New(), CurrentTurn: 5}
	_, err := f.Evaluate(req, meta)
	assert.ErrorIs(t, err, ErrSessionForkMetadataMismatch)
}

func TestSessionFork_EvaluateAllowsRoutineFullCopy(t *testing.T) {
	sid := uuid.New()
	f := NewSessionForker()
	req := SessionForkRequest{
		OriginalSessionID: sid,
		ForkedAtTurn:      3, Strategy: SessionForkFullCopy, Reason: "branch exploration",
	}
	meta := SessionMetadata{SessionID: sid, CurrentTurn: 5}
	res, err := f.Evaluate(req, meta)
	require.NoError(t, err)
	assert.True(t, res.Allowed)
	assert.Equal(t, SessionForkRestrictionNone, res.Restriction)
	assert.NotEqual(t, uuid.Nil, res.NewSessionID)
}

func TestSessionFork_EvaluateRejectsPastCurrentTurn(t *testing.T) {
	sid := uuid.New()
	f := NewSessionForker()
	req := SessionForkRequest{
		OriginalSessionID: sid,
		ForkedAtTurn:      10, Strategy: SessionForkFullCopy, Reason: "x",
	}
	meta := SessionMetadata{SessionID: sid, CurrentTurn: 5}
	res, _ := f.Evaluate(req, meta)
	assert.False(t, res.Allowed)
	assert.Equal(t, SessionForkRestrictionPastCurrentTurn, res.Restriction)
}

func TestSessionFork_EvaluateRejectsSealedSession(t *testing.T) {
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
}

func TestSessionFork_EvaluateRejectsForkPastCompactionWithoutSnapshot(t *testing.T) {
	sid := uuid.New()
	f := NewSessionForker()
	req := SessionForkRequest{
		OriginalSessionID: sid, ForkedAtTurn: 2,
		Strategy: SessionForkFullCopy, Reason: "x",
	}
	// Session has been compacted up to turn 3; fork at turn 2 falls
	// in compacted prefix.
	meta := SessionMetadata{SessionID: sid, CurrentTurn: 5, CompactedUpTo: 3}
	res, _ := f.Evaluate(req, meta)
	assert.False(t, res.Allowed)
	assert.Equal(t, SessionForkRestrictionPastCompactBoundary, res.Restriction)
}

func TestSessionFork_EvaluateAllowsSnapshotIsolatedPastCompact(t *testing.T) {
	sid := uuid.New()
	f := NewSessionForker()
	hashCalls := 0
	req := SessionForkRequest{
		OriginalSessionID: sid, ForkedAtTurn: 2,
		Strategy: SessionForkSnapshotIsolated, Reason: "x",
	}
	meta := SessionMetadata{
		SessionID: sid, CurrentTurn: 5, CompactedUpTo: 3,
		TranscriptPrefixHashFn: func(upto int) (string, error) {
			hashCalls++
			return "abc123", nil
		},
	}
	res, err := f.Evaluate(req, meta)
	require.NoError(t, err)
	assert.True(t, res.Allowed)
	assert.Equal(t, "abc123", res.SharedTranscriptPrefixHash)
	assert.Equal(t, 1, hashCalls)
}

func TestSessionFork_EvaluateHashFailurePropagates(t *testing.T) {
	sid := uuid.New()
	f := NewSessionForker()
	req := SessionForkRequest{
		OriginalSessionID: sid, ForkedAtTurn: 1,
		Strategy: SessionForkSnapshotIsolated, Reason: "x",
	}
	meta := SessionMetadata{
		SessionID: sid, CurrentTurn: 5,
		TranscriptPrefixHashFn: func(upto int) (string, error) {
			return "", errors.New("hash impl broken")
		},
	}
	_, err := f.Evaluate(req, meta)
	assert.ErrorIs(t, err, ErrSessionForkHashFailed)
}

func TestSessionFork_EvaluateAllowedFullCopyDoesNotHash(t *testing.T) {
	sid := uuid.New()
	f := NewSessionForker()
	hashCalls := 0
	req := SessionForkRequest{
		OriginalSessionID: sid, ForkedAtTurn: 3,
		Strategy: SessionForkFullCopy, Reason: "x",
	}
	meta := SessionMetadata{
		SessionID: sid, CurrentTurn: 5,
		TranscriptPrefixHashFn: func(upto int) (string, error) {
			hashCalls++
			return "irrelevant", nil
		},
	}
	res, _ := f.Evaluate(req, meta)
	assert.True(t, res.Allowed)
	assert.Empty(t, res.SharedTranscriptPrefixHash)
	assert.Equal(t, 0, hashCalls)
}

func TestSessionFork_EvaluateBranchPointerNoHash(t *testing.T) {
	sid := uuid.New()
	f := NewSessionForker()
	req := SessionForkRequest{
		OriginalSessionID: sid, ForkedAtTurn: 3,
		Strategy: SessionForkBranchPointer, Reason: "x",
	}
	meta := SessionMetadata{SessionID: sid, CurrentTurn: 5}
	res, _ := f.Evaluate(req, meta)
	assert.True(t, res.Allowed)
	assert.Empty(t, res.SharedTranscriptPrefixHash)
}

func TestSessionFork_SetClockInjectsTimestamp(t *testing.T) {
	sid := uuid.New()
	f := NewSessionForker()
	stamp := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	f.SetClock(func() time.Time { return stamp })
	req := SessionForkRequest{
		OriginalSessionID: sid, ForkedAtTurn: 1,
		Strategy: SessionForkFullCopy, Reason: "x",
	}
	meta := SessionMetadata{SessionID: sid, CurrentTurn: 5}
	res, _ := f.Evaluate(req, meta)
	assert.Equal(t, stamp, res.DecidedAt)
}

func TestSessionFork_SetClockNilIsNoop(t *testing.T) {
	sid := uuid.New()
	f := NewSessionForker()
	stamp := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	f.SetClock(func() time.Time { return stamp })
	f.SetClock(nil) // must not clear the prior clock
	req := SessionForkRequest{
		OriginalSessionID: sid, ForkedAtTurn: 1,
		Strategy: SessionForkFullCopy, Reason: "x",
	}
	meta := SessionMetadata{SessionID: sid, CurrentTurn: 5}
	res, _ := f.Evaluate(req, meta)
	assert.Equal(t, stamp, res.DecidedAt)
}

func TestSessionFork_SetIDGeneratorDeterministic(t *testing.T) {
	sid := uuid.New()
	fixed := uuid.New()
	f := NewSessionForker()
	f.SetIDGenerator(func() uuid.UUID { return fixed })
	req := SessionForkRequest{
		OriginalSessionID: sid, ForkedAtTurn: 1,
		Strategy: SessionForkFullCopy, Reason: "x",
	}
	meta := SessionMetadata{SessionID: sid, CurrentTurn: 5}
	res, _ := f.Evaluate(req, meta)
	assert.Equal(t, fixed, res.NewSessionID)
}

func TestSessionFork_SetIDGeneratorNilIsNoop(t *testing.T) {
	sid := uuid.New()
	fixed := uuid.New()
	f := NewSessionForker()
	f.SetIDGenerator(func() uuid.UUID { return fixed })
	f.SetIDGenerator(nil)
	req := SessionForkRequest{
		OriginalSessionID: sid, ForkedAtTurn: 1,
		Strategy: SessionForkFullCopy, Reason: "x",
	}
	meta := SessionMetadata{SessionID: sid, CurrentTurn: 5}
	res, _ := f.Evaluate(req, meta)
	assert.Equal(t, fixed, res.NewSessionID)
}

func TestSessionFork_ForkAtTurnZeroAllowed(t *testing.T) {
	// Fork at turn 0 (very beginning) is legal.
	sid := uuid.New()
	f := NewSessionForker()
	req := SessionForkRequest{
		OriginalSessionID: sid, ForkedAtTurn: 0,
		Strategy: SessionForkFullCopy, Reason: "fresh-branch",
	}
	meta := SessionMetadata{SessionID: sid, CurrentTurn: 5}
	res, _ := f.Evaluate(req, meta)
	assert.True(t, res.Allowed)
}

func TestSessionFork_ForkAtCurrentTurnAllowed(t *testing.T) {
	// Fork at the latest turn (full carry-forward) is legal.
	sid := uuid.New()
	f := NewSessionForker()
	req := SessionForkRequest{
		OriginalSessionID: sid, ForkedAtTurn: 5,
		Strategy: SessionForkFullCopy, Reason: "x",
	}
	meta := SessionMetadata{SessionID: sid, CurrentTurn: 5}
	res, _ := f.Evaluate(req, meta)
	assert.True(t, res.Allowed)
}

func TestSessionFork_HashTranscriptPrefixDeterministic(t *testing.T) {
	turns := []string{"turn0", "turn1", "turn2"}
	h1 := HashTranscriptPrefix(turns, 2)
	h2 := HashTranscriptPrefix(turns, 2)
	assert.Equal(t, h1, h2)
	assert.Equal(t, 64, len(h1)) // sha256 hex
}

func TestSessionFork_HashTranscriptPrefixDifferentSubset(t *testing.T) {
	turns := []string{"turn0", "turn1", "turn2"}
	full := HashTranscriptPrefix(turns, 2)
	partial := HashTranscriptPrefix(turns, 1)
	assert.NotEqual(t, full, partial)
}

func TestSessionFork_HashTranscriptPrefixNegativeReturnsEmpty(t *testing.T) {
	turns := []string{"a"}
	assert.Empty(t, HashTranscriptPrefix(turns, -1))
}

func TestSessionFork_HashTranscriptPrefixUptoBeyondClipped(t *testing.T) {
	turns := []string{"a", "b"}
	// uptoTurn 99 → end clipped to len(turns).
	h1 := HashTranscriptPrefix(turns, 99)
	h2 := HashTranscriptPrefix(turns, 1)
	assert.Equal(t, h1, h2)
}

func TestSessionFork_ResultCarriesRequestAndReason(t *testing.T) {
	sid := uuid.New()
	f := NewSessionForker()
	req := SessionForkRequest{
		OriginalSessionID: sid, ForkedAtTurn: 2,
		Strategy: SessionForkBranchPointer, Reason: "audit-ctx",
	}
	meta := SessionMetadata{SessionID: sid, CurrentTurn: 5}
	res, _ := f.Evaluate(req, meta)
	assert.Equal(t, req, res.Request)
	assert.NotEmpty(t, res.Reason)
}
