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

const csbtMigration = "000031_seed_context_section_budget_templates.up.sql"
const csbtMigrationDown = "000031_seed_context_section_budget_templates.down.sql"

func TestIntegration_CoreContextBudgetTemplate_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, csbtMigration)

	loader := core.NewCoreContextSectionBudgetTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedContextBudgetTemplateRowCount, len(got))
}

func TestIntegration_CoreContextBudgetTemplate_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreContextSectionBudgetTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreContextBudgetTemplate_FindBySlug_BalancedDefaultMatchesCTX001(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, csbtMigration)

	loader := core.NewCoreContextSectionBudgetTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "balanced-default")
	require.NoError(t, err)
	require.True(t, found)
	// CTX-001 DefaultAssemblerConfig: 32000 total budget.
	assert.Equal(t, 32000, tmpl.TotalBudgetTokens)
	// CTX-001 default per-section caps:
	//   skill_catalog/tool_catalog: 2000
	//   kb_summary: 1500
	//   recent_messages: 16000
	//   aux_prompt/scratchpad: 1000
	assert.Equal(t, 2000, tmpl.CapSkillCatalog)
	assert.Equal(t, 2000, tmpl.CapToolCatalog)
	assert.Equal(t, 1500, tmpl.CapKBSummary)
	assert.Equal(t, 16000, tmpl.CapRecentMessages)
	assert.Equal(t, 1000, tmpl.CapAuxPrompt)
	assert.Equal(t, 1000, tmpl.CapScratchpad)
}

func TestIntegration_CoreContextBudgetTemplate_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, csbtMigration)

	loader := core.NewCoreContextSectionBudgetTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreContextBudgetTemplate_LoadByUseCase_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, csbtMigration)

	loader := core.NewCoreContextSectionBudgetTemplateLoader(pool)
	general, err := loader.LoadByUseCase(context.Background(), "general")
	require.NoError(t, err)
	// balanced-default + small-context-tight + minimum-viable-4k = 3.
	assert.Equal(t, 3, len(general))

	for _, uc := range []string{"research", "conversation", "code"} {
		got, err := loader.LoadByUseCase(context.Background(), uc)
		require.NoError(t, err)
		assert.Equal(t, 1, len(got), "use case %q must have exactly 1 template", uc)
	}
}

func TestIntegration_CoreContextBudgetTemplate_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, csbtMigration)

	loader := core.NewCoreContextSectionBudgetTemplateLoader(pool)
	rec, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range rec {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedRecommendedContextBudgetTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreContextBudgetTemplate_AllUseCasesAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, csbtMigration)

	loader := core.NewCoreContextSectionBudgetTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, u := range core.SeedExpectedContextBudgetTemplateUseCases {
		allowed[u] = true
	}
	for _, p := range all {
		assert.True(t, allowed[p.TargetUseCase],
			"template %q has use case %q outside expected set", p.Slug, p.TargetUseCase)
	}
}

func TestIntegration_CoreContextBudgetTemplate_AllModelFamiliesAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, csbtMigration)

	loader := core.NewCoreContextSectionBudgetTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, m := range core.SeedExpectedContextBudgetTemplateModelFamilies {
		allowed[m] = true
	}
	for _, p := range all {
		assert.True(t, allowed[p.RecommendedForModelFamily],
			"template %q has model family %q outside expected set", p.Slug, p.RecommendedForModelFamily)
	}
}

func TestIntegration_CoreContextBudgetTemplate_TotalBudgetIsPositive(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, csbtMigration)

	loader := core.NewCoreContextSectionBudgetTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		assert.Positive(t, p.TotalBudgetTokens, "%s total_budget", p.Slug)
	}
}

func TestIntegration_CoreContextBudgetTemplate_AllCapsAreNonNegative(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, csbtMigration)

	loader := core.NewCoreContextSectionBudgetTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		for kind, cap := range p.PerSectionCaps() {
			assert.GreaterOrEqual(t, cap, 0,
				"%s cap for %s must be ≥ 0", p.Slug, kind)
		}
	}
}

func TestIntegration_CoreContextBudgetTemplate_RecentMessagesIsLargestCapInConversationProfile(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, csbtMigration)

	loader := core.NewCoreContextSectionBudgetTemplateLoader(pool)
	tmpl, _, err := loader.FindBySlug(context.Background(), "conversation-heavy-64k")
	require.NoError(t, err)
	caps := tmpl.PerSectionCaps()
	for kind, cap := range caps {
		if kind == "recent_messages" {
			continue
		}
		assert.GreaterOrEqual(t, caps["recent_messages"], cap,
			"recent_messages must be largest cap in conversation profile (kind=%s cap=%d)", kind, cap)
	}
}

