package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// conditional_tool_availability_registry_test.go — FEAT-046 unit tests
//
// Validates the ConditionalToolAvailabilityRegistry derived from
// arXiv:2604.14228v1 Appendix A §A.2 Table 8.

func TestConditionalToolAvailabilityRegistry_Singleton(t *testing.T) {
	r1 := ConditionalToolAvailabilityRegistryInstance()
	r2 := ConditionalToolAvailabilityRegistryInstance()
	assert.Same(t, r1, r2, "ConditionalToolAvailabilityRegistryInstance must return the same singleton")
}

func TestConditionalToolAvailabilityRegistry_Count(t *testing.T) {
	r := ConditionalToolAvailabilityRegistryInstance()
	count := r.Count()
	assert.GreaterOrEqual(t, count, 12, "registry must have at least 12 rules to cover all Table 8 entries")
}

func TestConditionalToolAvailabilityRegistry_AllRulesReturnsCopy(t *testing.T) {
	r := ConditionalToolAvailabilityRegistryInstance()
	rules1 := r.AllRules()
	rules2 := r.AllRules()
	assert.Equal(t, len(rules1), len(rules2))
	// Mutating the slice must not affect the registry.
	if len(rules1) > 0 {
		rules1[0].Label = "mutated"
		fresh := r.AllRules()
		assert.NotEqual(t, "mutated", fresh[0].Label, "AllRules must return a defensive copy")
	}
}

func TestConditionalToolAvailabilityRegistry_StructuralInvariants(t *testing.T) {
	r := ConditionalToolAvailabilityRegistryInstance()
	err := r.StructuralInvariants()
	require.NoError(t, err, "all structural invariants must hold for the production registry")
}

func TestConditionalToolAvailabilityRegistry_FourCategoriesRepresented(t *testing.T) {
	r := ConditionalToolAvailabilityRegistryInstance()
	cats := map[ConditionalToolCategory]int{}
	for _, rule := range r.AllRules() {
		cats[rule.Category]++
	}
	assert.Equal(t, 4, len(cats), "all four Table 8 categories must be represented")
	assert.Greater(t, cats[CategoryAlwaysIncluded], 0)
	assert.Greater(t, cats[CategoryEnvironment], 0)
	assert.Greater(t, cats[CategoryFeatureFlag], 0)
	assert.Greater(t, cats[CategoryNullChecked], 0)
}

func TestConditionalToolAvailabilityRegistry_FindByID_KnownRule(t *testing.T) {
	r := ConditionalToolAvailabilityRegistryInstance()
	rule, ok := r.FindByID("always-core-agent-tools")
	require.True(t, ok, "rule always-core-agent-tools must exist")
	assert.Equal(t, CategoryAlwaysIncluded, rule.Category)
	assert.True(t, rule.IsDefault)
	assert.False(t, rule.RequiresUserGrant)
}

func TestConditionalToolAvailabilityRegistry_FindByID_UnknownRule(t *testing.T) {
	r := ConditionalToolAvailabilityRegistryInstance()
	_, ok := r.FindByID("nonexistent-rule-xyz")
	assert.False(t, ok, "FindByID must return false for unknown rule IDs")
}

func TestConditionalToolAvailabilityRegistry_IsValidRuleID(t *testing.T) {
	r := ConditionalToolAvailabilityRegistryInstance()
	assert.True(t, r.IsValidRuleID("always-core-agent-tools"))
	assert.True(t, r.IsValidRuleID("flag-todo-v2"))
	assert.True(t, r.IsValidRuleID("null-sleep"))
	assert.False(t, r.IsValidRuleID(""))
	assert.False(t, r.IsValidRuleID("made-up-rule"))
}

func TestConditionalToolAvailabilityRegistry_RulesByCategory_AlwaysIncluded(t *testing.T) {
	r := ConditionalToolAvailabilityRegistryInstance()
	rules := r.RulesByCategory(CategoryAlwaysIncluded)
	require.NotEmpty(t, rules, "must have at least one AlwaysIncluded rule")
	for _, rule := range rules {
		assert.Equal(t, CategoryAlwaysIncluded, rule.Category)
		assert.False(t, rule.RequiresUserGrant, "AlwaysIncluded rules must not require user grant")
	}
}

func TestConditionalToolAvailabilityRegistry_RulesByCategory_Environment(t *testing.T) {
	r := ConditionalToolAvailabilityRegistryInstance()
	rules := r.RulesByCategory(CategoryEnvironment)
	require.NotEmpty(t, rules, "must have environment-gated rules")
	for _, rule := range rules {
		assert.Equal(t, CategoryEnvironment, rule.Category)
		assert.NotEmpty(t, rule.ActivationCondition)
	}
}

func TestConditionalToolAvailabilityRegistry_RulesByCategory_FeatureFlag(t *testing.T) {
	r := ConditionalToolAvailabilityRegistryInstance()
	rules := r.RulesByCategory(CategoryFeatureFlag)
	require.NotEmpty(t, rules, "must have feature-flag-gated rules")
	for _, rule := range rules {
		assert.Equal(t, CategoryFeatureFlag, rule.Category)
		assert.True(t, rule.RequiresUserGrant, "FeatureFlag rules must require user grant")
		assert.True(t, rule.OverridableByUser, "FeatureFlag rules must be overridable by user")
	}
}

