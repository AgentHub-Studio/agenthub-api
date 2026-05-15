package agentic

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validBoundary() CompactBoundary {
	return CompactBoundary{
		SessionID:        uuid.New(),
		SummaryMessageID: uuid.New(),
		HeadUUID:         uuid.New(),
		AnchorUUID:       uuid.New(),
		TailUUID:         uuid.New(),
		SummarizedCount:  10,
	}
}

func TestCompactBoundary_Record_AssignsIDAndOccurredAt(t *testing.T) {
	r := NewInMemoryCompactBoundaryRegistry()
	saved, err := r.Record(context.Background(), validBoundary())
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, saved.ID)
	assert.False(t, saved.OccurredAt.IsZero())
	assert.Equal(t, 1, saved.ChainDepth)
	assert.Equal(t, 10, saved.CumulativeSummarized)
}

func TestCompactBoundary_Record_RejectsMissingFields(t *testing.T) {
	r := NewInMemoryCompactBoundaryRegistry()

	for name, mutate := range map[string]func(*CompactBoundary){
		"session":  func(b *CompactBoundary) { b.SessionID = uuid.Nil },
		"summary":  func(b *CompactBoundary) { b.SummaryMessageID = uuid.Nil },
		"head":     func(b *CompactBoundary) { b.HeadUUID = uuid.Nil },
		"anchor":   func(b *CompactBoundary) { b.AnchorUUID = uuid.Nil },
		"tail":     func(b *CompactBoundary) { b.TailUUID = uuid.Nil },
	} {
		b := validBoundary()
		mutate(&b)
		_, err := r.Record(context.Background(), b)
		assert.Error(t, err, "missing %s must error", name)
	}
}

func TestCompactBoundary_Record_RejectsZeroSummarizedCount(t *testing.T) {
	r := NewInMemoryCompactBoundaryRegistry()
	b := validBoundary()
	b.SummarizedCount = 0
	_, err := r.Record(context.Background(), b)
	assert.True(t, errors.Is(err, ErrCompactBoundaryNegativeCount))
}

func TestCompactBoundary_Record_ChainDepthIncrementsForRecompaction(t *testing.T) {
	r := NewInMemoryCompactBoundaryRegistry()
	first, _ := r.Record(context.Background(), validBoundary())

	second := validBoundary()
	second.SessionID = first.SessionID
	second.PreviousBoundaryID = first.ID
	second.SummarizedCount = 5
	saved, err := r.Record(context.Background(), second)
	require.NoError(t, err)
	assert.Equal(t, 2, saved.ChainDepth)
	assert.Equal(t, 15, saved.CumulativeSummarized, "cumulative = 10 + 5")
}

func TestCompactBoundary_Record_RejectsOrphanPreviousBoundaryID(t *testing.T) {
	r := NewInMemoryCompactBoundaryRegistry()
	b := validBoundary()
	b.PreviousBoundaryID = uuid.New() // doesn't exist
	_, err := r.Record(context.Background(), b)
	assert.True(t, errors.Is(err, ErrCompactBoundaryChainBroken))
}

func TestCompactBoundary_Find_RoundTrips(t *testing.T) {
	r := NewInMemoryCompactBoundaryRegistry()
	saved, _ := r.Record(context.Background(), validBoundary())
	got, err := r.Find(context.Background(), saved.ID)
	require.NoError(t, err)
	assert.Equal(t, saved.ID, got.ID)
}

func TestCompactBoundary_Find_UnknownReturnsNotFound(t *testing.T) {
	r := NewInMemoryCompactBoundaryRegistry()
	_, err := r.Find(context.Background(), uuid.New())
	assert.True(t, errors.Is(err, ErrCompactBoundaryNotFound))
}

