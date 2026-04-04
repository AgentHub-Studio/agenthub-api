package agentic_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- Constants ---

func TestPermissionStatus_Values(t *testing.T) {
	assert.Equal(t, agentic.PermissionStatus("pending"), agentic.PermissionPending)
	assert.Equal(t, agentic.PermissionStatus("approved"), agentic.PermissionApproved)
	assert.Equal(t, agentic.PermissionStatus("rejected"), agentic.PermissionRejected)
}

func TestPermissionResolvedBy_Values(t *testing.T) {
	assert.Equal(t, agentic.PermissionResolvedBy("worker"), agentic.ResolvedByWorker)
	assert.Equal(t, agentic.PermissionResolvedBy("leader"), agentic.ResolvedByLeader)
}

func TestDefaultPermissionMaxAge(t *testing.T) {
	assert.Equal(t, time.Hour, agentic.DefaultPermissionMaxAge)
}

// --- NewPermissionRegistry ---

func TestNewPermissionRegistry(t *testing.T) {
	r := agentic.NewPermissionRegistry()
	assert.Equal(t, 0, r.PendingCount())
	assert.Equal(t, 0, r.ResolvedCount())
}

// --- GenerateRequestID ---

func TestPermissionRegistry_GenerateRequestID(t *testing.T) {
	r := agentic.NewPermissionRegistry()
	id1 := r.GenerateRequestID()
	id2 := r.GenerateRequestID()
	assert.Contains(t, id1, "perm-")
	assert.NotEqual(t, id1, id2)
}

// --- Submit ---

func TestPermissionRegistry_Submit(t *testing.T) {
	r := agentic.NewPermissionRegistry()
	req := r.Submit(agentic.PermissionRequest{
		WorkerID:   "w-1",
		WorkerName: "worker-1",
		ToolName:   "Bash",
		Description: "run tests",
	})

	assert.NotEmpty(t, req.ID)
	assert.Equal(t, agentic.PermissionPending, req.Status)
	assert.False(t, req.CreatedAt.IsZero())
	assert.Equal(t, 1, r.PendingCount())
}

func TestPermissionRegistry_Submit_WithID(t *testing.T) {
	r := agentic.NewPermissionRegistry()
	req := r.Submit(agentic.PermissionRequest{
		ID:       "custom-id",
		ToolName: "Read",
	})
	assert.Equal(t, "custom-id", req.ID)
}

// --- Resolve ---

func TestPermissionRegistry_Resolve_Approve(t *testing.T) {
	r := agentic.NewPermissionRegistry()
	req := r.Submit(agentic.PermissionRequest{ToolName: "Bash"})

	ok := r.Resolve(req.ID, agentic.PermissionResolution{
		Decision:   agentic.PermissionApproved,
		ResolvedBy: agentic.ResolvedByLeader,
	})
	assert.True(t, ok)
	assert.Equal(t, 0, r.PendingCount())
	assert.Equal(t, 1, r.ResolvedCount())
}

func TestPermissionRegistry_Resolve_Reject(t *testing.T) {
	r := agentic.NewPermissionRegistry()
	req := r.Submit(agentic.PermissionRequest{ToolName: "Write"})

	ok := r.Resolve(req.ID, agentic.PermissionResolution{
		Decision:   agentic.PermissionRejected,
		ResolvedBy: agentic.ResolvedByLeader,
		Feedback:   "too dangerous",
	})
	assert.True(t, ok)

	resolved := r.GetResolved(req.ID)
	require.NotNil(t, resolved)
	assert.Equal(t, agentic.PermissionRejected, resolved.Status)
	assert.Equal(t, "too dangerous", resolved.Feedback)
	assert.NotNil(t, resolved.ResolvedAt)
}

func TestPermissionRegistry_Resolve_NotFound(t *testing.T) {
	r := agentic.NewPermissionRegistry()
	ok := r.Resolve("nonexistent", agentic.PermissionResolution{
		Decision: agentic.PermissionApproved,
	})
	assert.False(t, ok)
}

func TestPermissionRegistry_Resolve_WithUpdatedInput(t *testing.T) {
	r := agentic.NewPermissionRegistry()
	req := r.Submit(agentic.PermissionRequest{
		ToolName: "Bash",
		Input:    map[string]interface{}{"command": "rm -rf /"},
	})

	r.Resolve(req.ID, agentic.PermissionResolution{
		Decision:     agentic.PermissionApproved,
		ResolvedBy:   agentic.ResolvedByLeader,
		UpdatedInput: map[string]interface{}{"command": "rm -rf /tmp/test"},
	})

	resolved := r.GetResolved(req.ID)
	assert.Equal(t, "rm -rf /tmp/test", resolved.UpdatedInput["command"])
}

// --- GetPending ---

func TestPermissionRegistry_GetPending_Sorted(t *testing.T) {
	r := agentic.NewPermissionRegistry()

	r.Submit(agentic.PermissionRequest{
		ID: "second", ToolName: "B",
		CreatedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
	})
	r.Submit(agentic.PermissionRequest{
		ID: "first", ToolName: "A",
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	})

	pending := r.GetPending()
	require.Len(t, pending, 2)
	assert.Equal(t, "first", pending[0].ID)
	assert.Equal(t, "second", pending[1].ID)
}

func TestPermissionRegistry_GetPending_Empty(t *testing.T) {
	r := agentic.NewPermissionRegistry()
	assert.Empty(t, r.GetPending())
}

// --- GetPendingByTeam ---

