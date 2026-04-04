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

// --- NewAbortController ---

func TestNewAbortController(t *testing.T) {
	ac := agentic.NewAbortController()
	assert.False(t, ac.IsAborted())
	assert.NotNil(t, ac.Context())
}

// --- Abort ---

func TestAbortController_Abort(t *testing.T) {
	ac := agentic.NewAbortController()
	ac.Abort()
	assert.True(t, ac.IsAborted())

	// Context should be cancelled.
	select {
	case <-ac.Context().Done():
		// OK
	default:
		t.Fatal("context should be done after abort")
	}
}

func TestAbortController_Abort_Idempotent(t *testing.T) {
	ac := agentic.NewAbortController()
	ac.Abort()
	ac.Abort() // should not panic
	assert.True(t, ac.IsAborted())
}

// --- OnAbort ---

func TestAbortController_OnAbort(t *testing.T) {
	ac := agentic.NewAbortController()
	var called bool
	ac.OnAbort(func() { called = true })

	ac.Abort()
	assert.True(t, called)
}

func TestAbortController_OnAbort_MultipleListeners(t *testing.T) {
	ac := agentic.NewAbortController()
	var count int32
	for i := 0; i < 5; i++ {
		ac.OnAbort(func() { atomic.AddInt32(&count, 1) })
	}

	ac.Abort()
	assert.Equal(t, int32(5), count)
}

func TestAbortController_OnAbort_AlreadyAborted(t *testing.T) {
	ac := agentic.NewAbortController()
	ac.Abort()

	var called bool
	ac.OnAbort(func() { called = true })
	assert.True(t, called, "listener should fire immediately when already aborted")
}

func TestAbortController_OnAbort_Cleanup(t *testing.T) {
	ac := agentic.NewAbortController()
	var called bool
	cleanup := ac.OnAbort(func() { called = true })

	cleanup() // remove listener

	ac.Abort()
	assert.False(t, called, "listener should not fire after cleanup")
}

// --- NewAbortControllerWithContext ---

func TestAbortControllerWithContext_ParentCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ac := agentic.NewAbortControllerWithContext(ctx)

	cancel()
	time.Sleep(10 * time.Millisecond) // give goroutine time to fire

	assert.True(t, ac.IsAborted())
}

func TestAbortControllerWithContext_SelfAbort(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ac := agentic.NewAbortControllerWithContext(ctx)
	ac.Abort()
	assert.True(t, ac.IsAborted())
}

// --- CreateChildAbortController ---

func TestCreateChildAbortController_ParentAbortsChlid(t *testing.T) {
	parent := agentic.NewAbortController()
	child := agentic.CreateChildAbortController(parent)

	parent.Abort()
	assert.True(t, child.IsAborted(), "child should abort when parent aborts")
}

func TestCreateChildAbortController_ChildDoesNotAbortParent(t *testing.T) {
	parent := agentic.NewAbortController()
	child := agentic.CreateChildAbortController(parent)

	child.Abort()
	assert.False(t, parent.IsAborted(), "parent should NOT abort when child aborts")
}

func TestCreateChildAbortController_ParentAlreadyAborted(t *testing.T) {
	parent := agentic.NewAbortController()
	parent.Abort()

	child := agentic.CreateChildAbortController(parent)
	assert.True(t, child.IsAborted(), "child should be immediately aborted")
}

func TestCreateChildAbortController_MultipleChildren(t *testing.T) {
	parent := agentic.NewAbortController()
	child1 := agentic.CreateChildAbortController(parent)
	child2 := agentic.CreateChildAbortController(parent)
	child3 := agentic.CreateChildAbortController(parent)

	parent.Abort()
	assert.True(t, child1.IsAborted())
	assert.True(t, child2.IsAborted())
	assert.True(t, child3.IsAborted())
}

func TestCreateChildAbortController_ChainDepth(t *testing.T) {
	root := agentic.NewAbortController()
	mid := agentic.CreateChildAbortController(root)
	leaf := agentic.CreateChildAbortController(mid)

	root.Abort()
	assert.True(t, mid.IsAborted())
	assert.True(t, leaf.IsAborted(), "grandchild should abort on root abort")
}

func TestCreateChildAbortController_ChildAbortCleansParentListener(t *testing.T) {
	parent := agentic.NewAbortController()
	child := agentic.CreateChildAbortController(parent)

	child.Abort()
	// Parent should still be functional.
	assert.False(t, parent.IsAborted())

	var parentCalled bool
	parent.OnAbort(func() { parentCalled = true })
	parent.Abort()
	assert.True(t, parentCalled)
}

// --- AbortGroup ---

func TestNewAbortGroup(t *testing.T) {
	g := agentic.NewAbortGroup()
	assert.Equal(t, 0, g.Count())
	assert.False(t, g.IsAborted())
}

func TestAbortGroup_AbortAll(t *testing.T) {
	g := agentic.NewAbortGroup()
	ac1 := agentic.NewAbortController()
	ac2 := agentic.NewAbortController()
	g.Add(ac1)
	g.Add(ac2)

	assert.Equal(t, 2, g.Count())

	g.AbortAll()
	assert.True(t, ac1.IsAborted())
	assert.True(t, ac2.IsAborted())
	assert.True(t, g.IsAborted())
}

func TestAbortGroup_AbortAll_Idempotent(t *testing.T) {
	g := agentic.NewAbortGroup()
	g.AbortAll()
	g.AbortAll() // should not panic
}

func TestAbortGroup_Add_AfterAborted(t *testing.T) {
	g := agentic.NewAbortGroup()
	g.AbortAll()

	ac := agentic.NewAbortController()
	g.Add(ac)
	assert.True(t, ac.IsAborted(), "controller added after abort should be immediately aborted")
}

// --- Concurrent access ---

func TestAbortController_ConcurrentAbort(t *testing.T) {
	ac := agentic.NewAbortController()
	var count int32
	for i := 0; i < 10; i++ {
		ac.OnAbort(func() { atomic.AddInt32(&count, 1) })
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ac.Abort()
		}()
	}
	wg.Wait()

	assert.True(t, ac.IsAborted())
	assert.Equal(t, int32(10), count, "each listener should fire exactly once")
}