func TestConditionalToolAvailabilityRegistry_RulesByCategory_NullChecked(t *testing.T) {
	r := ConditionalToolAvailabilityRegistryInstance()
	rules := r.RulesByCategory(CategoryNullChecked)
	require.NotEmpty(t, rules, "must have null-checked rules")
	for _, rule := range rules {
		assert.Equal(t, CategoryNullChecked, rule.Category)
		assert.False(t, rule.RequiresUserGrant, "NullChecked rules depend on capability, not user grant")
	}
}

func TestConditionalToolAvailabilityRegistry_DefaultRules(t *testing.T) {
	r := ConditionalToolAvailabilityRegistryInstance()
	defaults := r.DefaultRules()
	require.NotEmpty(t, defaults, "must have at least one default rule")
	for _, rule := range defaults {
		assert.True(t, rule.IsDefault)
	}
}

func TestConditionalToolAvailabilityRegistry_DefaultRulesAreNotUserGrantable(t *testing.T) {
	r := ConditionalToolAvailabilityRegistryInstance()
	// AlwaysIncluded rules are always default and never user-grantable.
	alwaysRules := r.RulesByCategory(CategoryAlwaysIncluded)
	for _, rule := range alwaysRules {
		assert.False(t, rule.RequiresUserGrant, "always-included (default) rules must not be user-grantable: %s", rule.RuleID)
	}
}

func TestConditionalToolAvailabilityRegistry_UserGrantableRules(t *testing.T) {
	r := ConditionalToolAvailabilityRegistryInstance()
	grantable := r.UserGrantableRules()
	require.NotEmpty(t, grantable, "must have user-grantable rules")
	for _, rule := range grantable {
		assert.True(t, rule.RequiresUserGrant)
		// User-grantable rules should not be AlwaysIncluded.
		assert.NotEqual(t, CategoryAlwaysIncluded, rule.Category, "always-included rules must never be user-grantable")
	}
}

func TestConditionalToolAvailabilityRegistry_OverridableRules(t *testing.T) {
	r := ConditionalToolAvailabilityRegistryInstance()
	overridable := r.OverridableRules()
	require.NotEmpty(t, overridable, "must have overridable rules")
	for _, rule := range overridable {
		assert.True(t, rule.OverridableByUser)
	}
}

func TestConditionalToolAvailabilityRegistry_RulesAffectingTool_BashTool(t *testing.T) {
	r := ConditionalToolAvailabilityRegistryInstance()
	rules := r.RulesAffectingTool("BashTool")
	require.NotEmpty(t, rules, "BashTool must appear in at least one rule")
	assert.Equal(t, CategoryAlwaysIncluded, rules[0].Category, "BashTool must be in the AlwaysIncluded category")
}

func TestConditionalToolAvailabilityRegistry_RulesAffectingTool_GlobTool(t *testing.T) {
	r := ConditionalToolAvailabilityRegistryInstance()
	rules := r.RulesAffectingTool("GlobTool")
	require.NotEmpty(t, rules)
	assert.Equal(t, CategoryEnvironment, rules[0].Category, "GlobTool must be in the Environment category")
}

func TestConditionalToolAvailabilityRegistry_RulesAffectingTool_PowerShellTool(t *testing.T) {
	r := ConditionalToolAvailabilityRegistryInstance()
	rules := r.RulesAffectingTool("PowerShellTool")
	require.NotEmpty(t, rules)
	assert.Equal(t, CategoryEnvironment, rules[0].Category)
}

func TestConditionalToolAvailabilityRegistry_RulesAffectingTool_EnterWorktreeTool(t *testing.T) {
	r := ConditionalToolAvailabilityRegistryInstance()
	rules := r.RulesAffectingTool("EnterWorktreeTool")
	require.NotEmpty(t, rules)
	assert.Equal(t, CategoryFeatureFlag, rules[0].Category)
	assert.True(t, rules[0].RequiresUserGrant)
}

func TestConditionalToolAvailabilityRegistry_RulesAffectingTool_SleepTool(t *testing.T) {
	r := ConditionalToolAvailabilityRegistryInstance()
	rules := r.RulesAffectingTool("SleepTool")
	require.NotEmpty(t, rules)
	assert.Equal(t, CategoryNullChecked, rules[0].Category)
}

func TestConditionalToolAvailabilityRegistry_RulesAffectingTool_Unknown(t *testing.T) {
	r := ConditionalToolAvailabilityRegistryInstance()
	rules := r.RulesAffectingTool("NonExistentTool")
	assert.Empty(t, rules, "unknown tool must not match any rules")
}

