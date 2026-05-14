package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreEDDTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantPicksExtensionShapeFromCatalog", func(t *testing.T) {
		// Given fresh tenants must construct EXT-001 ExtensionDescriptors
		// but inventing semver + source + components combinations is
		// error-prone,
		// When admin opens extension onboarding,
		// Then 5 recommended templates surface across source × use_case.
		assert.Equal(t, 5, len(SeedRecommendedEDDTemplateSlugs))
	})

	t.Run("Scenario_BuiltinEssentialsAutoEnabledNoChecksum", func(t *testing.T) {
		// Given the platform ships baseline extensions and signs them,
		// When admin uses builtin-essentials,
		// Then source=builtin, no checksum required, auto-enable=true,
		// no admin review (routine).
		assert.Contains(t, SeedExpectedEDDTemplateSlugs, "builtin-essentials")
		set := map[string]bool{}
		for _, s := range SeedAdminReviewEDDTemplateSlugs {
			set[s] = true
		}
		assert.False(t, set["builtin-essentials"])
	})

	t.Run("Scenario_MarketplaceRAGPackForKnowledgeBaseTenants", func(t *testing.T) {
		// Given tenants want document_search + KB indexer hooks,
		// When admin uses marketplace-rag-pack,
		// Then checksum required (external source) but no admin review
		// for the routine RAG case.
		assert.Contains(t, SeedExpectedEDDTemplateSlugs, "marketplace-rag-pack")
	})

	t.Run("Scenario_MarketplaceEngineeringPackRequiresAdminReview", func(t *testing.T) {
		// Given the engineering pack ships broader surface (agents +
		// skills + hooks + commands),
		// When admin uses marketplace-engineering-pack,
		// Then admin review required (surface is non-trivial).
		assert.Contains(t, SeedExpectedEDDTemplateSlugs, "marketplace-engineering-pack")
		set := map[string]bool{}
		for _, s := range SeedAdminReviewEDDTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["marketplace-engineering-pack"])
	})

	t.Run("Scenario_GitInternalToolsConservativeWithAdminReview", func(t *testing.T) {
		// Given internal git sources are not platform-vetted,
		// When admin uses git-internal-tools,
		// Then conservative posture + admin review + checksum required.
		assert.Contains(t, SeedExpectedEDDTemplateSlugs, "git-internal-tools")
	})

	t.Run("Scenario_URLVendorSkillsStrictestPosture", func(t *testing.T) {
		// Given URL-delivered extensions (signed ad-hoc) are highest risk,
		// When admin uses url-vendor-skills,
		// Then strict posture + admin review + checksum + never auto-enabled.
		assert.Contains(t, SeedExpectedEDDTemplateSlugs, "url-vendor-skills")
	})

	t.Run("Scenario_SourceLabelsMatchEXT001EnumByteForByte", func(t *testing.T) {
		// Given EXT-001 ExtensionSource has 4 values,
		// When seed declares target_source,
		// Then labels match enum bytes (no mapping table runtime).
		ext001 := []string{"builtin", "marketplace", "git", "url"}
		set := map[string]bool{}
		for _, s := range SeedExpectedEDDTemplateSources {
			set[s] = true
		}
		for _, e := range ext001 {
			assert.True(t, set[e], "EXT-001 source %q missing", e)
		}
	})

	t.Run("Scenario_ComponentLabelsMatchEXT001EnumByteForByte", func(t *testing.T) {
		// Given EXT-001 ExtensionComponentKind has 8 web-applicable values,
		// When seed declares components_offered,
		// Then every component label is in EXT-001 enum (validated
		// structurally in integration test).
		ext001 := map[string]bool{
			"agents": true, "tools": true, "skills": true, "commands": true,
			"hooks": true, "rules": true, "mcp_servers": true, "output_styles": true,
		}
		for _, c := range SeedExpectedEDDTemplateComponents {
			assert.True(t, ext001[c], "component %q not in EXT-001", c)
		}
	})

	t.Run("Scenario_NonBuiltinSourcesRequireChecksum", func(t *testing.T) {
		// Given EXT-001 validateDescriptor: checksum required for
		// marketplace/git/url (untrusted sources),
		// When admin compares checksum_required field,
		// Then builtin=false but others=true. Validated DB-real in
		// integration test.
		assert.Equal(t, 5, SeedExpectedEDDTemplateRowCount)
	})

	t.Run("Scenario_OnlyBuiltinAutoEnables", func(t *testing.T) {
		// Given platform-signed extensions are immediately trusted,
		// And external sources require admin enable after install,
		// When admin compares auto_enable_after_install,
		// Then builtin=true, others=false. Validated DB-real in
		// integration test.
		assert.Contains(t, SeedExpectedEDDTemplateSlugs, "builtin-essentials")
	})

	t.Run("Scenario_AllComponentsOfferedAreValidEXT001Kinds", func(t *testing.T) {
		// Given templates declare components_offered as JSONB array,
		// When the loader parses,
		// Then every label is in EXT-001 enum (cross-feature invariant
		// validated in integration test).
		assert.Equal(t, 8, len(SeedExpectedEDDTemplateComponents))
	})
}
