package agentic

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPermAuditQuery_MatchesNoFilterAcceptsAll(t *testing.T) {
	q := PermissionAuditQuery{}
	e := PermissionAuditEntry{ToolName: "Bash", Decision: AuditDecisionAllow}
	assert.True(t, q.Matches(e))
}

func TestPermAuditQuery_MatchesSessionFilter(t *testing.T) {
	sid := uuid.New()
	q := PermissionAuditQuery{SessionID: &sid}
	assert.True(t, q.Matches(PermissionAuditEntry{SessionID: sid}))
	assert.False(t, q.Matches(PermissionAuditEntry{SessionID: uuid.New()}))
}

func TestPermAuditQuery_MatchesRunFilterRequiresPointer(t *testing.T) {
	rid := uuid.New()
	q := PermissionAuditQuery{RunID: &rid}
	assert.True(t, q.Matches(PermissionAuditEntry{RunID: &rid}))
	assert.False(t, q.Matches(PermissionAuditEntry{RunID: nil}))
	other := uuid.New()
	assert.False(t, q.Matches(PermissionAuditEntry{RunID: &other}))
}

func TestPermAuditQuery_MatchesToolFilter(t *testing.T) {
	q := PermissionAuditQuery{ToolName: "Bash"}
	assert.True(t, q.Matches(PermissionAuditEntry{ToolName: "Bash"}))
	assert.False(t, q.Matches(PermissionAuditEntry{ToolName: "Read"}))
}

func TestPermAuditQuery_MatchesDecisionFilter(t *testing.T) {
	q := PermissionAuditQuery{Decision: AuditDecisionDeny}
	assert.True(t, q.Matches(PermissionAuditEntry{Decision: AuditDecisionDeny}))
	assert.False(t, q.Matches(PermissionAuditEntry{Decision: AuditDecisionAllow}))
}

func TestPermAuditQuery_MatchesTimeWindowInclusiveFrom(t *testing.T) {
	from := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	q := PermissionAuditQuery{FromInclusive: from}
	// At the from instant → included (inclusive).
	assert.True(t, q.Matches(PermissionAuditEntry{CreatedAt: from}))
	// 1ns before from → excluded.
	assert.False(t, q.Matches(PermissionAuditEntry{CreatedAt: from.Add(-time.Nanosecond)}))
}

func TestPermAuditQuery_MatchesTimeWindowExclusiveTo(t *testing.T) {
	to := time.Date(2026, 5, 12, 11, 0, 0, 0, time.UTC)
	q := PermissionAuditQuery{ToExclusive: to}
	// At the to instant → excluded (exclusive).
	assert.False(t, q.Matches(PermissionAuditEntry{CreatedAt: to}))
	// 1ns before to → included.
	assert.True(t, q.Matches(PermissionAuditEntry{CreatedAt: to.Add(-time.Nanosecond)}))
}

func TestPermAuditStore_LogAndListChronological(t *testing.T) {
	store := NewInMemoryPermissionAuditStore()
	sid := uuid.New()
	older := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)
	require.NoError(t, store.LogDecision(context.Background(),
		PermissionAuditEntry{SessionID: sid, ToolName: "Bash",
			Decision: AuditDecisionDeny, CreatedAt: newer}))
	require.NoError(t, store.LogDecision(context.Background(),
		PermissionAuditEntry{SessionID: sid, ToolName: "Read",
			Decision: AuditDecisionAllow, CreatedAt: older}))
	list, err := store.List(context.Background(), PermissionAuditQuery{})
	require.NoError(t, err)
	require.Equal(t, 2, len(list))
	// Oldest first.
	assert.Equal(t, older, list[0].CreatedAt)
	assert.Equal(t, newer, list[1].CreatedAt)
}

func TestPermAuditStore_ListAppliesFilters(t *testing.T) {
	store := NewInMemoryPermissionAuditStore()
	now := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	_ = store.LogDecision(context.Background(),
		PermissionAuditEntry{ToolName: "Bash", Decision: AuditDecisionDeny, CreatedAt: now})
	_ = store.LogDecision(context.Background(),
		PermissionAuditEntry{ToolName: "Read", Decision: AuditDecisionAllow, CreatedAt: now})
	denies, _ := store.List(context.Background(),
		PermissionAuditQuery{Decision: AuditDecisionDeny})
	require.Equal(t, 1, len(denies))
	assert.Equal(t, "Bash", denies[0].ToolName)
}

