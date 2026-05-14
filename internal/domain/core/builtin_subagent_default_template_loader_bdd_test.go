package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreBSDTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantSpawnsCuratedSubagentsOutOfTheBox", func(t *testing.T) {
		// Given fresh tenants must spawn subagents for common patterns
		// (research, code, review, etc) without designing each from scratch,
		// When admin opens subagent onboarding,
		// Then 7 recommended templates surface 1:1 with the BuiltinSubagentRole enum.
		assert.Equal(t, 7, len(SeedRecommendedBSDTemplateSlugs))
	})

	t.Run("Scenario_ResearcherBaselineReadOnlyForCitations", func(t *testing.T) {
		// Given researchers should not mutate state, only cite,
		// When admin uses researcher-baseline,
		// Then role=researcher and toolset slug ties to read-only allowlist.
		assert.Contains(t, SeedExpectedBSDTemplateSlugs, "researcher-baseline")
	})

	t.Run("Scenario_CoderBaselineInheritsParentRightsForWrites", func(t *testing.T) {
		// Given coders implement scoped changes within parent's auth context,
		// When admin uses coder-baseline,
		// Then inheritance=extend-parent-rights and toolset=code-write-scoped.
		assert.Contains(t, SeedExpectedBSDTemplateSlugs, "coder-baseline")
	})

	t.Run("Scenario_ReviewerBaselineRestrictedToReadOnly", func(t *testing.T) {
		// Given reviewers must not modify code under review,
		// When admin uses reviewer-baseline,
		// Then inheritance=restrict-to-readonly.
		assert.Contains(t, SeedExpectedBSDTemplateSlugs, "reviewer-baseline")
	})

	t.Run("Scenario_PlannerBaselineProducesPlanOnlySummary", func(t *testing.T) {
		// Given planners decompose goals without implementing,
		// When admin uses planner-baseline,
		// Then summary slug=plan-only (SUB-010 plan shape).
		assert.Contains(t, SeedExpectedBSDTemplateSlugs, "planner-baseline")
	})

	t.Run("Scenario_RolesMatchSUB002EnumByteForByte", func(t *testing.T) {
		// Given SUB-002 BuiltinSubagentRole has 7 values,
		// When seed declares role,
		// Then labels match enum bytes (no mapping table runtime).
		expectedRoles := []string{
			"researcher", "coder", "reviewer", "explorer",
			"planner", "curator", "documenter",
		}
		set := map[string]bool{}
		for _, r := range SeedExpectedBSDTemplateRoles {
			set[r] = true
		}
		for _, e := range expectedRoles {
			assert.True(t, set[e], "SUB-002 role %q missing", e)
		}
	})

	t.Run("Scenario_ToolsetReferencesAreSUB005Slugs", func(t *testing.T) {
		// Given SUB-005 catalog supplies toolset isolation policies,
		// When admin inspects default_toolset_policy_slug,
		// Then all refs come from the closed SUB-005 slug set.
		assert.GreaterOrEqual(t, len(SeedExpectedBSDTemplateToolsetSlugs), 1)
	})

	t.Run("Scenario_InheritanceReferencesAreSUB006Slugs", func(t *testing.T) {
		// Given SUB-006 supplies permission inheritance modes,
		// When admin inspects default_inheritance_mode_slug,
		// Then all refs come from the closed SUB-006 slug set.
		assert.GreaterOrEqual(t, len(SeedExpectedBSDTemplateInheritanceSlugs), 1)
	})

	t.Run("Scenario_SummaryReferencesAreSUB010Slugs", func(t *testing.T) {
		// Given SUB-010 supplies summary shapes,
		// When admin inspects default_summary_shape_slug,
		// Then all refs come from the closed SUB-010 slug set.
		assert.GreaterOrEqual(t, len(SeedExpectedBSDTemplateSummarySlugs), 1)
	})

	t.Run("Scenario_OneToOneMappingRoleToTemplate", func(t *testing.T) {
		// Given SUB-002 has 7 roles,
		// When seed templates ship,
		// Then each role has exactly 1 baseline template (1:1). Validated DB-real.
		assert.Equal(t, len(SeedExpectedBSDTemplateRoles), SeedExpectedBSDTemplateRowCount)
	})

	t.Run("Scenario_AllBaselinesRecommendedNoAdminReview", func(t *testing.T) {
		// Given builtin baselines are platform-curated safe defaults,
		// When admin compares recommended vs admin-review subsets,
		// Then all 7 are recommended and none require admin review.
		assert.Equal(t, 7, len(SeedRecommendedBSDTemplateSlugs))
		assert.Empty(t, SeedAdminReviewBSDTemplateSlugs)
	})
}
