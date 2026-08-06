package workflow

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
)

type stubService struct {
	createFn       func(context.Context, CreateRequest) (Workflow, error)
	getBySlugFn    func(context.Context, string) (Workflow, error)
	executeFn      func(context.Context, string, ExecuteRequest) (Execution, error)
	getExecutionFn func(context.Context, uuid.UUID) (Execution, error)
	resumeFn       func(context.Context, uuid.UUID, ResumeRequest) (Execution, error)
}

func newWorkflowRouter(h *Handler, roles ...string) *chi.Mux {
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := middleware.ContextWithRoles(r.Context(), roles...)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	h.RegisterRoutes(r)
	return r
}

func TestWorkflowHandler_AdministrativeRoutesRequireAdminRole(t *testing.T) {
	r := newWorkflowRouter(NewHandler(nil), "user")
	executionID := uuid.NewString()
	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPost, path: "/api/workflows", body: `{}`},
		{method: http.MethodGet, path: "/api/workflows/executions/" + executionID},
		{method: http.MethodPost, path: "/api/workflows/executions/" + executionID + "/resume", body: `{}`},
		{method: http.MethodGet, path: "/api/workflows/workflow"},
		{method: http.MethodPost, path: "/api/workflows/workflow/execute", body: `{}`},
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "missing required role") {
			t.Fatalf("%s %s: expected 403, got %d body=%s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
}

func (s stubService) Create(ctx context.Context, req CreateRequest) (Workflow, error) {
	return s.createFn(ctx, req)
}

func (s stubService) GetBySlug(ctx context.Context, slug string) (Workflow, error) {
	return s.getBySlugFn(ctx, slug)
}

func (s stubService) Execute(ctx context.Context, slug string, req ExecuteRequest) (Execution, error) {
	return s.executeFn(ctx, slug, req)
}

func (s stubService) GetExecution(ctx context.Context, id uuid.UUID) (Execution, error) {
	return s.getExecutionFn(ctx, id)
}

func (s stubService) Resume(ctx context.Context, id uuid.UUID, req ResumeRequest) (Execution, error) {
	return s.resumeFn(ctx, id, req)
}

func TestHandlerCreateReturnsCreated(t *testing.T) {
	h := NewHandler(stubService{
		createFn: func(_ context.Context, req CreateRequest) (Workflow, error) {
			return Workflow{ID: uuid.New(), Slug: req.Name, Name: req.Name, Steps: req.Steps}, nil
		},
	})
	r := newWorkflowRouter(h, "admin")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/workflows", strings.NewReader(`{"name":"aprovar-pedido","steps":[{"id":"s1","type":"agent"}]}`))
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandlerCreateRejectsTrailingJSONWithoutCallingService(t *testing.T) {
	called := false
	h := NewHandler(stubService{
		createFn: func(context.Context, CreateRequest) (Workflow, error) {
			called = true
			return Workflow{}, nil
		},
	})
	r := newWorkflowRouter(h, "admin")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/workflows", strings.NewReader(`{"name":"first","steps":[]}{"name":"ignored"}`))
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if called {
		t.Fatal("workflow service must not run for concatenated JSON")
	}
}

func TestHandlerCreateRejectsConflictingStepTypeAliasesWithoutCallingService(t *testing.T) {
	called := false
	h := NewHandler(stubService{
		createFn: func(context.Context, CreateRequest) (Workflow, error) {
			called = true
			return Workflow{}, nil
		},
	})
	r := newWorkflowRouter(h, "admin")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/workflows", strings.NewReader(`{"name":"conflicting-step","steps":[{"id":"s1","type":"agent","kind":"tool"}]}`))
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if called {
		t.Fatal("workflow service must not run when type and kind conflict")
	}
}

func TestHandlerExecuteReturnsAccepted(t *testing.T) {
	executionID := uuid.New()
	h := NewHandler(stubService{
		executeFn: func(_ context.Context, slug string, req ExecuteRequest) (Execution, error) {
			if slug != "aprovar-pedido" || req.Input["amount"].(float64) != 10000 {
				return Execution{}, errors.New("unexpected request")
			}
			return Execution{ID: executionID, WorkflowSlug: slug, State: ExecutionStateSuspended}, nil
		},
	})
	r := newWorkflowRouter(h, "admin")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/workflows/aprovar-pedido/execute", strings.NewReader(`{"input":{"amount":10000}}`))
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandlerExecuteRejectsTrailingJSONWithoutCallingService(t *testing.T) {
	called := false
	h := NewHandler(stubService{
		executeFn: func(context.Context, string, ExecuteRequest) (Execution, error) {
			called = true
			return Execution{}, nil
		},
	})
	r := newWorkflowRouter(h, "admin")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/workflows/approval/execute", strings.NewReader(`{"input":{}}{"input":{"ignored":true}}`))
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || called {
		t.Fatalf("expected 400 without execution, got %d called=%t body=%s", rec.Code, called, rec.Body.String())
	}
}

func TestHandlerResumeReturnsOK(t *testing.T) {
	executionID := uuid.New()
	h := NewHandler(stubService{
		resumeFn: func(_ context.Context, id uuid.UUID, req ResumeRequest) (Execution, error) {
			if id != executionID || req["decision"] != "approve" {
				return Execution{}, errors.New("unexpected request")
			}
			return Execution{ID: id, State: ExecutionStateCompleted}, nil
		},
	})
	r := newWorkflowRouter(h, "admin")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/workflows/executions/"+executionID.String()+"/resume", bytes.NewReader([]byte(`{"decision":"approve"}`)))
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandlerResumeRejectsTrailingJSONWithoutCallingService(t *testing.T) {
	called := false
	executionID := uuid.New()
	h := NewHandler(stubService{
		resumeFn: func(context.Context, uuid.UUID, ResumeRequest) (Execution, error) {
			called = true
			return Execution{}, nil
		},
	})
	r := newWorkflowRouter(h, "admin")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/workflows/executions/"+executionID.String()+"/resume", strings.NewReader(`{"decision":"approve"}{"decision":"ignored"}`))
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || called {
		t.Fatalf("expected 400 without resume, got %d called=%t body=%s", rec.Code, called, rec.Body.String())
	}
}

func TestHandlerGetExecutionInvalidIDReturnsBadRequest(t *testing.T) {
	h := NewHandler(stubService{})
	r := newWorkflowRouter(h, "admin")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/workflows/executions/not-a-uuid", nil)
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandlerNotFoundMapsTo404(t *testing.T) {
	h := NewHandler(stubService{
		getBySlugFn: func(context.Context, string) (Workflow, error) {
			return Workflow{}, ErrNotFound
		},
	})
	r := newWorkflowRouter(h, "admin")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/workflows/missing", nil)
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
