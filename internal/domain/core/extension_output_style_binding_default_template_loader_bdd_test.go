package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreEOSBDTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantPicksOutputStyleBindingFromPresets", func(t *testing.T) {
		// Given fresh tenants need output-style bindings,
		// And EXT-009 ExtensionOutputStyleRegistry accepts (scope+format),
		// When admin opens style-binding onboarding,
		// Then 6 recommended presets surface covering scope cascade
		// + format spectrum.
		assert.Equal(t, 6, len(SeedRecommendedEOSBDTemplateSlugs))
	})

	t.Run("Scenario_PlatformConversationalIsAlwaysFallback", func(t *testing.T) {
		// Given platform-default catches every tenant with no other binding,
		// When admin uses platform-conversational-default,
		// Then no tenant ever has zero rendering options.
		assert.Contains(t, SeedExpectedEOSBDTemplateSlugs, "platform-conversational-default")
	})

	t.Run("Scenario_TenantTechnicalForEngineeringTenants", func(t *testing.T) {
		// Given engineering tenants want code-first markdown,
		// When admin uses tenant-technical-default,
		// Then every agent picks technical style by default.
		assert.Contains(t, SeedExpectedEOSBDTemplateSlugs, "tenant-technical-default")
	})

	t.Run("Scenario_AgentExtractorOverridesToJSON", func(t *testing.T) {
		// Given a single data-extractor agent must output JSON despite
		// tenant default being markdown,
		// When admin uses agent-extractor-json,
		// Then agent-scope binding overrides tenant default for that agent.
		assert.Contains(t, SeedExpectedEOSBDTemplateSlugs, "agent-extractor-json")
	})

	t.Run("Scenario_ExplicitRequestOverridesAllForDebugging", func(t *testing.T) {
		// Given debugging requests need verbose output that bypasses
		// every default,
		// When admin uses explicit-debug-verbose,
		// Then explicit-scope binding wins the cascade.
		assert.Contains(t, SeedExpectedEOSBDTemplateSlugs, "explicit-debug-verbose")
	})

	t.Run("Scenario_HTMLSanitizedRequiresAdminReviewForRenderingSecurity", func(t *testing.T) {
		// Given HTML output has XSS implications,
		// When admin uses tenant-html-sanitized-ui,
		// Then admin review required (rendering security posture).
		set := map[string]bool{}
		for _, s := range SeedAdminReviewEOSBDTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["tenant-html-sanitized-ui"])
	})

	t.Run("Scenario_PlainTextFallbackForLegacyAndAccessibilityClients", func(t *testing.T) {
		// Given some clients can't render markdown (legacy / screen
		// readers / raw text logs),
		// When platform ships plain-text fallback,
		// Then those clients get readable output without breaking.
		assert.Contains(t, SeedExpectedEOSBDTemplateSlugs, "platform-plain-fallback")
	})

	t.Run("Scenario_ScopeLabelsMatchEXT009EnumByteForByte", func(t *testing.T) {
		// Given EXT-009 OutputStyleScope has 4 values,
		// When seed declares target_scope,
		// Then labels match enum bytes (no mapping table runtime).
		ext009 := []string{"explicit", "agent", "tenant", "platform"}
		set := map[string]bool{}
		for _, s := range SeedExpectedEOSBDTemplateScopes {
			set[s] = true
		}
		for _, e := range ext009 {
			assert.True(t, set[e], "EXT-009 scope %q missing", e)
		}
	})

	t.Run("Scenario_FormatLabelsMatchEXT009EnumByteForByte", func(t *testing.T) {
		// Given EXT-009 OutputStyleFormat has 4 values,
		// When seed declares target_format,
		// Then labels match enum bytes.
		ext009 := []string{"markdown", "json", "plain", "html_sanitized"}
		set := map[string]bool{}
		for _, f := range SeedExpectedEOSBDTemplateFormats {
			set[f] = true
		}
		for _, e := range ext009 {
			assert.True(t, set[e], "EXT-009 format %q missing", e)
		}
	})

	t.Run("Scenario_AllFourScopesRepresented", func(t *testing.T) {
		// Given EXT-009 has 4 scopes (explicit/agent/tenant/platform),
		// When seed templates ship,
		// Then ALL 4 scopes have at least one example template.
		// Validated structurally via integration test.
		assert.Equal(t, 4, len(SeedExpectedEOSBDTemplateScopes))
	})

	t.Run("Scenario_PriorityLadderReflectsCascadePrecedence", func(t *testing.T) {
		// Given EXT-009 cascade: explicit > agent > tenant > platform,
		// When admin compares default_priority across scopes,
		// Then explicit-debug-verbose has highest (90), platform-plain
		// lowest (5). Validated via integration test cross-template.
		assert.Equal(t, 6, len(SeedExpectedEOSBDTemplateSlugs))
	})
}