func TestPermAuditStore_ListAppliesLimit(t *testing.T) {
	store := NewInMemoryPermissionAuditStore()
	now := time.Now()
	for i := 0; i < 10; i++ {
		_ = store.LogDecision(context.Background(),
			PermissionAuditEntry{ToolName: "Read", Decision: AuditDecisionAllow,
				CreatedAt: now.Add(time.Duration(i) * time.Second)})
	}
	list, _ := store.List(context.Background(), PermissionAuditQuery{Limit: 3})
	assert.Equal(t, 3, len(list))
}

func TestPermAuditStore_AggregateCountsByDecision(t *testing.T) {
	store := NewInMemoryPermissionAuditStore()
	now := time.Now()
	for i := 0; i < 5; i++ {
		_ = store.LogDecision(context.Background(),
			PermissionAuditEntry{ToolName: "Bash", Decision: AuditDecisionDeny, CreatedAt: now})
	}
	for i := 0; i < 3; i++ {
		_ = store.LogDecision(context.Background(),
			PermissionAuditEntry{ToolName: "Read", Decision: AuditDecisionAllow, CreatedAt: now})
	}
	agg, _ := store.Aggregate(context.Background(), PermissionAuditQuery{})
	assert.Equal(t, 8, agg.TotalEntries)
	assert.Equal(t, 5, agg.CountByDecision[AuditDecisionDeny])
	assert.Equal(t, 3, agg.CountByDecision[AuditDecisionAllow])
}

func TestPermAuditStore_AggregateCountsByToolSortedDesc(t *testing.T) {
	store := NewInMemoryPermissionAuditStore()
	now := time.Now()
	for i := 0; i < 5; i++ {
		_ = store.LogDecision(context.Background(),
			PermissionAuditEntry{ToolName: "Bash", Decision: AuditDecisionDeny, CreatedAt: now})
	}
	for i := 0; i < 2; i++ {
		_ = store.LogDecision(context.Background(),
			PermissionAuditEntry{ToolName: "Read", Decision: AuditDecisionAllow, CreatedAt: now})
	}
	agg, _ := store.Aggregate(context.Background(), PermissionAuditQuery{})
	require.Equal(t, 2, len(agg.CountByTool))
	assert.Equal(t, "Bash", agg.CountByTool[0].ToolName)
	assert.Equal(t, 5, agg.CountByTool[0].Count)
	assert.Equal(t, "Read", agg.CountByTool[1].ToolName)
}

func TestPermAuditStore_AggregateTopDeniedTools(t *testing.T) {
	store := NewInMemoryPermissionAuditStore()
	now := time.Now()
	for i := 0; i < 7; i++ {
		_ = store.LogDecision(context.Background(),
			PermissionAuditEntry{ToolName: "Bash", Decision: AuditDecisionDeny, CreatedAt: now})
	}
	_ = store.LogDecision(context.Background(),
		PermissionAuditEntry{ToolName: "Write", Decision: AuditDecisionConfirmDenied, CreatedAt: now})
	_ = store.LogDecision(context.Background(),
		PermissionAuditEntry{ToolName: "Read", Decision: AuditDecisionAllow, CreatedAt: now})
	agg, _ := store.Aggregate(context.Background(), PermissionAuditQuery{})
	require.Equal(t, 2, len(agg.TopDeniedTools))
	assert.Equal(t, "Bash", agg.TopDeniedTools[0].ToolName)
	assert.Equal(t, "Write", agg.TopDeniedTools[1].ToolName)
}

func TestPermAuditStore_AggregateUniqueSessions(t *testing.T) {
	store := NewInMemoryPermissionAuditStore()
	now := time.Now()
	s1, s2 := uuid.New(), uuid.New()
	_ = store.LogDecision(context.Background(),
		PermissionAuditEntry{SessionID: s1, Decision: AuditDecisionAllow, CreatedAt: now})
	_ = store.LogDecision(context.Background(),
		PermissionAuditEntry{SessionID: s1, Decision: AuditDecisionAllow, CreatedAt: now})
	_ = store.LogDecision(context.Background(),
		PermissionAuditEntry{SessionID: s2, Decision: AuditDecisionAllow, CreatedAt: now})
	agg, _ := store.Aggregate(context.Background(), PermissionAuditQuery{})
	assert.Equal(t, 2, agg.UniqueSessions)
}

