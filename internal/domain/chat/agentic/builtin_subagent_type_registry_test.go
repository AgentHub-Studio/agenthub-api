package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Unit tests for BuiltinSubagentTypeRegistry (§8.1 built-in subagent types).
// All test functions are prefixed FEAT035 per the architectural loop convention.

// FEAT035_01 — registry seeds exactly six types
func TestFEAT035_01_SeedCountIsSix(t *testing.T) {
	assert.Equal(t, 6, SeedBuiltinSubagentTypeCount,
		"§8.1 enumerates exactly 6 built-in subagent types")
}

// FEAT035_02 — SeedBuiltinSubagentTypeSlugs has the right length
func TestFEAT035_02_SeedSlugSliceLength(t *testing.T) {
	assert.Len(t, SeedBuiltinSubagentTypeSlugs, SeedBuiltinSubagentTypeCount,
		"slug slice must match SeedBuiltinSubagentTypeCount")
}

// FEAT035_03 — NewBuiltinSubagentTypeRegistry returns non-nil
func TestFEAT035_03_NewRegistryNonNil(t *testing.T) {
	r := NewBuiltinSubagentTypeRegistry()
	require.NotNil(t, r, "NewBuiltinSubagentTypeRegistry must return non-nil")
}

// FEAT035_04 — AllBuiltinSubagentTypes returns six entries
func TestFEAT035_04_AllTypesReturnsSix(t *testing.T) {
	r := NewBuiltinSubagentTypeRegistry()
	all := r.AllBuiltinSubagentTypes()
	assert.Len(t, all, 6, "AllBuiltinSubagentTypes must return exactly 6 profiles")
}

// FEAT035_05 — AllBuiltinSubagentTypes returns a defensive copy
func TestFEAT035_05_AllTypesReturnsDefensiveCopy(t *testing.T) {
	r := NewBuiltinSubagentTypeRegistry()
	a := r.AllBuiltinSubagentTypes()
	b := r.AllBuiltinSubagentTypes()
	assert.Equal(t, a, b, "two calls must return equal slices")
	// Mutating one must not affect the other
	if len(a) > 0 {
		a[0].Label = "mutated"
		assert.NotEqual(t, a[0].Label, b[0].Label,
			"mutating returned slice must not affect subsequent calls")
	}
}

// FEAT035_06 — FindBuiltinSubagentTypeBySlug returns Explore profile correctly
func TestFEAT035_06_FindExplore(t *testing.T) {
	r := NewBuiltinSubagentTypeRegistry()
	p, ok := r.FindBuiltinSubagentTypeBySlug(BuiltinTypeExplore)
	require.True(t, ok, "Explore slug must exist")
	assert.Equal(t, BuiltinTypeExplore, p.Slug)
	assert.Equal(t, "§8.1", p.PDFSection)
	assert.Equal(t, "Explore", p.Label)
	assert.True(t, p.HasWriteToolsDenied,
		"§8.1: Explore has write and edit tools in its deny-list")
	assert.Equal(t, "read_only", p.ToolsetCategory)
	assert.Equal(t, "investigation", p.UseCaseCategory)
}

// FEAT035_07 — FindBuiltinSubagentTypeBySlug returns Plan profile correctly
func TestFEAT035_07_FindPlan(t *testing.T) {
	r := NewBuiltinSubagentTypeRegistry()
	p, ok := r.FindBuiltinSubagentTypeBySlug(BuiltinTypePlan)
	require.True(t, ok, "Plan slug must exist")
	assert.Equal(t, "Plan", p.Label)
	assert.False(t, p.HasWriteToolsDenied)
	assert.Equal(t, "plan_only", p.ToolsetCategory)
	assert.Equal(t, "planning", p.UseCaseCategory)
	assert.Empty(t, p.PermissionModelOverride,
		"Plan uses the standard permission model — no override")
}

// FEAT035_08 — FindBuiltinSubagentTypeBySlug returns GeneralPurpose profile
func TestFEAT035_08_FindGeneralPurpose(t *testing.T) {
	r := NewBuiltinSubagentTypeRegistry()
	p, ok := r.FindBuiltinSubagentTypeBySlug(BuiltinTypeGeneralPurpose)
	require.True(t, ok)
	assert.Equal(t, "General-purpose", p.Label)
	assert.True(t, p.MayRouteThroughForkSubagent,
		"§8.1: omitting the type may route to the fork-subagent path")
	assert.Equal(t, "full", p.ToolsetCategory)
}

