package experiment_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/experiment"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// mockExperimentSvc satisfies the private experimentService interface in experiment.Handler.
type mockExperimentSvc struct {
	experiments map[uuid.UUID]experiment.PromptExperiment
}

func newMockExperimentSvc() *mockExperimentSvc {
	return &mockExperimentSvc{experiments: make(map[uuid.UUID]experiment.PromptExperiment)}
}

func (m *mockExperimentSvc) ListAll(_ context.Context, _ string, pr pagination.PageRequest) ([]experiment.PromptExperiment, int, error) {
	items := make([]experiment.PromptExperiment, 0, len(m.experiments))
	for _, e := range m.experiments {
		items = append(items, e)
	}
	return items, len(items), nil
}

func (m *mockExperimentSvc) GetByID(_ context.Context, _ string, id uuid.UUID) (experiment.PromptExperiment, error) {
	e, ok := m.experiments[id]
	if !ok {
		return experiment.PromptExperiment{}, experiment.ErrNotFound
	}
	return e, nil
}

func (m *mockExperimentSvc) Create(_ context.Context, _ string, req experiment.CreateRequest) (experiment.PromptExperiment, error) {
	id := uuid.New()
	e := experiment.PromptExperiment{
		ID:      id,
		AgentID: req.AgentID,
		Name:    req.Name,
		Status:  experiment.ExperimentStatusDraft,
	}
	m.experiments[id] = e
	return e, nil
}

func (m *mockExperimentSvc) Update(_ context.Context, _ string, id uuid.UUID, req experiment.CreateRequest) (experiment.PromptExperiment, error) {
	e, ok := m.experiments[id]
	if !ok {
		return experiment.PromptExperiment{}, experiment.ErrNotFound
	}
	e.Name = req.Name
	m.experiments[id] = e
	return e, nil
}

func (m *mockExperimentSvc) Delete(_ context.Context, _ string, id uuid.UUID) error {
	if _, ok := m.experiments[id]; !ok {
		return experiment.ErrNotFound
	}
	delete(m.experiments, id)
	return nil
}

func (m *mockExperimentSvc) Activate(_ context.Context, _ string, id uuid.UUID) (experiment.PromptExperiment, error) {
	e, ok := m.experiments[id]
	if !ok {
		return experiment.PromptExperiment{}, experiment.ErrNotFound
	}
	e.Status = experiment.ExperimentStatusActive
	m.experiments[id] = e
	return e, nil
}

func (m *mockExperimentSvc) Pause(_ context.Context, _ string, id uuid.UUID) (experiment.PromptExperiment, error) {
	e, ok := m.experiments[id]
	if !ok {
		return experiment.PromptExperiment{}, experiment.ErrNotFound
	}
	e.Status = experiment.ExperimentStatusPaused
	m.experiments[id] = e
	return e, nil
}

func (m *mockExperimentSvc) Complete(_ context.Context, _ string, id uuid.UUID) (experiment.PromptExperiment, error) {
	e, ok := m.experiments[id]
	if !ok {
		return experiment.PromptExperiment{}, experiment.ErrNotFound
	}
	e.Status = experiment.ExperimentStatusCompleted
	m.experiments[id] = e
	return e, nil
}

func (m *mockExperimentSvc) RecordResult(_ context.Context, _ string, experimentID uuid.UUID, req experiment.RecordResultRequest) (experiment.ExperimentResult, error) {
	if _, ok := m.experiments[experimentID]; !ok {
		return experiment.ExperimentResult{}, experiment.ErrNotFound
	}
	return experiment.ExperimentResult{
		ID:           uuid.New(),
		ExperimentID: experimentID,
		VariantKey:   req.VariantKey,
	}, nil
}

func (m *mockExperimentSvc) GetResults(_ context.Context, _ string, experimentID uuid.UUID, pr pagination.PageRequest) ([]experiment.ExperimentResult, int, error) {
	return []experiment.ExperimentResult{}, 0, nil
}

func (m *mockExperimentSvc) GetSummary(_ context.Context, _ string, id uuid.UUID) (experiment.ExperimentSummary, error) {
	if _, ok := m.experiments[id]; !ok {
		return experiment.ExperimentSummary{}, experiment.ErrNotFound
	}
	return experiment.ExperimentSummary{ExperimentID: id}, nil
}

func (m *mockExperimentSvc) SelectVariant(_ context.Context, _ string, id uuid.UUID, _ string) (string, error) {
	if _, ok := m.experiments[id]; !ok {
		return "", experiment.ErrNotFound
	}
	return "A", nil
}

func setupExperiment() (*chi.Mux, *mockExperimentSvc) {
	svc := newMockExperimentSvc()
	h := experiment.NewHandler(svc)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := tenant.NewContext(r.Context(), "test-tenant")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r.Mount("/api/experiments", h.Routes())
	return r, svc
}

func TestExperimentHandler_List_Success(t *testing.T) {
	r, svc := setupExperiment()
	id := uuid.New()
	svc.experiments[id] = experiment.PromptExperiment{ID: id, Name: "Exp A", Status: experiment.ExperimentStatusDraft}

	req := httptest.NewRequest(http.MethodGet, "/api/experiments/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var page pagination.Page[experiment.PromptExperimentResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.Equal(t, int64(1), page.TotalElements)
}

func TestExperimentHandler_Create_Success(t *testing.T) {
	r, _ := setupExperiment()
	body, _ := json.Marshal(experiment.CreateRequest{
		AgentID: uuid.New(),
		Name:    "My Experiment",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/experiments/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp experiment.PromptExperimentResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "My Experiment", resp.Name)
}

func TestExperimentHandler_Create_InvalidBody(t *testing.T) {
	r, _ := setupExperiment()
	req := httptest.NewRequest(http.MethodPost, "/api/experiments/", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestExperimentHandler_GetByID_NotFound(t *testing.T) {
	r, _ := setupExperiment()
	req := httptest.NewRequest(http.MethodGet, "/api/experiments/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestExperimentHandler_Delete_Success(t *testing.T) {
	r, svc := setupExperiment()
	id := uuid.New()
	svc.experiments[id] = experiment.PromptExperiment{ID: id, Name: "To Delete"}

	req := httptest.NewRequest(http.MethodDelete, "/api/experiments/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestExperimentHandler_Activate_Success(t *testing.T) {
	r, svc := setupExperiment()
	id := uuid.New()
	svc.experiments[id] = experiment.PromptExperiment{ID: id, Name: "Exp", Status: experiment.ExperimentStatusDraft}

	req := httptest.NewRequest(http.MethodPost, "/api/experiments/"+id.String()+"/activate", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp experiment.PromptExperimentResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, experiment.ExperimentStatusActive, resp.Status)
}

func TestExperimentHandler_RecordResult_Success(t *testing.T) {
	r, svc := setupExperiment()
	id := uuid.New()
	svc.experiments[id] = experiment.PromptExperiment{ID: id, Name: "Exp", Status: experiment.ExperimentStatusActive}

	body, _ := json.Marshal(experiment.RecordResultRequest{VariantKey: "A", SessionID: "s1"})
	req := httptest.NewRequest(http.MethodPost, "/api/experiments/"+id.String()+"/results", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}
