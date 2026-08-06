//go:build integration

package workflow_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/workflow"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
)

func TestIntegration_WorkflowHTTPCanonicalTransitionAndResumeContract(t *testing.T) {
	pool, ctx := setupWorkflowResumeSchema(t)
	service := workflow.NewService(workflow.NewRepository(pool))
	router := workflowAdminRouter(service)

	created := workflowRequest(t, router, ctx, http.MethodPost, "/api/workflows", `{
"name":"HTTP canonical transitions",
"slug":"http-canonical-transitions",
"start":" entry ",
"steps":[
  {"id":" entry ","type":"agent","next":" approval "},
  {"id":" approval ","type":"branch","onTrue":" approve ","onFalse":" finish "},
  {"id":" approve ","type":"tool"},
  {"id":" finish ","type":"tool"}
]}`)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	var createdWorkflow workflow.Workflow
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &createdWorkflow))
	require.Equal(t, "entry", createdWorkflow.Start)
	require.Equal(t, "approval", createdWorkflow.Steps[0].Next)
	require.Equal(t, "approve", createdWorkflow.Steps[1].OnTrue)
	require.Equal(t, "finish", createdWorkflow.Steps[1].OnFalse)

	executed := workflowRequest(t, router, ctx, http.MethodPost, "/api/workflows/http-canonical-transitions/execute", `{"input":{"amount":10000}}`)
	require.Equal(t, http.StatusAccepted, executed.Code, executed.Body.String())
	var execution workflow.Execution
	require.NoError(t, json.Unmarshal(executed.Body.Bytes(), &execution))
	require.Equal(t, workflow.ExecutionStateSuspended, execution.State)
	require.NotNil(t, execution.CurrentStepID)
	require.Equal(t, "approval", *execution.CurrentStepID)

	fetched := workflowRequest(t, router, ctx, http.MethodGet, "/api/workflows/executions/"+execution.ID.String(), "")
	require.Equal(t, http.StatusOK, fetched.Code, fetched.Body.String())
	var persisted workflow.Execution
	require.NoError(t, json.Unmarshal(fetched.Body.Bytes(), &persisted))
	require.Equal(t, workflow.ExecutionStateSuspended, persisted.State)
	require.Equal(t, "approval", *persisted.CurrentStepID)

	resumed := workflowRequest(t, router, ctx, http.MethodPost, "/api/workflows/executions/"+execution.ID.String()+"/resume", `{"decision":"approve"}`)
	require.Equal(t, http.StatusOK, resumed.Code, resumed.Body.String())
	var completed workflow.Execution
	require.NoError(t, json.Unmarshal(resumed.Body.Bytes(), &completed))
	require.Equal(t, workflow.ExecutionStateCompleted, completed.State)
	require.Equal(t, "approve", completed.ResumeData["decision"])

	secondResume := workflowRequest(t, router, ctx, http.MethodPost, "/api/workflows/executions/"+execution.ID.String()+"/resume", `{"decision":"deny"}`)
	require.Equal(t, http.StatusConflict, secondResume.Code, secondResume.Body.String())
	require.Contains(t, secondResume.Body.String(), "workflow execution already resolved")
}

func workflowAdminRouter(service workflow.Service) http.Handler {
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(middleware.ContextWithRoles(r.Context(), "admin")))
		})
	})
	workflow.NewHandler(service).RegisterRoutes(router)
	return router
}

func workflowRequest(t *testing.T, router http.Handler, ctx context.Context, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(ctx)
	router.ServeHTTP(recorder, request)
	return recorder
}
