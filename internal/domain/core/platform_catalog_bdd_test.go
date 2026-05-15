package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBDD_PlatformCatalog(t *testing.T) {
	t.Run("Scenario_FreshTenantInheritsAllAhCoreCatalogsAtBoot", func(t *testing.T) {
		// Given the platform registers every ah_core catalog at boot,
		// When the registry is asked for size,
		// Then it reports all known catalogs.
		r := NewPlatformCatalogRegistry()
		m := validPlatformCatalog()
		require.NoError(t, r.Register(m))
		assert.Equal(t, 1, r.Size())
	})

	t.Run("Scenario_AdminUIGroupsCatalogsByKindForOnboarding", func(t *testing.T) {
		// Given admin needs to show "all subagent rosters" or "all fork
		// strategies" in one tile,
		// When ListByKind is called,
		// Then only matching manifests are returned.
		r := NewPlatformCatalogRegistry()
		m1 := validPlatformCatalog()
		m1.Slug = "subagent_table_a"
		m1.MigrationNumber = 61
		m1.Kind = PlatformCatalogKindSubagentRoster
		m2 := validPlatformCatalog()
		m2.Slug = "fork_table_a"
		m2.MigrationNumber = 60
		m2.Kind = PlatformCatalogKindForkStrategy
		require.NoError(t, r.Register(m1))
		require.NoError(t, r.Register(m2))
		got := r.ListByKind(PlatformCatalogKindSubagentRoster)
		assert.Equal(t, 1, len(got))
	})

	t.Run("Scenario_DuplicateMigrationNumberRejectedToPreventClash", func(t *testing.T) {
		// Given migration numbers are the source of truth for ordering,
		// When two catalogs claim the same migration number,
		// Then the second registration is rejected.
		r := NewPlatformCatalogRegistry()
		m1 := validPlatformCatalog()
		m2 := validPlatformCatalog()
		m2.Slug = "another_table"
		require.NoError(t, r.Register(m1))
		err := r.Register(m2)
		assert.ErrorIs(t, err, ErrPlatformCatalogDuplicateMigration)
	})

	t.Run("Scenario_AhStatusReportTotalRowCount", func(t *testing.T) {
		// Given /ah-status reports "ah_core ships N rows across M catalogs",
		// When the registry is queried for TotalExpectedRowCount,
		// Then the sum is correct.
		r := NewPlatformCatalogRegistry()
		m1 := validPlatformCatalog()
		m1.Slug = "table_one"
		m1.MigrationNumber = 100
		m1.ExpectedRowCount = 3
		m2 := validPlatformCatalog()
		m2.Slug = "table_two"
		m2.MigrationNumber = 101
		m2.ExpectedRowCount = 5
		require.NoError(t, r.Register(m1))
		require.NoError(t, r.Register(m2))
		assert.Equal(t, 8, r.TotalExpectedRowCount())
	})

	t.Run("Scenario_BadKindRejectedAtRegistration", func(t *testing.T) {
		// Given catalog kinds must come from a bounded enum,
		// When admin tries to register an unknown kind,
		// Then registration fails with ErrPlatformCatalogBadKind.
		r := NewPlatformCatalogRegistry()
		bad := validPlatformCatalog()
		bad.Kind = "futuristic_thing"
		err := r.Register(bad)
		assert.ErrorIs(t, err, ErrPlatformCatalogBadKind)
	})

	t.Run("Scenario_DuplicateSlugRejectedToPreventClash", func(t *testing.T) {
		// Given table slug is the identity key,
		// When the same slug is registered twice,
		// Then the second attempt is rejected.
		r := NewPlatformCatalogRegistry()
		m := validPlatformCatalog()
		require.NoError(t, r.Register(m))
		err := r.Register(m)
		assert.ErrorIs(t, err, ErrPlatformCatalogDuplicateSlug)
	})

	t.Run("Scenario_NonPositiveRowCountRejectedBecauseEmptyCatalogIsBug", func(t *testing.T) {
		// Given an ah_core catalog with 0 rows is useless (tenant would
		// see nothing),
		// When admin registers with ExpectedRowCount=0,
		// Then registration fails with ErrPlatformCatalogBadRowCount.
		r := NewPlatformCatalogRegistry()
		bad := validPlatformCatalog()
		bad.ExpectedRowCount = 0
		err := r.Register(bad)
		assert.ErrorIs(t, err, ErrPlatformCatalogBadRowCount)
	})

	t.Run("Scenario_MigrationOrderEnforcedAcrossCatalogs", func(t *testing.T) {
		// Given golang-migrate uses numeric ordering and duplicate numbers
		// break that contract,
		// When two catalogs declare the same migration,
		// Then registry refuses to enable both — admin must renumber.
		r := NewPlatformCatalogRegistry()
		m1 := validPlatformCatalog()
		m1.Slug = "cat_a"
		m1.MigrationNumber = 200
		m2 := validPlatformCatalog()
		m2.Slug = "cat_b"
		m2.MigrationNumber = 200
		require.NoError(t, r.Register(m1))
		err := r.Register(m2)
		assert.ErrorIs(t, err, ErrPlatformCatalogDuplicateMigration)
	})

	t.Run("Scenario_ListAllSortedSoAdminUIIsDeterministic", func(t *testing.T) {
		// Given admin UI prefers alphabetic ordering for stability,
		// When ListAll is called,
		// Then output is sorted by slug.
		r := NewPlatformCatalogRegistry()
		m1 := validPlatformCatalog()
		m1.Slug = "zzz_last"
		m1.MigrationNumber = 300
		m2 := validPlatformCatalog()
		m2.Slug = "aaa_first"
		m2.MigrationNumber = 301
		require.NoError(t, r.Register(m1))
		require.NoError(t, r.Register(m2))
		all := r.ListAll()
		require.Equal(t, 2, len(all))
		assert.Equal(t, "aaa_first", all[0].Slug)
	})

	t.Run("Scenario_ElevenKindsCoverThePDFManifestComponentSpace", func(t *testing.T) {
		// Given PDF §6.1 enumerates 10 plugin-manifest component types
		// + AgentHub-specific operational templates,
		// When admin lists all kinds,
		// Then 11 kinds are bounded.
		assert.Equal(t, 11, len(AllPlatformCatalogKinds()))
	})
}
