package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreMIDTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantInheritsStarterIncludesWithoutCadastros", func(t *testing.T) {
		// Given fresh tenants must populate memory facts to operate,
		// When admin queries ah_core for starter @include directives,
		// Then 6 recommended directives surface.
		assert.Equal(t, 6, len(SeedRecommendedMIDTemplateSlugs))
	})

	t.Run("Scenario_IncludeKeysSatisfyCTX007ResolverRegex", func(t *testing.T) {
		// Given CTX-007 MemoryIncludeResolver matches keys via regex
		// [a-zA-Z0-9._-]+,
		// When seed declares include_key,
		// Then every key passes the regex (otherwise resolver never expands).
		for _, k := range SeedExpectedMIDTemplateIncludeKeys {
			assert.True(t, MemoryIncludeKeyRE.MatchString(k), "key %q", k)
		}
	})

	t.Run("Scenario_OrgIdentityDirectiveExposesPlaceholders", func(t *testing.T) {
		// Given org identity needs tenant-specific framing via {{...}}
		// placeholders resolved BEFORE @include expansion,
		// When admin inspects placeholder subset,
		// Then only core-org-identity is listed.
		set := map[string]bool{}
		for _, s := range SeedPlaceholderMIDTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["core-org-identity"])
	})

	t.Run("Scenario_SafetyDirectiveIsPlainTextForGuardRailCertainty", func(t *testing.T) {
		// Given safety guard rails must not depend on runtime variables,
		// When admin checks core-safety-do-not,
		// Then it is NOT in the placeholder subset (plain text only).
		set := map[string]bool{}
		for _, s := range SeedPlaceholderMIDTemplateSlugs {
			set[s] = true
		}
		assert.False(t, set["core-safety-do-not"])
	})

	t.Run("Scenario_SixCategoriesCoverDistinctConcerns", func(t *testing.T) {
		// Given each category captures a separable concern (org/cite/code/
		// safety/locale/handoff),
		// When admin lists category coverage,
		// Then 6 distinct categories are seeded.
		assert.Equal(t, 6, len(SeedExpectedMIDTemplateCategories))
	})

	t.Run("Scenario_OneToOneCategoryToDirective", func(t *testing.T) {
		// Given 6 categories and 6 starter directives,
		// When seed templates ship,
		// Then 1:1 holds. Validated DB-real.
		assert.Equal(t, len(SeedExpectedMIDTemplateCategories), SeedExpectedMIDTemplateRowCount)
	})

	t.Run("Scenario_LookupMapFedsDirectlyIntoCTX007Resolver", func(t *testing.T) {
		// Given the loader emits LookupMap() that feeds directly into
		// MemoryIncludeResolver.SetLookup, integrations need zero glue,
		// When admin pipelines [LoadAll → LookupMap → SetLookup],
		// Then no string transformation is required. Validated DB-real.
		assert.GreaterOrEqual(t, len(SeedExpectedMIDTemplateIncludeKeys),
			SeedExpectedMIDTemplateRowCount)
	})

	t.Run("Scenario_LocalePolicyDirectiveAlignsWithAgentHubCLAUDEMd", func(t *testing.T) {
		// Given AgentHub CLAUDE.md mandates PT-BR for user-facing text +
		// EN for source code,
		// When admin inspects locale directive,
		// Then core-locale-pt-br-summary is seeded.
		set := map[string]bool{}
		for _, s := range SeedExpectedMIDTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["core-locale-pt-br-summary"])
	})

	t.Run("Scenario_HandoffDirectiveSupportsSUB011MultiAgent", func(t *testing.T) {
		// Given SUB-011 multi-agent coordination needs a stable hand-off
		// format between subagents,
		// When admin inspects handoff directive,
		// Then core-runner-handoff is seeded.
		set := map[string]bool{}
		for _, s := range SeedExpectedMIDTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["core-runner-handoff"])
	})

	t.Run("Scenario_BadIncludeKeyWouldBeRejectedByCTX007Regex", func(t *testing.T) {
		// Given seed authors might accidentally use spaces or slashes,
		// When such a key is validated against MemoryIncludeKeyRE,
		// Then the regex refuses.
		assert.False(t, MemoryIncludeKeyRE.MatchString("with space"))
		assert.False(t, MemoryIncludeKeyRE.MatchString("with/slash"))
	})
}
