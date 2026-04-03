package analytics_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/analytics"
)

// --- mock service ---

type mockSvc struct{}

func (m *mockSvc) AgentUsage(_ context.Context, agentID uuid.UUID, _ analytics.TimeRange) (analytics.UsageSummary, error) {
	return analytics.UsageSummary{
		AgentID:     agentID,
		TotalRuns:   42,
		TotalTokens: 100000,
		TotalCostUSD: 5.50,
		AvgDurationMs: 2300,
		AvgTurns: 3.5,
	}, nil
}

func (m *mockSvc) AgentCosts(_ context.Context, _ uuid.UUID, _ analytics.TimeRange) ([]analytics.CostSummary, error) {
	return []analytics.CostSummary{
		{Provider: "anthropic", Model: "claude-sonnet-4-6", Runs: 30, TotalTokens: 80000, CostUSD: 4.0},
		{Provider: "openai", Model: "gpt-4o", Runs: 12, TotalTokens: 20000, CostUSD: 1.5},
	}, nil
}

func (m *mockSvc) TopTools(_ context.Context, _ analytics.TimeRange, limit int) ([]analytics.TopTool, error) {
	tools := []analytics.TopTool{
		{ToolName: "document_search", Executions: 150, AvgDuration: 120, ErrorRate: 0.02},
		{ToolName: "execute_sql", Executions: 80, AvgDuration: 350, ErrorRate: 0.05},
	}
	if limit < len(tools) {
		tools = tools[:limit]
	}
	return tools, nil
}

func setupAnalytics() *chi.Mux {
	svc := &mockSvc{}
	h := analytics.NewHandler(svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r
}

func TestAnalyticsHandler_AgentUsage(t *testing.T) {
	r := setupAnalytics()
	agentID := uuid.New()

	req := httptest.NewRequest(http.MethodGet, "/api/analytics/agents/"+agentID.String()+"/usage", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result analytics.UsageSummary
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, 42, result.TotalRuns)
	assert.Equal(t, agentID, result.AgentID)
}

func TestAnalyticsHandler_AgentUsage_InvalidID(t *testing.T) {
	r := setupAnalytics()
	req := httptest.NewRequest(http.MethodGet, "/api/analytics/agents/invalid/usage", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAnalyticsHandler_AgentUsage_WithTimeRange(t *testing.T) {
	r := setupAnalytics()
	agentID := uuid.New()

	req := httptest.NewRequest(http.MethodGet, "/api/analytics/agents/"+agentID.String()+"/usage?from=2026-04-01T00:00:00Z&to=2026-04-03T00:00:00Z", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAnalyticsHandler_AgentUsage_InvalidFrom(t *testing.T) {
	r := setupAnalytics()
	agentID := uuid.New()

	req := httptest.NewRequest(http.MethodGet, "/api/analytics/agents/"+agentID.String()+"/usage?from=not-a-date", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAnalyticsHandler_AgentCosts(t *testing.T) {
	r := setupAnalytics()
	agentID := uuid.New()

	req := httptest.NewRequest(http.MethodGet, "/api/analytics/agents/"+agentID.String()+"/costs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result []analytics.CostSummary
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Len(t, result, 2)
}

func TestAnalyticsHandler_TopTools(t *testing.T) {
	r := setupAnalytics()

	req := httptest.NewRequest(http.MethodGet, "/api/analytics/tools/top", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result []analytics.TopTool
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Len(t, result, 2)
	assert.Equal(t, "document_search", result[0].ToolName)
}

func TestAnalyticsHandler_TopTools_WithLimit(t *testing.T) {
	r := setupAnalytics()

	req := httptest.NewRequest(http.MethodGet, "/api/analytics/tools/top?limit=1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result []analytics.TopTool
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Len(t, result, 1)
}
