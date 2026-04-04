package agentic_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- WithTeammateContext / GetTeammateContext ---

func TestTeammateContext_SetGet(t *testing.T) {
	tc := &agentic.TeammateContext{
		AgentID:   "agent-1",
		AgentName: "Coder",
		TeamName:  "backend",
		IsInProcess: true,
	}

	ctx := agentic.WithTeammateContext(context.Background(), tc)
	got := agentic.GetTeammateContext(ctx)
	require.NotNil(t, got)
	assert.Equal(t, "agent-1", got.AgentID)
	assert.Equal(t, "Coder", got.AgentName)
	assert.Equal(t, "backend", got.TeamName)
	assert.True(t, got.IsInProcess)
}

func TestTeammateContext_GetFromEmptyContext(t *testing.T) {
	got := agentic.GetTeammateContext(context.Background())
	assert.Nil(t, got)
}

// --- IsInProcessTeammate ---

func TestIsInProcessTeammate_True(t *testing.T) {
	tc := &agentic.TeammateContext{AgentID: "a", IsInProcess: true}
	ctx := agentic.WithTeammateContext(context.Background(), tc)
	assert.True(t, agentic.IsInProcessTeammate(ctx))
}

func TestIsInProcessTeammate_False_NotInProcess(t *testing.T) {
	tc := &agentic.TeammateContext{AgentID: "a", IsInProcess: false}
	ctx := agentic.WithTeammateContext(context.Background(), tc)
	assert.False(t, agentic.IsInProcessTeammate(ctx))
}

func TestIsInProcessTeammate_False_NoContext(t *testing.T) {
	assert.False(t, agentic.IsInProcessTeammate(context.Background()))
}

// --- RunWithTeammateContext ---

func TestRunWithTeammateContext(t *testing.T) {
	tc := &agentic.TeammateContext{AgentID: "runner", AgentName: "Runner", IsInProcess: true}

	result, err := agentic.RunWithTeammateContext(context.Background(), tc, func(ctx context.Context) (string, error) {
		got := agentic.GetTeammateContext(ctx)
		return got.AgentName, nil
	})

	require.NoError(t, err)
	assert.Equal(t, "Runner", result)
}

// --- TeammateRegistry ---

func TestTeammateRegistry_RegisterGet(t *testing.T) {
	r := agentic.NewTeammateRegistry()
	tc := &agentic.TeammateContext{AgentID: "a1", AgentName: "Agent1", TeamName: "team-x"}

	r.Register(tc)
	got := r.Get("a1")
	require.NotNil(t, got)
	assert.Equal(t, "Agent1", got.AgentName)
}

func TestTeammateRegistry_Get_NotFound(t *testing.T) {
	r := agentic.NewTeammateRegistry()
	assert.Nil(t, r.Get("nonexistent"))
}

func TestTeammateRegistry_Get_ReturnsCopy(t *testing.T) {
	r := agentic.NewTeammateRegistry()
	tc := &agentic.TeammateContext{AgentID: "a1", AgentName: "Original"}
	r.Register(tc)

	got := r.Get("a1")
	got.AgentName = "Modified"

	original := r.Get("a1")
	assert.Equal(t, "Original", original.AgentName)
}

func TestTeammateRegistry_Unregister(t *testing.T) {
	r := agentic.NewTeammateRegistry()
	r.Register(&agentic.TeammateContext{AgentID: "a1"})
	assert.Equal(t, 1, r.Count())

	r.Unregister("a1")
	assert.Equal(t, 0, r.Count())
	assert.Nil(t, r.Get("a1"))
}

func TestTeammateRegistry_GetByTeam(t *testing.T) {
	r := agentic.NewTeammateRegistry()
	r.Register(&agentic.TeammateContext{AgentID: "a1", TeamName: "backend"})
	r.Register(&agentic.TeammateContext{AgentID: "a2", TeamName: "backend"})
	r.Register(&agentic.TeammateContext{AgentID: "a3", TeamName: "frontend"})

	backend := r.GetByTeam("backend")
	assert.Len(t, backend, 2)

	frontend := r.GetByTeam("frontend")
	assert.Len(t, frontend, 1)

	empty := r.GetByTeam("devops")
	assert.Empty(t, empty)
}

func TestTeammateRegistry_Count(t *testing.T) {
	r := agentic.NewTeammateRegistry()
	assert.Equal(t, 0, r.Count())

	r.Register(&agentic.TeammateContext{AgentID: "a1"})
	r.Register(&agentic.TeammateContext{AgentID: "a2"})
	assert.Equal(t, 2, r.Count())
}

func TestTeammateRegistry_All(t *testing.T) {
	r := agentic.NewTeammateRegistry()
	r.Register(&agentic.TeammateContext{AgentID: "a1"})
	r.Register(&agentic.TeammateContext{AgentID: "a2"})

	all := r.All()
	assert.Len(t, all, 2)
}

func TestTeammateRegistry_All_ReturnsCopies(t *testing.T) {
	r := agentic.NewTeammateRegistry()
	r.Register(&agentic.TeammateContext{AgentID: "a1", AgentName: "Original"})

	all := r.All()
	all[0].AgentName = "Modified"

	got := r.Get("a1")
	assert.Equal(t, "Original", got.AgentName)
}

func TestTeammateRegistry_Clear(t *testing.T) {
	r := agentic.NewTeammateRegistry()
	r.Register(&agentic.TeammateContext{AgentID: "a1"})
	r.Register(&agentic.TeammateContext{AgentID: "a2"})
	assert.Equal(t, 2, r.Count())

	r.Clear()
	assert.Equal(t, 0, r.Count())
}

// --- Concurrent access ---

func TestTeammateRegistry_ConcurrentAccess(t *testing.T) {
	r := agentic.NewTeammateRegistry()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			tc := &agentic.TeammateContext{
				AgentID:   fmt.Sprintf("agent-%d", id),
				AgentName: fmt.Sprintf("Agent %d", id),
				TeamName:  "team",
			}
			r.Register(tc)
			r.Get(tc.AgentID)
			r.GetByTeam("team")
			r.Count()
			r.All()
		}(i)
	}
	wg.Wait()

	assert.Equal(t, 50, r.Count())
}

// --- Optional fields ---

func TestTeammateContext_OptionalFields(t *testing.T) {
	tc := &agentic.TeammateContext{
		AgentID:         "a1",
		AgentName:       "Agent",
		TeamName:        "team",
		Color:           "#FF0000",
		ParentSessionID: "sess-123",
		PlanModeRequired: true,
	}

	ctx := agentic.WithTeammateContext(context.Background(), tc)
	got := agentic.GetTeammateContext(ctx)
	assert.Equal(t, "#FF0000", got.Color)
	assert.Equal(t, "sess-123", got.ParentSessionID)
	assert.True(t, got.PlanModeRequired)
}
