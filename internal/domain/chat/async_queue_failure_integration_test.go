//go:build integration

package chat_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
)

func TestIntegration_AsyncQueueUnavailableMarksRunsFailedAndAllowsRetry(t *testing.T) {
	pool, ctx := setupTenantSchema(t)
	repo := chat.NewRepository(pool)

	session, err := repo.CreateSession(ctx, chat.ChatSession{
		Title:  "queue unavailable",
		Status: chat.StatusActive,
	})
	require.NoError(t, err)

	executor := chat.NewAsyncExecutor(repo, nil, "amqp://guest:guest@127.0.0.1:1/")
	runIDs := make([]uuid.UUID, 0, 2)
	for range 2 {
		runID, enqueueErr := executor.EnqueueRun(ctx, session.ID, provisioningTestTenant, "hello")
		require.ErrorIs(t, enqueueErr, chat.ErrQueueUnavailable)
		require.NotEqual(t, uuid.Nil, runID)
		runIDs = append(runIDs, runID)

		persisted, getErr := repo.GetRunByID(ctx, runID)
		require.NoError(t, getErr)
		assert.Equal(t, chat.ChatRunStatusFailed, persisted.Status)
		require.NotNil(t, persisted.FailureReason)
		assert.Equal(t, "chat queue is temporarily unavailable", *persisted.FailureReason)
	}

	assert.NotEqual(t, runIDs[0], runIDs[1])
	_, active, err := repo.GetActiveRunBySession(ctx, session.ID)
	require.NoError(t, err)
	assert.False(t, active)
}