// FEAT035_09 — FindBuiltinSubagentTypeBySlug returns ClaudeCodeGuide profile
func TestFEAT035_09_FindClaudeCodeGuide(t *testing.T) {
	r := NewBuiltinSubagentTypeRegistry()
	p, ok := r.FindBuiltinSubagentTypeBySlug(BuiltinTypeClaudeCodeGuide)
	require.True(t, ok)
	assert.Equal(t, "Claude Code Guide", p.Label)
	assert.NotEmpty(t, p.PermissionModelOverride,
		"§8.1: Claude Code Guide has its own permissionMode override")
	assert.True(t, p.IsFeatureGated,
		"Claude Code Guide availability depends on entrypoint")
	assert.Equal(t, "documentation", p.UseCaseCategory)
}

// FEAT035_10 — FindBuiltinSubagentTypeBySlug returns Verification profile
func TestFEAT035_10_FindVerification(t *testing.T) {
	r := NewBuiltinSubagentTypeRegistry()
	p, ok := r.FindBuiltinSubagentTypeBySlug(BuiltinTypeVerification)
	require.True(t, ok)
	assert.Equal(t, "Verification", p.Label)
	assert.Equal(t, "validation", p.ToolsetCategory)
	assert.Equal(t, "validation", p.UseCaseCategory)
	assert.False(t, p.HasWriteToolsDenied)
}

// FEAT035_11 — FindBuiltinSubagentTypeBySlug returns StatuslineSetup profile
func TestFEAT035_11_FindStatuslineSetup(t *testing.T) {
	r := NewBuiltinSubagentTypeRegistry()
	p, ok := r.FindBuiltinSubagentTypeBySlug(BuiltinTypeStatuslineSetup)
	require.True(t, ok)
	assert.Equal(t, "Statusline-setup", p.Label)
	assert.Equal(t, "terminal", p.ToolsetCategory)
	assert.Equal(t, "configuration", p.UseCaseCategory)
	assert.True(t, p.IsFeatureGated,
		"Statusline-setup availability depends on entrypoint")
}

// FEAT035_12 — FindBuiltinSubagentTypeBySlug returns false for unknown slug
func TestFEAT035_12_FindUnknownSlugReturnsFalse(t *testing.T) {
	r := NewBuiltinSubagentTypeRegistry()
	_, ok := r.FindBuiltinSubagentTypeBySlug("nonexistent_type")
	assert.False(t, ok, "unknown slug must return (zero, false)")
}

// FEAT035_13 — TypesWithWriteToolsDenied returns exactly Explore
func TestFEAT035_13_WriteToolsDeniedIsOnlyExplore(t *testing.T) {
	r := NewBuiltinSubagentTypeRegistry()
	denied := r.TypesWithWriteToolsDenied()
	require.Len(t, denied, 1,
		"§8.1 names only Explore as having write+edit tools in the deny-list")
	assert.Equal(t, BuiltinTypeExplore, denied[0].Slug)
}

// FEAT035_14 — TypesWithPermissionModeOverride returns exactly ClaudeCodeGuide
func TestFEAT035_14_PermissionOverrideIsOnlyClaudeCodeGuide(t *testing.T) {
	r := NewBuiltinSubagentTypeRegistry()
	overridden := r.TypesWithPermissionModeOverride()
	require.Len(t, overridden, 1,
		"§8.1 names only Claude Code Guide as having a permissionMode override")
	assert.Equal(t, BuiltinTypeClaudeCodeGuide, overridden[0].Slug)
}

// FEAT035_15 — TypesThatMayFork returns exactly GeneralPurpose
func TestFEAT035_15_ForkRoutingIsOnlyGeneralPurpose(t *testing.T) {
	r := NewBuiltinSubagentTypeRegistry()
	forkable := r.TypesThatMayFork()
	require.Len(t, forkable, 1,
		"§8.1 notes only General-purpose may route to fork-subagent path")
	assert.Equal(t, BuiltinTypeGeneralPurpose, forkable[0].Slug)
}

// FEAT035_16 — FeatureGatedTypes returns exactly ClaudeCodeGuide and StatuslineSetup
func TestFEAT035_16_FeatureGatedTypesCount(t *testing.T) {
	r := NewBuiltinSubagentTypeRegistry()
	gated := r.FeatureGatedTypes()
	assert.Len(t, gated, 2,
		"§8.1: two types depend on feature flags/entrypoint")
	slugs := make(map[BuiltinSubagentTypeSlug]bool, len(gated))
	for _, g := range gated {
		slugs[g.Slug] = true
	}
	assert.True(t, slugs[BuiltinTypeClaudeCodeGuide],
		"ClaudeCodeGuide must be feature-gated")
	assert.True(t, slugs[BuiltinTypeStatuslineSetup],
		"StatuslineSetup must be feature-gated")
}

