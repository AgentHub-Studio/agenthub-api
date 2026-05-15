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

const lhphMigration = "000029_seed_longhorizon_phase_templates.up.sql"
const lhphMigrationDown = "000029_seed_longhorizon_phase_templates.down.sql"

func TestIntegration_CoreLongHorizonPhaseTemplate_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, lhphMigration)

	loader := core.NewCoreLongHorizonPhaseTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedLongHorizonPhaseRowCount, len(got))
}

func TestIntegration_CoreLongHorizonPhaseTemplate_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreLongHorizonPhaseTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreLongHorizonPhaseTemplate_LoadByTaskTemplate_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, lhphMigration)

	loader := core.NewCoreLongHorizonPhaseTemplateLoader(pool)
	for tmpl, expectedPhases := range core.SeedExpectedLongHorizonPhaseSlugs {
		got, err := loader.LoadByTaskTemplate(context.Background(), tmpl)
		require.NoError(t, err)
		assert.Equal(t, len(expectedPhases), len(got),
			"template %q must have %d phases", tmpl, len(expectedPhases))
	}
}

func TestIntegration_CoreLongHorizonPhaseTemplate_FindByTaskAndPhase_ReturnsKnown(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, lhphMigration)

	loader := core.NewCoreLongHorizonPhaseTemplateLoader(pool)
	tmpl, found, err := loader.FindByTaskAndPhase(context.Background(),
		"customer-30day-monitoring", "daily-checkin-loop")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, 28, tmpl.EstimatedDays)
	assert.Equal(t, []string{"baseline-snapshot"}, tmpl.DependsOnList())
	assert.False(t, tmpl.RequiresAdminReview)
}

func TestIntegration_CoreLongHorizonPhaseTemplate_FindByTaskAndPhase_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, lhphMigration)

	loader := core.NewCoreLongHorizonPhaseTemplateLoader(pool)
	_, found, err := loader.FindByTaskAndPhase(context.Background(), "nope", "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreLongHorizonPhaseTemplate_ListTaskTemplateSlugs_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, lhphMigration)

	loader := core.NewCoreLongHorizonPhaseTemplateLoader(pool)
	got, err := loader.ListTaskTemplateSlugs(context.Background())
	require.NoError(t, err)

	expected := append([]string{}, core.SeedExpectedLongHorizonTaskTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreLongHorizonPhaseTemplate_AdminReviewMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, lhphMigration)

	loader := core.NewCoreLongHorizonPhaseTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	got := []string{}
	for _, p := range all {
		if p.RequiresAdminReview {
			got = append(got, p.TaskTemplateSlug+"/"+p.PhaseSlug)
		}
	}
	expected := append([]string{}, core.SeedAdminReviewLongHorizonPhases...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreLongHorizonPhaseTemplate_DependsOnReferencesPhasesInSameTemplate(t *testing.T) {
	// Critical invariant: every depends_on slug MUST exist as a
	// phase_slug WITHIN the SAME task_template_slug. App-level FK.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, lhphMigration)

	loader := core.NewCoreLongHorizonPhaseTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	// Build (template, phase) lookup.
	lookup := map[string]map[string]bool{}
	for _, p := range all {
		if lookup[p.TaskTemplateSlug] == nil {
			lookup[p.TaskTemplateSlug] = map[string]bool{}
		}
		lookup[p.TaskTemplateSlug][p.PhaseSlug] = true
	}

	for _, p := range all {
		for _, dep := range p.DependsOnList() {
			assert.True(t, lookup[p.TaskTemplateSlug][dep],
				"phase %q depends on %q which doesn't exist in template %q",
				p.PhaseSlug, dep, p.TaskTemplateSlug)
		}
	}
}

func TestIntegration_CoreLongHorizonPhaseTemplate_DependencyGraphIsAcyclic(t *testing.T) {
	// Critical invariant: the dependency graph PER template must be a
	// DAG (no cycles). Toposort sanity check.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, lhphMigration)

	loader := core.NewCoreLongHorizonPhaseTemplateLoader(pool)
	for _, tmpl := range core.SeedExpectedLongHorizonTaskTemplateSlugs {
		phases, err := loader.LoadByTaskTemplate(context.Background(), tmpl)
		require.NoError(t, err)
		assert.True(t, isAcyclicDAG(phases),
			"template %q must have acyclic dependency graph", tmpl)
	}
}

// isAcyclicDAG runs Kahn's toposort. Returns true iff no cycle.
func isAcyclicDAG(phases []core.CoreLongHorizonPhaseTemplate) bool {
	indegree := map[string]int{}
	adj := map[string][]string{}
	all := map[string]bool{}
	for _, p := range phases {
		all[p.PhaseSlug] = true
		if _, ok := indegree[p.PhaseSlug]; !ok {
			indegree[p.PhaseSlug] = 0
		}
	}
	for _, p := range phases {
		for _, dep := range p.DependsOnList() {
			adj[dep] = append(adj[dep], p.PhaseSlug)
			indegree[p.PhaseSlug]++
		}
	}
	queue := []string{}
	for slug, deg := range indegree {
		if deg == 0 {
			queue = append(queue, slug)
		}
	}
	visited := 0
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		visited++
		for _, nxt := range adj[curr] {
			indegree[nxt]--
			if indegree[nxt] == 0 {
				queue = append(queue, nxt)
			}
		}
	}
	return visited == len(all)
}

func TestIntegration_CoreLongHorizonPhaseTemplate_EveryTemplateHasExactlyOneEntryPhase(t *testing.T) {
	// The runtime needs an unambiguous starting phase per template.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, lhphMigration)

	loader := core.NewCoreLongHorizonPhaseTemplateLoader(pool)
	for _, tmpl := range core.SeedExpectedLongHorizonTaskTemplateSlugs {
		phases, err := loader.LoadByTaskTemplate(context.Background(), tmpl)
		require.NoError(t, err)
		entries := 0
		for _, p := range phases {
			if len(p.DependsOnList()) == 0 {
				entries++
			}
		}
		assert.Equal(t, 1, entries,
			"template %q must have exactly one entry phase, got %d", tmpl, entries)
	}
}

func TestIntegration_CoreLongHorizonPhaseTemplate_AdminReviewOnlyOnTerminalPhases(t *testing.T) {
	// Cross-row invariant: a phase that is RequiresAdminReview must be
	// terminal (no other phase depends on it). Admin sign-off on
	// intermediate phases would block auto-progress.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, lhphMigration)

	loader := core.NewCoreLongHorizonPhaseTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	// Build (template, phase) → set of phases depending on it.
	dependents := map[string]map[string]bool{}
	for _, p := range all {
		for _, dep := range p.DependsOnList() {
			key := p.TaskTemplateSlug + "/" + dep
			if dependents[key] == nil {
				dependents[key] = map[string]bool{}
			}
			dependents[key][p.PhaseSlug] = true
		}
	}
	for _, p := range all {
		if !p.RequiresAdminReview {
			continue
		}
		key := p.TaskTemplateSlug + "/" + p.PhaseSlug
		assert.Empty(t, dependents[key],
			"admin-review phase %q must be terminal (no dependents)", key)
	}
}

func TestIntegration_CoreLongHorizonPhaseTemplate_EstimatedDaysIsPositive(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, lhphMigration)

	loader := core.NewCoreLongHorizonPhaseTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		assert.Positive(t, p.EstimatedDays,
			"phase %s/%s must have positive estimated_days",
			p.TaskTemplateSlug, p.PhaseSlug)
	}
}