func TestConditionalToolAvailabilityRegistry_AllToolSlugs(t *testing.T) {
	r := ConditionalToolAvailabilityRegistryInstance()
	slugs := r.AllToolSlugs()
	assert.NotEmpty(t, slugs, "AllToolSlugs must return at least one tool")

	// Verify specific expected tools appear.
	slugSet := make(map[string]bool, len(slugs))
	for _, s := range slugs {
		slugSet[s] = true
	}
	expectedTools := []string{
		"AgentTool", "BashTool", "FileReadTool", "FileEditTool", "FileWriteTool",
		"WebFetchTool", "WebSearchTool", "GlobTool", "GrepTool",
		"PowerShellTool", "EnterWorktreeTool", "SleepTool",
	}
	for _, tool := range expectedTools {
		assert.True(t, slugSet[tool], "expected tool %q not found in AllToolSlugs", tool)
	}
}

func TestConditionalToolAvailabilityRegistry_AllToolSlugsAreSorted(t *testing.T) {
	r := ConditionalToolAvailabilityRegistryInstance()
	slugs := r.AllToolSlugs()
	for i := 1; i < len(slugs); i++ {
		assert.LessOrEqual(t, slugs[i-1], slugs[i], "AllToolSlugs must be in ascending order")
	}
}

func TestConditionalToolAvailabilityRegistry_CoreAgentToolsRule(t *testing.T) {
	r := ConditionalToolAvailabilityRegistryInstance()
	rule, ok := r.FindByID("always-core-agent-tools")
	require.True(t, ok)

	// Must include all eight tools documented in Table 8 AlwaysIncluded row.
	required := []string{
		"AgentTool", "BashTool", "FileReadTool", "FileEditTool",
		"FileWriteTool", "SkillTool", "WebFetchTool", "WebSearchTool",
	}
	toolSet := make(map[string]bool)
	for _, t := range rule.AffectedTools {
		toolSet[t] = true
	}
	for _, req := range required {
		assert.True(t, toolSet[req], "core agent tools rule must include %q", req)
	}
}

func TestConditionalToolAvailabilityRegistry_TodoV2FlagRule(t *testing.T) {
	r := ConditionalToolAvailabilityRegistryInstance()
	rule, ok := r.FindByID("flag-todo-v2")
	require.True(t, ok)

	assert.Equal(t, CategoryFeatureFlag, rule.Category)
	assert.True(t, rule.RequiresUserGrant)
	assert.True(t, rule.OverridableByUser)
	assert.False(t, rule.IsDefault)

	toolSet := make(map[string]bool)
	for _, t := range rule.AffectedTools {
		toolSet[t] = true
	}
	for _, expected := range []string{"TaskCreateTool", "TaskGetTool", "TaskUpdateTool", "TaskListTool"} {
		assert.True(t, toolSet[expected], "todoV2 rule must list %q", expected)
	}
}

func TestConditionalToolAvailabilityRegistry_NullCheckedRulesHaveDeactivationCondition(t *testing.T) {
	r := ConditionalToolAvailabilityRegistryInstance()
	nullRules := r.RulesByCategory(CategoryNullChecked)
	for _, rule := range nullRules {
		assert.NotEmpty(t, rule.DeactivationCondition,
			"NullChecked rule %q must document its deactivation condition", rule.RuleID)
	}
}

func TestConditionalToolAvailabilityRegistry_AllRulesHavePDFSection(t *testing.T) {
	r := ConditionalToolAvailabilityRegistryInstance()
	for _, rule := range r.AllRules() {
		assert.NotEmpty(t, rule.PDFSection, "rule %q must reference a PDF section", rule.RuleID)
	}
}

func TestConditionalToolAvailabilityRegistry_AllRulesHaveLabel(t *testing.T) {
	r := ConditionalToolAvailabilityRegistryInstance()
	for _, rule := range r.AllRules() {
		assert.NotEmpty(t, rule.Label, "rule %q must have a non-empty Label", rule.RuleID)
	}
}

func TestConditionalToolAvailabilityRegistry_AllRulesHaveDescription(t *testing.T) {
	r := ConditionalToolAvailabilityRegistryInstance()
	for _, rule := range r.AllRules() {
		assert.NotEmpty(t, rule.Description, "rule %q must have a non-empty Description", rule.RuleID)
	}
}

func TestConditionalToolCategory_Constants(t *testing.T) {
	// Verify the four category constant values match the Table 8 names.
	assert.Equal(t, ConditionalToolCategory("always_included"), CategoryAlwaysIncluded)
	assert.Equal(t, ConditionalToolCategory("environment"), CategoryEnvironment)
	assert.Equal(t, ConditionalToolCategory("feature_flag"), CategoryFeatureFlag)
	assert.Equal(t, ConditionalToolCategory("null_checked"), CategoryNullChecked)
}

func TestConditionalToolAvailabilityRegistry_SwarmsFlagRule(t *testing.T) {
	r := ConditionalToolAvailabilityRegistryInstance()
	rule, ok := r.FindByID("flag-swarms")
	require.True(t, ok, "swarms feature flag rule must exist")
	assert.Equal(t, CategoryFeatureFlag, rule.Category)
	assert.Contains(t, rule.AffectedTools, "TeamTools")
	assert.True(t, rule.RequiresUserGrant)
}
