package agentic

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubagentToolset_IsValidMode(t *testing.T) {
	for _, m := range allSubagentToolsetIsolationModes {
		assert.True(t, IsValidSubagentToolsetIsolationMode(m))
	}
	assert.False(t, IsValidSubagentToolsetIsolationMode(SubagentToolsetIsolationMode("nope")))
}

func TestSubagentToolset_ValidateBadMode(t *testing.T) {
	p := SubagentToolsetPolicy{Mode: SubagentToolsetIsolationMode("nope")}
	assert.ErrorIs(t, p.Validate(), ErrSubagentToolsetBadMode)
}

func TestSubagentToolset_ValidateAllowlistRequiresNames(t *testing.T) {
	p := SubagentToolsetPolicy{Mode: SubagentToolsetExplicitAllowlist}
	assert.ErrorIs(t, p.Validate(), ErrSubagentToolsetAllowlistEmpty)
}

func TestSubagentToolset_ValidateCategoryRequiresPatterns(t *testing.T) {
	p := SubagentToolsetPolicy{Mode: SubagentToolsetCategoricalExclusion}
	assert.ErrorIs(t, p.Validate(), ErrSubagentToolsetCategoryEmpty)
}

func TestSubagentToolset_ValidateDepthRejectsEmptyName(t *testing.T) {
	p := SubagentToolsetPolicy{
		Mode: SubagentToolsetDepthFiltered,
		ExcludedAtDepthGreaterThan: map[string]int{"": 1},
	}
	assert.ErrorIs(t, p.Validate(), ErrSubagentToolsetEmptyName)
}

func TestSubagentToolset_ValidateDepthRejectsNegative(t *testing.T) {
	p := SubagentToolsetPolicy{
		Mode: SubagentToolsetDepthFiltered,
		ExcludedAtDepthGreaterThan: map[string]int{"Agent": -1},
	}
	assert.ErrorIs(t, p.Validate(), ErrSubagentToolsetBadDepth)
}

func TestSubagentToolset_ResolveRejectsBadDepthInput(t *testing.T) {
	p := SubagentToolsetPolicy{
		Mode: SubagentToolsetExplicitAllowlist,
		AllowedToolNames: []string{"Read"},
	}
	_, _, err := ResolveSubagentToolset(nil, p, -1)
	assert.ErrorIs(t, err, ErrSubagentToolsetBadDepthInput)
}

func TestSubagentToolset_ExplicitAllowlistKeepsListedOnly(t *testing.T) {
	parent := []ToolPoolEntry{
		{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "h"},
		{Name: "Bash", Source: ToolSourceBuiltin, ProviderID: "h"},
		{Name: "Edit", Source: ToolSourceBuiltin, ProviderID: "h"},
	}
	policy := SubagentToolsetPolicy{
		Mode: SubagentToolsetExplicitAllowlist,
		AllowedToolNames: []string{"Read"},
	}
	kept, res, err := ResolveSubagentToolset(parent, policy, 1)
	require.NoError(t, err)
	require.Equal(t, 1, len(kept))
	assert.Equal(t, "Read", kept[0].Name)
	assert.ElementsMatch(t, []string{"Bash", "Edit"}, res.DroppedToolNames)
}

func TestSubagentToolset_ParentMinusBlocklistDropsListed(t *testing.T) {
	parent := []ToolPoolEntry{
		{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "h"},
		{Name: "Bash", Source: ToolSourceBuiltin, ProviderID: "h"},
		{Name: "Edit", Source: ToolSourceBuiltin, ProviderID: "h"},
	}
	policy := SubagentToolsetPolicy{
		Mode: SubagentToolsetParentMinusBlocklist,
		BlockedToolNames: []string{"Bash"},
	}
	kept, res, err := ResolveSubagentToolset(parent, policy, 1)
	require.NoError(t, err)
	assert.Equal(t, 2, len(kept))
	assert.Equal(t, []string{"Bash"}, res.DroppedToolNames)
}

func TestSubagentToolset_DepthFilteredExcludesAtDepthGreaterThan(t *testing.T) {
	parent := []ToolPoolEntry{
		{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "h"},
		{Name: "Agent", Source: ToolSourceBuiltin, ProviderID: "h"},
	}
	policy := SubagentToolsetPolicy{
		Mode: SubagentToolsetDepthFiltered,
		ExcludedAtDepthGreaterThan: map[string]int{"Agent": 1},
	}
	// At depth 1 (≤1): Agent still allowed.
	kept, _, err := ResolveSubagentToolset(parent, policy, 1)
	require.NoError(t, err)
	assert.Equal(t, 2, len(kept))
	// At depth 2 (>1): Agent excluded.
	kept2, res2, err := ResolveSubagentToolset(parent, policy, 2)
	require.NoError(t, err)
	assert.Equal(t, 1, len(kept2))
	assert.Equal(t, []string{"Agent"}, res2.DroppedToolNames)
}

