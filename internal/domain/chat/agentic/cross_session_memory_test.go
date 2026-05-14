package agentic

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validUserMemory() CrossSessionMemoryEntry {
	return CrossSessionMemoryEntry{
		TenantID:    "tenant-x",
		Scope:       MemoryScopeUser,
		Subject:     "user-alice",
		Kind:        MemoryKindPreference,
		Topic:       "preferred_language",
		Content:     "Alice prefers responses in Portuguese (BR)",
		SourceRunID: "run-1",
	}
}

func validTenantGlobalMemory() CrossSessionMemoryEntry {
	return CrossSessionMemoryEntry{
		TenantID: "tenant-x",
		Scope:    MemoryScopeTenantGlobal,
		Kind:     MemoryKindFact,
		Topic:    "fiscal_calendar",
		Content:  "Tenant uses fiscal year Jul-Jun, Q4=Apr-Jun",
	}
}

func TestMemory_ScopeEnumIsBounded(t *testing.T) {
	for _, s := range AllMemoryScopes() {
		assert.True(t, IsValidMemoryScope(s))
	}
	assert.False(t, IsValidMemoryScope(MemoryScope("unknown")))
}

func TestMemory_AllScopesCount(t *testing.T) {
	assert.Equal(t, 3, len(AllMemoryScopes()))
}

func TestMemory_KindEnumIsBounded(t *testing.T) {
	for _, k := range AllMemoryKinds() {
		assert.True(t, IsValidMemoryKind(k))
	}
	assert.False(t, IsValidMemoryKind(MemoryKind("unknown")))
}

func TestMemory_AllKindsCount(t *testing.T) {
	// 5 kinds: fact/preference/feedback/reference/relationship.
	assert.Equal(t, 5, len(AllMemoryKinds()))
}

func TestMemory_Save_AssignsIDAndDefaults(t *testing.T) {
	sub := NewInMemoryCrossSessionMemorySubstrate()
	saved, err := sub.Save(context.Background(), validUserMemory())
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, saved.ID)
	assert.False(t, saved.CreatedAt.IsZero())
	assert.Equal(t, 1.0, saved.Confidence, "default confidence = 1.0")
	assert.False(t, saved.Archived)
}

func TestMemory_Save_RejectsEmptyTenantID(t *testing.T) {
	sub := NewInMemoryCrossSessionMemorySubstrate()
	e := validUserMemory()
	e.TenantID = ""
	_, err := sub.Save(context.Background(), e)
	assert.Error(t, err)
}

func TestMemory_Save_RejectsInvalidScope(t *testing.T) {
	sub := NewInMemoryCrossSessionMemorySubstrate()
	e := validUserMemory()
	e.Scope = MemoryScope("global")
	_, err := sub.Save(context.Background(), e)
	assert.True(t, errors.Is(err, ErrInvalidMemoryScope))
}

func TestMemory_Save_RejectsInvalidKind(t *testing.T) {
	sub := NewInMemoryCrossSessionMemorySubstrate()
	e := validUserMemory()
	e.Kind = MemoryKind("bogus")
	_, err := sub.Save(context.Background(), e)
	assert.True(t, errors.Is(err, ErrInvalidMemoryKind))
}

func TestMemory_Save_UserScopeRequiresSubject(t *testing.T) {
	sub := NewInMemoryCrossSessionMemorySubstrate()
	e := validUserMemory()
	e.Subject = ""
	_, err := sub.Save(context.Background(), e)
	assert.Error(t, err)
}

func TestMemory_Save_TenantGlobalForbidsSubject(t *testing.T) {
	sub := NewInMemoryCrossSessionMemorySubstrate()
	e := validTenantGlobalMemory()
	e.Subject = "should-not-be-here"
	_, err := sub.Save(context.Background(), e)
	assert.Error(t, err, "tenant_global must have empty subject")
}

