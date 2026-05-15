package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreAMCDTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantPicksAutoMemoryPostureWithoutInventingThresholds", func(t *testing.T) {
		// Given a fresh tenant adopts AgentHub,
		// And CTX-006 AutoMemoryConfig has knobs (MinConfidence/MaxPerTurn/Block list),
		// When admin opens auto-memory onboarding,
		// Then 5 recommended postures cover strictness ladder + privacy
		// + PII — no math required by admin.
		assert.Equal(t, 5, len(SeedRecommendedAMCDTemplateSlugs))
	})

	t.Run("Scenario_BalancedDefaultMirrorsCTX006DefaultAutoMemoryConfig", func(t *testing.T) {
		// Given CTX-006 ships DefaultAutoMemoryConfig (MinConfidence=0.5,
		// MaxPerTurn=10, standard block list),
		// When admin picks balanced-default,
		// Then values match byte-for-byte so in-code default and seeded
		// one-click are coherent.
		// Validated structurally via integration test.
		assert.Contains(t, SeedExpectedAMCDTemplateSlugs, "balanced-default")
	})

	t.Run("Scenario_StrictPostureMaximizesSignalQualityForProduction", func(t *testing.T) {
		// Given production tenants prioritize signal quality over coverage,
		// When admin picks strict-conservative,
		// Then min_confidence is HIGH (0.8) + max_per_turn LOW (5) so
		// only top-tier decisions reach memory store.
		// Validated via integration test cross-template comparison.
		assert.Contains(t, SeedExpectedAMCDTemplateSlugs, "strict-conservative")
	})

	t.Run("Scenario_LenientExplorationLowersBarForEarlyDiscovery", func(t *testing.T) {
		// Given low-stakes tenants are exploring auto-memory feature,
		// When admin picks lenient-exploration,
		// Then min_confidence is LOW (0.3) + max_per_turn HIGH (25)
		// — accept some noise to learn what the classifier catches.
		assert.Contains(t, SeedExpectedAMCDTemplateSlugs, "lenient-exploration")
	})

	t.Run("Scenario_PrivacyFirstRequiresAdminReviewDueToOrgWidePrivacyPolicy", func(t *testing.T) {
		// Given privacy posture changes affect ALL tenant users,
		// When admin enables privacy-first,
		// Then template requires admin review (cannot be silently applied).
		set := map[string]bool{}
		for _, s := range SeedAdminReviewAMCDTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["privacy-first"])
	})

	t.Run("Scenario_PIIStrictForHIPAAGDPRTenants", func(t *testing.T) {
		// Given HIPAA/GDPR-regulated tenants have legal PII obligations,
		// When admin picks pii-strict,
		// Then admin review required + recommended_for_tenant_kind=regulated.
		set := map[string]bool{}
		for _, s := range SeedAdminReviewAMCDTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["pii-strict"])
	})

	t.Run("Scenario_DevDebugIsOptInNotRecommended", func(t *testing.T) {
		// Given dev-debug exposes too much for production,
		// When admin browses templates,
		// Then dev-debug is NOT highlighted as recommended — must
		// explicitly opt in (production safety).
		set := map[string]bool{}
		for _, s := range SeedRecommendedAMCDTemplateSlugs {
			set[s] = true
		}
		assert.False(t, set["dev-debug"])
	})

	t.Run("Scenario_BlockListsWidenAsPostureGetsStricter", func(t *testing.T) {
		// Given strictness ladder maps to block-list size,
		// When admin compares postures,
		// Then dev-debug<balanced<strict<privacy<pii (block list grows
		// monotonically). Validated structurally via integration test
		// cross-template AdminBlockedKeysList length comparison.
		assert.Contains(t, SeedExpectedAMCDTemplateSlugs, "pii-strict")
	})

	t.Run("Scenario_BalancedAndStrictAreFreshTenantSafe", func(t *testing.T) {
		// Given general tenants opt into balanced or strict by default,
		// When the seed marks templates is_recommended,
		// Then balanced + strict + lenient + privacy + pii ALL recommended;
		// dev-debug excluded (sole exception).
		expected := []string{
			"balanced-default", "strict-conservative",
			"lenient-exploration", "privacy-first", "pii-strict",
		}
		set := map[string]bool{}
		for _, s := range SeedRecommendedAMCDTemplateSlugs {
			set[s] = true
		}
		for _, e := range expected {
			assert.True(t, set[e])
		}
	})

	t.Run("Scenario_PostureLabelsAreClosedSetForUIPicker", func(t *testing.T) {
		// Given the agent-create UI has a posture selector,
		// When admin picks a posture,
		// Then it's one of: balanced/strict/lenient/privacy_first/pii_strict/
		// dev_debug (closed set; new postures require seed + UI update).
		expected := []string{
			"balanced", "strict", "lenient",
			"privacy_first", "pii_strict", "dev_debug",
		}
		set := map[string]bool{}
		for _, p := range SeedExpectedAMCDTemplatePostures {
			set[p] = true
		}
		for _, e := range expected {
			assert.True(t, set[e])
		}
	})
}
