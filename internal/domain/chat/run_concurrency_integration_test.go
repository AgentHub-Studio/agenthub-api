//go:build integration

package chat_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
)

func TestIntegration_CreateRunAllowsOnlyOneActiveRunPerSession(t *testing.T) {
	pool, ctx := setupTenantSchema(t)
	repo := chat.NewRepository(pool)

	session, err := repo.CreateSession(ctx, chat.ChatSession{
		Title:  "concurrent run lock",
		Status: chat.StatusActive,
	})
	require.NoError(t, err)

	const contenders = 8
	start := make(chan struct{})
	errs := make(chan error, contenders)
	var wg sync.WaitGroup
	for range contenders {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, createErr := repo.CreateRun(ctx, chat.ChatRun{
				SessionID: session.ID,
				TenantID:  provisioningTestTenant,
				Status:    chat.ChatRunStatusActive,
			})
			errs <- createErr
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	var successful, alreadyActive int
	for createErr := range errs {
		switch {
		case createErr == nil:
			successful++
		case errors.Is(createErr, chat.ErrRunAlreadyActive):
			alreadyActive++
		default:
			require.NoError(t, createErr)
		}
	}
	assert.Equal(t, 1, successful)
	assert.Equal(t, contenders-1, alreadyActive)

	active, found, err := repo.GetActiveRunBySession(ctx, session.ID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, session.ID, active.SessionID)
	assert.Equal(t, chat.ChatRunStatusActive, active.Status)
}
