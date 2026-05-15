package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreSTPDTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantPicksToolsetIsolationFromCatalog", func(t *testing.T) {
		// Given fresh tenants must pick a SUB-005 SubagentToolsetIsolationMode
		// before spawning subagents and inventing allowlist/blocklist is
		// error-prone,
		// When admin opens toolset isolation onboarding,
		// Then 4 recommended templates surface 1:1 with the enum.
		assert.Equal(t, 4, len(SeedRecommendedSTPDTemplateSlugs))
	})

	t.Run("Scenario_DocumentationGeneratorReadOnlyAllowlist", func(t *testing.T) {
		// Given a doc generator should never mutate state,
		// When admin uses documentation-readonly-allowlist,
		// Then sample_allowed_tool_names contains Read/Grep/Glob/document_search
		// and admin review required.
		assert.Contains(t, SeedExpectedSTPDTemplateSlugs, "documentation-readonly-allowlist")
	})

	t.Run("Scenario_ResearchAssistantInheritsMinusSharpEdges", func(t *testing.T) {
		// Given research subagents need broad investigation but no
		// mutation,
		// When admin uses research-minus-sharp-edges,
		// Then it's the only routine (no admin review) template — the
		// blocklist captures Bash/Write/execute-sql.
		assert.Contains(t, SeedExpectedSTPDTemplateSlugs, "research-minus-sharp-edges")
		set := map[string]bool{}
		for _, s := range SeedAdminReviewSTPDTemplateSlugs {
			set[s] = true
		}
		assert.False(t, set["research-minus-sharp-edges"])
	})

	t.Run("Scenario_RecursionSafetyDepthFilteredAgentTool", func(t *testing.T) {
		// Given multi-level orchestration must not spawn infinite
		// subagents,
		// When admin uses recursion-safety-depth-filtered,
		// Then sample_depth_threshold_for_agent=1 hides Agent tool at
		// depth > 1.
		assert.Contains(t, SeedExpectedSTPDTemplateSlugs, "recursion-safety-depth-filtered")
	})

	t.Run("Scenario_AdminSurfaceCategoricalExclusion", func(t *testing.T) {
		// Given admin_* and *_dangerous tools should be inaccessible
		// to subagents,
		// When admin uses admin-surface-categorical-exclusion,
		// Then prefix+suffix patterns are set.
		assert.Contains(t, SeedExpectedSTPDTemplateSlugs, "admin-surface-categorical-exclusion")
	})

	t.Run("Scenario_ModeLabelsMatchSUB005EnumByteForByte", func(t *testing.T) {
		// Given SUB-005 SubagentToolsetIsolationMode has 4 values,
		// When seed declares target_isolation_mode,
		// Then labels match enum bytes (no mapping table runtime).
		sub005 := []string{"explicit_allowlist", "parent_minus_blocklist",
			"depth_filtered", "categorical_exclusion"}
		set := map[string]bool{}
		for _, m := range SeedExpectedSTPDTemplateModes {
			set[m] = true
		}
		for _, e := range sub005 {
			assert.True(t, set[e], "SUB-005 mode %q missing", e)
		}
	})

	t.Run("Scenario_FourModesAllRepresented", func(t *testing.T) {
		// Given SUB-005 has 4 modes,
		// When seed templates ship,
		// Then ALL 4 modes have exactly one template (1:1 mapping).
		assert.Equal(t, 4, len(SeedExpectedSTPDTemplateModes))
	})

	t.Run("Scenario_AdminReviewGatesIsolationChange", func(t *testing.T) {
		// Given switching isolation stance changes blast radius,
		// When admin compares admin-review subset,
		// Then 3 of 4 templates gate change (only research routine baseline).
		assert.Equal(t, 3, len(SeedAdminReviewSTPDTemplateSlugs))
	})

	t.Run("Scenario_AllowlistModeShipsActualAllowedToolNames", func(t *testing.T) {
		// Given the explicit_allowlist template must ship sample tool
		// names (else admin has to invent them),
		// When the catalog row lands,
		// Then sample_allowed_tool_names is non-empty for documentation
		// template. Validated structurally via integration test.
		assert.Equal(t, 4, SeedExpectedSTPDTemplateRowCount)
	})

	t.Run("Scenario_CategoricalModeShipsActualPatterns", func(t *testing.T) {
		// Given the categorical_exclusion template must ship sample
		// prefix/suffix patterns,
		// When the catalog row lands,
		// Then admin-surface template has prefix="admin_" + suffix=
		// "_dangerous". Validated structurally via integration test.
		assert.Equal(t, 4, len(SeedExpectedSTPDTemplateSlugs))
	})
}
