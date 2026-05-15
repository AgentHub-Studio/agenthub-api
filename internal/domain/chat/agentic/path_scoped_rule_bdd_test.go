package agentic

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBDD_PathScopedRule(t *testing.T) {
	t.Run("Scenario_ToolSpecificRuleAppliesOnlyWhenAgentInvokesThatTool", func(t *testing.T) {
		// Given a rule "execute_sql tool must use prepared statements"
		// scoped to "tools/execute_sql/**",
		// And a rule "all Go files no secrets" scoped to "**/*.go",
		// When the runtime asks "what rules apply when invoking
		// tools/execute_sql/run",
		// Then it gets the SQL rule but NOT the Go-files rule (path
		// match required).
		r := NewInMemoryPathScopedRuleRegistry()
		sqlRule := validPathRule()
		sqlRule.Slug = "sql-prepared-statements"
		sqlRule.Scope = PathRuleScopeTool
		sqlRule.PathGlob = "tools/execute_sql/**"
		_, _ = r.Register(context.Background(), sqlRule)

		goRule := validPathRule()
		goRule.Slug = "go-no-secrets"
		_, _ = r.Register(context.Background(), goRule)

		matched, err := r.Match(context.Background(), "t-1", "tools/execute_sql/run")
		require.NoError(t, err)
		assert.Len(t, matched, 1)
		assert.Equal(t, "sql-prepared-statements", matched[0].Slug)
	})

	t.Run("Scenario_MoreSpecificGlobsBeatWildcardsForAuthority", func(t *testing.T) {
		// Given a global wildcard rule + a specific path rule both match,
		// When the runtime gets the matched list,
		// Then the specific rule comes FIRST (more specific = more
		// authoritative; PDF §7.8 specificity ordering).
		r := NewInMemoryPathScopedRuleRegistry()
		wildcard := validPathRule()
		wildcard.Slug = "global"
		wildcard.PathGlob = "**"
		wildcard.Scope = PathRuleScopeGlobal
		_, _ = r.Register(context.Background(), wildcard)

		specific := validPathRule()
		specific.Slug = "specific"
		specific.PathGlob = "config/secrets.yaml"
		_, _ = r.Register(context.Background(), specific)

		matched, _ := r.Match(context.Background(), "t-1", "config/secrets.yaml")
		require.Len(t, matched, 2)
		assert.Equal(t, "specific", matched[0].Slug)
	})

	t.Run("Scenario_DisabledRulesNeverAppearInMatches", func(t *testing.T) {
		// Given a rule was disabled by admin (Enabled=false),
		// When path matching runs,
		// Then disabled rules are EXCLUDED — no surprise enforcement
		// from rules admin thought were paused.
		r := NewInMemoryPathScopedRuleRegistry()
		rule := validPathRule()
		rule.Enabled = false
		_, _ = r.Register(context.Background(), rule)

		matched, _ := r.Match(context.Background(), "t-1", "internal/agent.go")
		assert.Empty(t, matched)
	})

	t.Run("Scenario_TenantIsolationPreventsCrossTenantRuleBleed", func(t *testing.T) {
		// Given tenant-A registers a rule "no_secrets" globally,
		// When tenant-B matches paths,
		// Then tenant-B's matches do NOT include tenant-A's rules
		// (per-tenant scope_id keying).
		r := NewInMemoryPathScopedRuleRegistry()
		_, _ = r.Register(context.Background(), validPathRule())
		matched, _ := r.Match(context.Background(), "t-2", "internal/agent.go")
		assert.Empty(t, matched)
	})

	t.Run("Scenario_PriorityBreaksTiesSoAdminCanForceWin", func(t *testing.T) {
		// Given two rules both match the same path with same specificity,
		// When admin sets Priority=100 on the preferred rule,
		// Then the high-priority one wins ordering — admin can force
		// authoritative answer without tampering with glob specificity.
		r := NewInMemoryPathScopedRuleRegistry()
		low := validPathRule()
		low.Slug = "low"
		low.Priority = 10
		_, _ = r.Register(context.Background(), low)

		high := validPathRule()
		high.Slug = "high"
		high.Priority = 100
		_, _ = r.Register(context.Background(), high)

		matched, _ := r.Match(context.Background(), "t-1", "internal/agent.go")
		require.Len(t, matched, 2)
		assert.Equal(t, "high", matched[0].Slug)
	})

	t.Run("Scenario_RecursiveDoubleStarMatchesDeepNesting", func(t *testing.T) {
		// Given a rule scoped to "tools/**" should apply to any depth
		// under the tools/ tree,
		// When the runtime asks about a deeply-nested path,
		// Then ** matches all intermediate segments.
		assert.True(t, matchPathGlob("tools/**", "tools/sql/v1/run.json"))
		assert.True(t, matchPathGlob("tools/**", "tools/api/v2/legacy/handler"))
	})

	t.Run("Scenario_FiveScopesCoverTaxonomyForUI", func(t *testing.T) {
		// Given the admin UI categorizes rules by scope,
		// When admin browses,
		// Then 5 scopes exist (global/tool/file/directory/agent) — closed
		// set so the UI doesn't grow unbounded categories over time.
		assert.Equal(t, 5, len(AllPathScopedRuleScopes()))
	})

	t.Run("Scenario_ListByScopeFiltersForUIPanel", func(t *testing.T) {
		// Given the admin "Tool rules" panel needs only tool-scoped rules,
		// When ListByScope(tool) runs,
		// Then file/global rules excluded — clean UI without manual filter.
		r := NewInMemoryPathScopedRuleRegistry()
		toolR := validPathRule()
		toolR.Slug = "tool-r"
		toolR.Scope = PathRuleScopeTool
		toolR.PathGlob = "tools/**"
		_, _ = r.Register(context.Background(), toolR)

		fileR := validPathRule()
		fileR.Slug = "file-r"
		_, _ = r.Register(context.Background(), fileR)

		got, _ := r.ListByScope(context.Background(), "t-1", PathRuleScopeTool)
		require.Len(t, got, 1)
		assert.Equal(t, PathRuleScopeTool, got[0].Scope)
	})

	t.Run("Scenario_DuplicateSlugRejectedToPreventOverwrite", func(t *testing.T) {
		// Given admin uniqueness contract: per-tenant slug is the
		// permanent identifier (UI references by slug),
		// When admin tries to Register a duplicate,
		// Then rejection prevents silent overwrite (admin must Delete
		// first to update — explicit lifecycle).
		r := NewInMemoryPathScopedRuleRegistry()
		_, _ = r.Register(context.Background(), validPathRule())
		_, err := r.Register(context.Background(), validPathRule())
		assert.Error(t, err)
	})

	t.Run("Scenario_DocumentationAuditableViaRuleContent", func(t *testing.T) {
		// Given GOV-001 audits which rule fired for which decision,
		// When a rule matches,
		// Then the rule carries auditable Content (the human-readable
		// explanation) so audit log shows WHY a path was governed by
		// this rule.
		r := NewInMemoryPathScopedRuleRegistry()
		_, _ = r.Register(context.Background(), validPathRule())
		matched, _ := r.Match(context.Background(), "t-1", "internal/x.go")
		require.Len(t, matched, 1)
		assert.NotEmpty(t, matched[0].Content)
	})
}
