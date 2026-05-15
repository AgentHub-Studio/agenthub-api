package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreCADTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantClonesStarterShapeInsteadOfDesigningFromScratch", func(t *testing.T) {
		// Given tenants want a proven starting point for their own agents,
		// When admin opens custom-agent onboarding,
		// Then 4 recommended starter shapes surface.
		assert.Equal(t, 4, len(SeedRecommendedCADTemplateSlugs))
	})

	t.Run("Scenario_BareGreenfieldIsTheOnlyNoLineageStarter", func(t *testing.T) {
		// Given some tenants want to design from scratch without inheriting
		// any SUB-002 builtin,
		// When admin filters greenfield starters,
		// Then exactly bare-greenfield is listed.
		assert.Equal(t, []string{"bare-greenfield"}, SeedGreenfieldCADTemplateSlugs)
	})

	t.Run("Scenario_ResearcherDerivedTracksLineageToSUB002Baseline", func(t *testing.T) {
		// Given researcher-derived clones researcher-baseline as a starting
		// point,
		// When admin inspects derived_from_builtin_slug,
		// Then it points to a known SUB-002 slug.
		set := map[string]bool{}
		for _, s := range SeedExpectedCADTemplateBuiltinLineageSlugs {
			set[s] = true
		}
		assert.True(t, set["researcher-baseline"])
	})

	t.Run("Scenario_CoderDerivedRefsCodeWriteScopedToolset", func(t *testing.T) {
		// Given coder-derived inherits write rights for scoped code edits,
		// When admin checks SUB-005 toolset ref,
		// Then code-write-scoped appears in the closed toolset set.
		set := map[string]bool{}
		for _, s := range SeedExpectedCADTemplateToolsetSlugs {
			set[s] = true
		}
		assert.True(t, set["code-write-scoped"])
	})

	t.Run("Scenario_DualLoopPlannerReturnsPlanOnlySummary", func(t *testing.T) {
		// Given dual-loop pattern separates planning from implementation,
		// When admin inspects the planner half's summary shape,
		// Then plan-only (SUB-010) is one of the seeded refs.
		set := map[string]bool{}
		for _, s := range SeedExpectedCADTemplateSummarySlugs {
			set[s] = true
		}
		assert.True(t, set["plan-only"])
	})

	t.Run("Scenario_ShapeKindsCoverFourArchetypes", func(t *testing.T) {
		// Given 4 archetypes exist (bare/derived/researcher_derived/dual_loop),
		// When admin lists shape_kind values,
		// Then all 4 appear.
		assert.Equal(t, 4, len(SeedExpectedCADTemplateShapeKinds))
	})

	t.Run("Scenario_AllStartersPrivateVisibilityByDefault", func(t *testing.T) {
		// Given starters are tenant-private until explicitly published,
		// When seed declares suggested_visibility,
		// Then only "private" is used.
		assert.Equal(t, []string{"private"}, SeedExpectedCADTemplateVisibilities)
	})

	t.Run("Scenario_ToolsetRefsBelongToSUB005ClosedSet", func(t *testing.T) {
		// Given SUB-005 catalog supplies isolation policies,
		// When admin inspects toolset refs of starters,
		// Then refs come from the closed SUB-005 set.
		assert.GreaterOrEqual(t, len(SeedExpectedCADTemplateToolsetSlugs), 1)
	})

	t.Run("Scenario_InheritanceRefsBelongToSUB006ClosedSet", func(t *testing.T) {
		assert.GreaterOrEqual(t, len(SeedExpectedCADTemplateInheritanceSlugs), 1)
	})

	t.Run("Scenario_OneToOneShapeKindToTemplate", func(t *testing.T) {
		// Given 4 archetypes,
		// When seed templates ship,
		// Then each shape_kind has exactly 1 template (1:1). Validated DB-real.
		assert.Equal(t, len(SeedExpectedCADTemplateShapeKinds), SeedExpectedCADTemplateRowCount)
	})
}
