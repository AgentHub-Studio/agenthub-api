package chat_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
)

func TestBackgroundRunRegistry_RegisterAndGet(t *testing.T) {
	reg := chat.NewBackgroundRunRegistry(1 * time.Minute)
	sessionID := uuid.New()

	runID, ctx := reg.Register(sessionID)
	require.NotEmpty(t, runID)
	require.NotNil(t, ctx)

	run := reg.Get(runID)
	require.NotNil(t, run)
	assert.Equal(t, runID, run.RunID)
	assert.Equal(t, sessionID, run.SessionID)
	assert.Equal(t, chat.RunStatusActive, run.Status)
}

func TestBackgroundRunRegistry_GetNonexistent(t *testing.T) {
	reg := chat.NewBackgroundRunRegistry(1 * time.Minute)
	assert.Nil(t, reg.Get("nonexistent"))
}

func TestBackgroundRunRegistry_GetBySession(t *testing.T) {
	reg := chat.NewBackgroundRunRegistry(1 * time.Minute)
	sessionID := uuid.New()

	runID, _ := reg.Register(sessionID)

	found := reg.GetBySession(sessionID)
	require.NotNil(t, found)
	assert.Equal(t, runID, found.RunID)

	// No active run for random session.
	assert.Nil(t, reg.GetBySession(uuid.New()))
}

func TestBackgroundRunRegistry_MarkCompleted(t *testing.T) {
	reg := chat.NewBackgroundRunRegistry(1 * time.Minute)
	sessionID := uuid.New()

	runID, _ := reg.Register(sessionID)
	reg.MarkCompleted(runID)

	run := reg.Get(runID)
	require.NotNil(t, run)
	assert.Equal(t, chat.RunStatusCompleted, run.Status)
	assert.False(t, run.CompletedAt.IsZero())

	// GetBySession should not return completed runs.
	assert.Nil(t, reg.GetBySession(sessionID))
}

func TestBackgroundRunRegistry_Cancel(t *testing.T) {
	reg := chat.NewBackgroundRunRegistry(1 * time.Minute)
	sessionID := uuid.New()

	runID, ctx := reg.Register(sessionID)

	reg.Cancel(runID)

	// Context should be cancelled.
	assert.Error(t, ctx.Err())

	run := reg.Get(runID)
	require.NotNil(t, run)
	assert.Equal(t, chat.RunStatusCancelled, run.Status)
}

func TestBackgroundRunRegistry_MarkCompletedDoesNotOverrideCancelled(t *testing.T) {
	reg := chat.NewBackgroundRunRegistry(1 * time.Minute)
	runID, _ := reg.Register(uuid.New())

	reg.Cancel(runID)
	reg.MarkCompleted(runID)

	run := reg.Get(runID)
	require.NotNil(t, run)
	assert.Equal(t, chat.RunStatusCancelled, run.Status)
}

func TestBackgroundRunRegistry_CancelNonexistent(t *testing.T) {
	reg := chat.NewBackgroundRunRegistry(1 * time.Minute)
	// Should not panic.
	reg.Cancel("nonexistent")
}

func TestBackgroundRunRegistry_CancelAlreadyCompleted(t *testing.T) {
	reg := chat.NewBackgroundRunRegistry(1 * time.Minute)
	runID, _ := reg.Register(uuid.New())

	reg.MarkCompleted(runID)
	// Cancel on completed run should be a no-op.
	reg.Cancel(runID)

	run := reg.Get(runID)
	assert.Equal(t, chat.RunStatusCompleted, run.Status)
}

func TestBackgroundRunRegistry_ActiveRuns(t *testing.T) {
	reg := chat.NewBackgroundRunRegistry(1 * time.Minute)

	assert.Equal(t, 0, reg.ActiveRuns())

	runID1, _ := reg.Register(uuid.New())
	reg.Register(uuid.New())
	assert.Equal(t, 2, reg.ActiveRuns())

	reg.MarkCompleted(runID1)
	assert.Equal(t, 1, reg.ActiveRuns())
}

func TestBackgroundRunRegistry_AttachAndGetEvents(t *testing.T) {
	reg := chat.NewBackgroundRunRegistry(1 * time.Minute)
	runID, _ := reg.Register(uuid.New())

	// No events before attach.
	assert.Nil(t, reg.Events(runID))
	assert.Nil(t, reg.Events("nonexistent"))

	ch := make(chan chat.RunEvent, 1)
	reg.AttachEvents(runID, ch)

	assert.NotNil(t, reg.Events(runID))
}

func TestBackgroundRunRegistry_DefaultTTL(t *testing.T) {
	reg := chat.NewBackgroundRunRegistry(0)
	require.NotNil(t, reg)
}

func TestBackgroundRunRegistry_MultipleSessionRuns(t *testing.T) {
	reg := chat.NewBackgroundRunRegistry(1 * time.Minute)
	sessionID := uuid.New()

	runID1, _ := reg.Register(sessionID)
	reg.MarkCompleted(runID1)

	runID2, _ := reg.Register(sessionID)

	// GetBySession returns the active one.
	found := reg.GetBySession(sessionID)
	require.NotNil(t, found)
	assert.Equal(t, runID2, found.RunID)
}