func TestMemory_Save_RejectsEmptyTopic(t *testing.T) {
	sub := NewInMemoryCrossSessionMemorySubstrate()
	e := validUserMemory()
	e.Topic = ""
	_, err := sub.Save(context.Background(), e)
	assert.Error(t, err)
}

func TestMemory_Save_RejectsEmptyContent(t *testing.T) {
	sub := NewInMemoryCrossSessionMemorySubstrate()
	e := validUserMemory()
	e.Content = ""
	_, err := sub.Save(context.Background(), e)
	assert.Error(t, err)
}

func TestMemory_Save_TruncatesLongContent(t *testing.T) {
	sub := NewInMemoryCrossSessionMemorySubstrate()
	e := validUserMemory()
	e.Content = strings.Repeat("x", 2000)
	saved, err := sub.Save(context.Background(), e)
	require.NoError(t, err)
	assert.Equal(t, 1000, len(saved.Content))
}

func TestMemory_FindByID_RoundTrips(t *testing.T) {
	sub := NewInMemoryCrossSessionMemorySubstrate()
	saved, _ := sub.Save(context.Background(), validUserMemory())
	got, err := sub.FindByID(context.Background(), saved.ID)
	require.NoError(t, err)
	assert.Equal(t, saved.Content, got.Content)
}

func TestMemory_FindByID_UnknownReturnsErrNotFound(t *testing.T) {
	sub := NewInMemoryCrossSessionMemorySubstrate()
	_, err := sub.FindByID(context.Background(), uuid.New())
	assert.True(t, errors.Is(err, ErrMemoryNotFound))
}

func TestMemory_Recall_FiltersByScopeSubjectTopic(t *testing.T) {
	sub := NewInMemoryCrossSessionMemorySubstrate()
	_, _ = sub.Save(context.Background(), validUserMemory())
	bobMem := validUserMemory()
	bobMem.Subject = "user-bob"
	bobMem.Content = "Bob prefers English"
	_, _ = sub.Save(context.Background(), bobMem)

	got, err := sub.Recall(context.Background(), MemoryRecallQuery{
		TenantID: "tenant-x", Scope: MemoryScopeUser, Subject: "user-alice",
	})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "user-alice", got[0].Subject)
}

func TestMemory_Recall_RequiresTenantID(t *testing.T) {
	sub := NewInMemoryCrossSessionMemorySubstrate()
	_, err := sub.Recall(context.Background(), MemoryRecallQuery{})
	assert.Error(t, err)
}

func TestMemory_Recall_TenantIsolation(t *testing.T) {
	sub := NewInMemoryCrossSessionMemorySubstrate()
	_, _ = sub.Save(context.Background(), validUserMemory())
	got, _ := sub.Recall(context.Background(), MemoryRecallQuery{TenantID: "different-tenant"})
	assert.Empty(t, got)
}

func TestMemory_Recall_TouchesLastRecalledAt(t *testing.T) {
	sub := NewInMemoryCrossSessionMemorySubstrate()
	saved, _ := sub.Save(context.Background(), validUserMemory())
	assert.True(t, saved.LastRecalledAt.IsZero(), "before recall")

	_, _ = sub.Recall(context.Background(), MemoryRecallQuery{TenantID: "tenant-x"})
	got, _ := sub.FindByID(context.Background(), saved.ID)
	assert.False(t, got.LastRecalledAt.IsZero(), "recall touches timestamp")
}

func TestMemory_Recall_ExcludesSuperseded(t *testing.T) {
	sub := NewInMemoryCrossSessionMemorySubstrate()
	old, _ := sub.Save(context.Background(), validUserMemory())
	newE, _ := sub.Save(context.Background(), validUserMemory())
	require.NoError(t, sub.Supersede(context.Background(), old.ID, newE.ID))

	got, _ := sub.Recall(context.Background(), MemoryRecallQuery{TenantID: "tenant-x"})
	for _, e := range got {
		assert.Nil(t, e.SupersededBy, "superseded entries excluded from recall")
		assert.NotEqual(t, old.ID, e.ID)
	}
}

