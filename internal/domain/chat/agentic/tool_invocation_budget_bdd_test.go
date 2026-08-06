package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BDD: TOOL-010 Tool invocation budget (PDF §7 — context management budgets)

func TestBDD_TOOL010_UnlimitedBudgetNeverBlocks(t *testing.T) {
	// GIVEN an agent with no budget cap configured
	// WHEN hundreds of tool calls are recorded
	// THEN none are blocked — unlimited mode is the safe default
	b := DefaultUnlimitedBudget()
	for i := 0; i < 500; i++ {
		require.NoError(t, b.Record(ToolCategoryRead),
			"unlimited budget must not block at call %d", i)
	}
}

func TestBDD_TOOL010_DenyPolicyBlocksExactlyAtCap(t *testing.T) {
	// GIVEN a session with TotalCap=5 and Deny policy
	// WHEN exactly 5 calls are made
	// THEN call 5 succeeds and call 6 is denied
	b, _ := NewToolInvocationBudget(ToolInvocationBudgetConfig{
		TotalCap: 5,
		Policy:   ToolBudgetPolicyDeny,
	})
	for i := 0; i < 5; i++ {
		require.NoError(t, b.Record(ToolCategoryExternal))
	}
	err := b.Record(ToolCategoryExternal)
	require.ErrorIs(t, err, ErrTIBTotalBudgetExhausted)
}

func TestBDD_TOOL010_CategoryCapIsolatedFromOtherCategories(t *testing.T) {
	// GIVEN a budget with mutate capped at 3 but read unlimited
	// WHEN 3 mutate calls exhaust the category budget
	// THEN read calls still succeed (category isolation)
	b, _ := NewToolInvocationBudget(ToolInvocationBudgetConfig{
		TotalCap: 1000,
		Policy:   ToolBudgetPolicyDeny,
		CategoryCaps: map[ToolInvocationCategory]int{
			ToolCategoryMutate: 3,
		},
	})
	for i := 0; i < 3; i++ {
		require.NoError(t, b.Record(ToolCategoryMutate))
	}
	assert.ErrorIs(t, b.Record(ToolCategoryMutate), ErrTIBCategoryBudgetExhausted)
	// read is NOT capped
	require.NoError(t, b.Record(ToolCategoryRead), "read calls must not be blocked by mutate cap")
}

func TestBDD_TOOL010_WarnPolicyRecordsButDoesNotBlock(t *testing.T) {
	// GIVEN a budget with warn policy (non-blocking, for observability)
	// WHEN cap is exceeded
	// THEN no error is returned but IsExhausted() reports the breach
	b, _ := NewToolInvocationBudget(ToolInvocationBudgetConfig{
		TotalCap: 2,
		Policy:   ToolBudgetPolicyWarn,
	})
	require.NoError(t, b.Record(ToolCategoryRead))
	require.NoError(t, b.Record(ToolCategoryRead))
	// exceed the cap
	require.NoError(t, b.Record(ToolCategoryRead), "warn policy must not return error")
	snap := b.Snapshot()
	assert.True(t, snap.IsExhausted(), "IsExhausted should report true even under warn policy")
}

func TestBDD_TOOL010_PercentUsedReflectsProgress(t *testing.T) {
	// GIVEN a session cap of 100 tool calls
	// WHEN 75 calls are made
	// THEN PercentUsed() reports 75 for budget dashboard
	b, _ := NewToolInvocationBudget(ToolInvocationBudgetConfig{
		TotalCap: 100,
		Policy:   ToolBudgetPolicyWarn,
	})
	for i := 0; i < 75; i++ {
		require.NoError(t, b.Record(ToolCategoryInternal))
	}
	assert.Equal(t, 75, b.Snapshot().PercentUsed())
}

func TestBDD_TOOL010_MCPToolsClassifiedAsExternal(t *testing.T) {
	// GIVEN MCP tools from the mcp_server_preset_template seed
	// WHEN classified for budget tracking
	// THEN they all map to the "external" category (higher cost tier)
	mcpSlugs := []string{
		"mcp__github__search_repositories",
		"mcp__filesystem__read_file",
		"mcp__brave_search__search",
	}
	for _, slug := range mcpSlugs {
		cat := ClassifyToolForBudget(slug)
		assert.Equal(t, ToolCategoryExternal, cat,
			"MCP tool %q must be classified as external", slug)
	}
}
