package metrics_test

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

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/metrics"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// mockMetricsSvc satisfies the private metricsService interface in metrics.Handler.
type mockMetricsSvc struct {
	records map[uuid.UUID]metrics.AgentMetrics
}

func newMockMetricsSvc() *mockMetricsSvc {
	return &mockMetricsSvc{records: make(map[uuid.UUID]metrics.AgentMetrics)}
}

func (m *mockMetricsSvc) ListByAgent(_ context.Context, _ string, agentID uuid.UUID, pr pagination.PageRequest) ([]metrics.AgentMetrics, int, error) {
	var items []metrics.AgentMetrics
	for _, rec := range m.records {
		if rec.AgentID == agentID {
			items = append(items, rec)
		}
	}
	return items, len(items), nil
}

func (m *mockMetricsSvc) Record(_ context.Context, _ string, req metrics.RecordRequest) (metrics.AgentMetrics, error) {
	id := uuid.New()
	rec := metrics.AgentMetrics{
		ID:      id,
		AgentID: req.AgentID,
	}
	m.records[id] = rec
	return rec, nil
}

func (m *mockMetricsSvc) GetAgentSummary(_ context.Context, _ string, _ uuid.UUID) (metrics.MetricsSummary, error) {
	return metrics.MetricsSummary{TotalExecutions: 5}, nil
}

func (m *mockMetricsSvc) GetTenantSummary(_ context.Context, _ string) (metrics.MetricsSummary, error) {
	return metrics.MetricsSummary{TotalExecutions: 10}, nil
}

func (m *mockMetricsSvc) TopAgents(_ context.Context, _ string, _ int) ([]metrics.AgentUsage, error) {
	return []metrics.AgentUsage{}, nil
}

func (m *mockMetricsSvc) CostBreakdown(_ context.Context, _ string) ([]metrics.CostBreakdownEntry, error) {
	return []metrics.CostBreakdownEntry{}, nil
}

func setupMetrics() (*chi.Mux, *mockMetricsSvc) {
	svc := newMockMetricsSvc()
	h := metrics.NewHandler(svc)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := tenant.NewContext(r.Context(), "test-tenant")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r.Mount("/api/metrics", h.Routes())
	r.Mount("/api/agents/{id}/metrics", h.AgentRoutes())
	return r, svc
}

func TestMetricsHandler_ListByAgent_Success(t *testing.T) {
	r, svc := setupMetrics()
	agentID := uuid.New()
	id := uuid.New()
	svc.records[id] = metrics.AgentMetrics{ID: id, AgentID: agentID}

	req := httptest.NewRequest(http.MethodGet, "/api/agents/"+agentID.String()+"/metrics", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMetricsHandler_Record_Success(t *testing.T) {
	r, _ := setupMetrics()
	body, _ := json.Marshal(metrics.RecordRequest{
		AgentID:          uuid.New(),
		AgentExecutionID: uuid.New(),
		ModelName:        "gpt-4",
		Provider:         "openai",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/metrics/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestMetricsHandler_Record_InvalidBody(t *testing.T) {
	r, _ := setupMetrics()
	req := httptest.NewRequest(http.MethodPost, "/api/metrics/", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestMetricsHandler_Record_InvalidSessionIDReturnsUnprocessableEntity(t *testing.T) {
	r, svc := setupMetrics()
	req := httptest.NewRequest(http.MethodPost, "/api/metrics/", bytes.NewBufferString(`{"agentId":"`+uuid.NewString()+`","sessionId":"not-a-uuid","modelName":"gpt-4","provider":"openai"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Empty(t, svc.records)
}

func TestMetricsHandler_RecordRejectsTrailingJSONWithoutServiceEffects(t *testing.T) {
	r, svc := setupMetrics()
	req := httptest.NewRequest(http.MethodPost, "/api/metrics/", bytes.NewBufferString(`{"agentId":"`+uuid.NewString()+`","agentExecutionId":"`+uuid.NewString()+`","modelName":"gpt-4","provider":"openai"} {"modelName":"ignored"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, svc.records)
}

func TestMetricsHandler_AgentSummary_Success(t *testing.T) {
	r, _ := setupMetrics()
	agentID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/agents/"+agentID.String()+"/metrics/summary", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var summary metrics.MetricsSummary
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &summary))
	assert.Equal(t, 5, summary.TotalExecutions)
}

func TestMetricsHandler_TenantSummary_Success(t *testing.T) {
	r, _ := setupMetrics()
	req := httptest.NewRequest(http.MethodGet, "/api/metrics/tenant", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var summary metrics.MetricsSummary
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &summary))
	assert.Equal(t, 10, summary.TotalExecutions)
}
