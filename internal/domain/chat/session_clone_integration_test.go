//go:build integration

package chat_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
)

func TestIntegration_CloneSession_HTTPPersistsIndependentTranscript(t *testing.T) {
	pool, ctx := setupTenantSchema(t)
	repo := chat.NewRepository(pool)
	svc := chat.NewService(repo, nil)

	prompt := "Keep the original agent snapshot."
	source, err := repo.CreateSession(ctx, chat.ChatSession{
		Title:                 "Original conversation",
		Status:                chat.StatusActive,
		SystemPromptSnapshot:  &prompt,
		ModelConfigSnapshot:   json.RawMessage(`{"provider":"test","model":"scripted"}`),
		SkillBindingsSnapshot: json.RawMessage(`{"skillIds":[]}`),
		AgentSnapshot:         json.RawMessage(`{"systemPrompt":"Keep the original agent snapshot.","modelConfig":{"provider":"test","model":"scripted"}}`),
		AgentSnapshotHash:     stringPtr("snapshot-hash"),
		ConfigHash:            stringPtr("config-hash"),
	})
	require.NoError(t, err)

	first, err := repo.CreateMessage(ctx, chat.ChatMessage{
		SessionID: source.ID, Role: "user", Content: "First branchable message",
		MessageType: chat.MessageTypeText, TurnIndex: 0,
	})
	require.NoError(t, err)
	_, err = repo.CreateMessage(ctx, chat.ChatMessage{
		SessionID: source.ID, Role: "assistant", Content: "Message after the branch point",
		MessageType: chat.MessageTypeText, TurnIndex: 1,
	})
	require.NoError(t, err)

	handler := chat.NewHandler(svc, nil)
	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	body, err := json.Marshal(chat.CloneSessionRequest{
		UntilMessageID: &first.ID,
		Title:          "Alternative path",
	})
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, "/api/chat/sessions/"+source.ID.String()+"/clone", bytes.NewReader(body)).WithContext(ctx)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusCreated, response.Code, response.Body.String())

	var cloned chat.ChatSessionResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &cloned))
	require.NotEqual(t, source.ID, cloned.ID)
	require.NotNil(t, cloned.ClonedFromSessionID)
	assert.Equal(t, source.ID, *cloned.ClonedFromSessionID)
	require.NotNil(t, cloned.ClonedFromSessionTitle)
	assert.Equal(t, source.Title, *cloned.ClonedFromSessionTitle)
	assert.Equal(t, "Alternative path", cloned.Title)

	addBody := bytes.NewBufferString(`{"role":"user","content":"Only the clone receives this message"}`)
	addRequest := httptest.NewRequest(http.MethodPost, "/api/chat/sessions/"+cloned.ID.String()+"/messages", addBody).WithContext(ctx)
	addRequest.Header.Set("Content-Type", "application/json")
	addResponse := httptest.NewRecorder()
	router.ServeHTTP(addResponse, addRequest)
	require.Equal(t, http.StatusCreated, addResponse.Code, addResponse.Body.String())

	// Read through a new pool to ensure the public write and lineage survive a
	// process reconnect, rather than relying on in-process state.
	restartedPool, err := newPool(ctx, pool)
	require.NoError(t, err)
	t.Cleanup(restartedPool.Close)
	restartedRepo := chat.NewRepository(restartedPool)

	persistedClone, err := restartedRepo.GetSessionByID(ctx, cloned.ID)
	require.NoError(t, err)
	require.NotNil(t, persistedClone.SystemPromptSnapshot)
	assert.Equal(t, prompt, *persistedClone.SystemPromptSnapshot)
	assert.JSONEq(t, string(source.ModelConfigSnapshot), string(persistedClone.ModelConfigSnapshot))
	assert.JSONEq(t, string(source.AgentSnapshot), string(persistedClone.AgentSnapshot))
	require.NotNil(t, persistedClone.ConfigHash)
	require.NotNil(t, source.ConfigHash)
	assert.Equal(t, *source.ConfigHash, *persistedClone.ConfigHash)

	originalMessages, err := restartedRepo.FindAllMessages(ctx, source.ID)
	require.NoError(t, err)
	clonedMessages, err := restartedRepo.FindAllMessages(ctx, cloned.ID)
	require.NoError(t, err)
	require.Len(t, originalMessages, 2)
	require.Len(t, clonedMessages, 2)
	assert.Equal(t, "First branchable message", clonedMessages[0].Content)
	assert.NotEqual(t, first.ID, clonedMessages[0].ID)
	assert.Equal(t, "Only the clone receives this message", clonedMessages[1].Content)
}

func TestIntegration_CloneSession_RollsBackWhenTranscriptCopyFails(t *testing.T) {
	pool, ctx := setupTenantSchema(t)
	repo := chat.NewRepository(pool)
	svc := chat.NewService(repo, nil)

	source, err := repo.CreateSession(ctx, chat.ChatSession{
		Title:  "Source for transactional clone",
		Status: chat.StatusActive,
	})
	require.NoError(t, err)
	_, err = repo.CreateMessage(ctx, chat.ChatMessage{
		SessionID: source.ID, Role: "user", Content: "prefix",
		MessageType: chat.MessageTypeText, TurnIndex: 0,
	})
	require.NoError(t, err)
	boundary, err := repo.CreateMessage(ctx, chat.ChatMessage{
		SessionID: source.ID, Role: "assistant", Content: "copy fault",
		MessageType: chat.MessageTypeText, TurnIndex: 1,
	})
	require.NoError(t, err)

	conn, release, err := database.AcquireWithTenant(ctx, pool, provisioningTestTenant)
	require.NoError(t, err)
	defer release()
	_, err = conn.Exec(ctx, fmt.Sprintf(`
		CREATE FUNCTION fail_clone_transcript_copy() RETURNS trigger AS $$
		BEGIN
			IF NEW.session_id <> '%s'::uuid AND NEW.content = 'copy fault' THEN
				RAISE EXCEPTION 'forced clone transcript failure';
			END IF;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER fail_clone_transcript_copy
		BEFORE INSERT ON chat_message
		FOR EACH ROW EXECUTE FUNCTION fail_clone_transcript_copy();
	`, source.ID))
	require.NoError(t, err)

	_, err = svc.CloneSession(ctx, source.ID, chat.CloneSessionRequest{UntilMessageID: &boundary.ID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forced clone transcript failure")

	var sessionCount, messageCount int
	require.NoError(t, conn.QueryRow(ctx, "SELECT COUNT(*) FROM chat_session").Scan(&sessionCount))
	require.NoError(t, conn.QueryRow(ctx, "SELECT COUNT(*) FROM chat_message").Scan(&messageCount))
	assert.Equal(t, 1, sessionCount, "failed clone must not leave a child session")
	assert.Equal(t, 2, messageCount, "failed clone must not leave a partial copied transcript")
}

func stringPtr(value string) *string {
	return &value
}

func newPool(ctx context.Context, pool *pgxpool.Pool) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, pool.Config().ConnString())
}
