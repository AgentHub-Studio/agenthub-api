package agentic_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

func orchExecutor(_ context.Context, name string, _ map[string]any) (*agentic.OrchestratedToolResult, error) {
	return &agentic.OrchestratedToolResult{
		Output: map[string]any{"tool": name, "status": "ok"},
	}, nil
}

func orchSlowExecutor(delay time.Duration) agentic.OrchestratorExecutorFunc {
	return func(_ context.Context, name string, _ map[string]any) (*agentic.OrchestratedToolResult, error) {
		time.Sleep(delay)
		return &agentic.OrchestratedToolResult{
			Output: map[string]any{"tool": name},
		}, nil
	}
}

// --- Basic execution ---

func TestToolOrchestrator_ExecuteAll_Empty(t *testing.T) {
	orch := agentic.NewToolOrchestrator(orchExecutor)
	results := orch.ExecuteAll(context.Background(), nil)
	assert.Nil(t, results)
}

func TestToolOrchestrator_ExecuteAll_Single(t *testing.T) {
	orch := agentic.NewToolOrchestrator(orchExecutor)
	results := orch.ExecuteAll(context.Background(), []agentic.OrchestratedTool{
		{ID: "tc-1", Name: "search", Input: map[string]any{"q": "test"}},
	})

	require.Len(t, results, 1)
	assert.Equal(t, "search", results[0].Name)
	assert.NotNil(t, results[0].Result)
	assert.Nil(t, results[0].Error)
	assert.Equal(t, "ok", results[0].Result.Output["status"])
}

// --- Deterministic order ---

func TestToolOrchestrator_PreservesOrder(t *testing.T) {
	orch := agentic.NewToolOrchestrator(orchExecutor)
	tools := []agentic.OrchestratedTool{
		{ID: "tc-1", Name: "first"},
		{ID: "tc-2", Name: "second"},
		{ID: "tc-3", Name: "third"},
	}
	results := orch.ExecuteAll(context.Background(), tools)

	require.Len(t, results, 3)
	assert.Equal(t, "first", results[0].Name)
	assert.Equal(t, "second", results[1].Name)
	assert.Equal(t, "third", results[2].Name)
}

// --- Concurrent execution ---

