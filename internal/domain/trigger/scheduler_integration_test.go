//go:build integration

package trigger

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

type schedulerIntegrationCron struct{}

func (schedulerIntegrationCron) Validate(string) error { return nil }

func (schedulerIntegrationCron) NextRun(_ string, from time.Time) (time.Time, error) {
	return from.Add(time.Minute), nil
}

type schedulerIntegrationFirer struct {
	sessionID uuid.UUID
	runID     uuid.UUID
	sessions  []schedulerSessionCall
	runs      []schedulerRunCall
}

type schedulerSessionCall struct {
	tenantID string
	agentID  uuid.UUID
	title    string
}

type schedulerRunCall struct {
	tenantID  string
	sessionID uuid.UUID
	message   string
}

func (f *schedulerIntegrationFirer) CreateSessionForTrigger(_ context.Context, tenantID string, agentID uuid.UUID, title string) (uuid.UUID, error) {
	f.sessions = append(f.sessions, schedulerSessionCall{tenantID: tenantID, agentID: agentID, title: title})
	return f.sessionID, nil
}

func (f *schedulerIntegrationFirer) EnqueueRun(_ context.Context, sessionID uuid.UUID, tenantID, message string) (uuid.UUID, error) {
	f.runs = append(f.runs, schedulerRunCall{tenantID: tenantID, sessionID: sessionID, message: message})
	return f.runID, nil
}

func TestIntegration_Scheduler_FiresDueTriggerAndPersistsRun(t *testing.T) {
	ctx := context.Background()
	pool := testutil.NewPostgresContainer(t)
	tenantID := "scheduler" + strings.ReplaceAll(uuid.NewString(), "-", "")
	schema := "ah_" + tenantID
	testutil.CreateTenantSchema(t, pool, tenantID)

	testutil.MustExec(t, pool, fmt.Sprintf(`
		CREATE TABLE %s.agent_trigger (
			id UUID PRIMARY KEY,
			agent_id UUID NOT NULL,
			name VARCHAR(255) NOT NULL,
			cron_expression VARCHAR(255) NOT NULL,
			enabled BOOLEAN NOT NULL DEFAULT TRUE,
			input_template JSONB NOT NULL DEFAULT '{}'::jsonb,
			last_run_at TIMESTAMPTZ,
			next_run_at TIMESTAMPTZ,
			run_count INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE %s.agent_trigger_run (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			trigger_id UUID NOT NULL,
			session_id UUID NOT NULL,
			status VARCHAR(50) NOT NULL,
			started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			completed_at TIMESTAMPTZ,
			total_turns INTEGER,
			total_tokens INTEGER,
			error TEXT
		);
	`, schema, schema))

	triggerID := uuid.New()
	agentID := uuid.New()
	testutil.MustExec(t, pool, fmt.Sprintf(`
		INSERT INTO %s.agent_trigger (id, agent_id, name, cron_expression, enabled, input_template, next_run_at)
		VALUES ($1, $2, 'daily-digest', '* * * * *', true, '{"message":"send digest"}'::jsonb, NOW() - INTERVAL '1 minute')
	`, schema), triggerID, agentID)

	firer := &schedulerIntegrationFirer{sessionID: uuid.New(), runID: uuid.New()}
	scheduler := NewScheduler(pool, nil, NewRepository(pool), schedulerIntegrationCron{}).WithFirer(firer)
	scheduler.tickTenant(ctx, tenantID)

	require.Equal(t, []schedulerSessionCall{{tenantID: tenantID, agentID: agentID, title: "trigger:daily-digest"}}, firer.sessions)
	require.Equal(t, []schedulerRunCall{{tenantID: tenantID, sessionID: firer.sessionID, message: "send digest"}}, firer.runs)

	var runCount int
	var lastRunAt, nextRunAt time.Time
	err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT run_count, last_run_at, next_run_at FROM %s.agent_trigger WHERE id = $1`, schema), triggerID).
		Scan(&runCount, &lastRunAt, &nextRunAt)
	require.NoError(t, err)
	require.Equal(t, 1, runCount)
	require.False(t, lastRunAt.IsZero())
	require.True(t, nextRunAt.After(lastRunAt))

	var persistedRun AgentTriggerRun
	err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT %s FROM %s.agent_trigger_run WHERE trigger_id = $1`, runColumns, schema), triggerID).
		Scan(&persistedRun.ID, &persistedRun.TriggerID, &persistedRun.SessionID, &persistedRun.Status, &persistedRun.StartedAt,
			&persistedRun.CompletedAt, &persistedRun.TotalTurns, &persistedRun.TotalTokens, &persistedRun.Error)
	require.NoError(t, err)
	require.Equal(t, firer.sessionID, persistedRun.SessionID)
	require.Equal(t, RunStatusRunning, persistedRun.Status)

	// Exercise the user-visible history route against the same run that the
	// scheduler just persisted, rather than relying on a separately seeded row.
	router := chi.NewRouter()
	NewHandler(NewService(NewRepository(pool), nil)).RegisterRoutes(router)
	adminCtx := middleware.ContextWithRoles(tenant.NewContext(ctx, tenantID), "admin")
	request := httptest.NewRequest(http.MethodGet,
		"/api/agents/"+agentID.String()+"/triggers/"+triggerID.String()+"/runs?page=0&size=10", nil).
		WithContext(adminCtx)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)

	var history pagination.Page[AgentTriggerRun]
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &history))
	require.Equal(t, int64(1), history.TotalElements)
	require.Len(t, history.Content, 1)
	assert.Equal(t, persistedRun.ID, history.Content[0].ID)
	assert.Equal(t, firer.sessionID, history.Content[0].SessionID)
	assert.Equal(t, RunStatusRunning, history.Content[0].Status)

	completedTurns, completedTokens := 3, 18
	require.NoError(t, NewRepository(pool).CompleteRunBySession(
		tenant.NewContext(ctx, tenantID),
		firer.sessionID,
		RunStatusCompleted,
		&completedTurns,
		&completedTokens,
		nil,
	))

	completedRequest := httptest.NewRequest(http.MethodGet,
		"/api/agents/"+agentID.String()+"/triggers/"+triggerID.String()+"/runs?page=0&size=10", nil).
		WithContext(adminCtx)
	completedResponse := httptest.NewRecorder()
	router.ServeHTTP(completedResponse, completedRequest)
	require.Equal(t, http.StatusOK, completedResponse.Code)
	var completedHistory pagination.Page[AgentTriggerRun]
	require.NoError(t, json.Unmarshal(completedResponse.Body.Bytes(), &completedHistory))
	require.Len(t, completedHistory.Content, 1)
	assert.Equal(t, RunStatusCompleted, completedHistory.Content[0].Status)
	require.NotNil(t, completedHistory.Content[0].TotalTurns)
	require.NotNil(t, completedHistory.Content[0].TotalTokens)
	assert.Equal(t, completedTurns, *completedHistory.Content[0].TotalTurns)
	assert.Equal(t, completedTokens, *completedHistory.Content[0].TotalTokens)

	// MarkRun moves the trigger forward, so a second tick does not enqueue it again.
	scheduler.tickTenant(ctx, tenantID)
	require.Len(t, firer.sessions, 1)
	require.Len(t, firer.runs, 1)
}