func TestCompactBoundary_FindLatestForSession_PicksHighestChainDepth(t *testing.T) {
	r := NewInMemoryCompactBoundaryRegistry()
	now := time.Now()
	r.SetClock(func() time.Time { return now })

	first, _ := r.Record(context.Background(), validBoundary())
	now = now.Add(time.Minute)
	second := validBoundary()
	second.SessionID = first.SessionID
	second.PreviousBoundaryID = first.ID
	saved, _ := r.Record(context.Background(), second)

	latest, err := r.FindLatestForSession(context.Background(), first.SessionID)
	require.NoError(t, err)
	assert.Equal(t, saved.ID, latest.ID)
	assert.Equal(t, 2, latest.ChainDepth)
}

func TestCompactBoundary_FindLatestForSession_UnknownReturnsNotFound(t *testing.T) {
	r := NewInMemoryCompactBoundaryRegistry()
	_, err := r.FindLatestForSession(context.Background(), uuid.New())
	assert.True(t, errors.Is(err, ErrCompactBoundaryNotFound))
}

func TestCompactBoundary_ListChain_OrderedByChainDepthAscending(t *testing.T) {
	r := NewInMemoryCompactBoundaryRegistry()
	first, _ := r.Record(context.Background(), validBoundary())
	for i := 0; i < 3; i++ {
		next := validBoundary()
		next.SessionID = first.SessionID
		latest, _ := r.FindLatestForSession(context.Background(), first.SessionID)
		next.PreviousBoundaryID = latest.ID
		_, _ = r.Record(context.Background(), next)
	}
	chain, err := r.ListChain(context.Background(), first.SessionID)
	require.NoError(t, err)
	require.Len(t, chain, 4)
	for i := 1; i < len(chain); i++ {
		assert.Less(t, chain[i-1].ChainDepth, chain[i].ChainDepth)
	}
	assert.Equal(t, 1, chain[0].ChainDepth)
	assert.Equal(t, 4, chain[3].ChainDepth)
}

func TestCompactBoundary_ListChain_TenantIsolation(t *testing.T) {
	r := NewInMemoryCompactBoundaryRegistry()
	a, _ := r.Record(context.Background(), validBoundary())
	_, _ = r.Record(context.Background(), validBoundary())
	chain, _ := r.ListChain(context.Background(), a.SessionID)
	assert.Len(t, chain, 1, "session isolation: only A's boundaries returned")
}

func TestCompactBoundary_VerifyChain_PassesOnCleanChain(t *testing.T) {
	r := NewInMemoryCompactBoundaryRegistry()
	first, _ := r.Record(context.Background(), validBoundary())
	for i := 0; i < 3; i++ {
		next := validBoundary()
		next.SessionID = first.SessionID
		latest, _ := r.FindLatestForSession(context.Background(), first.SessionID)
		next.PreviousBoundaryID = latest.ID
		_, _ = r.Record(context.Background(), next)
	}
	err := r.VerifyChain(context.Background(), first.SessionID)
	assert.NoError(t, err)
}

func TestCompactBoundary_VerifyChain_DetectsBrokenLink(t *testing.T) {
	r := NewInMemoryCompactBoundaryRegistry()
	first, _ := r.Record(context.Background(), validBoundary())
	second := validBoundary()
	second.SessionID = first.SessionID
	second.PreviousBoundaryID = first.ID
	saved, _ := r.Record(context.Background(), second)

	// Adversary deletes the first boundary directly (chain broken).
	r.mu.Lock()
	delete(r.boundaries, first.ID)
	r.mu.Unlock()

	err := r.VerifyChain(context.Background(), saved.SessionID)
	assert.True(t, errors.Is(err, ErrCompactBoundaryChainBroken))
}

func TestCompactBoundary_VerifyChain_EmptySessionIsValid(t *testing.T) {
	r := NewInMemoryCompactBoundaryRegistry()
	err := r.VerifyChain(context.Background(), uuid.New())
	assert.NoError(t, err, "empty chain valid by definition")
}

