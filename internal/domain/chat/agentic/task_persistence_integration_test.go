//go:build integration

package agentic_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/task"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

const wp03IntegrationTenant = "wp03integration"

func wp03MigrationsDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs("../../../../migrations/schemas")
	require.NoError(t, err)
	require.DirExists(t, dir)
	return dir
}

func wp03MigrationFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(wp03MigrationsDir(t))
	require.NoError(t, err)

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	require.NotEmpty(t, files)
	return files
}

func setupWP03IntegrationTenant(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()

	pool := testutil.NewPostgresContainer(t)
	ctx := context.Background()
	schema := "ah_" + wp03IntegrationTenant
	testutil.MustExec(t, pool, "CREATE SCHEMA IF NOT EXISTS "+schema)

	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	_, err = conn.Exec(ctx, "SET search_path TO "+schema)
	require.NoError(t, err)

	for _, name := range wp03MigrationFiles(t) {
		sql, err := os.ReadFile(filepath.Join(wp03MigrationsDir(t), name))
		require.NoError(t, err, "read migration %s", name)
		_, err = conn.Exec(ctx, string(sql))
		require.NoError(t, err, "apply migration %s", name)
	}

	return pool, tenant.NewContext(ctx, wp03IntegrationTenant)
}

// TestIntegration_WP03_DelegationPersistsAndListsTasks exercises the production
// SessionRunnerAdapter path with a scripted model, real PostgreSQL migrations,
// a fresh database pool after the run, and the public task and notification
// HTTP routes.
func TestIntegration_WP03_DelegationPersistsAndListsTasks(t *testing.T) {
	pool, ctx := setupWP03IntegrationTenant(t)
	chatRepo := chat.NewRepository(pool)
	taskRepo := task.NewRepository(pool)

	session, err := chatRepo.CreateSession(ctx, chat.ChatSession{
		Title:  "WP-03 delegated persistence",
		Status: chat.StatusActive,
	})
	require.NoError(t, err)

	model := &mockChatModel{
		streamFn: func(callIndex int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			switch callIndex {
			case 0:
				return makeToolCallStream("root-delegation", "agent", `{"prompt":"Inspect the persisted task lifecycle"}`), nil
			case 1:
				return makeTextStream("The delegated work completed."), nil
			default:
				return makeTextStream("The parent received the delegated result."), nil
			}
		},
	}
	agentID := uuid.New()
	skills := &mockSkillLister{}
	kbs := &mockKBLister{}
	adapter := agentic.NewSessionRunnerAdapter(
		model,
		nil,
		agentic.NewPromptBuilder(skills, kbs, nil, agentic.DefaultPromptConfig()),
		agentic.NewToolSchemaBuilder(skills, newMockToolsBySkill(), kbs),
		nil,
		nil,
		nil,
		chatRepo,
		&mockAgentConfigLoader{cfg: &chat.AgentRunConfig{
			ID:          agentID,
			Status:      "PUBLISHED",
			ModelConfig: json.RawMessage(`{"provider":"test","model":"scripted"}`),
		}},
		nil,
		nil,
		nil,
		nil,
		nil,
	).WithTaskRepository(taskRepo)

	runEvents, err := adapter.RunSession(ctx, chat.RunInput{
		SessionID:   session.ID,
		AgentID:     agentID,
		UserMessage: "Delegate this work and persist its lifecycle.",
		TenantID:    wp03IntegrationTenant,
	})
	require.NoError(t, err)

	var events []chat.RunEvent
	for event := range runEvents {
		events = append(events, event)
	}

	// Create a fresh pool before reading persisted state. This represents a new
	// API process reconnecting after the delegating adapter has completed.
	restartedPool, err := pgxpool.New(ctx, pool.Config().ConnString())
	require.NoError(t, err)
	t.Cleanup(restartedPool.Close)
	restartedChatRepo := chat.NewRepository(restartedPool)
	restartedTaskRepo := task.NewRepository(restartedPool)

	var started, completed agentic.SubtaskStartData
	for _, event := range events {
		switch event.Type {
		case string(agentic.EventSubtaskStart):
			require.NoError(t, json.Unmarshal(event.Data, &started))
		case string(agentic.EventSubtaskComplete):
			var data agentic.SubtaskCompleteData
			require.NoError(t, json.Unmarshal(event.Data, &data))
			completed = agentic.SubtaskStartData{ID: data.ID}
		}
	}
	require.NotEmpty(t, started.ID, "delegation must emit subtask_start")
	assert.Equal(t, started.ID, completed.ID, "subtask completion must retain the SSE task ID")

	persisted, err := restartedTaskRepo.GetTask(ctx, started.ID)
	require.NoError(t, err)
	assert.Equal(t, session.ID, persisted.SessionID)
	assert.Equal(t, "implementation", persisted.Phase)
	assert.Equal(t, "completed", persisted.Status)
	assert.NotEmpty(t, persisted.AssignedTo)
	require.NotNil(t, persisted.CompletedAt)

	api := chat.NewHandler(chat.NewService(restartedChatRepo, nil), nil).WithTaskRepository(restartedTaskRepo)
	router := chi.NewRouter()
	api.RegisterRoutes(router)

	tasksRequest := httptest.NewRequest(http.MethodGet, "/api/chat/sessions/"+session.ID.String()+"/tasks", nil).WithContext(ctx)
	tasksResponse := httptest.NewRecorder()
	router.ServeHTTP(tasksResponse, tasksRequest)
	require.Equal(t, http.StatusOK, tasksResponse.Code, tasksResponse.Body.String())

	var tasksPage pagination.Page[task.Task]
	require.NoError(t, json.Unmarshal(tasksResponse.Body.Bytes(), &tasksPage))
	require.Equal(t, int64(1), tasksPage.TotalElements)
	require.Len(t, tasksPage.Content, 1)
	assert.Equal(t, started.ID, tasksPage.Content[0].ID)
	assert.Equal(t, "completed", tasksPage.Content[0].Status)

	notificationsRequest := httptest.NewRequest(http.MethodGet, "/api/chat/sessions/"+session.ID.String()+"/tasks/"+started.ID+"/notifications", nil).WithContext(ctx)
	notificationsResponse := httptest.NewRecorder()
	router.ServeHTTP(notificationsResponse, notificationsRequest)
	require.Equal(t, http.StatusOK, notificationsResponse.Code, notificationsResponse.Body.String())

	var notifications []task.Notification
	require.NoError(t, json.Unmarshal(notificationsResponse.Body.Bytes(), &notifications))
	require.Len(t, notifications, 1)
	assert.Equal(t, started.ID, notifications[0].TaskID)
	assert.Equal(t, "completed", notifications[0].Status)
	assert.Contains(t, notifications[0].Summary, "delegated work completed")

	conn, release, err := database.AcquireWithTenant(ctx, restartedPool, wp03IntegrationTenant)
	require.NoError(t, err)
	defer release()
	var taskCount, notificationCount int
	require.NoError(t, conn.QueryRow(ctx, "SELECT COUNT(*) FROM coordinator_task WHERE session_id = $1", session.ID).Scan(&taskCount))
	require.NoError(t, conn.QueryRow(ctx, "SELECT COUNT(*) FROM task_notification WHERE task_id = $1", started.ID).Scan(&notificationCount))
	assert.Equal(t, 1, taskCount)
	assert.Equal(t, 1, notificationCount)
}
