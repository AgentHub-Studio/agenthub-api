package metrics_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/metrics"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

type mockMetricsRepo struct {
	data []metrics.AgentMetrics
}

func newMockRepo() *mockMetricsRepo { return &mockMetricsRepo{} }

func (m *mockMetricsRepo) ListByAgent(_ context.Context, _ string, agentID uuid.UUID, _ pagination.PageRequest) ([]metrics.AgentMetrics, int, error) {
	var out []metrics.AgentMetrics
	for _, r := range m.data {
		if r.AgentID == agentID {
			out = append(out, r)
		}
	}
	return out, len(out), nil
}

func (m *mockMetricsRepo) ListAll(_ context.Context, _ string, _ pagination.PageRequest) ([]metrics.AgentMetrics, int, error) {
	return m.data, len(m.data), nil
}

func (m *mockMetricsRepo) Record(_ context.Context, _ string, r metrics.AgentMetrics) (metrics.AgentMetrics, error) {
	r.ID = uuid.New()
	m.data = append(m.data, r)
	return r, nil
}

func (m *mockMetricsRepo) GetAgentSummary(_ context.Context, _ string, agentID uuid.UUID) (metrics.MetricsSummary, error) {
	var total int
	for _, r := range m.data {
		if r.AgentID == agentID {
			total += r.TotalTokens
		}
	}
	return metrics.MetricsSummary{TotalTokens: int64(total)}, nil
}

func (m *mockMetricsRepo) GetTenantSummary(_ context.Context, _ string) (metrics.MetricsSummary, error) {
	var total int
	for _, r := range m.data {
		total += r.TotalTokens
	}
	return metrics.MetricsSummary{TotalTokens: int64(total)}, nil
}

const tenantID = "test-tenant"

func TestMetricsService_Record_Success(t *testing.T) {
	svc := metrics.NewService(newMockRepo())
	agentID := uuid.New()
	rec, err := svc.Record(context.Background(), tenantID, metrics.RecordRequest{
		AgentID:          agentID,
		ModelName:        "gpt-4o",
		Provider:         "openai",
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
		LatencyMs:        250,
	})
	require.NoError(t, err)
	assert.Equal(t, agentID, rec.AgentID)
	assert.Equal(t, 150, rec.TotalTokens)
}

func TestMetricsService_Record_AutoCostCalculation(t *testing.T) {
	svc := metrics.NewService(newMockRepo())
	rec, err := svc.Record(context.Background(), tenantID, metrics.RecordRequest{
		AgentID:      uuid.New(),
		Provider:     "openai",
		ModelName:    "gpt-4o",
		TotalTokens:  1000,
		// EstimatedCostUSD deliberately omitted
	})
	require.NoError(t, err)
	assert.Greater(t, rec.EstimatedCostUSD, 0.0, "cost should be auto-calculated for known model")
}

func TestMetricsService_Record_ExplicitCostNotOverridden(t *testing.T) {
	svc := metrics.NewService(newMockRepo())
	rec, err := svc.Record(context.Background(), tenantID, metrics.RecordRequest{
		AgentID:          uuid.New(),
		Provider:         "openai",
		ModelName:        "gpt-4o",
		TotalTokens:      1000,
		EstimatedCostUSD: 99.0,
	})
	require.NoError(t, err)
	assert.Equal(t, 99.0, rec.EstimatedCostUSD, "explicit cost should not be overridden")
}

func TestMetricsService_Record_UnknownModelZeroCost(t *testing.T) {
	svc := metrics.NewService(newMockRepo())
	rec, err := svc.Record(context.Background(), tenantID, metrics.RecordRequest{
		AgentID:     uuid.New(),
		Provider:    "unknown-provider",
		ModelName:   "unknown-model",
		TotalTokens: 1000,
	})
	require.NoError(t, err)
	assert.Equal(t, 0.0, rec.EstimatedCostUSD, "unknown model should produce zero cost")
}

