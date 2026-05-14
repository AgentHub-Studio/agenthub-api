//go:build integration

package core_test

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/core"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

// Integration tests for CoreRuleLoader against a real Postgres
// (testcontainers pgvector:pg16). Build tag `integration`.
//
//   go test -tags=integration ./internal/domain/core/ -run TestIntegration_CoreRule

func TestIntegration_CoreRule_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)

	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000009_seed_rules.up.sql")

	loader := core.NewCoreRuleLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	assert.Equal(t, len(core.SeedExpectedRuleSlugs), len(got),
		"DB row count must match SeedExpectedRuleSlugs canonical count")

	gotSlugs := map[string]core.CoreRule{}
	for _, r := range got {
		gotSlugs[r.Slug] = r
	}
	for _, slug := range core.SeedExpectedRuleSlugs {
		assert.Contains(t, gotSlugs, slug,
			"DB must contain seeded slug %q", slug)
	}
}

func TestIntegration_CoreRule_LoadAll_ReturnsEmptyWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreRuleLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err,
		"missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got)
}

func TestIntegration_CoreRule_FindBySlug_ReturnsKnownRule(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000009_seed_rules.up.sql")

	loader := core.NewCoreRuleLoader(pool)
	rule, found, err := loader.FindBySlug(context.Background(), "safety-no-secret-disclosure")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "safety-no-secret-disclosure", rule.Slug)
	assert.Equal(t, "safety", rule.Category)
	assert.Equal(t, "global", rule.Scope)
	assert.True(t, rule.RequiresAdminToDisable,
		"safety rule must be admin-only-disable in DB")
	assert.NotEmpty(t, rule.Content,
		"rule content must be non-empty (it is what gets injected into prompt)")
}

func TestIntegration_CoreRule_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000009_seed_rules.up.sql")

	loader := core.NewCoreRuleLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "this-rule-does-not-exist")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreRule_OrderingIsPriorityDescThenSortOrder(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000009_seed_rules.up.sql")

	loader := core.NewCoreRuleLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, got)

	// Verify priority is monotonically non-increasing across the slice.
	prevPrio := got[0].Priority + 1 // anything strictly greater than first
	for i, r := range got {
		assert.LessOrEqual(t, r.Priority, prevPrio,
			"rule at index %d (priority %d) must have priority <= previous (%d)",
			i, r.Priority, prevPrio)
		prevPrio = r.Priority
	}

	// Highest-priority slot must be the cross-tenant-leak guard (priority=110).
	assert.Equal(t, "privacy-no-cross-tenant-leak", got[0].Slug,
		"highest-priority rule must be the cross-tenant guard")
	assert.Equal(t, 110, got[0].Priority,
		"cross-tenant guard priority is the explicit ceiling")
}

func TestIntegration_CoreRule_AdminOnlyDisable_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000009_seed_rules.up.sql")

	loader := core.NewCoreRuleLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	// Collect slugs whose DB row says requires_admin_to_disable=true.
	dbAdminOnly := []string{}
	for _, r := range got {
		if r.RequiresAdminToDisable {
			dbAdminOnly = append(dbAdminOnly, r.Slug)
		}
	}
	sort.Strings(dbAdminOnly)
	expected := append([]string{}, core.SeedAdminOnlyDisableSlugs...)
	sort.Strings(expected)

	assert.Equal(t, expected, dbAdminOnly,
		"DB admin-only set must EXACTLY match Go constant SeedAdminOnlyDisableSlugs")
}

func TestIntegration_CoreRule_LoadByScope_ReturnsGlobalPlusTargets(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000009_seed_rules.up.sql")

	loader := core.NewCoreRuleLoader(pool)

	// Insert a tool-scoped rule for testing the scope filter (the seed
	// only contains globals — augment with one tool-scoped row).
	testutil.MustExec(t, pool, `
		INSERT INTO ah_core.rule (slug, name, content, category, scope, priority)
		VALUES ('test-tool-rule', 'Test Tool Rule', 'directive text', 'safety',
		        'tool:execute-sql', 50)`)

	// LoadByScope("tool:execute-sql") should return globals + the tool-scoped one.
	got, err := loader.LoadByScope(context.Background(), "tool:execute-sql")
	require.NoError(t, err)

	gotSlugs := map[string]bool{}
	for _, r := range got {
		gotSlugs[r.Slug] = true
	}
	assert.True(t, gotSlugs["test-tool-rule"],
		"tool-scoped rule must be returned when target matches")
	assert.True(t, gotSlugs["safety-no-secret-disclosure"],
		"global rules must always be returned regardless of target")

	// Without the target, the tool-scoped rule must NOT be returned.
	gotGlobalOnly, err := loader.LoadByScope(context.Background())
	require.NoError(t, err)
	gotGlobalOnlySlugs := map[string]bool{}
	for _, r := range gotGlobalOnly {
		gotGlobalOnlySlugs[r.Slug] = true
	}
	assert.False(t, gotGlobalOnlySlugs["test-tool-rule"],
		"tool-scoped rule must NOT be returned without matching target")
	assert.True(t, gotGlobalOnlySlugs["safety-no-secret-disclosure"],
		"global rules always returned even with no targets")
}

func TestIntegration_CoreRule_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000009_seed_rules.up.sql")

	loader := core.NewCoreRuleLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	seen := map[string]bool{}
	for _, r := range got {
		assert.False(t, seen[r.Slug], "duplicate slug %q in DB", r.Slug)
		seen[r.Slug] = true
	}
}

func TestIntegration_CoreRule_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000009_seed_rules.up.sql")

	loader := core.NewCoreRuleLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, got, "precondition: seed produced rows")

	applyMigration(t, pool, migDir, "000009_seed_rules.down.sql")

	gotAfterDown, err := loader.LoadAll(context.Background())
	require.NoError(t, err,
		"missing table after down must NOT error (non-fatal contract)")
	assert.Empty(t, gotAfterDown,
		"down migration must drop the table cleanly")
}

func TestIntegration_CoreRule_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000009_seed_rules.up.sql")

	loader := core.NewCoreRuleLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbSlugs := make([]string, 0, len(got))
	for _, r := range got {
		dbSlugs = append(dbSlugs, r.Slug)
	}
	sort.Strings(dbSlugs)
	expected := append([]string{}, core.SeedExpectedRuleSlugs...)
	sort.Strings(expected)

	assert.Equal(t, expected, dbSlugs,
		"DB slugs must EXACTLY match SeedExpectedRuleSlugs (no missing, no extras)")
}

func TestIntegration_CoreRule_AllSeedRulesAreGlobalScope(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000009_seed_rules.up.sql")

	loader := core.NewCoreRuleLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	// Every seed row must be global by default — tool/skill-specific
	// overrides are tenant additions, not platform defaults.
	for _, r := range got {
		assert.Equal(t, "global", r.Scope,
			"seed rule %q must be scope=global (got %q)", r.Slug, r.Scope)
	}
}

func TestIntegration_CoreRule_ContentIsNeverEmpty(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000009_seed_rules.up.sql")

	loader := core.NewCoreRuleLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, r := range got {
		assert.NotEmpty(t, r.Content,
			"rule %q must have non-empty content (NOT NULL constraint)", r.Slug)
		assert.GreaterOrEqual(t, len(r.Content), 30,
			"rule %q content too short (%d chars) — directive must be informative",
			r.Slug, len(r.Content))
	}
}
