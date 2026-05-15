package agentic

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBDD_PermissionAuditQuery(t *testing.T) {
	t.Run("Scenario_ComplianceAsksWhichToolsAgentHadDeniedLastWeek", func(t *testing.T) {
		// Given a session ran for two weeks logging permission decisions,
		// And compliance asks "which tools were denied in the last 7 days?",
		// When the auditor queries by (Decision=Deny, FromInclusive=7d ago),
		// Then they see exactly the denied tools in that window.
		store := NewInMemoryPermissionAuditStore()
		now := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
		eightDaysAgo := now.Add(-8 * 24 * time.Hour)
		threeDaysAgo := now.Add(-3 * 24 * time.Hour)

		_ = store.LogDecision(context.Background(),
			PermissionAuditEntry{ToolName: "Bash", Decision: AuditDecisionDeny, CreatedAt: eightDaysAgo})
		_ = store.LogDecision(context.Background(),
			PermissionAuditEntry{ToolName: "Edit", Decision: AuditDecisionDeny, CreatedAt: threeDaysAgo})
		_ = store.LogDecision(context.Background(),
			PermissionAuditEntry{ToolName: "Read", Decision: AuditDecisionAllow, CreatedAt: threeDaysAgo})

		q := PermissionAuditQuery{
			Decision:      AuditDecisionDeny,
			FromInclusive: now.Add(-7 * 24 * time.Hour),
		}
		matches, _ := store.List(context.Background(), q)
		require.Equal(t, 1, len(matches))
		assert.Equal(t, "Edit", matches[0].ToolName)
	})

	t.Run("Scenario_DashboardShowsTopDeniedTool", func(t *testing.T) {
		// Given an admin dashboard wants the top-denied tool per tenant,
		// When the auditor computes the aggregate,
		// Then TopDeniedTools is sorted by count desc and surfaces the
		// chronic offender.
		store := NewInMemoryPermissionAuditStore()
		now := time.Now()
		for i := 0; i < 12; i++ {
			_ = store.LogDecision(context.Background(),
				PermissionAuditEntry{ToolName: "Bash", Decision: AuditDecisionDeny, CreatedAt: now})
		}
		for i := 0; i < 4; i++ {
			_ = store.LogDecision(context.Background(),
				PermissionAuditEntry{ToolName: "Write", Decision: AuditDecisionConfirmDenied, CreatedAt: now})
		}
		agg, _ := store.Aggregate(context.Background(), PermissionAuditQuery{})
		require.GreaterOrEqual(t, len(agg.TopDeniedTools), 2)
		assert.Equal(t, "Bash", agg.TopDeniedTools[0].ToolName)
		assert.Equal(t, 12, agg.TopDeniedTools[0].Count)
	})

	t.Run("Scenario_RetentionDropsAllowsButKeepsDenies", func(t *testing.T) {
		// Given a tenant policy: allows kept 90d, denies kept 7y,
		// And the audit has entries 1 year old,
		// When retention is applied today,
		// Then 1-year-old allows are dropped, 1-year-old denies survive.
		store := NewInMemoryPermissionAuditStore()
		now := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
		oneYearAgo := now.AddDate(-1, 0, 0)

		_ = store.LogDecision(context.Background(),
			PermissionAuditEntry{ToolName: "Bash", Decision: AuditDecisionAllow, CreatedAt: oneYearAgo})
		_ = store.LogDecision(context.Background(),
			PermissionAuditEntry{ToolName: "Write", Decision: AuditDecisionDeny, CreatedAt: oneYearAgo})

		policy := PermissionAuditRetentionPolicy{
			AllowTTL: 90 * 24 * time.Hour,
			DenyTTL:  7 * 365 * 24 * time.Hour,
		}
		dropped, _ := store.ApplyRetention(policy, now)
		assert.Equal(t, 1, dropped)
		assert.Equal(t, 1, store.Size())
		surviving := store.Snapshot()[0]
		assert.Equal(t, AuditDecisionDeny, surviving.Decision)
	})

	t.Run("Scenario_RedactedExportForExternalAuditor", func(t *testing.T) {
		// Given compliance must export the audit log to a 3rd party
		// auditor without leaking customer SQL,
		// When the export pipeline redacts InputSnippet,
		// Then the exported entries carry "[REDACTED]" instead of the
		// raw input but every other field is intact for analysis.
		entries := []PermissionAuditEntry{
			{ToolName: "execute-sql", Decision: AuditDecisionAllow,
				InputSnippet: "SELECT * FROM customer_pii"},
			{ToolName: "Bash", Decision: AuditDecisionDeny,
				InputSnippet: "rm -rf /"},
		}
		policy := PermissionAuditRedactionPolicy{RedactInputSnippet: true}
		exported := policy.RedactAll(entries)
		require.Equal(t, 2, len(exported))
		assert.Equal(t, redactedMarker, exported[0].InputSnippet)
		assert.Equal(t, redactedMarker, exported[1].InputSnippet)
		// Decisions and tool names preserved for analytics.
		assert.Equal(t, "execute-sql", exported[0].ToolName)
		assert.Equal(t, AuditDecisionDeny, exported[1].Decision)
		// Originals untouched (re-export must produce same redacted output).
		assert.Equal(t, "SELECT * FROM customer_pii", entries[0].InputSnippet)
	})

	t.Run("Scenario_SessionScopedQueryFiltersAcrossTenants", func(t *testing.T) {
		// Given entries from multiple sessions,
		// When the auditor filters by SessionID,
		// Then they see only that session's decisions — multi-tenant
		// isolation at query time.
		store := NewInMemoryPermissionAuditStore()
		now := time.Now()
		s1, s2 := uuid.New(), uuid.New()
		_ = store.LogDecision(context.Background(),
			PermissionAuditEntry{SessionID: s1, ToolName: "Bash",
				Decision: AuditDecisionDeny, CreatedAt: now})
		_ = store.LogDecision(context.Background(),
			PermissionAuditEntry{SessionID: s2, ToolName: "Edit",
				Decision: AuditDecisionAllow, CreatedAt: now})
		matches, _ := store.List(context.Background(),
			PermissionAuditQuery{SessionID: &s1})
		require.Equal(t, 1, len(matches))
		assert.Equal(t, "Bash", matches[0].ToolName)
	})

	t.Run("Scenario_LimitTrimsLargeResultSets", func(t *testing.T) {
		// Given a UI shows only the latest N decisions (paginated),
		// When the query carries Limit=20,
		// Then at most 20 entries return.
		store := NewInMemoryPermissionAuditStore()
		now := time.Now()
		for i := 0; i < 100; i++ {
			_ = store.LogDecision(context.Background(),
				PermissionAuditEntry{ToolName: "X", Decision: AuditDecisionAllow,
					CreatedAt: now.Add(time.Duration(i) * time.Second)})
		}
		matches, _ := store.List(context.Background(),
			PermissionAuditQuery{Limit: 20})
		assert.Equal(t, 20, len(matches))
	})

	t.Run("Scenario_AggregateIgnoresLimitForAnalyticsAccuracy", func(t *testing.T) {
		// Given a dashboard wants total counts (not paginated),
		// When the aggregator runs with Limit=10 in the query,
		// Then the aggregate still counts all 100 entries — analytics
		// must not lie because of pagination.
		store := NewInMemoryPermissionAuditStore()
		now := time.Now()
		for i := 0; i < 100; i++ {
			_ = store.LogDecision(context.Background(),
				PermissionAuditEntry{ToolName: "X", Decision: AuditDecisionAllow,
					CreatedAt: now})
		}
		agg, _ := store.Aggregate(context.Background(), PermissionAuditQuery{Limit: 10})
		assert.Equal(t, 100, agg.TotalEntries)
	})

	t.Run("Scenario_HumanSummaryRendersForLogLines", func(t *testing.T) {
		// Given an operator wants a one-line "today's audit at a glance"
		// for log lines or dashboards,
		// When FormatAggregateSummary runs,
		// Then it produces a deterministic sorted summary string.
		agg := PermissionAuditAggregate{
			TotalEntries: 10, UniqueSessions: 4,
			CountByDecision: map[PermissionAuditDecision]int{
				AuditDecisionAllow: 6,
				AuditDecisionDeny:  3,
				AuditDecisionConfirmDenied: 1,
			},
		}
		summary := FormatAggregateSummary(agg)
		assert.Contains(t, summary, "total=10")
		assert.Contains(t, summary, "sessions=4")
		assert.Contains(t, summary, "allow=6")
		assert.Contains(t, summary, "deny=3")
		assert.Contains(t, summary, "confirm_denied=1")
	})

	t.Run("Scenario_StoreSatisfiesBothInterfacesForRunnerWiring", func(t *testing.T) {
		// Given the harness wants ONE backing store that both writes
		// (PermissionAuditLogger) and reads (PermissionAuditReader),
		// When the same store is passed to both consumer types,
		// Then it satisfies both interfaces — single source of truth.
		store := NewInMemoryPermissionAuditStore()
		var logger PermissionAuditLogger = store
		var reader PermissionAuditReader = store
		_ = logger.LogDecision(context.Background(),
			PermissionAuditEntry{ToolName: "Bash", Decision: AuditDecisionDeny,
				CreatedAt: time.Now()})
		list, _ := reader.List(context.Background(), PermissionAuditQuery{})
		assert.Equal(t, 1, len(list))
	})

	t.Run("Scenario_ZeroTTLMeansKeepForever", func(t *testing.T) {
		// Given a tenant wants to keep DENIES forever (for compliance),
		// When DenyTTL=0 in the policy,
		// Then retention never expires deny entries regardless of age.
		store := NewInMemoryPermissionAuditStore()
		veryOld := time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC)
		_ = store.LogDecision(context.Background(),
			PermissionAuditEntry{Decision: AuditDecisionDeny, CreatedAt: veryOld})
		policy := PermissionAuditRetentionPolicy{
			AllowTTL: time.Second,
			DenyTTL:  0, // forever
		}
		dropped, _ := store.ApplyRetention(policy, time.Now())
		assert.Equal(t, 0, dropped)
		assert.Equal(t, 1, store.Size())
	})
}
