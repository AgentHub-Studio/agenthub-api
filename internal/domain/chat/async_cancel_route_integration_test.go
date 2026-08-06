//go:build integration

package chat_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

func TestIntegration_CancelQueuedRunRoutePersistsCancellation(t *testing.T) {
	pool, ctx := setupTenantSchema(t)
	repo := chat.NewRepository(pool)

	session, err := repo.CreateSession(ctx, chat.ChatSession{
		Title:  "queued run cancellation",
		Status: chat.StatusActive,
	})
	require.NoError(t, err)
	run, err := repo.CreateRun(ctx, chat.ChatRun{
		SessionID: session.ID,
		TenantID:  provisioningTestTenant,
		Status:    chat.ChatRunStatusQueued,
	})
	require.NoError(t, err)

	router := chi.NewRouter()
	executor := chat.NewAsyncExecutor(repo, nil, "")
	chat.NewHandler(nil, executor).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost,
		"/api/chat/sessions/"+session.ID.String()+"/run/"+run.ID.String()+"/cancel", nil)
	req = req.WithContext(tenant.NewContext(req.Context(), provisioningTestTenant))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"status":"cancelled"}`, rec.Body.String())

	persisted, err := repo.GetRunByID(ctx, run.ID)
	require.NoError(t, err)
	assert.Equal(t, chat.ChatRunStatusCancelled, persisted.Status)
}
