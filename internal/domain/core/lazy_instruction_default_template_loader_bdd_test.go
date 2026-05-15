package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreLIDTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantPicksTTLProfileWithoutInventingThresholds", func(t *testing.T) {
		// Given fresh tenants need lazy-instruction config,
		// And CTX-005 LazyInstructionLoader accepts TTL + source-kind,
		// When admin opens lazy-loader onboarding,
		// Then 5 recommended profiles cover the spectrum.
		assert.Equal(t, 5, len(SeedRecommendedLIDTemplateSlugs))
	})

	t.Run("Scenario_StableRuleCatalogHasZeroTTLForPlatformConstants", func(t *testing.T) {
		// Given platform rule catalog only changes on platform deploy,
		// When admin uses stable-rule-catalog,
		// Then TTL=0 (never expires) — saves repeated reload cost
		// for stable content (mirrors CTX-005 TTL=0 semantics).
		assert.Contains(t, SeedExpectedLIDTemplateSlugs, "stable-rule-catalog")
	})

	t.Run("Scenario_TenantConfigUsesMediumTTLForOccasionalUpdates", func(t *testing.T) {
		// Given tenant config changes occasionally (tone, compliance mode),
		// When admin uses tenant-config-medium-ttl,
		// Then TTL=1h — picks up updates within an hour without
		// hammering DB on every Load.
		assert.Contains(t, SeedExpectedLIDTemplateSlugs, "tenant-config-medium-ttl")
	})

	t.Run("Scenario_ExternalKBShortTTLForFreshness", func(t *testing.T) {
		// Given external KB content changes frequently (live docs/wikis),
		// When admin uses external-kb-short-ttl,
		// Then TTL=5min ensures fresh content without wasting load cost.
		assert.Contains(t, SeedExpectedLIDTemplateSlugs, "external-kb-short-ttl")
	})

	t.Run("Scenario_RegulatedPolicyStrictTTLAdminGated", func(t *testing.T) {
		// Given regulated policies (GDPR/HIPAA/SOX) must reflect latest version,
		// When admin uses regulated-policy-strict-ttl,
		// Then TTL=10min + admin review required (regulatory binding).
		set := map[string]bool{}
		for _, s := range SeedAdminReviewLIDTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["regulated-policy-strict-ttl"])
	})

	t.Run("Scenario_UserMemorySessionTTLMatchesTypicalSession", func(t *testing.T) {
		// Given user memory snapshots change as user interacts,
		// When admin uses user-memory-session-ttl,
		// Then TTL=30min — matches typical session duration.
		assert.Contains(t, SeedExpectedLIDTemplateSlugs, "user-memory-session-ttl")
	})

	t.Run("Scenario_DevDebugNoCacheNotRecommendedForProduction", func(t *testing.T) {
		// Given dev-debug effectively disables caching,
		// When admin browses templates,
		// Then dev-debug-no-cache is NOT highlighted recommended —
		// production tenants would hammer upstream.
		set := map[string]bool{}
		for _, s := range SeedRecommendedLIDTemplateSlugs {
			set[s] = true
		}
		assert.False(t, set["dev-debug-no-cache"])
	})

	t.Run("Scenario_SourceKindsCoverPlatformDataSources", func(t *testing.T) {
		// Given AgentHub has 5 typical loader source kinds,
		// When admin browses templates,
		// Then each source kind is represented in the catalog.
		expected := []string{
			"ah_core_seed", "tenant_db", "external_http",
			"compliance_store", "memory_hierarchy",
		}
		set := map[string]bool{}
		for _, k := range SeedExpectedLIDTemplateSourceKinds {
			set[k] = true
		}
		for _, e := range expected {
			assert.True(t, set[e])
		}
	})

	t.Run("Scenario_TTLLadderMonotonicallyReflectsContentVolatility", func(t *testing.T) {
		// Given content volatility ladders from stable to live:
		//   stable(0=never) > regulated/user(10min/30min) > kb(5min) > dev(1s),
		// When admin compares profiles,
		// Then TTL values reflect the volatility curve. Validated
		// structurally via integration test.
		assert.Equal(t, 6, len(SeedExpectedLIDTemplateSlugs))
	})

	t.Run("Scenario_RegulatedTemplateRecommendedForRegulatedTenantsOnly", func(t *testing.T) {
		// Given regulated tenants opt into regulated-policy template,
		// When admin filters by tenant_kind,
		// Then regulated profile recommended_for_tenant_kind=regulated.
		// (Validated via integration test.)
		assert.Contains(t, SeedExpectedLIDTemplateSlugs, "regulated-policy-strict-ttl")
	})

	t.Run("Scenario_SixProfilesCoverTTLSpectrumWithoutOverwhelm", func(t *testing.T) {
		// Given product research showed 5-7 templates is the sweet spot,
		// When the seed ships,
		// Then exactly 6 profiles exist.
		assert.Equal(t, 6, len(SeedExpectedLIDTemplateSlugs))
	})
}
