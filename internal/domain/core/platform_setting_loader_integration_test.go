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

// Integration tests for CorePlatformSettingLoader against real Postgres
// (testcontainers pgvector:pg16). Build tag `integration`.

func TestIntegration_CorePlatformSetting_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000013_seed_platform_settings.up.sql")

	loader := core.NewCorePlatformSettingLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	assert.Equal(t, len(core.SeedExpectedPlatformSettingKeys), len(got),
		"DB count must match SeedExpectedPlatformSettingKeys canonical count")

	gotKeys := map[string]core.CorePlatformSetting{}
	for _, s := range got {
		gotKeys[s.Key] = s
	}
	for _, k := range core.SeedExpectedPlatformSettingKeys {
		assert.Contains(t, gotKeys, k, "DB must contain seeded key %q", k)
	}
}

func TestIntegration_CorePlatformSetting_LoadAll_ReturnsEmptyWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCorePlatformSettingLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err, "missing schema must NOT error (non-fatal)")
	assert.Empty(t, got)
}

func TestIntegration_CorePlatformSetting_FindByKey_ReturnsKnownSetting(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000013_seed_platform_settings.up.sql")

	loader := core.NewCorePlatformSettingLoader(pool)
	s, found, err := loader.FindByKey(context.Background(), "runner.default_max_iterations")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "runner.default_max_iterations", s.Key)
	assert.Equal(t, "25", s.Value)
	assert.Equal(t, "number", s.ValueType)
	assert.Equal(t, "runner", s.Category)
	assert.True(t, s.IsOverridable)
	assert.NotEmpty(t, s.Description)

	// Test parser:
	intVal, err := s.AsInt()
	assert.NoError(t, err)
	assert.Equal(t, 25, intVal)
}

func TestIntegration_CorePlatformSetting_FindByKey_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000013_seed_platform_settings.up.sql")

	loader := core.NewCorePlatformSettingLoader(pool)
	_, found, err := loader.FindByKey(context.Background(), "does.not.exist")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CorePlatformSetting_LoadByCategory_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000013_seed_platform_settings.up.sql")

	loader := core.NewCorePlatformSettingLoader(pool)
	got, err := loader.LoadByCategory(context.Background(), "runner")
	require.NoError(t, err)

	assert.Equal(t, 4, len(got), "runner category must return 4 settings")
	for _, s := range got {
		assert.Equal(t, "runner", s.Category)
		assert.True(t, strings.HasPrefix(s.Key, "runner."))
	}
}

func TestIntegration_CorePlatformSetting_LoadByCategory_UnknownReturnsEmpty(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000013_seed_platform_settings.up.sql")

	loader := core.NewCorePlatformSettingLoader(pool)
	got, err := loader.LoadByCategory(context.Background(), "unknown")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CorePlatformSetting_AllCategoriesAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000013_seed_platform_settings.up.sql")

	loader := core.NewCorePlatformSettingLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, c := range core.SeedExpectedPlatformSettingCategories {
		allowed[c] = true
	}
	for _, s := range got {
		assert.True(t, allowed[s.Category],
			"DB setting %q references category %q outside SeedExpectedPlatformSettingCategories",
			s.Key, s.Category)
	}
}

func TestIntegration_CorePlatformSetting_AllValueTypesAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000013_seed_platform_settings.up.sql")

	loader := core.NewCorePlatformSettingLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, vt := range core.SeedExpectedPlatformSettingValueTypes {
		allowed[vt] = true
	}
	for _, s := range got {
		assert.True(t, allowed[s.ValueType],
			"DB setting %q has value_type %q outside allowlist",
			s.Key, s.ValueType)
	}
}

func TestIntegration_CorePlatformSetting_NonOverridableMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000013_seed_platform_settings.up.sql")

	loader := core.NewCorePlatformSettingLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbNonOver := []string{}
	for _, s := range got {
		if !s.IsOverridable {
			dbNonOver = append(dbNonOver, s.Key)
		}
	}
	sort.Strings(dbNonOver)
	expected := append([]string{}, core.SeedNonOverridablePlatformSettingKeys...)
	sort.Strings(expected)

	assert.Equal(t, expected, dbNonOver,
		"DB non-overridable set must EXACTLY match SeedNonOverridablePlatformSettingKeys")
}

