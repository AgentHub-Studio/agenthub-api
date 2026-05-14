package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// KeyFileRegistry unit tests  (FEAT024)
// ---------------------------------------------------------------------------

func TestFEAT024_KeyFileRegistry_SeedCount(t *testing.T) {
	r := NewKeyFileRegistry()
	assert.Len(t, r.AllKeyFiles(), SeedKeyFileCount)
}

func TestFEAT024_KeyFileRegistry_SeedSlugsConst(t *testing.T) {
	assert.Len(t, SeedKeyFileSlugs, SeedKeyFileCount)
}

func TestFEAT024_KeyFileRegistry_FindBySlug_Hit(t *testing.T) {
	r := NewKeyFileRegistry()
	p, ok := r.FindKeyFileBySlug("main_tsx")
	require.True(t, ok)
	assert.Equal(t, "main.tsx", p.Label)
}

func TestFEAT024_KeyFileRegistry_FindBySlug_Miss(t *testing.T) {
	r := NewKeyFileRegistry()
	_, ok := r.FindKeyFileBySlug("nonexistent_file")
	assert.False(t, ok)
}

func TestFEAT024_KeyFileRegistry_AllSlugsPresent(t *testing.T) {
	r := NewKeyFileRegistry()
	for _, slug := range SeedKeyFileSlugs {
		_, ok := r.FindKeyFileBySlug(slug)
		assert.True(t, ok, "slug not found: %s", slug)
	}
}

func TestFEAT024_KeyFileRegistry_AllProfilesHaveNonEmptyLabel(t *testing.T) {
	r := NewKeyFileRegistry()
	for _, p := range r.AllKeyFiles() {
		assert.NotEmpty(t, p.Label, "empty Label for slug %s", p.Slug)
	}
}

func TestFEAT024_KeyFileRegistry_AllProfilesHaveNonEmptyResponsibility(t *testing.T) {
	r := NewKeyFileRegistry()
	for _, p := range r.AllKeyFiles() {
		assert.NotEmpty(t, p.Responsibility, "empty Responsibility for slug %s", p.Slug)
	}
}

func TestFEAT024_KeyFileRegistry_AllProfilesHaveNonEmptyPDFSection(t *testing.T) {
	r := NewKeyFileRegistry()
	for _, p := range r.AllKeyFiles() {
		assert.NotEmpty(t, p.PDFSection, "empty PDFSection for slug %s", p.Slug)
	}
}

func TestFEAT024_KeyFileRegistry_AllProfilesHaveNonEmptyLayer(t *testing.T) {
	r := NewKeyFileRegistry()
	for _, p := range r.AllKeyFiles() {
		assert.NotEmpty(t, string(p.Layer), "empty Layer for slug %s", p.Slug)
	}
}

func TestFEAT024_KeyFileRegistry_EntryPointsNonEmpty(t *testing.T) {
	r := NewKeyFileRegistry()
	eps := r.EntryPoints()
	assert.NotEmpty(t, eps)
}

func TestFEAT024_KeyFileRegistry_MainTsxIsEntryPoint(t *testing.T) {
	r := NewKeyFileRegistry()
	p, ok := r.FindKeyFileBySlug("main_tsx")
	require.True(t, ok)
	assert.True(t, p.IsEntryPoint)
}

func TestFEAT024_KeyFileRegistry_QueryTsNotEntryPoint(t *testing.T) {
	r := NewKeyFileRegistry()
	p, ok := r.FindKeyFileBySlug("query_ts")
	require.True(t, ok)
	assert.False(t, p.IsEntryPoint)
}

func TestFEAT024_KeyFileRegistry_ByLayer_CoreLoop(t *testing.T) {
	r := NewKeyFileRegistry()
	files := r.ByLayer(KeyFileLayerCoreLoop)
	assert.GreaterOrEqual(t, len(files), 2, "expected query.ts and QueryEngine.ts in core_loop layer")
}

func TestFEAT024_KeyFileRegistry_ByLayer_EntryAndStartup(t *testing.T) {
	r := NewKeyFileRegistry()
	files := r.ByLayer(KeyFileLayerEntryAndStartup)
	assert.GreaterOrEqual(t, len(files), 1)
}