func TestMemory_Recall_ExcludesArchived(t *testing.T) {
	sub := NewInMemoryCrossSessionMemorySubstrate()
	_, _ = sub.Save(context.Background(), validUserMemory())
	_, err := sub.ArchiveExpired(context.Background(), "tenant-x", time.Now().Add(time.Hour))
	require.NoError(t, err)
	got, _ := sub.Recall(context.Background(), MemoryRecallQuery{TenantID: "tenant-x"})
	assert.Empty(t, got, "archived entries excluded from recall")
}

func TestMemory_Recall_RespectsLimit(t *testing.T) {
	sub := NewInMemoryCrossSessionMemorySubstrate()
	for i := 0; i < 5; i++ {
		e := validUserMemory()
		e.Topic = "topic-" + string(rune('a'+i))
		_, _ = sub.Save(context.Background(), e)
	}
	got, _ := sub.Recall(context.Background(), MemoryRecallQuery{TenantID: "tenant-x", Limit: 2})
	assert.Len(t, got, 2)
}

func TestMemory_Supersede_FirstWins(t *testing.T) {
	sub := NewInMemoryCrossSessionMemorySubstrate()
	old, _ := sub.Save(context.Background(), validUserMemory())
	first, _ := sub.Save(context.Background(), validUserMemory())
	second, _ := sub.Save(context.Background(), validUserMemory())
	require.NoError(t, sub.Supersede(context.Background(), old.ID, first.ID))
	err := sub.Supersede(context.Background(), old.ID, second.ID)
	assert.True(t, errors.Is(err, ErrMemoryAlreadySuperseded))
}

func TestMemory_Supersede_RejectsUnknownNewID(t *testing.T) {
	sub := NewInMemoryCrossSessionMemorySubstrate()
	old, _ := sub.Save(context.Background(), validUserMemory())
	err := sub.Supersede(context.Background(), old.ID, uuid.New())
	assert.Error(t, err)
}

func TestMemory_Supersede_UnknownOldReturnsNotFound(t *testing.T) {
	sub := NewInMemoryCrossSessionMemorySubstrate()
	newE, _ := sub.Save(context.Background(), validUserMemory())
	err := sub.Supersede(context.Background(), uuid.New(), newE.ID)
	assert.True(t, errors.Is(err, ErrMemoryNotFound))
}