func TestSubagentToolset_DepthFilteredAtDepthZeroAllowsAll(t *testing.T) {
	parent := []ToolPoolEntry{
		{Name: "Agent", Source: ToolSourceBuiltin, ProviderID: "h"},
	}
	policy := SubagentToolsetPolicy{
		Mode: SubagentToolsetDepthFiltered,
		ExcludedAtDepthGreaterThan: map[string]int{"Agent": 1},
	}
	kept, _, err := ResolveSubagentToolset(parent, policy, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, len(kept))
}

func TestSubagentToolset_CategoricalExclusionByPrefix(t *testing.T) {
	parent := []ToolPoolEntry{
		{Name: "admin_create", Source: ToolSourceBuiltin, ProviderID: "h"},
		{Name: "admin_delete", Source: ToolSourceBuiltin, ProviderID: "h"},
		{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "h"},
	}
	policy := SubagentToolsetPolicy{
		Mode: SubagentToolsetCategoricalExclusion,
		CategoryPrefixesExcluded: []string{"admin_"},
	}
	kept, res, err := ResolveSubagentToolset(parent, policy, 1)
	require.NoError(t, err)
	assert.Equal(t, 1, len(kept))
	assert.Equal(t, "Read", kept[0].Name)
	assert.ElementsMatch(t, []string{"admin_create", "admin_delete"}, res.DroppedToolNames)
}

func TestSubagentToolset_CategoricalExclusionBySuffix(t *testing.T) {
	parent := []ToolPoolEntry{
		{Name: "file_dangerous", Source: ToolSourceBuiltin, ProviderID: "h"},
		{Name: "file_safe", Source: ToolSourceBuiltin, ProviderID: "h"},
	}
	policy := SubagentToolsetPolicy{
		Mode: SubagentToolsetCategoricalExclusion,
		CategorySuffixesExcluded: []string{"_dangerous"},
	}
	kept, _, err := ResolveSubagentToolset(parent, policy, 1)
	require.NoError(t, err)
	assert.Equal(t, 1, len(kept))
	assert.Equal(t, "file_safe", kept[0].Name)
}

func TestSubagentToolset_CategoricalExclusionBothFilters(t *testing.T) {
	parent := []ToolPoolEntry{
		{Name: "admin_x", Source: ToolSourceBuiltin, ProviderID: "h"},
		{Name: "y_dangerous", Source: ToolSourceBuiltin, ProviderID: "h"},
		{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "h"},
	}
	policy := SubagentToolsetPolicy{
		Mode: SubagentToolsetCategoricalExclusion,
		CategoryPrefixesExcluded: []string{"admin_"},
		CategorySuffixesExcluded: []string{"_dangerous"},
	}
	kept, _, err := ResolveSubagentToolset(parent, policy, 1)
	require.NoError(t, err)
	assert.Equal(t, 1, len(kept))
	assert.Equal(t, "Read", kept[0].Name)
}

func TestSubagentToolset_EmptyParentPoolReturnsEmpty(t *testing.T) {
	policy := SubagentToolsetPolicy{
		Mode: SubagentToolsetExplicitAllowlist,
		AllowedToolNames: []string{"Read"},
	}
	kept, _, err := ResolveSubagentToolset(nil, policy, 1)
	require.NoError(t, err)
	assert.Empty(t, kept)
}

func TestSubagentToolset_InputsNotMutated(t *testing.T) {
	parent := []ToolPoolEntry{
		{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "h"},
	}
	policy := SubagentToolsetPolicy{
		Mode: SubagentToolsetParentMinusBlocklist,
		BlockedToolNames: []string{"Bash"},
	}
	originalLen := len(parent)
	originalName := parent[0].Name
	_, _, _ = ResolveSubagentToolset(parent, policy, 1)
	assert.Equal(t, originalLen, len(parent))
	assert.Equal(t, originalName, parent[0].Name)
}

func TestSubagentToolset_DroppedNamesSortedDeterministic(t *testing.T) {
	parent := []ToolPoolEntry{
		{Name: "Z_tool", Source: ToolSourceBuiltin, ProviderID: "h"},
		{Name: "A_tool", Source: ToolSourceBuiltin, ProviderID: "h"},
	}
	policy := SubagentToolsetPolicy{
		Mode: SubagentToolsetExplicitAllowlist,
		AllowedToolNames: []string{"X"},
	}
	_, res, _ := ResolveSubagentToolset(parent, policy, 1)
	assert.Equal(t, []string{"A_tool", "Z_tool"}, res.DroppedToolNames)
}

func TestSubagentToolset_ReasonPopulated(t *testing.T) {
	parent := []ToolPoolEntry{{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "h"}}
	for _, mode := range allSubagentToolsetIsolationModes {
		p := buildPolicyForMode(mode)
		_, res, err := ResolveSubagentToolset(parent, p, 1)
		require.NoError(t, err, "mode %q", mode)
		assert.NotEmpty(t, res.Reason, "mode %q missing reason", mode)
	}
}