func TestIntegration_CorePlatformSetting_AllKeysAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000013_seed_platform_settings.up.sql")

	loader := core.NewCorePlatformSettingLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	seen := map[string]bool{}
	for _, s := range got {
		assert.False(t, seen[s.Key], "duplicate key %q in DB", s.Key)
		seen[s.Key] = true
	}
}

func TestIntegration_CorePlatformSetting_AllDescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000013_seed_platform_settings.up.sql")

	loader := core.NewCorePlatformSettingLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, s := range got {
		assert.NotEmpty(t, s.Description,
			"setting %q must have description for UI/docs", s.Key)
		assert.GreaterOrEqual(t, len(s.Description), 20,
			"setting %q description too short (%d chars)", s.Key, len(s.Description))
	}
}

func TestIntegration_CorePlatformSetting_NumberValuesAreParseable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000013_seed_platform_settings.up.sql")

	loader := core.NewCorePlatformSettingLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, s := range got {
		if s.ValueType != "number" {
			continue
		}
		_, err := s.AsFloat()
		assert.NoError(t, err,
			"setting %q value_type=number must parse: value=%q", s.Key, s.Value)
	}
}

func TestIntegration_CorePlatformSetting_BooleanValuesAreParseable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000013_seed_platform_settings.up.sql")

	loader := core.NewCorePlatformSettingLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, s := range got {
		if s.ValueType != "boolean" {
			continue
		}
		_, err := s.AsBool()
		assert.NoError(t, err,
			"setting %q value_type=boolean must parse: value=%q", s.Key, s.Value)
	}
}

func TestIntegration_CorePlatformSetting_KeyCategoryPrefixMatchesCategoryColumn(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000013_seed_platform_settings.up.sql")

	loader := core.NewCorePlatformSettingLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, s := range got {
		idx := strings.Index(s.Key, ".")
		require.Greater(t, idx, 0, "key %q must have dot", s.Key)
		prefix := s.Key[:idx]
		assert.Equal(t, prefix, s.Category,
			"key prefix %q must match category column %q for setting %q",
			prefix, s.Category, s.Key)
	}
}

func TestIntegration_CorePlatformSetting_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000013_seed_platform_settings.up.sql")

	loader := core.NewCorePlatformSettingLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, got)

	applyMigration(t, pool, migDir, "000013_seed_platform_settings.down.sql")
	gotAfterDown, err := loader.LoadAll(context.Background())
	require.NoError(t, err, "post-down LoadAll must NOT error")
	assert.Empty(t, gotAfterDown)
}

func TestIntegration_CorePlatformSetting_SeedKeysMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000013_seed_platform_settings.up.sql")

	loader := core.NewCorePlatformSettingLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbKeys := []string{}
	for _, s := range got {
		dbKeys = append(dbKeys, s.Key)
	}
	sort.Strings(dbKeys)
	expected := append([]string{}, core.SeedExpectedPlatformSettingKeys...)
	sort.Strings(expected)

	assert.Equal(t, expected, dbKeys,
		"DB keys must EXACTLY match SeedExpectedPlatformSettingKeys")
}

func TestIntegration_CorePlatformSetting_OrderingIsBySortOrderThenKey(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000013_seed_platform_settings.up.sql")

	loader := core.NewCorePlatformSettingLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, got)

	prevSort := got[0].SortOrder - 1
	for i, s := range got {
		assert.GreaterOrEqual(t, s.SortOrder, prevSort,
			"setting at %d (sort %d) must have sort >= previous (%d)",
			i, s.SortOrder, prevSort)
		prevSort = s.SortOrder
	}
	// First sorted row must be a runner setting (sort=10).
	assert.Equal(t, "runner", got[0].Category,
		"first by sort_order must be a runner setting (sort=10)")
}
