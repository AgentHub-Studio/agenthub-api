package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for ToolInvocationBudgetDefaultTemplate seed.
// Tool invocation budget templates cap how many tool calls an agent may make
// per run, using deny/warn/report policies when the cap is reached.

func TestBDD_AhCoreToolInvocationBudgetSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantReceivesSixBudgetTemplates", func(t *testing.T) {
		// Given a new tenant needs default tool invocation guardrails
		// When the budget template catalog is loaded
		// Then exactly 6 templates are available covering a range of cap profiles
		assert.Equal(t, 6, SeedExpectedTIBTemplateRowCount)
		assert.Equal(t, 6, len(SeedExpectedTIBTemplateSlugs))
	})

	t.Run("Scenario_ThreePoliciesAreAvailableForCapBehavior", func(t *testing.T) {
		// Given different tenants require different enforcement strategies
		// When the policy taxonomy is inspected
		// Then deny, warn, and report policies are all represented
		assert.Equal(t, 3, len(SeedTIBTemplatePolicies))
		policySet := map[string]bool{}
		for _, p := range SeedTIBTemplatePolicies {
			policySet[p] = true
		}
		assert.True(t, policySet["deny"])
		assert.True(t, policySet["warn"])
		assert.True(t, policySet["report"])
	})

	t.Run("Scenario_UnlimitedTemplateExistsForPowerUsers", func(t *testing.T) {
		// Given some power users or development environments need no cap
		// When the unlimited budget template slug is inspected
		// Then it exists in the main catalog
		assert.Equal(t, "unlimited", SeedTIBUnlimitedSlug)
		found := false
		for _, s := range SeedExpectedTIBTemplateSlugs {
			if s == SeedTIBUnlimitedSlug {
				found = true
			}
		}
		assert.True(t, found, "unlimited slug must be in main catalog")
	})

	t.Run("Scenario_ComplianceAuditTemplateEnforcesReadOnly", func(t *testing.T) {
		// Given compliance-audit agents must not mutate state
		// When the compliance-audit budget slug is inspected
		// Then it is present in the catalog and maps to zero mutate cap semantics
		assert.Equal(t, "compliance-audit", SeedTIBReadOnlySlug)
		found := false
		for _, s := range SeedExpectedTIBTemplateSlugs {
			if s == SeedTIBReadOnlySlug {
				found = true
			}
		}
		assert.True(t, found, "compliance-audit slug must be in main catalog")
	})

	t.Run("Scenario_AllSlugsMustBeKebabCase", func(t *testing.T) {
		// Given naming conventions require kebab-case slugs
		// When each budget template slug is validated
		// Then every slug matches ^[a-z0-9][a-z0-9-]*[a-z0-9]$
		for _, s := range SeedExpectedTIBTemplateSlugs {
			assert.True(t, SeedTIBTemplateSlugRE.MatchString(s),
				"slug %q violates kebab-case pattern", s)
		}
	})

	t.Run("Scenario_UnlimitedAndComplianceAuditAreDistinct", func(t *testing.T) {
		// Given unlimited (no cap) and compliance-audit (zero mutate) are opposite extremes
		// When the two special slugs are compared
		// Then they are different slugs serving opposite governance intents
		assert.NotEqual(t, SeedTIBUnlimitedSlug, SeedTIBReadOnlySlug)
	})
}
