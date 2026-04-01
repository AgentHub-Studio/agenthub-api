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
	return metrics.MetricsSummary{TotalTokens: total}, nil
}

func (m *mockMetricsRepo) GetTenantSummary(_ context.Context, _ string) (metrics.MetricsSummary, error) {
	var total int
	for _, r := range m.data {
		total += r.TotalTokens
	}
	return metrics.MetricsSummary{TotalTokens: total}, nil
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
		LatencyMs:        250,
	})
	require.NoError(t, err)
	assert.Equal(t, agentID, rec.AgentID)
	assert.Equal(t, 150, rec.TotalTokens)
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
		PromptTokens: 100, CompletionTokens: 50,
	})
	require.NoError(t, err)
	summary, err := svc.GetAgentSummary(context.Background(), tenantID, agentID)
	require.NoError(t, err)
	assert.Equal(t, 150, summary.TotalTokens)
}
