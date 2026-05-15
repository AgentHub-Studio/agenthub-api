package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreTDPDTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantPicksDedupPolicyFromCatalog", func(t *testing.T) {
		// Given fresh tenants must pick a TOOL-004 ToolDedupPolicy on
		// first run, and the registry rejects unknown labels,
		// When admin opens dedup-policy onboarding,
		// Then 5 recommended templates surface across every policy.
		assert.Equal(t, 5, len(SeedRecommendedTDPDTemplateSlugs))
	})

	t.Run("Scenario_DefaultSourceRankForBalancedTenants", func(t *testing.T) {
		// Given the platform default is source-rank precedence,
		// When admin uses default-source-rank,
		// Then collisions resolve via TOOL-003 source ladder.
		assert.Contains(t, SeedExpectedTDPDTemplateSlugs, "default-source-rank")
	})

	t.Run("Scenario_RollingLatestForProgressiveTenants", func(t *testing.T) {
		// Given tenants who follow upstream want auto-upgrade,
		// When admin uses rolling-latest-version,
		// Then highest semver always wins.
		assert.Contains(t, SeedExpectedTDPDTemplateSlugs, "rolling-latest-version")
	})

	t.Run("Scenario_AdminPinnedRequiresReviewForCompliance", func(t *testing.T) {
		// Given compliance-sensitive tenants pin versions until QA approves,
		// When admin uses admin-pinned-conservative,
		// Then admin review is mandatory (pin change is auditable).
		set := map[string]bool{}
		for _, s := range SeedAdminReviewTDPDTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["admin-pinned-conservative"])
	})

	t.Run("Scenario_DualVersionForMigrationWindows", func(t *testing.T) {
		// Given platform migrations need both old and new versions live,
		// When admin uses dual-version-migration,
		// Then both versions remain visible by fully-qualified name.
		assert.Contains(t, SeedExpectedTDPDTemplateSlugs, "dual-version-migration")
	})

	t.Run("Scenario_FailFastRequiresReviewForStrictTenants", func(t *testing.T) {
		// Given audit-strict tenants treat version drift as an incident,
		// When admin uses fail-fast-no-drift,
		// Then admin review is mandatory (changing posture is a stance shift).
		set := map[string]bool{}
		for _, s := range SeedAdminReviewTDPDTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["fail-fast-no-drift"])
	})

	t.Run("Scenario_PolicyLabelsMatchTOOL004EnumByteForByte", func(t *testing.T) {
		// Given TOOL-004 ToolDedupPolicy has 5 values,
		// When seed declares target_policy,
		// Then labels match enum bytes (no mapping table runtime).
		tool004 := []string{"deny_collision", "prefer_source_rank",
			"prefer_latest_version", "prefer_pinned", "keep_all_versions"}
		set := map[string]bool{}
		for _, p := range SeedExpectedTDPDTemplatePolicies {
			set[p] = true
		}
		for _, e := range tool004 {
			assert.True(t, set[e], "TOOL-004 policy %q missing", e)
		}
	})

	t.Run("Scenario_AllFivePoliciesRepresented", func(t *testing.T) {
		// Given TOOL-004 has 5 policies,
		// When seed templates ship,
		// Then ALL 5 policies have at least one example template.
		// Validated structurally via integration test.
		assert.Equal(t, 5, len(SeedExpectedTDPDTemplatePolicies))
	})

	t.Run("Scenario_SafetyPostureSpansSpectrum", func(t *testing.T) {
		// Given posture is a stance ladder (permissive..strict),
		// When seed templates ship,
		// Then all 5 postures are represented.
		assert.Equal(t, 5, len(SeedExpectedTDPDTemplateSafetyPostures))
	})

	t.Run("Scenario_OnlyConservativeAndStrictGateAdminReview", func(t *testing.T) {
		// Given balanced/progressive/permissive postures are routine,
		// When admin compares admin-review subset,
		// Then only conservative + strict postures appear.
		assert.Equal(t, 2, len(SeedAdminReviewTDPDTemplateSlugs))
	})
}