// FEAT035_17 — TypesByUseCaseCategory finds investigation type correctly
func TestFEAT035_17_TypesByUseCaseCategoryInvestigation(t *testing.T) {
	r := NewBuiltinSubagentTypeRegistry()
	types := r.TypesByUseCaseCategory("investigation")
	require.Len(t, types, 1)
	assert.Equal(t, BuiltinTypeExplore, types[0].Slug)
}

// FEAT035_18 — TypesByUseCaseCategory returns empty for unknown category
func TestFEAT035_18_TypesByUseCaseCategoryUnknown(t *testing.T) {
	r := NewBuiltinSubagentTypeRegistry()
	types := r.TypesByUseCaseCategory("nonexistent")
	assert.Empty(t, types)
}

// FEAT035_19 — TypesByToolsetCategory finds read_only type correctly
func TestFEAT035_19_TypesByToolsetCategoryReadOnly(t *testing.T) {
	r := NewBuiltinSubagentTypeRegistry()
	types := r.TypesByToolsetCategory("read_only")
	require.Len(t, types, 1)
	assert.Equal(t, BuiltinTypeExplore, types[0].Slug)
}

// FEAT035_20 — TypesByToolsetCategory finds validation type correctly
func TestFEAT035_20_TypesByToolsetCategoryValidation(t *testing.T) {
	r := NewBuiltinSubagentTypeRegistry()
	types := r.TypesByToolsetCategory("validation")
	require.Len(t, types, 1)
	assert.Equal(t, BuiltinTypeVerification, types[0].Slug)
}

// FEAT035_21 — ReadOnlyType returns Explore and true
func TestFEAT035_21_ReadOnlyTypeReturnsExplore(t *testing.T) {
	r := NewBuiltinSubagentTypeRegistry()
	p, ok := r.ReadOnlyType()
	require.True(t, ok, "ReadOnlyType must find exactly one type")
	assert.Equal(t, BuiltinTypeExplore, p.Slug,
		"§8.1: Explore is the read-only type")
}

// FEAT035_22 — IsValidBuiltinSubagentTypeSlug returns true for all known slugs
func TestFEAT035_22_IsValidSlugForAllSeeds(t *testing.T) {
	for _, slug := range SeedBuiltinSubagentTypeSlugs {
		assert.True(t, IsValidBuiltinSubagentTypeSlug(slug),
			"IsValidBuiltinSubagentTypeSlug must return true for seed slug %q", slug)
	}
}

// FEAT035_23 — IsValidBuiltinSubagentTypeSlug returns false for unknown slug
func TestFEAT035_23_IsValidSlugForUnknown(t *testing.T) {
	assert.False(t, IsValidBuiltinSubagentTypeSlug("unknown_type"),
		"IsValidBuiltinSubagentTypeSlug must return false for unknown slug")
}

// FEAT035_24 — every profile has a non-empty PDFSection, Label, Description
func TestFEAT035_24_AllProfilesHaveRequiredFields(t *testing.T) {
	r := NewBuiltinSubagentTypeRegistry()
	for _, p := range r.AllBuiltinSubagentTypes() {
		assert.NotEmpty(t, p.PDFSection,
			"profile %q must have PDFSection", p.Slug)
		assert.NotEmpty(t, p.Label,
			"profile %q must have Label", p.Slug)
		assert.NotEmpty(t, p.Description,
			"profile %q must have Description", p.Slug)
		assert.NotEmpty(t, p.UseCaseCategory,
			"profile %q must have UseCaseCategory", p.Slug)
		assert.NotEmpty(t, p.ToolsetCategory,
			"profile %q must have ToolsetCategory", p.Slug)
		assert.NotEmpty(t, p.AgenthubMapping,
			"profile %q must have AgenthubMapping", p.Slug)
	}
}

// FEAT035_25 — all six use-case categories are distinct
func TestFEAT035_25_UseCaseCategoriesAreDistinct(t *testing.T) {
	r := NewBuiltinSubagentTypeRegistry()
	seen := make(map[string]bool)
	for _, p := range r.AllBuiltinSubagentTypes() {
		assert.False(t, seen[p.UseCaseCategory],
			"use-case category %q appears more than once — each §8.1 type must have a unique category",
			p.UseCaseCategory)
		seen[p.UseCaseCategory] = true
	}
	assert.Len(t, seen, 6, "six types must produce six distinct use-case categories")
}
