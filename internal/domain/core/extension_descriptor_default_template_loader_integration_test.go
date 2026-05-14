//go:build integration

package core_test

import (
	"context"
	"regexp"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/core"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

const eddMigration = "000056_seed_extension_descriptor_default_templates.up.sql"
const eddMigrationDown = "000056_seed_extension_descriptor_default_templates.down.sql"

var semverRE = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

func TestIntegration_CoreEDD_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eddMigration)

	loader := core.NewCoreExtensionDescriptorDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedEDDTemplateRowCount, len(got))
}

func TestIntegration_CoreEDD_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreExtensionDescriptorDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreEDD_FindBySlug_BuiltinEssentialsShape(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eddMigration)

	loader := core.NewCoreExtensionDescriptorDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "builtin-essentials")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "builtin", tmpl.TargetSource)
	assert.False(t, tmpl.ChecksumRequired)
	assert.True(t, tmpl.AutoEnableAfterInstall)
	assert.False(t, tmpl.RequiresAdminReview)
	components, err := tmpl.ComponentsOffered()
	require.NoError(t, err)
	assert.Contains(t, components, "agents")
}

func TestIntegration_CoreEDD_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eddMigration)

	loader := core.NewCoreExtensionDescriptorDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreEDD_LoadBySource(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eddMigration)

	loader := core.NewCoreExtensionDescriptorDefaultTemplateLoader(pool)
	// builtin: 1; marketplace: 2 (rag + engineering); git: 1; url: 1.
	builtin, err := loader.LoadBySource(context.Background(), "builtin")
	require.NoError(t, err)
	assert.Equal(t, 1, len(builtin))
	market, _ := loader.LoadBySource(context.Background(), "marketplace")
	assert.Equal(t, 2, len(market))
	git, _ := loader.LoadBySource(context.Background(), "git")
	assert.Equal(t, 1, len(git))
	url, _ := loader.LoadBySource(context.Background(), "url")
	assert.Equal(t, 1, len(url))
}

func TestIntegration_CoreEDD_LoadByUseCase_OneToOne(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eddMigration)

	loader := core.NewCoreExtensionDescriptorDefaultTemplateLoader(pool)
	for _, uc := range core.SeedExpectedEDDTemplateUseCases {
		matched, err := loader.LoadByUseCase(context.Background(), uc)
		require.NoError(t, err)
		assert.Equal(t, 1, len(matched), "use case %q must have exactly 1 template (1:1)", uc)
	}
}

func TestIntegration_CoreEDD_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eddMigration)

	loader := core.NewCoreExtensionDescriptorDefaultTemplateLoader(pool)
	rec, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range rec {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedRecommendedEDDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreEDD_AdminReviewMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eddMigration)

	loader := core.NewCoreExtensionDescriptorDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		if p.RequiresAdminReview {
			got = append(got, p.Slug)
		}
	}
	expected := append([]string{}, core.SeedAdminReviewEDDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreEDD_AllSourcesInEXT001Enum(t *testing.T) {
	// Cross-feature invariant: every target_source must be in EXT-001
	// ExtensionSource enum.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eddMigration)

	loader := core.NewCoreExtensionDescriptorDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, s := range core.SeedExpectedEDDTemplateSources {
		allowed[s] = true
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.TargetSource],
			"%s source %q outside EXT-001 enum", t2.Slug, t2.TargetSource)
	}
}

func TestIntegration_CoreEDD_AllComponentsInEXT001Enum(t *testing.T) {
	// Cross-feature invariant: every label in components_offered must
	// be in EXT-001 ExtensionComponentKind enum.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eddMigration)

	loader := core.NewCoreExtensionDescriptorDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, c := range core.SeedExpectedEDDTemplateComponents {
		allowed[c] = true
	}
	for _, t2 := range all {
		components, err := t2.ComponentsOffered()
		require.NoError(t, err)
		for _, c := range components {
			assert.True(t, allowed[c],
				"%s component %q outside EXT-001 enum", t2.Slug, c)
		}
	}
}

