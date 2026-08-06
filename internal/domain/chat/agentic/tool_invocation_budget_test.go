package agentic

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolInvocationCategory_AllCount(t *testing.T) {
	assert.Equal(t, 5, len(AllToolInvocationCategories))
}

func TestToolInvocationCategory_IsValid(t *testing.T) {
	assert.True(t, ToolCategoryRead.IsValid())
	assert.True(t, ToolCategoryMutate.IsValid())
	assert.True(t, ToolCategoryExternal.IsValid())
	assert.True(t, ToolCategoryInternal.IsValid())
	assert.True(t, ToolCategoryUnknown.IsValid())
	assert.False(t, ToolInvocationCategory("db").IsValid())
}

func TestToolBudgetPolicy_AllCount(t *testing.T) {
	assert.Equal(t, 3, len(AllToolBudgetPolicies))
}

func TestToolBudgetPolicy_IsValid(t *testing.T) {
	assert.True(t, ToolBudgetPolicyDeny.IsValid())
	assert.True(t, ToolBudgetPolicyWarn.IsValid())
	assert.True(t, ToolBudgetPolicyReport.IsValid())
	assert.False(t, ToolBudgetPolicy("block").IsValid())
}

func TestToolInvocationBudgetConfig_Validate_Happy(t *testing.T) {
	cfg := ToolInvocationBudgetConfig{
		TotalCap: 100,
		Policy:   ToolBudgetPolicyDeny,
		CategoryCaps: map[ToolInvocationCategory]int{
			ToolCategoryMutate: 10,
		},
	}
	assert.NoError(t, cfg.Validate())
}

func TestToolInvocationBudgetConfig_Validate_NegativeCap(t *testing.T) {
	cfg := ToolInvocationBudgetConfig{TotalCap: -1, Policy: ToolBudgetPolicyDeny}
	assert.ErrorIs(t, cfg.Validate(), ErrTIBNegativeCap)
}

func TestToolInvocationBudgetConfig_Validate_InvalidPolicy(t *testing.T) {
	cfg := ToolInvocationBudgetConfig{TotalCap: 0, Policy: "noop"}
	assert.ErrorIs(t, cfg.Validate(), ErrTIBInvalidPolicy)
}

func TestToolInvocationBudgetConfig_Validate_InvalidCategory(t *testing.T) {
	cfg := ToolInvocationBudgetConfig{
		Policy: ToolBudgetPolicyDeny,
		CategoryCaps: map[ToolInvocationCategory]int{
			"bad": 5,
		},
	}
	assert.ErrorIs(t, cfg.Validate(), ErrTIBInvalidCategory)
}

func TestDefaultUnlimitedBudget(t *testing.T) {
	b := DefaultUnlimitedBudget()
	require.NotNil(t, b)
	snap := b.Snapshot()
	assert.Equal(t, 0, snap.TotalCap)
	assert.Equal(t, ToolBudgetPolicyWarn, snap.Policy)
}

func TestToolInvocationBudget_Record_NoCapAlwaysSucceeds(t *testing.T) {
	b := DefaultUnlimitedBudget()
	for i := 0; i < 1000; i++ {
		require.NoError(t, b.Record(ToolCategoryRead))
	}
	snap := b.Snapshot()
	assert.Equal(t, 1000, snap.TotalCalled)
}

func TestToolInvocationBudget_Record_TotalDenyBlocks(t *testing.T) {
	b, _ := NewToolInvocationBudget(ToolInvocationBudgetConfig{
		TotalCap: 3,
		Policy:   ToolBudgetPolicyDeny,
	})
	require.NoError(t, b.Record(ToolCategoryRead))
	require.NoError(t, b.Record(ToolCategoryRead))
	require.NoError(t, b.Record(ToolCategoryRead))
	err := b.Record(ToolCategoryRead)
	assert.ErrorIs(t, err, ErrTIBTotalBudgetExhausted)
}

func TestToolInvocationBudget_Record_TotalWarnAllows(t *testing.T) {
	b, _ := NewToolInvocationBudget(ToolInvocationBudgetConfig{
		TotalCap: 1,
		Policy:   ToolBudgetPolicyWarn,
	})
	require.NoError(t, b.Record(ToolCategoryRead))
	// second call exceeds cap but policy is warn — no error
	require.NoError(t, b.Record(ToolCategoryRead))
	snap := b.Snapshot()
	assert.Equal(t, 2, snap.TotalCalled)
}

