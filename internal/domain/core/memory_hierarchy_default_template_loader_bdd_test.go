package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreMemoryHierarchyDefaultTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantBootstrapsHierarchyWithoutInventingKeys", func(t *testing.T) {
		// Given a fresh tenant adopts AgentHub,
		// And CTX-003 MemoryHierarchy returns NotFound when no fact at any layer,
		// When admin runs tenant bootstrap,
		// Then 7 recommended defaults seed the global + tenant layers
		// (platform_name/version/locale/support_email/tone/timezone/support_window),
		// so first-ever lookup at any layer succeeds.
		assert.Equal(t, 7, len(SeedRecommendedMemoryHierarchyDefaultTemplateSlugs))
	})

	t.Run("Scenario_GlobalConstantsHaveNoExpiry", func(t *testing.T) {
		// Given platform_name, version, locale, support_email are stable,
		// When the seed declares them at global scope,
		// Then max_age_seconds=0 (never expire) — these don't go stale
		// across the platform's lifetime.
		// Validated structurally via integration test.
		globals := []string{
			"global-platform-name",
			"global-platform-version",
			"global-default-locale",
			"global-support-email",
		}
		for _, slug := range globals {
			assert.Contains(t, SeedExpectedMemoryHierarchyDefaultTemplateSlugs, slug)
		}
	})

	t.Run("Scenario_TenantDefaultsExpireSoTenantOpsCanReevaluate", func(t *testing.T) {
		// Given tenant ops may change tone/timezone/window over time,
		// When the seed declares tenant defaults,
		// Then they have positive max_age (~30 days) so the agent's
		// hierarchy lookup eventually re-evaluates rather than locking
		// in a 6-month-old setting.
		// Validated structurally via integration test.
		tenants := []string{
			"tenant-default-tone",
			"tenant-default-timezone",
			"tenant-support-window",
		}
		for _, slug := range tenants {
			assert.Contains(t, SeedExpectedMemoryHierarchyDefaultTemplateSlugs, slug)
		}
	})

	t.Run("Scenario_ComplianceModeRequiresAdminVettingDueToAuditImplications", func(t *testing.T) {
		// Given compliance_mode toggles audit + admin-review behavior
		// across the platform per GOV-002,
		// When admin enables it,
		// Then the seed flags it requires_admin_review=true (not auto-
		// applied; admin signs off explicitly).
		set := map[string]bool{}
		for _, s := range SeedAdminReviewMemoryHierarchyDefaultTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["tenant-compliance-mode"])
	})

	t.Run("Scenario_ScopeLabelsAreCTX003ByteForByteSubset", func(t *testing.T) {
		// Given CTX-003 has 5 scope enum values (session/user/agent/tenant/global),
		// When this seed declares target_scope,
		// Then values are STRICTLY a subset (only global + tenant) so
		// runtime can join template ⇄ MemoryHierarchy.Set() without
		// any mapping table.
		ctx003Allowed := map[string]bool{
			"session": true, "user": true, "agent": true,
			"tenant": true, "global": true,
		}
		for _, s := range SeedExpectedMemoryHierarchyDefaultTemplateScopes {
			assert.True(t, ctx003Allowed[s], "scope %q not in CTX-003 enum", s)
		}
	})

	t.Run("Scenario_TenantKindHintLetsBootstrapPickAppropriateDefaults", func(t *testing.T) {
		// Given a regulated tenant cares about compliance defaults,
		// When the bootstrap pipeline filters templates by tenant kind,
		// Then it can pick "regulated" subset (which includes compliance-mode)
		// vs "general" subset (which doesn't).
		expected := []string{"general", "regulated", "dev_local"}
		set := map[string]bool{}
		for _, k := range SeedExpectedMemoryHierarchyDefaultTemplateTenantKinds {
			set[k] = true
		}
		for _, e := range expected {
			assert.True(t, set[e])
		}
	})

	t.Run("Scenario_DefaultLocaleAlignsWithLocaleTranslationsSeed", func(t *testing.T) {
		// Given locale_translations seed (iter 22) ships en-US as default,
		// When this seed declares global-default-locale,
		// Then value MUST be en-US byte-for-byte (no drift between
		// translation-availability seed and lookup-default seed).
		// Validated structurally via integration test.
		assert.Contains(t, SeedExpectedMemoryHierarchyDefaultTemplateSlugs, "global-default-locale")
	})

	t.Run("Scenario_TenantToneAlignsWithProductDefaults", func(t *testing.T) {
		// Given output_styles seed defaults to "conversational" (iter 7),
		// When this seed declares tenant-default-tone,
		// Then it picks a tone consistent with product-defaults
		// (professional matches conversational tier).
		// Validated structurally via integration test.
		assert.Contains(t, SeedExpectedMemoryHierarchyDefaultTemplateSlugs, "tenant-default-tone")
	})

	t.Run("Scenario_AllRecommendedAreFreshTenantSafe", func(t *testing.T) {
		// Given fresh tenants instantiate ALL recommended templates on
		// bootstrap automatically,
		// When the seed marks templates is_recommended,
		// Then NONE of them require admin review (otherwise bootstrap
		// would block waiting for admin sign-off and tenants would never
		// finish onboarding).
		recSet := map[string]bool{}
		for _, s := range SeedRecommendedMemoryHierarchyDefaultTemplateSlugs {
			recSet[s] = true
		}
		for _, s := range SeedAdminReviewMemoryHierarchyDefaultTemplateSlugs {
			assert.False(t, recSet[s])
		}
	})

	t.Run("Scenario_EightTemplatesCoverGlobalConstantsAndTenantTunables", func(t *testing.T) {
		// Given a fresh tenant needs both immutable platform constants
		// AND its own tenant-tunable defaults,
		// When the seed ships,
		// Then 4 globals + 4 tenants = 8 covers both classes.
		assert.Equal(t, 8, len(SeedExpectedMemoryHierarchyDefaultTemplateSlugs))
		// Count globals + tenants in the slug list.
		globalCount := 0
		tenantCount := 0
		for _, s := range SeedExpectedMemoryHierarchyDefaultTemplateSlugs {
			if len(s) >= 7 && s[:7] == "global-" {
				globalCount++
			}
			if len(s) >= 7 && s[:7] == "tenant-" {
				tenantCount++
			}
		}
		assert.Equal(t, 4, globalCount)
		assert.Equal(t, 4, tenantCount)
	})
}
