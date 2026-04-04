package agentic_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- NewPermissionBridge ---

func TestNewPermissionBridge(t *testing.T) {
	b := agentic.NewPermissionBridge()
	assert.NotNil(t, b)
	assert.Equal(t, 0, b.PendingCount())
}

// --- Request + Resolve ---

func TestPermissionBridge_RequestResolve(t *testing.T) {
	b := agentic.NewPermissionBridge()

	var decision agentic.PermissionBridgeDecision
	done := make(chan struct{})
	go func() {
		decision = b.Request(context.Background(), agentic.PermissionBridgeRequest{
			ID:         "r1",
			WorkerName: "agent-1",
			ToolName:   "bash",
			Input:      "rm -rf /tmp/test",
		}, nil)
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, 1, b.PendingCount())

	ok := b.Resolve("r1", agentic.PermissionBridgeDecision{
		Behavior: agentic.BridgePermAllow,
		Feedback: "ok",
	})
	assert.True(t, ok)

	<-done
	assert.Equal(t, agentic.BridgePermAllow, decision.Behavior)
	assert.Equal(t, "ok", decision.Feedback)
}

// --- Request cancelled by context ---

func TestPermissionBridge_Request_ContextCancel(t *testing.T) {
	b := agentic.NewPermissionBridge()
	ctx, cancel := context.WithCancel(context.Background())

	var decision agentic.PermissionBridgeDecision
	done := make(chan struct{})
	go func() {
		decision = b.Request(ctx, agentic.PermissionBridgeRequest{
			ID: "r1", WorkerName: "w", ToolName: "bash",
		}, nil)
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	<-done
	assert.Equal(t, agentic.BridgePermDeny, decision.Behavior)
	assert.Equal(t, "context cancelled", decision.Reason)
	assert.Equal(t, 0, b.PendingCount())
}

// --- Resolve unknown ---

func TestPermissionBridge_Resolve_Unknown(t *testing.T) {
	b := agentic.NewPermissionBridge()
	ok := b.Resolve("nonexistent", agentic.PermissionBridgeDecision{Behavior: agentic.BridgePermAllow})
	assert.False(t, ok)
}

// --- OnRequest callback ---

func TestPermissionBridge_OnRequest(t *testing.T) {
	b := agentic.NewPermissionBridge()

	var notified atomic.Int32
	b.OnRequest(func(req agentic.PermissionBridgeRequest) {
		notified.Add(1)
	})

	go func() {
		b.Request(context.Background(), agentic.PermissionBridgeRequest{
			ID: "r1", WorkerName: "w", ToolName: "bash",
		}, nil)
	}()

	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, int32(1), notified.Load())
	b.Resolve("r1", agentic.PermissionBridgeDecision{Behavior: agentic.BridgePermDeny})
}

// --- Wait time tracking ---

func TestPermissionBridge_WaitTimeTracking(t *testing.T) {
	b := agentic.NewPermissionBridge()

	var waitMs atomic.Int64
	done := make(chan struct{})
	go func() {
		b.Request(context.Background(), agentic.PermissionBridgeRequest{
			ID: "r1", WorkerName: "w", ToolName: "bash",
		}, func(ms int64) {
			waitMs.Store(ms)
		})
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	b.Resolve("r1", agentic.PermissionBridgeDecision{Behavior: agentic.BridgePermAllow})

	<-done
	assert.GreaterOrEqual(t, waitMs.Load(), int64(30))
}

// --- PendingRequests ---

func TestPermissionBridge_PendingRequests(t *testing.T) {
	b := agentic.NewPermissionBridge()

	go func() {
		b.Request(context.Background(), agentic.PermissionBridgeRequest{
			ID: "r1", WorkerName: "w1", ToolName: "bash",
		}, nil)
	}()
	go func() {
		b.Request(context.Background(), agentic.PermissionBridgeRequest{
			ID: "r2", WorkerName: "w2", ToolName: "edit",
		}, nil)
	}()

	time.Sleep(30 * time.Millisecond)
	pending := b.PendingRequests()
	assert.Len(t, pending, 2)

	// Clean up
	b.ResolveAll(agentic.PermissionBridgeDecision{Behavior: agentic.BridgePermDeny})
}

// --- ResolveAll ---

func TestPermissionBridge_ResolveAll(t *testing.T) {
	b := agentic.NewPermissionBridge()

	var wg sync.WaitGroup
	var decisions [2]agentic.PermissionBridgeDecision
	for i := 0; i < 2; i++ {
		wg.Add(1)
		idx := i
		go func() {
			defer wg.Done()
			decisions[idx] = b.Request(context.Background(), agentic.PermissionBridgeRequest{
				ID: string(rune('a' + idx)), WorkerName: "w", ToolName: "bash",
			}, nil)
		}()
	}

	time.Sleep(30 * time.Millisecond)
	n := b.ResolveAll(agentic.PermissionBridgeDecision{Behavior: agentic.BridgePermDeny, Reason: "shutdown"})
	assert.Equal(t, 2, n)

	wg.Wait()
	for _, d := range decisions {
		assert.Equal(t, agentic.BridgePermDeny, d.Behavior)
	}
	assert.Equal(t, 0, b.PendingCount())
}

// --- Concurrent requests ---

func TestPermissionBridge_ConcurrentRequests(t *testing.T) {
	b := agentic.NewPermissionBridge()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		id := string(rune('a' + i))
		go func() {
			defer wg.Done()
			b.Request(context.Background(), agentic.PermissionBridgeRequest{
				ID: id, WorkerName: "w", ToolName: "t",
			}, nil)
		}()
	}

	time.Sleep(30 * time.Millisecond)
	assert.Equal(t, 10, b.PendingCount())

	b.ResolveAll(agentic.PermissionBridgeDecision{Behavior: agentic.BridgePermAllow})
	wg.Wait()
	assert.Equal(t, 0, b.PendingCount())
}

// --- Behaviors ---

func TestPermissionBehavior_Values(t *testing.T) {
	assert.Equal(t, agentic.BridgePermBehavior("allow"), agentic.BridgePermAllow)
	assert.Equal(t, agentic.BridgePermBehavior("deny"), agentic.BridgePermDeny)
	assert.Equal(t, agentic.BridgePermBehavior("ask"), agentic.BridgePermAsk)
}