func TestPermAuditStore_AggregateIgnoresLimit(t *testing.T) {
	store := NewInMemoryPermissionAuditStore()
	now := time.Now()
	for i := 0; i < 10; i++ {
		_ = store.LogDecision(context.Background(),
			PermissionAuditEntry{ToolName: "X", Decision: AuditDecisionAllow,
				CreatedAt: now.Add(time.Duration(i) * time.Second)})
	}
	agg, _ := store.Aggregate(context.Background(), PermissionAuditQuery{Limit: 3})
	assert.Equal(t, 10, agg.TotalEntries)
}

func TestPermAuditRetention_ValidateNegativeRejected(t *testing.T) {
	p := PermissionAuditRetentionPolicy{AllowTTL: -time.Second}
	assert.ErrorIs(t, p.Validate(), ErrPermissionAuditNegativeTTL)
}

func TestPermAuditRetention_ZeroTTLKeepsForever(t *testing.T) {
	store := NewInMemoryPermissionAuditStore()
	veryOld := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	_ = store.LogDecision(context.Background(),
		PermissionAuditEntry{Decision: AuditDecisionAllow, CreatedAt: veryOld})
	dropped, err := store.ApplyRetention(PermissionAuditRetentionPolicy{}, time.Now())
	require.NoError(t, err)
	assert.Equal(t, 0, dropped)
	assert.Equal(t, 1, store.Size())
}

func TestPermAuditRetention_AllowDroppedAfterTTL(t *testing.T) {
	store := NewInMemoryPermissionAuditStore()
	now := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	old := now.Add(-7 * 24 * time.Hour)
	_ = store.LogDecision(context.Background(),
		PermissionAuditEntry{Decision: AuditDecisionAllow, CreatedAt: old})
	policy := PermissionAuditRetentionPolicy{AllowTTL: 24 * time.Hour}
	dropped, _ := store.ApplyRetention(policy, now)
	assert.Equal(t, 1, dropped)
	assert.Equal(t, 0, store.Size())
}

func TestPermAuditRetention_DenyKeptLongerThanAllow(t *testing.T) {
	// Common compliance pattern: denies retained 7y, allows retained 90d.
	store := NewInMemoryPermissionAuditStore()
	now := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	oneYearAgo := now.AddDate(-1, 0, 0)
	_ = store.LogDecision(context.Background(),
		PermissionAuditEntry{Decision: AuditDecisionAllow, CreatedAt: oneYearAgo})
	_ = store.LogDecision(context.Background(),
		PermissionAuditEntry{Decision: AuditDecisionDeny, CreatedAt: oneYearAgo})
	policy := PermissionAuditRetentionPolicy{
		AllowTTL: 90 * 24 * time.Hour,
		DenyTTL:  7 * 365 * 24 * time.Hour,
	}
	dropped, _ := store.ApplyRetention(policy, now)
	assert.Equal(t, 1, dropped)
	assert.Equal(t, 1, store.Size())
	assert.Equal(t, AuditDecisionDeny, store.Snapshot()[0].Decision)
}

func TestPermAuditRetention_EachDecisionTierIndependent(t *testing.T) {
	store := NewInMemoryPermissionAuditStore()
	now := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	old := now.Add(-48 * time.Hour)
	for _, d := range []PermissionAuditDecision{
		AuditDecisionAllow, AuditDecisionDeny,
		AuditDecisionConfirmApproved, AuditDecisionConfirmDenied,
		AuditDecisionConfirmEscalated,
	} {
		_ = store.LogDecision(context.Background(),
			PermissionAuditEntry{Decision: d, CreatedAt: old, ToolName: string(d)})
	}
	policy := PermissionAuditRetentionPolicy{
		AllowTTL:            24 * time.Hour, // expires
		ConfirmApprovedTTL:  24 * time.Hour, // expires
		ConfirmEscalatedTTL: 24 * time.Hour, // expires
		DenyTTL:             0,              // forever
		ConfirmDeniedTTL:    0,              // forever
	}
	dropped, _ := store.ApplyRetention(policy, now)
	assert.Equal(t, 3, dropped)
	assert.Equal(t, 2, store.Size())
}