func TestIntegration_CoreContextBudgetTemplate_KBSummaryIsLargestInResearchProfile(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, csbtMigration)

	loader := core.NewCoreContextSectionBudgetTemplateLoader(pool)
	research, _, err := loader.FindBySlug(context.Background(), "research-heavy-128k")
	require.NoError(t, err)
	conversation, _, err := loader.FindBySlug(context.Background(), "conversation-heavy-64k")
	require.NoError(t, err)
	// Research must allocate MORE KB tokens than conversation profile.
	assert.Greater(t, research.CapKBSummary, conversation.CapKBSummary,
		"research profile KB cap must exceed conversation profile KB cap")
}

func TestIntegration_CoreContextBudgetTemplate_CodeHeavyHasLargerScratchpadThanConversation(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, csbtMigration)

	loader := core.NewCoreContextSectionBudgetTemplateLoader(pool)
	code, _, err := loader.FindBySlug(context.Background(), "code-heavy-64k")
	require.NoError(t, err)
	conv, _, err := loader.FindBySlug(context.Background(), "conversation-heavy-64k")
	require.NoError(t, err)
	assert.Greater(t, code.CapScratchpad, conv.CapScratchpad,
		"code-heavy must allocate more scratchpad for reasoning than conversation-heavy")
	assert.Greater(t, code.CapToolCatalog, conv.CapToolCatalog,
		"code-heavy must allocate more tool_catalog (more code tools) than conversation-heavy")
}

func TestIntegration_CoreContextBudgetTemplate_TotalBudgetMonotonicAcrossTiers(t *testing.T) {
	// minimum-viable < small < balanced < conversation/code < research.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, csbtMigration)

	loader := core.NewCoreContextSectionBudgetTemplateLoader(pool)
	totals := map[string]int{}
	for _, slug := range core.SeedExpectedContextBudgetTemplateSlugs {
		tmpl, _, err := loader.FindBySlug(context.Background(), slug)
		require.NoError(t, err)
		totals[slug] = tmpl.TotalBudgetTokens
	}
	assert.Less(t, totals["minimum-viable-4k"], totals["small-context-tight"])
	assert.Less(t, totals["small-context-tight"], totals["balanced-default"])
	assert.Less(t, totals["balanced-default"], totals["conversation-heavy-64k"])
	assert.Less(t, totals["conversation-heavy-64k"], totals["research-heavy-128k"])
}

func TestIntegration_CoreContextBudgetTemplate_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, csbtMigration)

	loader := core.NewCoreContextSectionBudgetTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, p := range all {
		assert.False(t, seen[p.Slug])
		seen[p.Slug] = true
	}
}

func TestIntegration_CoreContextBudgetTemplate_AllDescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, csbtMigration)

	loader := core.NewCoreContextSectionBudgetTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		assert.GreaterOrEqual(t, len(p.Description), 30, "%s description", p.Slug)
	}
}

func TestIntegration_CoreContextBudgetTemplate_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, csbtMigration)

	loader := core.NewCoreContextSectionBudgetTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, all)
	for i := 1; i < len(all); i++ {
		if all[i-1].SortOrder == all[i].SortOrder {
			assert.LessOrEqual(t, all[i-1].Slug, all[i].Slug)
		} else {
			assert.Less(t, all[i-1].SortOrder, all[i].SortOrder)
		}
	}
	assert.Equal(t, "balanced-default", all[0].Slug,
		"first by sort_order=10 must be balanced-default")
}

func TestIntegration_CoreContextBudgetTemplate_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, csbtMigration)
	applyMigration(t, pool, migDir, csbtMigration)

	loader := core.NewCoreContextSectionBudgetTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedContextBudgetTemplateRowCount, len(got))
}

func TestIntegration_CoreContextBudgetTemplate_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, csbtMigration)
	applyMigration(t, pool, migDir, csbtMigrationDown)

	loader := core.NewCoreContextSectionBudgetTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreContextBudgetTemplate_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, csbtMigration)

	loader := core.NewCoreContextSectionBudgetTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedExpectedContextBudgetTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