func TestMemory_ArchiveExpired_MovesEntriesPastCutoff(t *testing.T) {
	sub := NewInMemoryCrossSessionMemorySubstrate()
	_, _ = sub.Save(context.Background(), validUserMemory())
	_, _ = sub.Save(context.Background(), validUserMemory())

	// Archive everything not recalled since 1 hour from now (=all).
	count, err := sub.ArchiveExpired(context.Background(), "tenant-x", time.Now().Add(time.Hour))
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestMemory_ArchiveExpired_SkipsAlreadyArchived(t *testing.T) {
	sub := NewInMemoryCrossSessionMemorySubstrate()
	_, _ = sub.Save(context.Background(), validUserMemory())
	_, _ = sub.ArchiveExpired(context.Background(), "tenant-x", time.Now().Add(time.Hour))
	count2, err := sub.ArchiveExpired(context.Background(), "tenant-x", time.Now().Add(time.Hour))
	require.NoError(t, err)
	assert.Equal(t, 0, count2, "second pass = 0 (idempotent)")
}

func TestMemory_ArchiveExpired_SkipsRecentEntries(t *testing.T) {
	sub := NewInMemoryCrossSessionMemorySubstrate()
	_, _ = sub.Save(context.Background(), validUserMemory())
	count, err := sub.ArchiveExpired(context.Background(), "tenant-x", time.Now().Add(-time.Hour))
	require.NoError(t, err)
	assert.Equal(t, 0, count, "cutoff in past = nothing to archive")
}

func TestMemory_CountByScope_AlwaysAllScopes(t *testing.T) {
	sub := NewInMemoryCrossSessionMemorySubstrate()
	_, _ = sub.Save(context.Background(), validUserMemory())
	_, _ = sub.Save(context.Background(), validTenantGlobalMemory())

	hist, err := sub.CountByScope(context.Background(), "tenant-x")
	require.NoError(t, err)
	for _, s := range AllMemoryScopes() {
		_, ok := hist[s]
		assert.True(t, ok)
	}
	assert.Equal(t, 1, hist[MemoryScopeUser])
	assert.Equal(t, 1, hist[MemoryScopeTenantGlobal])
	assert.Equal(t, 0, hist[MemoryScopeAgent])
}

func TestMemory_CountByScope_ExcludesArchivedAndSuperseded(t *testing.T) {
	sub := NewInMemoryCrossSessionMemorySubstrate()
	old, _ := sub.Save(context.Background(), validUserMemory())
	newE, _ := sub.Save(context.Background(), validUserMemory())
	require.NoError(t, sub.Supersede(context.Background(), old.ID, newE.ID))

	other, _ := sub.Save(context.Background(), validUserMemory())
	_ = other

	hist, _ := sub.CountByScope(context.Background(), "tenant-x")
	// 3 saved, 1 superseded → 2 active.
	assert.Equal(t, 2, hist[MemoryScopeUser])
}

func TestMemory_CompactRecallSummary_FormatsEntries(t *testing.T) {
	entries := []CrossSessionMemoryEntry{
		{Scope: MemoryScopeUser, Kind: MemoryKindPreference,
			Topic: "lang", Content: "Portuguese"},
	}
	out := CompactRecallSummary(entries, 10)
	assert.Contains(t, out, "Cross-session memory:")
	assert.Contains(t, out, "[user/preference] lang: Portuguese")
}

func TestMemory_CompactRecallSummary_EmptyReturnsEmpty(t *testing.T) {
	assert.Equal(t, "", CompactRecallSummary(nil, 10))
}

func TestMemory_CompactRecallSummary_TruncatesAndIndicatesElided(t *testing.T) {
	entries := []CrossSessionMemoryEntry{}
	for i := 0; i < 5; i++ {
		entries = append(entries, CrossSessionMemoryEntry{
			Scope: MemoryScopeUser, Kind: MemoryKindFact,
			Topic: "t", Content: "c",
		})
	}
	out := CompactRecallSummary(entries, 2)
	assert.Contains(t, out, "(... 3 more entries elided)")
}

func TestMemory_ConcurrentSaveIsSafe(t *testing.T) {
	sub := NewInMemoryCrossSessionMemorySubstrate()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = sub.Save(context.Background(), validUserMemory())
		}()
	}
	wg.Wait()

	got, _ := sub.Recall(context.Background(), MemoryRecallQuery{TenantID: "tenant-x"})
	assert.Len(t, got, 50)
}

func TestMemory_ContextCancelledOperationsError(t *testing.T) {
	sub := NewInMemoryCrossSessionMemorySubstrate()
	saved, _ := sub.Save(context.Background(), validUserMemory())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := sub.Save(ctx, validUserMemory())
	assert.Error(t, err)

	_, err = sub.FindByID(ctx, saved.ID)
	assert.Error(t, err)

	_, err = sub.Recall(ctx, MemoryRecallQuery{TenantID: "tenant-x"})
	assert.Error(t, err)

	err = sub.Supersede(ctx, saved.ID, saved.ID)
	assert.Error(t, err)

	_, err = sub.ArchiveExpired(ctx, "tenant-x", time.Now())
	assert.Error(t, err)

	_, err = sub.CountByScope(ctx, "tenant-x")
	assert.Error(t, err)
}