func TestIntegration_CoreEDD_TargetVersionIsSemver(t *testing.T) {
	// Cross-feature invariant: target_version MUST match EXT-001
	// semverPattern `^\d+\.\d+\.\d+$` (else validateDescriptor would
	// reject at install time).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eddMigration)

	loader := core.NewCoreExtensionDescriptorDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.True(t, semverRE.MatchString(t2.TargetVersion),
			"%s version %q must be semver MAJOR.MINOR.PATCH", t2.Slug, t2.TargetVersion)
	}
}

func TestIntegration_CoreEDD_NonBuiltinSourcesRequireChecksum(t *testing.T) {
	// Cross-feature invariant: matches EXT-001 ErrExtensionChecksumRequired
	// rule (checksum required for marketplace/git/url sources).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eddMigration)

	loader := core.NewCoreExtensionDescriptorDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		if t2.TargetSource != "builtin" {
			assert.True(t, t2.ChecksumRequired,
				"%s (source=%s) must require checksum",
				t2.Slug, t2.TargetSource)
		}
	}
}

func TestIntegration_CoreEDD_OnlyBuiltinAutoEnables(t *testing.T) {
	// Cross-row invariant: builtin extensions can auto-enable (platform
	// trusted); external sources require admin enable.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eddMigration)

	loader := core.NewCoreExtensionDescriptorDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		if t2.TargetSource == "builtin" {
			assert.True(t, t2.AutoEnableAfterInstall,
				"%s (builtin) should auto-enable", t2.Slug)
		} else {
			assert.False(t, t2.AutoEnableAfterInstall,
				"%s (%s) should NOT auto-enable", t2.Slug, t2.TargetSource)
		}
	}
}

func TestIntegration_CoreEDD_ComponentsOfferedNonEmpty(t *testing.T) {
	// Cross-row invariant: every template must offer ≥ 1 component
	// (else it's not useful as an extension shape).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eddMigration)

	loader := core.NewCoreExtensionDescriptorDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		components, err := t2.ComponentsOffered()
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(components), 1,
			"%s must offer at least 1 component", t2.Slug)
	}
}

func TestIntegration_CoreEDD_SlugsAreKebabCase(t *testing.T) {
	// Cross-feature invariant: EXT-001 slugPattern requires kebab-case
	// slugs (matches `^[a-z0-9][a-z0-9-]*[a-z0-9]$`).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eddMigration)

	loader := core.NewCoreExtensionDescriptorDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	slugRE := regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)
	for _, t2 := range all {
		assert.True(t, slugRE.MatchString(t2.Slug),
			"%s slug must be kebab-case", t2.Slug)
	}
}

func TestIntegration_CoreEDD_DescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eddMigration)

	loader := core.NewCoreExtensionDescriptorDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.GreaterOrEqual(t, len(t2.Description), 40, "%s description", t2.Slug)
	}
}

func TestIntegration_CoreEDD_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eddMigration)

	loader := core.NewCoreExtensionDescriptorDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, t2 := range all {
		assert.False(t, seen[t2.Slug])
		seen[t2.Slug] = true
	}
}

func TestIntegration_CoreEDD_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eddMigration)

	loader := core.NewCoreExtensionDescriptorDefaultTemplateLoader(pool)
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
	assert.Equal(t, "builtin-essentials", all[0].Slug)
}

func TestIntegration_CoreEDD_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eddMigration)
	applyMigration(t, pool, migDir, eddMigration)

	loader := core.NewCoreExtensionDescriptorDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedEDDTemplateRowCount, len(got))
}

func TestIntegration_CoreEDD_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eddMigration)
	applyMigration(t, pool, migDir, eddMigrationDown)

	loader := core.NewCoreExtensionDescriptorDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreEDD_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eddMigration)

	loader := core.NewCoreExtensionDescriptorDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, t2 := range all {
		got = append(got, t2.Slug)
	}
	expected := append([]string{}, core.SeedExpectedEDDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