func TestPermAuditRedaction_LeavesUnflaggedFields(t *testing.T) {
	rid := uuid.New()
	e := PermissionAuditEntry{
		ToolName: "Bash", MatchedRule: "Bash(rm)",
		InputSnippet: "rm -rf /", RunID: &rid,
	}
	policy := PermissionAuditRedactionPolicy{RedactInputSnippet: true}
	out := policy.Redact(e)
	assert.Equal(t, redactedMarker, out.InputSnippet)
	// Other fields untouched.
	assert.Equal(t, "Bash(rm)", out.MatchedRule)
	assert.Equal(t, &rid, out.RunID)
}

func TestPermAuditRedaction_RedactsMatchedRule(t *testing.T) {
	e := PermissionAuditEntry{MatchedRule: "Bash(rm)"}
	policy := PermissionAuditRedactionPolicy{RedactMatchedRule: true}
	out := policy.Redact(e)
	assert.Equal(t, redactedMarker, out.MatchedRule)
}

func TestPermAuditRedaction_RedactsRunID(t *testing.T) {
	rid := uuid.New()
	e := PermissionAuditEntry{RunID: &rid}
	policy := PermissionAuditRedactionPolicy{RedactRunID: true}
	out := policy.Redact(e)
	assert.Nil(t, out.RunID)
}

func TestPermAuditRedaction_EmptyFieldNotMarkedRedacted(t *testing.T) {
	// If InputSnippet is already empty, redacting should NOT replace
	// it with "[REDACTED]" (avoids audit confusion).
	e := PermissionAuditEntry{InputSnippet: ""}
	policy := PermissionAuditRedactionPolicy{RedactInputSnippet: true}
	out := policy.Redact(e)
	assert.Empty(t, out.InputSnippet)
}

func TestPermAuditRedaction_OriginalNotMutated(t *testing.T) {
	e := PermissionAuditEntry{InputSnippet: "secret"}
	policy := PermissionAuditRedactionPolicy{RedactInputSnippet: true}
	_ = policy.Redact(e)
	assert.Equal(t, "secret", e.InputSnippet)
}

func TestPermAuditRedaction_AllAppliesToSlice(t *testing.T) {
	entries := []PermissionAuditEntry{
		{InputSnippet: "secret-a"},
		{InputSnippet: "secret-b"},
	}
	policy := PermissionAuditRedactionPolicy{RedactInputSnippet: true}
	out := policy.RedactAll(entries)
	require.Equal(t, 2, len(out))
	assert.Equal(t, redactedMarker, out[0].InputSnippet)
	assert.Equal(t, redactedMarker, out[1].InputSnippet)
	// Originals untouched.
	assert.Equal(t, "secret-a", entries[0].InputSnippet)
}

func TestPermAuditStore_ConcurrentLogIsRaceFree(t *testing.T) {
	store := NewInMemoryPermissionAuditStore()
	done := make(chan struct{})
	for i := 0; i < 50; i++ {
		go func() {
			_ = store.LogDecision(context.Background(),
				PermissionAuditEntry{ToolName: "X", Decision: AuditDecisionAllow,
					CreatedAt: time.Now()})
			done <- struct{}{}
		}()
	}
	for i := 0; i < 50; i++ {
		<-done
	}
	assert.Equal(t, 50, store.Size())
}

func TestPermAuditFormat_SummaryIncludesAllCounts(t *testing.T) {
	agg := PermissionAuditAggregate{
		TotalEntries:   8,
		UniqueSessions: 3,
		CountByDecision: map[PermissionAuditDecision]int{
			AuditDecisionAllow: 5,
			AuditDecisionDeny:  3,
		},
	}
	got := FormatAggregateSummary(agg)
	assert.True(t, strings.Contains(got, "total=8"))
	assert.True(t, strings.Contains(got, "sessions=3"))
	assert.True(t, strings.Contains(got, "allow=5"))
	assert.True(t, strings.Contains(got, "deny=3"))
}

func TestPermAuditFormat_SummaryDeterministic(t *testing.T) {
	agg := PermissionAuditAggregate{
		TotalEntries: 2,
		CountByDecision: map[PermissionAuditDecision]int{
			AuditDecisionDeny:  1,
			AuditDecisionAllow: 1,
		},
	}
	s1 := FormatAggregateSummary(agg)
	s2 := FormatAggregateSummary(agg)
	assert.Equal(t, s1, s2)
}

func TestPermAuditStore_SatisfiesBothLoggerAndReader(t *testing.T) {
	store := NewInMemoryPermissionAuditStore()
	var logger PermissionAuditLogger = store
	var reader PermissionAuditReader = store
	assert.NotNil(t, logger)
	assert.NotNil(t, reader)
}