func TestFEAT024_KeyFileRegistry_BySizeCategory_Large(t *testing.T) {
	r := NewKeyFileRegistry()
	large := r.BySizeCategory(KeyFileSizeLarge)
	// mcp/client.ts, compact.ts, AgentTool.tsx, runAgent.ts are Large
	assert.GreaterOrEqual(t, len(large), 4)
}

func TestFEAT024_KeyFileRegistry_BySizeCategory_Small(t *testing.T) {
	r := NewKeyFileRegistry()
	small := r.BySizeCategory(KeyFileSizeSmall)
	// Tool.ts (30 KB) and history.ts (14 KB) are Small
	assert.GreaterOrEqual(t, len(small), 2)
}

func TestFEAT024_KeyFileRegistry_MainTsxSize804KB(t *testing.T) {
	r := NewKeyFileRegistry()
	p, ok := r.FindKeyFileBySlug("main_tsx")
	require.True(t, ok)
	assert.Equal(t, 804, p.ApproxSizeKB)
}

func TestFEAT024_KeyFileRegistry_HistoryTsLayer(t *testing.T) {
	r := NewKeyFileRegistry()
	p, ok := r.FindKeyFileBySlug("history_ts")
	require.True(t, ok)
	assert.Equal(t, KeyFileLayerPersistence, p.Layer)
}

func TestFEAT024_KeyFileRegistry_CompactTsLayer(t *testing.T) {
	r := NewKeyFileRegistry()
	p, ok := r.FindKeyFileBySlug("compact_ts")
	require.True(t, ok)
	assert.Equal(t, KeyFileLayerContextAndMemory, p.Layer)
}

func TestFEAT024_KeyFileRegistry_MustContainAllTable7Files(t *testing.T) {
	r := NewKeyFileRegistry()
	table7Files := []string{
		"main_tsx",
		"query_ts",
		"QueryEngine_ts",
		"Tool_ts",
		"history_ts",
		"mcp_client_ts",
		"compact_ts",
		"AgentTool_tsx",
		"runAgent_ts",
	}
	for _, slug := range table7Files {
		_, ok := r.FindKeyFileBySlug(slug)
		assert.True(t, ok, "Table 7 file missing from registry: %s", slug)
	}
}

// ---------------------------------------------------------------------------
// ConditionalToolRegistry unit tests  (FEAT024)
// ---------------------------------------------------------------------------

func TestFEAT024_ConditionalToolRegistry_SeedCount(t *testing.T) {
	r := NewConditionalToolRegistry()
	assert.Len(t, r.AllConditionalTools(), SeedConditionalToolCount)
}

func TestFEAT024_ConditionalToolRegistry_SeedSlugsConst(t *testing.T) {
	assert.Len(t, SeedConditionalToolSlugs, SeedConditionalToolCount)
}

func TestFEAT024_ConditionalToolRegistry_FindBySlug_Hit(t *testing.T) {
	r := NewConditionalToolRegistry()
	p, ok := r.FindConditionalToolBySlug("bash-tool")
	require.True(t, ok)
	assert.Equal(t, ToolAvailAlwaysIncluded, p.Category)
}

func TestFEAT024_ConditionalToolRegistry_FindBySlug_Miss(t *testing.T) {
	r := NewConditionalToolRegistry()
	_, ok := r.FindConditionalToolBySlug("no-such-tool")
	assert.False(t, ok)
}

func TestFEAT024_ConditionalToolRegistry_AllSlugsPresent(t *testing.T) {
	r := NewConditionalToolRegistry()
	for _, slug := range SeedConditionalToolSlugs {
		_, ok := r.FindConditionalToolBySlug(slug)
		assert.True(t, ok, "slug not found: %s", slug)
	}
}

func TestFEAT024_ConditionalToolRegistry_AllProfilesHaveNonEmptyLabel(t *testing.T) {
	r := NewConditionalToolRegistry()
	for _, p := range r.AllConditionalTools() {
		assert.NotEmpty(t, p.Label, "empty Label for slug %s", p.Slug)
	}
}