func TestSubagentToolset_AsToolPoolFilterRejectsBadPolicy(t *testing.T) {
	p := SubagentToolsetPolicy{Mode: SubagentToolsetIsolationMode("nope")}
	_, err := p.AsToolPoolFilter(1)
	assert.ErrorIs(t, err, ErrSubagentToolsetBadMode)
}

func TestSubagentToolset_AsToolPoolFilterRejectsBadDepth(t *testing.T) {
	p := SubagentToolsetPolicy{
		Mode: SubagentToolsetExplicitAllowlist,
		AllowedToolNames: []string{"Read"},
	}
	_, err := p.AsToolPoolFilter(-1)
	assert.ErrorIs(t, err, ErrSubagentToolsetBadDepthInput)
}

func TestSubagentToolset_FilterIntegratesWithAssembler(t *testing.T) {
	// Wire a SubagentToolsetPolicy into a ToolPoolAssembler via the
	// filter adapter — demonstrates TOOL-003 integration.
	a := NewToolPoolAssembler()
	a.Register(NewStaticToolPoolProvider(ToolSourceBuiltin, []ToolPoolEntry{
		{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "h"},
		{Name: "Bash", Source: ToolSourceBuiltin, ProviderID: "h"},
	}))
	policy := SubagentToolsetPolicy{
		Mode: SubagentToolsetParentMinusBlocklist,
		BlockedToolNames: []string{"Bash"},
	}
	f, err := policy.AsToolPoolFilter(1)
	require.NoError(t, err)
	a.AddFilter(f)
	snap, err := a.Assemble(context.Background(), "iter-sub-1")
	require.NoError(t, err)
	assert.Equal(t, []string{"Read"}, snap.Names())
}

func TestSubagentToolset_FilterDepthAwareAtAssemblyTime(t *testing.T) {
	parent := []ToolPoolEntry{
		{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "h"},
		{Name: "Agent", Source: ToolSourceBuiltin, ProviderID: "h"},
	}
	policy := SubagentToolsetPolicy{
		Mode: SubagentToolsetDepthFiltered,
		ExcludedAtDepthGreaterThan: map[string]int{"Agent": 1},
	}

	// depth=2 → Agent dropped.
	deepFilter, _ := policy.AsToolPoolFilter(2)
	assert.False(t, deepFilter(parent[1]))

	// depth=1 → Agent kept.
	shallowFilter, _ := policy.AsToolPoolFilter(1)
	assert.True(t, shallowFilter(parent[1]))
}

func TestSubagentToolset_ParentMinusBlocklistEmptyBlocklistKeepsAll(t *testing.T) {
	parent := []ToolPoolEntry{
		{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "h"},
	}
	policy := SubagentToolsetPolicy{Mode: SubagentToolsetParentMinusBlocklist}
	kept, _, err := ResolveSubagentToolset(parent, policy, 1)
	require.NoError(t, err)
	assert.Equal(t, 1, len(kept))
}

func TestSubagentToolset_ResolutionPreservesDepth(t *testing.T) {
	policy := SubagentToolsetPolicy{
		Mode: SubagentToolsetExplicitAllowlist,
		AllowedToolNames: []string{"Read"},
	}
	_, res, _ := ResolveSubagentToolset(nil, policy, 7)
	assert.Equal(t, 7, res.CurrentDepth)
}

func TestSubagentToolset_KeptToolNamesSortedDeterministic(t *testing.T) {
	parent := []ToolPoolEntry{
		{Name: "Z", Source: ToolSourceBuiltin, ProviderID: "h"},
		{Name: "A", Source: ToolSourceBuiltin, ProviderID: "h"},
		{Name: "M", Source: ToolSourceBuiltin, ProviderID: "h"},
	}
	policy := SubagentToolsetPolicy{Mode: SubagentToolsetParentMinusBlocklist}
	_, res, _ := ResolveSubagentToolset(parent, policy, 0)
	assert.Equal(t, []string{"A", "M", "Z"}, res.KeptToolNames)
}

func buildPolicyForMode(m SubagentToolsetIsolationMode) SubagentToolsetPolicy {
	switch m {
	case SubagentToolsetExplicitAllowlist:
		return SubagentToolsetPolicy{Mode: m, AllowedToolNames: []string{"Read"}}
	case SubagentToolsetDepthFiltered:
		return SubagentToolsetPolicy{Mode: m, ExcludedAtDepthGreaterThan: map[string]int{"Agent": 1}}
	case SubagentToolsetCategoricalExclusion:
		return SubagentToolsetPolicy{Mode: m, CategoryPrefixesExcluded: []string{"admin_"}}
	}
	return SubagentToolsetPolicy{Mode: m}
}