func TestPermissionRegistry_GetPendingByTeam(t *testing.T) {
	r := agentic.NewPermissionRegistry()
	r.Submit(agentic.PermissionRequest{ToolName: "A", TeamName: "alpha"})
	r.Submit(agentic.PermissionRequest{ToolName: "B", TeamName: "beta"})
	r.Submit(agentic.PermissionRequest{ToolName: "C", TeamName: "alpha"})

	alpha := r.GetPendingByTeam("alpha")
	assert.Len(t, alpha, 2)

	beta := r.GetPendingByTeam("beta")
	assert.Len(t, beta, 1)
}

// --- GetResolved ---

func TestPermissionRegistry_GetResolved_NotFound(t *testing.T) {
	r := agentic.NewPermissionRegistry()
	assert.Nil(t, r.GetResolved("nope"))
}

func TestPermissionRegistry_GetResolved_ReturnsCopy(t *testing.T) {
	r := agentic.NewPermissionRegistry()
	req := r.Submit(agentic.PermissionRequest{ToolName: "Bash"})
	r.Resolve(req.ID, agentic.PermissionResolution{
		Decision: agentic.PermissionApproved,
	})

	resolved := r.GetResolved(req.ID)
	resolved.Feedback = "modified"

	original := r.GetResolved(req.ID)
	assert.Empty(t, original.Feedback, "should return a copy")
}

// --- DeleteResolved ---

func TestPermissionRegistry_DeleteResolved(t *testing.T) {
	r := agentic.NewPermissionRegistry()
	req := r.Submit(agentic.PermissionRequest{ToolName: "Bash"})
	r.Resolve(req.ID, agentic.PermissionResolution{Decision: agentic.PermissionApproved})

	ok := r.DeleteResolved(req.ID)
	assert.True(t, ok)
	assert.Equal(t, 0, r.ResolvedCount())
}

func TestPermissionRegistry_DeleteResolved_NotFound(t *testing.T) {
	r := agentic.NewPermissionRegistry()
	assert.False(t, r.DeleteResolved("nope"))
}

// --- CleanupOldResolutions ---

func TestPermissionRegistry_CleanupOldResolutions(t *testing.T) {
	r := agentic.NewPermissionRegistryWithMaxAge(50 * time.Millisecond)

	req := r.Submit(agentic.PermissionRequest{ToolName: "Bash"})
	r.Resolve(req.ID, agentic.PermissionResolution{Decision: agentic.PermissionApproved})

	time.Sleep(100 * time.Millisecond)

	cleaned := r.CleanupOldResolutions()
	assert.Equal(t, 1, cleaned)
	assert.Equal(t, 0, r.ResolvedCount())
}

func TestPermissionRegistry_CleanupOldResolutions_KeepsRecent(t *testing.T) {
	r := agentic.NewPermissionRegistryWithMaxAge(time.Hour)

	req := r.Submit(agentic.PermissionRequest{ToolName: "Bash"})
	r.Resolve(req.ID, agentic.PermissionResolution{Decision: agentic.PermissionApproved})

	cleaned := r.CleanupOldResolutions()
	assert.Equal(t, 0, cleaned)
	assert.Equal(t, 1, r.ResolvedCount())
}

// --- Clear ---

func TestPermissionRegistry_Clear(t *testing.T) {
	r := agentic.NewPermissionRegistry()
	r.Submit(agentic.PermissionRequest{ToolName: "A"})
	r.Submit(agentic.PermissionRequest{ToolName: "B"})

	req := r.Submit(agentic.PermissionRequest{ToolName: "C"})
	r.Resolve(req.ID, agentic.PermissionResolution{Decision: agentic.PermissionApproved})

	r.Clear()
	assert.Equal(t, 0, r.PendingCount())
	assert.Equal(t, 0, r.ResolvedCount())
}

// --- Concurrent access ---

func TestPermissionRegistry_ConcurrentAccess(t *testing.T) {
	r := agentic.NewPermissionRegistry()
	var wg sync.WaitGroup

	// Submit 50 concurrent requests.
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.Submit(agentic.PermissionRequest{ToolName: "Bash"})
		}()
	}
	wg.Wait()

	assert.Equal(t, 50, r.PendingCount())

	// Resolve them all concurrently.
	pending := r.GetPending()
	for _, req := range pending {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			r.Resolve(id, agentic.PermissionResolution{
				Decision: agentic.PermissionApproved,
			})
		}(req.ID)
	}
	wg.Wait()

	assert.Equal(t, 0, r.PendingCount())
	assert.Equal(t, 50, r.ResolvedCount())
}

// --- Full lifecycle ---

func TestPermissionRegistry_FullLifecycle(t *testing.T) {
	r := agentic.NewPermissionRegistry()

	// Worker submits request.
	req := r.Submit(agentic.PermissionRequest{
		WorkerID:    "w-1",
		WorkerName:  "worker-1",
		TeamName:    "team-alpha",
		ToolName:    "Bash",
		Description: "execute test suite",
		Input:       map[string]interface{}{"command": "go test ./..."},
	})

	// Leader sees pending.
	pending := r.GetPendingByTeam("team-alpha")
	require.Len(t, pending, 1)
	assert.Equal(t, "Bash", pending[0].ToolName)

	// Leader approves.
	r.Resolve(req.ID, agentic.PermissionResolution{
		Decision:   agentic.PermissionApproved,
		ResolvedBy: agentic.ResolvedByLeader,
	})

	// Worker polls for result.
	resolved := r.GetResolved(req.ID)
	require.NotNil(t, resolved)
	assert.Equal(t, agentic.PermissionApproved, resolved.Status)
	assert.Equal(t, agentic.ResolvedByLeader, resolved.ResolvedBy)

	// Worker cleans up.
	r.DeleteResolved(req.ID)
	assert.Nil(t, r.GetResolved(req.ID))
}