func TestMetricsService_ListByAgent(t *testing.T) {
	svc := metrics.NewService(newMockRepo())
	agentID := uuid.New()
	for i := 0; i < 3; i++ {
		_, err := svc.Record(context.Background(), tenantID, metrics.RecordRequest{
			AgentID: agentID, ModelName: "gpt-4o", Provider: "openai",
		})
		require.NoError(t, err)
	}
	items, total, err := svc.ListByAgent(context.Background(), tenantID, agentID, pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, items, 3)
}

func TestMetricsService_GetAgentSummary(t *testing.T) {
	svc := metrics.NewService(newMockRepo())
	agentID := uuid.New()
	_, err := svc.Record(context.Background(), tenantID, metrics.RecordRequest{
		AgentID: agentID, ModelName: "gpt-4o", Provider: "openai",
		PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150,
	})
	require.NoError(t, err)
	summary, err := svc.GetAgentSummary(context.Background(), tenantID, agentID)
	require.NoError(t, err)
	assert.Equal(t, int64(150), summary.TotalTokens)
}

func TestMetricsService_TopAgents_Ordering(t *testing.T) {
	svc := metrics.NewService(newMockRepo())
	agentA := uuid.New()
	agentB := uuid.New()

	_, err := svc.Record(context.Background(), tenantID, metrics.RecordRequest{
		AgentID: agentA, Provider: "openai", ModelName: "gpt-4o", TotalTokens: 500,
	})
	require.NoError(t, err)
	_, err = svc.Record(context.Background(), tenantID, metrics.RecordRequest{
		AgentID: agentB, Provider: "openai", ModelName: "gpt-4o", TotalTokens: 1000,
	})
	require.NoError(t, err)

	top, err := svc.TopAgents(context.Background(), tenantID, 10)
	require.NoError(t, err)
	require.Len(t, top, 2)
	assert.Equal(t, agentB, top[0].AgentID, "agent with more tokens should rank first")
}

func TestMetricsService_TopAgents_LimitRespected(t *testing.T) {
	svc := metrics.NewService(newMockRepo())
	for i := 0; i < 5; i++ {
		_, err := svc.Record(context.Background(), tenantID, metrics.RecordRequest{
			AgentID: uuid.New(), Provider: "openai", ModelName: "gpt-4o", TotalTokens: 100,
		})
		require.NoError(t, err)
	}

	top, err := svc.TopAgents(context.Background(), tenantID, 3)
	require.NoError(t, err)
	assert.Len(t, top, 3)
}

func TestMetricsService_CostBreakdown(t *testing.T) {
	svc := metrics.NewService(newMockRepo())
	agentID := uuid.New()

	for i := 0; i < 2; i++ {
		_, err := svc.Record(context.Background(), tenantID, metrics.RecordRequest{
			AgentID: agentID, Provider: "openai", ModelName: "gpt-4o", TotalTokens: 1000,
		})
		require.NoError(t, err)
	}
	_, err := svc.Record(context.Background(), tenantID, metrics.RecordRequest{
		AgentID: agentID, Provider: "anthropic", ModelName: "claude-sonnet-4-6", TotalTokens: 500,
	})
	require.NoError(t, err)

	breakdown, err := svc.CostBreakdown(context.Background(), tenantID)
	require.NoError(t, err)
	assert.Len(t, breakdown, 2, "should have one entry per provider:model combination")

	providerModels := make(map[string]bool)
	for _, e := range breakdown {
		providerModels[e.Provider+":"+e.ModelName] = true
		assert.Greater(t, e.Executions, 0)
	}
	assert.True(t, providerModels["openai:gpt-4o"])
	assert.True(t, providerModels["anthropic:claude-sonnet-4-6"])
}

func TestEstimateCost_KnownModel(t *testing.T) {
	cost := metrics.EstimateCost("openai", "gpt-4o", 1000)
	assert.Greater(t, cost, 0.0)
}

func TestEstimateCost_UnknownModel(t *testing.T) {
	cost := metrics.EstimateCost("unknown", "unknown-model", 1000)
	assert.Equal(t, 0.0, cost)
}

func TestEstimateCost_CaseInsensitive(t *testing.T) {
	lower := metrics.EstimateCost("openai", "gpt-4o", 1000)
	upper := metrics.EstimateCost("OpenAI", "GPT-4O", 1000)
	assert.Equal(t, lower, upper)
}
