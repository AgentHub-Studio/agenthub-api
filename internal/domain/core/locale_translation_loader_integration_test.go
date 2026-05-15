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

func TestIntegration_CoreLocaleTranslation_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000025_seed_locale_translations.up.sql")

	loader := core.NewCoreLocaleTranslationLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedTranslationRowCount, len(got),
		"expected 4 locales × 6 keys = 24 rows")
}

func TestIntegration_CoreLocaleTranslation_LoadAll_ReturnsEmptyWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreLocaleTranslationLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreLocaleTranslation_FindByLocaleAndKey_ReturnsKnown(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000025_seed_locale_translations.up.sql")

	loader := core.NewCoreLocaleTranslationLoader(pool)
	tr, found, err := loader.FindByLocaleAndKey(context.Background(), "pt-BR", "greeting.hello")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "pt-BR", tr.LocaleCode)
	assert.Equal(t, "greeting", tr.Category)
	assert.Contains(t, tr.Value, "Olá")
}

func TestIntegration_CoreLocaleTranslation_LoadByLocale_ReturnsFullKeyset(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000025_seed_locale_translations.up.sql")

	loader := core.NewCoreLocaleTranslationLoader(pool)
	got, err := loader.LoadByLocale(context.Background(), "en-US")
	require.NoError(t, err)
	assert.Equal(t, len(core.SeedExpectedTranslationKeys), len(got),
		"every locale has all expected keys")
}

func TestIntegration_CoreLocaleTranslation_LoadByCategory_ReturnsAllLocalesForCategory(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000025_seed_locale_translations.up.sql")

	loader := core.NewCoreLocaleTranslationLoader(pool)
	got, err := loader.LoadByCategory(context.Background(), "greeting")
	require.NoError(t, err)
	assert.Equal(t, len(core.SeedExpectedLocales), len(got),
		"category has one row per locale")
}

func TestIntegration_CoreLocaleTranslation_AllLocalesAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000025_seed_locale_translations.up.sql")

	loader := core.NewCoreLocaleTranslationLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, l := range core.SeedExpectedLocales {
		allowed[l] = true
	}
	for _, tr := range got {
		assert.True(t, allowed[tr.LocaleCode])
	}
}

func TestIntegration_CoreLocaleTranslation_AllCategoriesAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000025_seed_locale_translations.up.sql")

	loader := core.NewCoreLocaleTranslationLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, c := range core.SeedExpectedTranslationCategories {
		allowed[c] = true
	}
	for _, tr := range got {
		assert.True(t, allowed[tr.Category])
	}
}

func TestIntegration_CoreLocaleTranslation_AllKeysExistForEveryLocale(t *testing.T) {
	// Coverage matrix invariant: every locale must have every key.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000025_seed_locale_translations.up.sql")

	loader := core.NewCoreLocaleTranslationLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	// Build (locale, key) → exists map.
	have := map[string]bool{}
	for _, tr := range got {
		have[tr.LocaleCode+"|"+tr.Key] = true
	}
	for _, locale := range core.SeedExpectedLocales {
		for _, key := range core.SeedExpectedTranslationKeys {
			assert.True(t, have[locale+"|"+key],
				"missing translation for locale %q key %q", locale, key)
		}
	}
}

func TestIntegration_CoreLocaleTranslation_KeyPrefixMatchesCategoryColumn(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000025_seed_locale_translations.up.sql")

	loader := core.NewCoreLocaleTranslationLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tr := range got {
		idx := strings.Index(tr.Key, ".")
		require.Greater(t, idx, 0)
		prefix := tr.Key[:idx]
		assert.Equal(t, prefix, tr.Category,
			"key prefix %q must match category column %q for translation %q", prefix, tr.Category, tr.Key)
	}
}

func TestIntegration_CoreLocaleTranslation_AllValuesAreNonEmpty(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000025_seed_locale_translations.up.sql")

	loader := core.NewCoreLocaleTranslationLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tr := range got {
		assert.NotEmpty(t, tr.Value,
			"translation %s/%s must have non-empty value", tr.LocaleCode, tr.Key)
	}
}

func TestIntegration_CoreLocaleTranslation_LocalePlusKeyIsUnique(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000025_seed_locale_translations.up.sql")

	loader := core.NewCoreLocaleTranslationLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	seen := map[string]bool{}
	for _, tr := range got {
		key := tr.LocaleCode + "|" + tr.Key
		assert.False(t, seen[key], "duplicate (locale, key) pair: %s", key)
		seen[key] = true
	}
}

func TestIntegration_CoreLocaleTranslation_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000025_seed_locale_translations.up.sql")

	loader := core.NewCoreLocaleTranslationLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, got)

	applyMigration(t, pool, migDir, "000025_seed_locale_translations.down.sql")
	gotAfterDown, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, gotAfterDown)
}

func TestIntegration_CoreLocaleTranslation_PortugueseHasCorrectTone(t *testing.T) {
	// AgentHub is pt-BR primary; verify Brazilian Portuguese tone correct.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000025_seed_locale_translations.up.sql")

	loader := core.NewCoreLocaleTranslationLoader(pool)
	got, err := loader.LoadByLocale(context.Background(), "pt-BR")
	require.NoError(t, err)

	// Spot check key Brazilian terms.
	values := map[string]string{}
	for _, tr := range got {
		values[tr.Key] = tr.Value
	}
	assert.Contains(t, values["greeting.hello"], "Olá",
		"pt-BR greeting must use 'Olá'")
	assert.Contains(t, values["closing.farewell"], "Concluído",
		"pt-BR closing must use 'Concluído'")
}

func TestIntegration_CoreLocaleTranslation_FrenchHandlesEscapeApostrophes(t *testing.T) {
	// French uses lots of apostrophes — verify SQL escaping correct.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000025_seed_locale_translations.up.sql")

	loader := core.NewCoreLocaleTranslationLoader(pool)
	tr, found, err := loader.FindByLocaleAndKey(context.Background(), "fr-FR", "greeting.hello")
	require.NoError(t, err)
	require.True(t, found)
	assert.Contains(t, tr.Value, "puis-je",
		"french greeting must roundtrip apostrophe-escaped SQL correctly")
}

func TestIntegration_CoreLocaleTranslation_SortStableByLocaleAndKey(t *testing.T) {
	// LoadAll uses ORDER BY locale_code, key — verify sort order stable.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000025_seed_locale_translations.up.sql")

	loader := core.NewCoreLocaleTranslationLoader(pool)
	first, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	second, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	require.Equal(t, len(first), len(second))
	firstSlugs := []string{}
	secondSlugs := []string{}
	for i := range first {
		firstSlugs = append(firstSlugs, first[i].LocaleCode+"/"+first[i].Key)
		secondSlugs = append(secondSlugs, second[i].LocaleCode+"/"+second[i].Key)
	}
	sort.Strings(firstSlugs)
	sort.Strings(secondSlugs)
	assert.Equal(t, firstSlugs, secondSlugs, "stable sort across calls")
}