func TestToolOrchestrator_ConcurrentTools_RunInParallel(t *testing.T) {
	var running int32
	var maxRunning int32

	executor := func(_ context.Context, name string, _ map[string]any) (*agentic.OrchestratedToolResult, error) {
		cur := atomic.AddInt32(&running, 1)
		for {
			old := atomic.LoadInt32(&maxRunning)
			if cur <= old || atomic.CompareAndSwapInt32(&maxRunning, old, cur) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
		atomic.AddInt32(&running, -1)
		return &agentic.OrchestratedToolResult{Output: map[string]any{"tool": name}}, nil
	}

	orch := agentic.NewToolOrchestrator(executor)
	tools := []agentic.OrchestratedTool{
		{ID: "tc-1", Name: "tool-a"},
		{ID: "tc-2", Name: "tool-b"},
		{ID: "tc-3", Name: "tool-c"},
	}
	orch.ExecuteAll(context.Background(), tools)

	assert.Greater(t, atomic.LoadInt32(&maxRunning), int32(1), "should run tools concurrently")
}

// --- Exclusive tools run sequentially ---

func TestToolOrchestrator_ExclusiveTools_RunSequentially(t *testing.T) {
	var running int32
	var maxRunning int32

	executor := func(_ context.Context, name string, _ map[string]any) (*agentic.OrchestratedToolResult, error) {
		cur := atomic.AddInt32(&running, 1)
		for {
			old := atomic.LoadInt32(&maxRunning)
			if cur <= old || atomic.CompareAndSwapInt32(&maxRunning, old, cur) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		atomic.AddInt32(&running, -1)
		return &agentic.OrchestratedToolResult{Output: map[string]any{"tool": name}}, nil
	}

	orch := agentic.NewToolOrchestrator(executor)
	orch.SetSafetyClassifier(func(_ string) agentic.ToolSafety {
		return agentic.ToolSafeExclusive
	})

	tools := []agentic.OrchestratedTool{
		{ID: "tc-1", Name: "write-a"},
		{ID: "tc-2", Name: "write-b"},
		{ID: "tc-3", Name: "write-c"},
	}
	orch.ExecuteAll(context.Background(), tools)

	assert.Equal(t, int32(1), atomic.LoadInt32(&maxRunning), "exclusive tools should run one at a time")
}

// --- Mixed safety ---

func TestToolOrchestrator_MixedSafety(t *testing.T) {
	var order []string
	var mu sync.Mutex

	executor := func(_ context.Context, name string, _ map[string]any) (*agentic.OrchestratedToolResult, error) {
		time.Sleep(10 * time.Millisecond)
		mu.Lock()
		order = append(order, name)
		mu.Unlock()
		return &agentic.OrchestratedToolResult{}, nil
	}

	orch := agentic.NewToolOrchestrator(executor)
	orch.SetSafetyClassifier(func(name string) agentic.ToolSafety {
		if name == "write" {
			return agentic.ToolSafeExclusive
		}
		return agentic.ToolSafeConcurrent
	})

	tools := []agentic.OrchestratedTool{
		{ID: "tc-1", Name: "read-a"},
		{ID: "tc-2", Name: "read-b"},
		{ID: "tc-3", Name: "write"},
	}
	results := orch.ExecuteAll(context.Background(), tools)
	require.Len(t, results, 3)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, "write", order[len(order)-1], "exclusive tool should run last")
}

// --- Error handling ---

func TestToolOrchestrator_ToolError(t *testing.T) {
	executor := func(_ context.Context, name string, _ map[string]any) (*agentic.OrchestratedToolResult, error) {
		if name == "fail" {
			return nil, errors.New("tool failed")
		}
		return &agentic.OrchestratedToolResult{}, nil
	}

	orch := agentic.NewToolOrchestrator(executor)
	tools := []agentic.OrchestratedTool{
		{ID: "tc-1", Name: "ok"},
		{ID: "tc-2", Name: "fail"},
	}
	results := orch.ExecuteAll(context.Background(), tools)

	assert.Nil(t, results[0].Error)
	assert.NotNil(t, results[1].Error)
	assert.True(t, results[1].Result.IsError)
	assert.Contains(t, results[1].Result.ErrorMessage, "tool failed")
	assert.True(t, orch.HasErrors())
}

func TestToolOrchestrator_NoErrors(t *testing.T) {
	orch := agentic.NewToolOrchestrator(orchExecutor)
	orch.ExecuteAll(context.Background(), []agentic.OrchestratedTool{
		{ID: "tc-1", Name: "ok"},
	})
	assert.False(t, orch.HasErrors())
}

// --- Callbacks ---

func TestToolOrchestrator_OnStartOnComplete(t *testing.T) {
	var starts, completes []string

	orch := agentic.NewToolOrchestrator(orchExecutor)
	orch.OnStart(func(ot agentic.OrchestratedTool) {
		starts = append(starts, ot.Name)
	})
	orch.OnComplete(func(ot agentic.OrchestratedTool) {
		completes = append(completes, ot.Name)
	})

	orch.ExecuteAll(context.Background(), []agentic.OrchestratedTool{
		{ID: "tc-1", Name: "tool-a"},
	})

	assert.Equal(t, []string{"tool-a"}, starts)
	assert.Equal(t, []string{"tool-a"}, completes)
}

// --- Duration ---

func TestOrchestratedTool_Duration(t *testing.T) {
	orch := agentic.NewToolOrchestrator(orchSlowExecutor(20 * time.Millisecond))
	results := orch.ExecuteAll(context.Background(), []agentic.OrchestratedTool{
		{ID: "tc-1", Name: "slow"},
	})

	require.Len(t, results, 1)
	assert.GreaterOrEqual(t, results[0].Duration().Milliseconds(), int64(15))
}

// --- Context cancellation ---

func TestToolOrchestrator_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	executor := func(ctx context.Context, name string, _ map[string]any) (*agentic.OrchestratedToolResult, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(200 * time.Millisecond):
			return &agentic.OrchestratedToolResult{}, nil
		}
	}

	orch := agentic.NewToolOrchestrator(executor)
	orch.SetSafetyClassifier(func(_ string) agentic.ToolSafety {
		return agentic.ToolSafeExclusive
	})

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	tools := []agentic.OrchestratedTool{
		{ID: "tc-1", Name: "slow-1"},
		{ID: "tc-2", Name: "slow-2"},
	}
	results := orch.ExecuteAll(ctx, tools)
	require.Len(t, results, 2)

	hasCtxErr := false
	for _, r := range results {
		if r.Error != nil {
			hasCtxErr = true
		}
	}
	assert.True(t, hasCtxErr)
}

// --- Summary ---

func TestToolOrchestrator_Summary(t *testing.T) {
	executor := func(_ context.Context, name string, _ map[string]any) (*agentic.OrchestratedToolResult, error) {
		if name == "fail" {
			return nil, errors.New("boom")
		}
		return &agentic.OrchestratedToolResult{}, nil
	}

	orch := agentic.NewToolOrchestrator(executor)
	orch.ExecuteAll(context.Background(), []agentic.OrchestratedTool{
		{ID: "tc-1", Name: "ok"},
		{ID: "tc-2", Name: "fail"},
		{ID: "tc-3", Name: "ok2"},
	})

	summary := orch.Summary()
	assert.Contains(t, summary, "2 succeeded")
	assert.Contains(t, summary, "1 failed")
	assert.Contains(t, summary, "3 tools")
}

// --- ToolCount ---

func TestToolOrchestrator_ToolCount(t *testing.T) {
	orch := agentic.NewToolOrchestrator(orchExecutor)
	assert.Equal(t, 0, orch.ToolCount())

	orch.ExecuteAll(context.Background(), []agentic.OrchestratedTool{
		{ID: "tc-1", Name: "a"},
		{ID: "tc-2", Name: "b"},
	})
	assert.Equal(t, 2, orch.ToolCount())
}
