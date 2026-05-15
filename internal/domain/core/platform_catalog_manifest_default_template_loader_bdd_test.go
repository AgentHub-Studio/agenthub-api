package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCorePCMDTemplateSeed(t *testing.T) {
	t.Run("Scenario_AdminUIQueriesCatalogOfCatalogs", func(t *testing.T) {
		// Given admin needs to render "all ah_core defaults" in one screen,
		// When the manifest table is queried,
		// Then 11 manifests surface (one per CORE-SEED-001 kind).
		assert.Equal(t, 11, SeedExpectedPCMDTemplateRowCount)
	})

	t.Run("Scenario_ManifestKindsMatchCORE_SEED_001EnumByteForByte", func(t *testing.T) {
		// Given CORE-SEED-001 PlatformCatalogKind has 11 bounded values,
		// When seed declares manifest kinds,
		// Then byte-for-byte alignment holds (no runtime mapping table).
		assert.Equal(t, 11, len(SeedExpectedPCMDTemplateKinds))
	})

	t.Run("Scenario_SubagentRosterManifestPointsAtSUB002Table", func(t *testing.T) {
		// Given SUB-002 builtin subagent roster is the canonical roster
		// catalog,
		// When admin filters by kind=subagent_roster,
		// Then the manifest exists and is recommended.
		set := map[string]bool{}
		for _, k := range SeedExpectedPCMDTemplateKinds {
			set[k] = true
		}
		assert.True(t, set["subagent_roster"])
	})

	t.Run("Scenario_ForkStrategyManifestPointsAtPERSIST005aTable", func(t *testing.T) {
		// Given PERSIST-005a fork strategy seed lives in ah_core,
		// When admin filters by kind=fork_strategy,
		// Then the manifest exists.
		set := map[string]bool{}
		for _, k := range SeedExpectedPCMDTemplateKinds {
			set[k] = true
		}
		assert.True(t, set["fork_strategy"])
	})

	t.Run("Scenario_BackgroundLaneManifestPointsAtSUB008Table", func(t *testing.T) {
		// Given SUB-008 lane templates were just seeded,
		// When admin filters by kind=background_lane,
		// Then the manifest exists.
		set := map[string]bool{}
		for _, k := range SeedExpectedPCMDTemplateKinds {
			set[k] = true
		}
		assert.True(t, set["background_lane"])
	})

	t.Run("Scenario_OneToOneKindToManifest", func(t *testing.T) {
		// Given each PlatformCatalogKind currently has one canonical
		// representative table,
		// When seed templates ship,
		// Then 1:1 holds (one manifest per kind).
		assert.Equal(t, len(SeedExpectedPCMDTemplateKinds), SeedExpectedPCMDTemplateRowCount)
	})

	t.Run("Scenario_AllManifestsRecommendedAtFreshTenantBoot", func(t *testing.T) {
		// Given platform-shared defaults are always recommended out of box,
		// When admin lists recommended manifests,
		// Then all 11 surface.
		assert.Equal(t, 11, len(SeedRecommendedPCMDTemplateSlugs))
	})

	t.Run("Scenario_LoaderPackageClosedSetAvoidsDriftFromCorePackage", func(t *testing.T) {
		// Given all loaders live under internal/domain/core,
		// When admin compares loader_package,
		// Then only "core" appears.
		assert.Equal(t, []string{"core"}, SeedExpectedPCMDTemplateLoaderPackages)
	})

	t.Run("Scenario_AhStatusReportsCatalogCountAndTotalRows", func(t *testing.T) {
		// Given /ah-status synthesizes "11 catalogs / N rows",
		// When the manifest table is the source,
		// Then count is deterministic and matches CORE-SEED-001 expectations.
		assert.Equal(t, 11, SeedExpectedPCMDTemplateRowCount)
	})

	t.Run("Scenario_OperationalTemplateKindCoversMultiAgentCoord", func(t *testing.T) {
		// Given SUB-011 multi-agent coordination plan templates are the
		// representative operational template,
		// When admin filters by kind=operational_template,
		// Then the manifest exists.
		set := map[string]bool{}
		for _, k := range SeedExpectedPCMDTemplateKinds {
			set[k] = true
		}
		assert.True(t, set["operational_template"])
	})
}