func TestIntegration_CoreLongHorizonPhaseTemplate_MultiWeekTotalDuration(t *testing.T) {
	// Sanity invariant: sum(estimated_days) per template must be ≥7
	// (otherwise it doesn't deserve "long horizon" classification —
	// would fit in a single workflow_template instead).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, lhphMigration)

	loader := core.NewCoreLongHorizonPhaseTemplateLoader(pool)
	for _, tmpl := range core.SeedExpectedLongHorizonTaskTemplateSlugs {
		phases, err := loader.LoadByTaskTemplate(context.Background(), tmpl)
		require.NoError(t, err)
		total := 0
		for _, p := range phases {
			total += p.EstimatedDays
		}
		assert.GreaterOrEqual(t, total, 7,
			"template %q total %d days < 7 (not long-horizon)", tmpl, total)
	}
}

func TestIntegration_CoreLongHorizonPhaseTemplate_AllSlugsAreUniqueWithinTemplateInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, lhphMigration)

	loader := core.NewCoreLongHorizonPhaseTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	seen := map[string]bool{}
	for _, p := range all {
		key := p.TaskTemplateSlug + "/" + p.PhaseSlug
		assert.False(t, seen[key], "duplicate phase %q in DB", key)
		seen[key] = true
	}
}

func TestIntegration_CoreLongHorizonPhaseTemplate_AllDescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, lhphMigration)

	loader := core.NewCoreLongHorizonPhaseTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		assert.GreaterOrEqual(t, len(p.Description), 30,
			"phase %s/%s description must be ≥30 chars", p.TaskTemplateSlug, p.PhaseSlug)
		assert.NotEmpty(t, p.ExpectedDeliverables,
			"phase %s/%s must declare deliverables", p.TaskTemplateSlug, p.PhaseSlug)
	}
}

func TestIntegration_CoreLongHorizonPhaseTemplate_OrderingIsBySortOrderThenPhaseSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, lhphMigration)

	loader := core.NewCoreLongHorizonPhaseTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, all)
	for i := 1; i < len(all); i++ {
		if all[i-1].SortOrder == all[i].SortOrder {
			assert.LessOrEqual(t, all[i-1].PhaseSlug, all[i].PhaseSlug)
		} else {
			assert.Less(t, all[i-1].SortOrder, all[i].SortOrder)
		}
	}
	assert.Equal(t, "baseline-snapshot", all[0].PhaseSlug,
		"first by sort_order=10 must be baseline-snapshot")
}

func TestIntegration_CoreLongHorizonPhaseTemplate_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, lhphMigration)
	applyMigration(t, pool, migDir, lhphMigration)

	loader := core.NewCoreLongHorizonPhaseTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedLongHorizonPhaseRowCount, len(got))
}

func TestIntegration_CoreLongHorizonPhaseTemplate_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, lhphMigration)
	applyMigration(t, pool, migDir, lhphMigrationDown)

	loader := core.NewCoreLongHorizonPhaseTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreLongHorizonPhaseTemplate_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, lhphMigration)

	loader := core.NewCoreLongHorizonPhaseTemplateLoader(pool)
	for tmpl, expectedPhases := range core.SeedExpectedLongHorizonPhaseSlugs {
		phases, err := loader.LoadByTaskTemplate(context.Background(), tmpl)
		require.NoError(t, err)
		got := []string{}
		for _, p := range phases {
			got = append(got, p.PhaseSlug)
		}
		expected := append([]string{}, expectedPhases...)
		sort.Strings(got)
		sort.Strings(expected)
		assert.Equal(t, expected, got, "template %q phase set must match", tmpl)
	}
}
