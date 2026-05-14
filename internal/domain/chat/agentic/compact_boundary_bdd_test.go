package agentic

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBDD_CompactBoundary(t *testing.T) {
	t.Run("Scenario_RuntimeProjectsPostBoundaryViewToSendShortContextToLLM", func(t *testing.T) {
		// Given a session with 10 messages and a boundary at message 5,
		// When the runtime renders for the LLM,
		// Then it sees: [summary, msg5, msg6, msg7, msg8, msg9, msg10] —
		// pre-anchor messages dropped (PDF §7.3 boundary-aware projection).
		anchor := uuid.New()
		transcript := []uuid.UUID{}
		for i := 0; i < 4; i++ {
			transcript = append(transcript, uuid.New())
		}
		transcript = append(transcript, anchor)
		for i := 0; i < 5; i++ {
			transcript = append(transcript, uuid.New())
		}
		b := validBoundary()
		b.AnchorUUID = anchor

		got, err := ProjectTranscript(transcript, b)
		require.NoError(t, err)
		assert.Equal(t, 4, got.DroppedCount, "4 pre-anchor dropped")
		assert.Equal(t, 6, len(got.PreservedMessageIDs), "anchor + 5 post")
	})

	t.Run("Scenario_RecompactionChainTracksCumulativeSummarized", func(t *testing.T) {
		// Given an agent runs long enough to compact 3 times,
		// When admin queries the chain,
		// Then each boundary records cumulative summarized count
		// (analytics: "this session has compacted 50 total messages over
		// 3 boundaries").
		r := NewInMemoryCompactBoundaryRegistry()
		first := validBoundary()
		first.SummarizedCount = 10
		f, _ := r.Record(context.Background(), first)

		second := validBoundary()
		second.SessionID = f.SessionID
		second.PreviousBoundaryID = f.ID
		second.SummarizedCount = 15
		s, _ := r.Record(context.Background(), second)

		third := validBoundary()
		third.SessionID = f.SessionID
		third.PreviousBoundaryID = s.ID
		third.SummarizedCount = 25
		tBound, _ := r.Record(context.Background(), third)

		assert.Equal(t, 1, f.ChainDepth)
		assert.Equal(t, 2, s.ChainDepth)
		assert.Equal(t, 3, tBound.ChainDepth)
		assert.Equal(t, 50, tBound.CumulativeSummarized, "10+15+25 = 50")
	})

	t.Run("Scenario_FindLatestForSessionAnchorsRunnerToCurrentBoundary", func(t *testing.T) {
		// Given the runner needs to know "where are we in the compaction
		// chain right now",
		// When it asks FindLatestForSession,
		// Then it gets the boundary with highest ChainDepth (the most
		// recent compaction event) — used to anchor read-time projection.
		r := NewInMemoryCompactBoundaryRegistry()
		first, _ := r.Record(context.Background(), validBoundary())
		second := validBoundary()
		second.SessionID = first.SessionID
		second.PreviousBoundaryID = first.ID
		_, _ = r.Record(context.Background(), second)

		latest, err := r.FindLatestForSession(context.Background(), first.SessionID)
		require.NoError(t, err)
		assert.Equal(t, 2, latest.ChainDepth)
	})

	t.Run("Scenario_ChainBreakDetectedForGOV001Audit", func(t *testing.T) {
		// Given GOV-001 audits boundary chain integrity periodically,
		// When a boundary is removed mid-chain (admin error or storage
		// corruption),
		// Then VerifyChain reports the broken link with which boundary
		// is missing — auditor can investigate.
		r := NewInMemoryCompactBoundaryRegistry()
		first, _ := r.Record(context.Background(), validBoundary())
		second := validBoundary()
		second.SessionID = first.SessionID
		second.PreviousBoundaryID = first.ID
		_, _ = r.Record(context.Background(), second)

		// Simulate corruption.
		r.mu.Lock()
		delete(r.boundaries, first.ID)
		r.mu.Unlock()

		err := r.VerifyChain(context.Background(), first.SessionID)
		assert.True(t, errors.Is(err, ErrCompactBoundaryChainBroken))
	})

	t.Run("Scenario_AnchorMissingFromTranscriptIsAuditableError", func(t *testing.T) {
		// Given the boundary metadata says anchor=X but transcript
		// arrived without X (resume from corrupted state, replay drift),
		// When ProjectTranscript runs,
		// Then it returns ErrCompactBoundaryAnchorNotFound — caller
		// must re-derive (compaction-from-scratch) rather than silently
		// returning empty view.
		transcript := []uuid.UUID{uuid.New(), uuid.New()}
		b := validBoundary()
		_, err := ProjectTranscript(transcript, b)
		assert.True(t, errors.Is(err, ErrCompactBoundaryAnchorNotFound))
	})

	t.Run("Scenario_RootBoundaryHasNoPrevSoTrueRoot", func(t *testing.T) {
		// Given the first compaction in a session,
		// When boundary.IsRootBoundary is checked,
		// Then it returns true (PreviousBoundaryID == uuid.Nil).
		// Used by the runtime to know "this is the bottom of the chain".
		root := CompactBoundary{}
		assert.True(t, root.IsRootBoundary())
		assert.False(t, root.IsRecompaction())
	})

	t.Run("Scenario_RecompactionFlagDistinguishesFollowupFromRoot", func(t *testing.T) {
		// Given a compaction that follows a previous one,
		// When boundary.IsRecompaction is checked,
		// Then it returns true so analytics can categorize event:
		// "first-compaction" vs "subsequent-recompaction".
		rec := CompactBoundary{PreviousBoundaryID: uuid.New()}
		assert.True(t, rec.IsRecompaction())
		assert.False(t, rec.IsRootBoundary())
	})

	t.Run("Scenario_BoundaryAwareViewAnchorsResumeAcrossSessions", func(t *testing.T) {
		// Given a session is paused and resumed days later,
		// When PDF §9.2 resume reconstruction runs,
		// Then it queries FindLatestForSession + ProjectTranscript and
		// the LLM sees the COMPACTED VIEW (not the pre-compaction full
		// history that may exceed context window).
		anchor := uuid.New()
		transcript := []uuid.UUID{
			uuid.New(), uuid.New(), uuid.New(),
			anchor,
			uuid.New(),
		}
		b := validBoundary()
		b.AnchorUUID = anchor

		got, err := ProjectTranscript(transcript, b)
		require.NoError(t, err)
		assert.Equal(t, b.SummaryMessageID, got.SummaryMessageID,
			"summary first — replaces dropped pre-anchor history")
		assert.Equal(t, 2, len(got.PreservedMessageIDs))
	})

	t.Run("Scenario_DropBeforeSweepsOldBoundariesForGCJob", func(t *testing.T) {
		// Given a long-running session accumulates many boundaries,
		// When a GC job sweeps old ones,
		// Then DropBefore with a cutoff removes them — registry stays
		// bounded over time.
		r := NewInMemoryCompactBoundaryRegistry()
		first, _ := r.Record(context.Background(), validBoundary())
		count, _ := r.DropBefore(context.Background(), first.SessionID, first.OccurredAt.Add(time.Hour*1))
		assert.Equal(t, 1, count)
	})

	t.Run("Scenario_ChainDepthEnablesRunnerToShowRecompactionDepthInUI", func(t *testing.T) {
		// Given the UI displays "Compaction #3 of this session",
		// When the runner reads the latest boundary,
		// Then ChainDepth gives the position immediately (no chain walk
		// at read time — pre-computed at Record time).
		r := NewInMemoryCompactBoundaryRegistry()
		first, _ := r.Record(context.Background(), validBoundary())
		latest := first
		for i := 0; i < 4; i++ {
			next := validBoundary()
			next.SessionID = first.SessionID
			next.PreviousBoundaryID = latest.ID
			latest, _ = r.Record(context.Background(), next)
		}
		assert.Equal(t, 5, latest.ChainDepth, "5 boundaries chained")
	})
}