func TestCompactBoundary_DropBefore_RemovesOlderThanCutoff(t *testing.T) {
	r := NewInMemoryCompactBoundaryRegistry()
	now := time.Now()
	r.SetClock(func() time.Time { return now })

	first, _ := r.Record(context.Background(), validBoundary())
	now = now.Add(time.Hour)
	_, _ = r.Record(context.Background(), validBoundary())
	cutoff := now.Add(-time.Minute)

	count, err := r.DropBefore(context.Background(), first.SessionID, cutoff)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestCompactBoundary_IsRootBoundary(t *testing.T) {
	root := CompactBoundary{}
	assert.True(t, root.IsRootBoundary())
	rec := CompactBoundary{PreviousBoundaryID: uuid.New()}
	assert.False(t, rec.IsRootBoundary())
}

func TestCompactBoundary_IsRecompaction(t *testing.T) {
	root := CompactBoundary{}
	assert.False(t, root.IsRecompaction())
	rec := CompactBoundary{PreviousBoundaryID: uuid.New()}
	assert.True(t, rec.IsRecompaction())
}

func TestProjectTranscript_KeepsAnchorAndPostMessages(t *testing.T) {
	anchor := uuid.New()
	transcript := []uuid.UUID{
		uuid.New(), uuid.New(), uuid.New(), // pre-anchor (will be dropped)
		anchor,                              // anchor (kept)
		uuid.New(), uuid.New(),              // post-anchor (kept)
	}
	b := validBoundary()
	b.AnchorUUID = anchor

	got, err := ProjectTranscript(transcript, b)
	require.NoError(t, err)
	assert.Equal(t, 3, len(got.PreservedMessageIDs), "anchor + 2 post = 3")
	assert.Equal(t, anchor, got.PreservedMessageIDs[0], "anchor first")
	assert.Equal(t, 3, got.DroppedCount, "3 pre-anchor dropped")
	assert.Equal(t, b.SummaryMessageID, got.SummaryMessageID)
}

func TestProjectTranscript_AnchorAtStartDropsZero(t *testing.T) {
	anchor := uuid.New()
	transcript := []uuid.UUID{anchor, uuid.New()}
	b := validBoundary()
	b.AnchorUUID = anchor
	got, err := ProjectTranscript(transcript, b)
	require.NoError(t, err)
	assert.Equal(t, 0, got.DroppedCount)
	assert.Equal(t, 2, len(got.PreservedMessageIDs))
}

func TestProjectTranscript_AnchorMissingReturnsError(t *testing.T) {
	transcript := []uuid.UUID{uuid.New(), uuid.New()}
	b := validBoundary()
	b.AnchorUUID = uuid.New() // not in transcript
	_, err := ProjectTranscript(transcript, b)
	assert.True(t, errors.Is(err, ErrCompactBoundaryAnchorNotFound))
}

func TestProjectTranscript_ValidatesBoundaryFirst(t *testing.T) {
	transcript := []uuid.UUID{uuid.New()}
	b := CompactBoundary{} // missing fields
	_, err := ProjectTranscript(transcript, b)
	assert.Error(t, err)
}

func TestCompactBoundary_ConcurrentRecordIsSafe(t *testing.T) {
	r := NewInMemoryCompactBoundaryRegistry()
	sessionID := uuid.New()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b := validBoundary()
			b.SessionID = sessionID
			_, _ = r.Record(context.Background(), b)
		}()
	}
	wg.Wait()
	chain, _ := r.ListChain(context.Background(), sessionID)
	assert.Len(t, chain, 50)
}

func TestCompactBoundary_ContextCancelled(t *testing.T) {
	r := NewInMemoryCompactBoundaryRegistry()
	saved, _ := r.Record(context.Background(), validBoundary())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := r.Record(ctx, validBoundary())
	assert.Error(t, err)
	_, err = r.Find(ctx, saved.ID)
	assert.Error(t, err)
	_, err = r.FindLatestForSession(ctx, saved.SessionID)
	assert.Error(t, err)
	_, err = r.ListChain(ctx, saved.SessionID)
	assert.Error(t, err)
	err = r.VerifyChain(ctx, saved.SessionID)
	assert.Error(t, err)
	_, err = r.DropBefore(ctx, saved.SessionID, time.Now())
	assert.Error(t, err)
}