func TestFEAT024_ConditionalToolRegistry_AllProfilesHaveNonEmptyCondition(t *testing.T) {
	r := NewConditionalToolRegistry()
	for _, p := range r.AllConditionalTools() {
		assert.NotEmpty(t, p.InclusionCondition, "empty InclusionCondition for slug %s", p.Slug)
	}
}

func TestFEAT024_ConditionalToolRegistry_AlwaysIncluded_HasMinimum8(t *testing.T) {
	r := NewConditionalToolRegistry()
	tools := r.AlwaysIncluded()
	// Table 8 lists 8 always-included tools
	assert.GreaterOrEqual(t, len(tools), 8)
}

func TestFEAT024_ConditionalToolRegistry_EnvironmentGated_Has3(t *testing.T) {
	r := NewConditionalToolRegistry()
	tools := r.EnvironmentGated()
	// GlobTool/GrepTool, ConfigTool, PowerShellTool
	assert.Equal(t, 3, len(tools))
}

func TestFEAT024_ConditionalToolRegistry_FeatureFlagGated_Has4(t *testing.T) {
	r := NewConditionalToolRegistry()
	tools := r.FeatureFlagGated()
	// todoV2, worktree, swarms, ToolSearchTool
	assert.Equal(t, 4, len(tools))
}

func TestFEAT024_ConditionalToolRegistry_NullChecked_Has5(t *testing.T) {
	r := NewConditionalToolRegistry()
	tools := r.NullChecked()
	assert.Equal(t, 5, len(tools))
}

func TestFEAT024_ConditionalToolRegistry_FourCategories(t *testing.T) {
	cats := []ToolAvailabilityCategory{
		ToolAvailAlwaysIncluded,
		ToolAvailEnvironment,
		ToolAvailFeatureFlag,
		ToolAvailNullChecked,
	}
	r := NewConditionalToolRegistry()
	for _, cat := range cats {
		tools := r.ByCategory(cat)
		assert.NotEmpty(t, tools, "no tools for category %s", cat)
	}
}

func TestFEAT024_ConditionalToolRegistry_EnterWorktreeIsFeatureFlag(t *testing.T) {
	r := NewConditionalToolRegistry()
	p, ok := r.FindConditionalToolBySlug("enter-worktree-tool")
	require.True(t, ok)
	assert.Equal(t, ToolAvailFeatureFlag, p.Category)
}

func TestFEAT024_ConditionalToolRegistry_PowerShellIsEnvironment(t *testing.T) {
	r := NewConditionalToolRegistry()
	p, ok := r.FindConditionalToolBySlug("powershell-tool")
	require.True(t, ok)
	assert.Equal(t, ToolAvailEnvironment, p.Category)
	assert.Contains(t, p.InclusionCondition, "Windows")
}

func TestFEAT024_ConditionalToolRegistry_MonitorToolIsNullChecked(t *testing.T) {
	r := NewConditionalToolRegistry()
	p, ok := r.FindConditionalToolBySlug("monitor-tool")
	require.True(t, ok)
	assert.Equal(t, ToolAvailNullChecked, p.Category)
}

func TestFEAT024_ConditionalToolRegistry_AgentToolAlwaysIncludedWithMinSet(t *testing.T) {
	r := NewConditionalToolRegistry()
	p, ok := r.FindConditionalToolBySlug("agent-tool")
	require.True(t, ok)
	assert.Equal(t, ToolAvailAlwaysIncluded, p.Category)
	// simple mode has 3 tools minimum (Bash, Read, Edit)
	assert.Equal(t, 3, p.MinToolSetSize)
}

func TestFEAT024_ConditionalToolRegistry_Table8_AllCategories_SumToTotal(t *testing.T) {
	r := NewConditionalToolRegistry()
	total := len(r.AlwaysIncluded()) +
		len(r.EnvironmentGated()) +
		len(r.FeatureFlagGated()) +
		len(r.NullChecked())
	assert.Equal(t, SeedConditionalToolCount, total)
}
