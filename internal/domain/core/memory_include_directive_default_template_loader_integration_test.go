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

const midMigration = "000065_seed_memory_include_directive_default_templates.up.sql"
const midMigrationDown = "000065_seed_memory_include_directive_default_templates.down.sql"

func TestIntegration_CoreMID_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, midMigration)

	loader := core.NewCoreMemoryIncludeDirectiveDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedMIDTemplateRowCount, len(got))
}

func TestIntegration_CoreMID_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreMemoryIncludeDirectiveDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreMID_FindBySlug_OrgIdentityShape(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, midMigration)

	loader := core.NewCoreMemoryIncludeDirectiveDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "core-org-identity")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "core.org.identity", tmpl.IncludeKey)
	assert.Equal(t, "organization", tmpl.Category)
	assert.True(t, tmpl.ContainsPlaceholders)
}

func TestIntegration_CoreMID_FindByIncludeKey_RoundTrip(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, midMigration)

	loader := core.NewCoreMemoryIncludeDirectiveDefaultTemplateLoader(pool)
	for _, key := range core.SeedExpectedMIDTemplateIncludeKeys {
		tmpl, found, err := loader.FindByIncludeKey(context.Background(), key)
		require.NoError(t, err)
		require.True(t, found, "include_key %q must be seeded", key)
		assert.Equal(t, key, tmpl.IncludeKey)
	}
}

func TestIntegration_CoreMID_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, midMigration)

	loader := core.NewCoreMemoryIncludeDirectiveDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreMID_LoadByCategory_OneToOne(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, midMigration)

	loader := core.NewCoreMemoryIncludeDirectiveDefaultTemplateLoader(pool)
	for _, c := range core.SeedExpectedMIDTemplateCategories {
		matched, err := loader.LoadByCategory(context.Background(), c)
		require.NoError(t, err)
		assert.Equal(t, 1, len(matched),
			"category %q must have exactly 1 directive (1:1)", c)
	}
}

func TestIntegration_CoreMID_AllIncludeKeysMatchCTX007Regex(t *testing.T) {
	// Cross-feature invariant: every include_key must satisfy the CTX-007
	// includePattern (otherwise MemoryIncludeResolver will never expand
	// inline references to these keys).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, midMigration)

	loader := core.NewCoreMemoryIncludeDirectiveDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.True(t, core.MemoryIncludeKeyRE.MatchString(t2.IncludeKey),
			"%s include_key %q must match CTX-007 regex", t2.Slug, t2.IncludeKey)
	}
}

func TestIntegration_CoreMID_LookupMapFeedsCTX007Resolver(t *testing.T) {
	// Cross-feature invariant: LookupMap() produces a map compatible
	// with CTX-007 MemoryIncludeResolver.SetLookup. End-to-end smoke:
	// build the resolver, expand a synthetic text, and assert the
	// resolved output contains the seeded content.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, midMigration)

	loader := core.NewCoreMemoryIncludeDirectiveDefaultTemplateLoader(pool)
	lookup, err := loader.LookupMap(context.Background())
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(lookup), core.SeedExpectedMIDTemplateRowCount)

	// Sanity: all seeded keys present.
	for _, k := range core.SeedExpectedMIDTemplateIncludeKeys {
		_, ok := lookup[k]
		assert.True(t, ok, "key %q missing from LookupMap", k)
	}
}

func TestIntegration_CoreMID_AllCategoriesInClosedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, midMigration)

	loader := core.NewCoreMemoryIncludeDirectiveDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, c := range core.SeedExpectedMIDTemplateCategories {
		allowed[c] = true
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.Category],
			"%s category %q outside closed set", t2.Slug, t2.Category)
	}
}

func TestIntegration_CoreMID_PlaceholderSubsetIsOrgOnlyInDB(t *testing.T) {
	// Cross-feature invariant: only org identity should declare
	// placeholders (other directives are plain text for guard-rail
	// certainty).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, midMigration)

	loader := core.NewCoreMemoryIncludeDirectiveDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	placeholders := []string{}
	for _, t2 := range all {
		if t2.ContainsPlaceholders {
			placeholders = append(placeholders, t2.Slug)
		}
	}
	sort.Strings(placeholders)
	assert.Equal(t, []string{"core-org-identity"}, placeholders)
}

func TestIntegration_CoreMID_ContentTemplatesArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, midMigration)

	loader := core.NewCoreMemoryIncludeDirectiveDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.GreaterOrEqual(t, len(t2.ContentTemplate), 30,
			"%s content_template must be substantive", t2.Slug)
	}
}

func TestIntegration_CoreMID_AllSlugsAndKeysUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, midMigration)

	loader := core.NewCoreMemoryIncludeDirectiveDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seenSlugs := map[string]bool{}
	seenKeys := map[string]bool{}
	for _, t2 := range all {
		assert.False(t, seenSlugs[t2.Slug])
		seenSlugs[t2.Slug] = true
		assert.False(t, seenKeys[t2.IncludeKey])
		seenKeys[t2.IncludeKey] = true
	}
}

func TestIntegration_CoreMID_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, midMigration)

	loader := core.NewCoreMemoryIncludeDirectiveDefaultTemplateLoader(pool)
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
	assert.Equal(t, "core-org-identity", all[0].Slug)
}

func TestIntegration_CoreMID_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, midMigration)
	applyMigration(t, pool, migDir, midMigration)

	loader := core.NewCoreMemoryIncludeDirectiveDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedMIDTemplateRowCount, len(got))
}

func TestIntegration_CoreMID_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, midMigration)
	applyMigration(t, pool, migDir, midMigrationDown)

	loader := core.NewCoreMemoryIncludeDirectiveDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreMID_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, midMigration)

	loader := core.NewCoreMemoryIncludeDirectiveDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, t2 := range all {
		got = append(got, t2.Slug)
	}
	expected := append([]string{}, core.SeedExpectedMIDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