func TestToolInvocationBudget_Record_CategoryDenyBlocks(t *testing.T) {
	b, _ := NewToolInvocationBudget(ToolInvocationBudgetConfig{
		TotalCap: 100,
		Policy:   ToolBudgetPolicyDeny,
		CategoryCaps: map[ToolInvocationCategory]int{
			ToolCategoryMutate: 2,
		},
	})
	require.NoError(t, b.Record(ToolCategoryMutate))
	require.NoError(t, b.Record(ToolCategoryMutate))
	err := b.Record(ToolCategoryMutate)
	assert.ErrorIs(t, err, ErrTIBCategoryBudgetExhausted)
	// read tools are not capped
	require.NoError(t, b.Record(ToolCategoryRead))
}

func TestToolInvocationBudget_Record_InvalidCategoryError(t *testing.T) {
	b := DefaultUnlimitedBudget()
	err := b.Record(ToolInvocationCategory("bad"))
	assert.ErrorIs(t, err, ErrTIBInvalidCategory)
}

func TestToolInvocationBudget_Snapshot_PercentUsed(t *testing.T) {
	b, _ := NewToolInvocationBudget(ToolInvocationBudgetConfig{
		TotalCap: 10,
		Policy:   ToolBudgetPolicyWarn,
	})
	for i := 0; i < 5; i++ {
		require.NoError(t, b.Record(ToolCategoryRead))
	}
	snap := b.Snapshot()
	assert.Equal(t, 50, snap.PercentUsed())
}

func TestToolInvocationBudget_Snapshot_IsExhausted(t *testing.T) {
	b, _ := NewToolInvocationBudget(ToolInvocationBudgetConfig{
		TotalCap: 2,
		Policy:   ToolBudgetPolicyWarn,
	})
	assert.False(t, b.Snapshot().IsExhausted())
	require.NoError(t, b.Record(ToolCategoryRead))
	require.NoError(t, b.Record(ToolCategoryRead))
	assert.True(t, b.Snapshot().IsExhausted())
}

func TestToolInvocationBudget_Reset(t *testing.T) {
	b, _ := NewToolInvocationBudget(ToolInvocationBudgetConfig{
		TotalCap: 10,
		Policy:   ToolBudgetPolicyDeny,
	})
	require.NoError(t, b.Record(ToolCategoryRead))
	require.NoError(t, b.Record(ToolCategoryMutate))
	b.Reset()
	snap := b.Snapshot()
	assert.Equal(t, 0, snap.TotalCalled)
	for _, v := range snap.CategoryCalled {
		assert.Equal(t, 0, v)
	}
}

func TestToolInvocationBudget_ConcurrentSafe(t *testing.T) {
	b, _ := NewToolInvocationBudget(ToolInvocationBudgetConfig{
		TotalCap: 0,
		Policy:   ToolBudgetPolicyWarn,
	})
	var wg sync.WaitGroup
	errs := make(chan error, 50)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- b.Record(ToolCategoryRead)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	assert.Equal(t, 50, b.Snapshot().TotalCalled)
}

func TestClassifyToolForBudget_MCPIsExternal(t *testing.T) {
	assert.Equal(t, ToolCategoryExternal, ClassifyToolForBudget("mcp__github__create_issue"))
}

func TestClassifyToolForBudget_HTTPIsExternal(t *testing.T) {
	assert.Equal(t, ToolCategoryExternal, ClassifyToolForBudget("http-get"))
}

func TestClassifyToolForBudget_CreateIsMutate(t *testing.T) {
	assert.Equal(t, ToolCategoryMutate, ClassifyToolForBudget("create-agent"))
}

func TestClassifyToolForBudget_DeleteIsMutate(t *testing.T) {
	assert.Equal(t, ToolCategoryMutate, ClassifyToolForBudget("delete-knowledge-base"))
}

func TestClassifyToolForBudget_ListIsRead(t *testing.T) {
	assert.Equal(t, ToolCategoryRead, ClassifyToolForBudget("list-agents"))
}

func TestClassifyToolForBudget_EmptyIsUnknown(t *testing.T) {
	assert.Equal(t, ToolCategoryUnknown, ClassifyToolForBudget(""))
}
