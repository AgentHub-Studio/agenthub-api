package agentic

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// conditional_tool_availability_registry_bdd_test.go — FEAT-046 BDD tests
//
// Behaviour-Driven scenarios for the ConditionalToolAvailabilityRegistry,
// modelled on Appendix A §A.2 Table 8 of arXiv:2604.14228v1.

// BDD scenario 1 ──────────────────────────────────────────────────────────────
// GIVEN the registry is initialised
// WHEN the structural invariants are evaluated
// THEN all invariants must hold without error
func TestConditionalToolBDD_StructuralInvariantsHold(t *testing.T) {
	// GIVEN
	r := ConditionalToolAvailabilityRegistryInstance()

	// WHEN
	err := r.StructuralInvariants()

	// THEN
	require.NoError(t, err, "all structural invariants must hold")
}

// BDD scenario 2 ──────────────────────────────────────────────────────────────
// GIVEN the Table 8 taxonomy with four categories
// WHEN rules are queried by category
// THEN each of the four categories has at least one rule
//   AND the AlwaysIncluded category contains the eight core agent tools
func TestConditionalToolBDD_FourCategoriesEachHaveRules(t *testing.T) {
	// GIVEN
	r := ConditionalToolAvailabilityRegistryInstance()
	categories := []ConditionalToolCategory{
		CategoryAlwaysIncluded,
		CategoryEnvironment,
		CategoryFeatureFlag,
		CategoryNullChecked,
	}

	for _, cat := range categories {
		t.Run(fmt.Sprintf("category_%s", cat), func(t *testing.T) {
			// WHEN
			rules := r.RulesByCategory(cat)

			// THEN
			assert.NotEmpty(t, rules, "category %q must have at least one rule", cat)
		})
	}

	// AND — AlwaysIncluded has the eight core tools
	coreRule, ok := r.FindByID("always-core-agent-tools")
	require.True(t, ok)
	coreTools := map[string]bool{}
	for _, tool := range coreRule.AffectedTools {
		coreTools[tool] = true
	}
	for _, expected := range []string{
		"AgentTool", "BashTool", "FileReadTool", "FileEditTool",
		"FileWriteTool", "SkillTool", "WebFetchTool", "WebSearchTool",
	} {
		assert.True(t, coreTools[expected], "core rule must include %q", expected)
	}
}

// BDD scenario 3 ──────────────────────────────────────────────────────────────
// GIVEN a tool in the AlwaysIncluded category
// WHEN the user's default session is assembled
// THEN the tool is present without any user action required
//   AND the rule is marked IsDefault=true and RequiresUserGrant=false
func TestConditionalToolBDD_AlwaysIncludedToolsRequireNoUserAction(t *testing.T) {
	// GIVEN
	r := ConditionalToolAvailabilityRegistryInstance()
	alwaysRules := r.RulesByCategory(CategoryAlwaysIncluded)
	require.NotEmpty(t, alwaysRules)

	for _, rule := range alwaysRules {
		t.Run(rule.RuleID, func(t *testing.T) {
			// WHEN / THEN
			assert.True(t, rule.IsDefault,
				"always-included rule %q must be IsDefault=true", rule.RuleID)
			assert.False(t, rule.RequiresUserGrant,
				"always-included rule %q must not require user grant", rule.RuleID)
			assert.False(t, rule.OverridableByUser,
				"always-included rule %q must not be user-overridable", rule.RuleID)
		})
	}
}

// BDD scenario 4 ──────────────────────────────────────────────────────────────
// GIVEN a tool gated by a feature flag (todoV2, worktree, swarms, ToolSearch)
// WHEN the corresponding flag is not enabled
// THEN the tool must NOT appear in the default tool set
//   AND the rule's RequiresUserGrant must be true
func TestConditionalToolBDD_FeatureFlagToolsAreOptIn(t *testing.T) {
	// GIVEN
	r := ConditionalToolAvailabilityRegistryInstance()
	flagRules := r.RulesByCategory(CategoryFeatureFlag)
	require.NotEmpty(t, flagRules)

	for _, rule := range flagRules {
		t.Run(rule.RuleID, func(t *testing.T) {
			// WHEN: no flag is enabled (default session)
			// THEN:
			assert.False(t, rule.IsDefault,
				"feature-flag rule %q must NOT be in the default tool set", rule.RuleID)
			assert.True(t, rule.RequiresUserGrant,
				"feature-flag rule %q must require a user grant", rule.RuleID)
			assert.True(t, rule.OverridableByUser,
				"feature-flag rule %q must be overridable by the user", rule.RuleID)
		})
	}
}

