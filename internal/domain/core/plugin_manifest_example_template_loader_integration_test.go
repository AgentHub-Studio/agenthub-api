//go:build integration

package core_test

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/core"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

const pmetMigration = "000040_seed_plugin_manifest_example_templates.up.sql"
const pmetMigrationDown = "000040_seed_plugin_manifest_example_templates.down.sql"

func TestIntegration_CorePMET_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pmetMigration)

	loader := core.NewCorePluginManifestExampleTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedPMETemplateRowCount, len(got))
}

func TestIntegration_CorePMET_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCorePluginManifestExampleTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CorePMET_FindBySlug_ReturnsKnown(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pmetMigration)

	loader := core.NewCorePluginManifestExampleTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "example-skill-pack")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "skill_pack", tmpl.ManifestKind)
	assert.NotEmpty(t, tmpl.ExampleManifestJSON)
}

func TestIntegration_CorePMET_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pmetMigration)

	loader := core.NewCorePluginManifestExampleTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CorePMET_LoadByKind_OneTemplatePerKind(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pmetMigration)

	loader := core.NewCorePluginManifestExampleTemplateLoader(pool)
	for _, kind := range core.SeedExpectedPMETemplateKinds {
		got, err := loader.LoadByKind(context.Background(), kind)
		require.NoError(t, err)
		assert.Equal(t, 1, len(got), "kind %q must have exactly 1 example", kind)
	}
}

func TestIntegration_CorePMET_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pmetMigration)

	loader := core.NewCorePluginManifestExampleTemplateLoader(pool)
	rec, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range rec {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedRecommendedPMETemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CorePMET_AllKindsAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pmetMigration)

	loader := core.NewCorePluginManifestExampleTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, k := range core.SeedExpectedPMETemplateKinds {
		allowed[k] = true
	}
	for _, p := range all {
		assert.True(t, allowed[p.ManifestKind])
	}
}

func TestIntegration_CorePMET_AllAudiencesAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pmetMigration)

	loader := core.NewCorePluginManifestExampleTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, a := range core.SeedExpectedPMETemplateAudiences {
		allowed[a] = true
	}
	for _, p := range all {
		assert.True(t, allowed[p.TargetAudience])
	}
}

func TestIntegration_CorePMET_AllExamplesParseValidJSON(t *testing.T) {
	// Cross-row invariant: every example_manifest_json must parse cleanly.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pmetMigration)

	loader := core.NewCorePluginManifestExampleTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		parsed, err := p.ParseExample()
		require.NoError(t, err, "%s example must parse as JSON", p.Slug)
		assert.NotEmpty(t, parsed)
	}
}

func TestIntegration_CorePMET_ExamplesContainPlaceholderIDs(t *testing.T) {
	// Cross-row invariant: every example uses `example-vendor/*` ID
	// so authors KNOW to replace before publishing.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pmetMigration)

	loader := core.NewCorePluginManifestExampleTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		parsed, _ := p.ParseExample()
		id, ok := parsed["id"].(string)
		require.True(t, ok, "%s missing id field", p.Slug)
		assert.True(t, strings.HasPrefix(id, "example-vendor/"),
			"%s id %q must be placeholder example-vendor/*", p.Slug, id)
	}
}

func TestIntegration_CorePMET_ExamplesIncludeRequiredFields(t *testing.T) {
	// Cross-row invariant: every example contains the core fields
	// (schemaVersion, id, name, version, kind, minPlatformVersion) so
	// vendor sees the REQUIRED shape, not a minimal sketch.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pmetMigration)

	loader := core.NewCorePluginManifestExampleTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	required := []string{"schemaVersion", "id", "name", "version", "kind", "minPlatformVersion"}
	for _, p := range all {
		parsed, _ := p.ParseExample()
		for _, field := range required {
			_, ok := parsed[field]
			assert.True(t, ok, "%s missing required field %q", p.Slug, field)
		}
	}
}

func TestIntegration_CorePMET_BundleExampleHasEmptyComponentsAndDependencies(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pmetMigration)

	loader := core.NewCorePluginManifestExampleTemplateLoader(pool)
	tmpl, _, err := loader.FindBySlug(context.Background(), "example-bundle-meta")
	require.NoError(t, err)
	parsed, _ := tmpl.ParseExample()
	components, _ := parsed["components"].([]any)
	deps, _ := parsed["dependencies"].([]any)
	assert.Empty(t, components, "bundle has no own components")
	assert.NotEmpty(t, deps, "bundle composes via dependencies")
}

func TestIntegration_CorePMET_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pmetMigration)

	loader := core.NewCorePluginManifestExampleTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, p := range all {
		assert.False(t, seen[p.Slug])
		seen[p.Slug] = true
	}
}

func TestIntegration_CorePMET_AllDescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pmetMigration)

	loader := core.NewCorePluginManifestExampleTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		assert.GreaterOrEqual(t, len(p.Description), 30, "%s description", p.Slug)
	}
}

func TestIntegration_CorePMET_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pmetMigration)

	loader := core.NewCorePluginManifestExampleTemplateLoader(pool)
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
	assert.Equal(t, "example-agent-pack", all[0].Slug)
}

func TestIntegration_CorePMET_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pmetMigration)
	applyMigration(t, pool, migDir, pmetMigration)

	loader := core.NewCorePluginManifestExampleTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedPMETemplateRowCount, len(got))
}

func TestIntegration_CorePMET_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pmetMigration)
	applyMigration(t, pool, migDir, pmetMigrationDown)

	loader := core.NewCorePluginManifestExampleTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CorePMET_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pmetMigration)

	loader := core.NewCorePluginManifestExampleTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedExpectedPMETemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