// BDD scenario 5 ──────────────────────────────────────────────────────────────
// GIVEN the null-checked tool category (SuggestBackgroundPR, WebBrowser,
//
//	RemoteTrigger, Monitor, Sleep)
//
// WHEN the underlying capability is absent (null slot at assembly time)
// THEN the tool is excluded without any error
//   AND the rule's RequiresUserGrant is false (it is a capability gate, not a user gate)
func TestConditionalToolBDD_NullCheckedToolsExcludedWhenCapabilityAbsent(t *testing.T) {
	// GIVEN
	r := ConditionalToolAvailabilityRegistryInstance()
	nullRules := r.RulesByCategory(CategoryNullChecked)
	require.NotEmpty(t, nullRules)

	expectedTools := map[string]bool{
		"SuggestBackgroundPRTool": false,
		"WebBrowserTool":          false,
		"RemoteTriggerTool":       false,
		"MonitorTool":             false,
		"SleepTool":               false,
	}

	for _, rule := range nullRules {
		t.Run(rule.RuleID, func(t *testing.T) {
			// WHEN capability is null (THEN checks)
			assert.False(t, rule.IsDefault, "null-checked rule must not be in the default set")
			assert.False(t, rule.RequiresUserGrant, "null-checked rule must not require user grant — it's a capability gate")
			assert.NotEmpty(t, rule.DeactivationCondition, "must document what 'null' means for this tool")
		})

		for _, tool := range rule.AffectedTools {
			if _, known := expectedTools[tool]; known {
				expectedTools[tool] = true
			}
		}
	}

	// All five expected null-checked tools must appear.
	for tool, found := range expectedTools {
		assert.True(t, found, "expected null-checked tool %q not found in any rule", tool)
	}
}

// BDD scenario 6 ──────────────────────────────────────────────────────────────
// GIVEN a tool name
// WHEN RulesAffectingTool is called
// THEN the category of the returned rules matches Table 8's assignment for that tool
func TestConditionalToolBDD_ToolCategoryLookupMatchesTable8(t *testing.T) {
	r := ConditionalToolAvailabilityRegistryInstance()

	cases := []struct {
		tool             string
		expectedCategory ConditionalToolCategory
	}{
		{"BashTool", CategoryAlwaysIncluded},
		{"AgentTool", CategoryAlwaysIncluded},
		{"FileReadTool", CategoryAlwaysIncluded},
		{"WebFetchTool", CategoryAlwaysIncluded},
		{"GlobTool", CategoryEnvironment},
		{"GrepTool", CategoryEnvironment},
		{"PowerShellTool", CategoryEnvironment},
		{"ConfigTool", CategoryEnvironment},
		{"TaskCreateTool", CategoryFeatureFlag},
		{"EnterWorktreeTool", CategoryFeatureFlag},
		{"TeamTools", CategoryFeatureFlag},
		{"ToolSearchTool", CategoryFeatureFlag},
		{"SuggestBackgroundPRTool", CategoryNullChecked},
		{"WebBrowserTool", CategoryNullChecked},
		{"RemoteTriggerTool", CategoryNullChecked},
		{"MonitorTool", CategoryNullChecked},
		{"SleepTool", CategoryNullChecked},
	}

	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			// GIVEN / WHEN
			rules := r.RulesAffectingTool(tc.tool)

			// THEN
			require.NotEmpty(t, rules, "tool %q must appear in at least one rule", tc.tool)
			assert.Equal(t, tc.expectedCategory, rules[0].Category,
				"tool %q must be in category %q per Table 8", tc.tool, tc.expectedCategory)
		})
	}
}
