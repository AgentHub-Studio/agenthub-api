package analytics_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/analytics"
)

// --- mock store ---

type mockStore struct{}

func (m *mockStore) AgentUsage(_ context.Context, agentID uuid.UUID, _ analytics.TimeRange) (analytics.UsageSummary, error) {
	return analytics.UsageSummary{
		AgentID:     agentID,
		TotalRuns:   10,
		TotalTokens: 5000,
	}, nil
}

func (m *mockStore) AgentCosts(_ context.Context, _ uuid.UUID, _ analytics.TimeRange) ([]analytics.CostSummary, error) {
	return []analytics.CostSummary{
		{Provider: "anthropic", Model: "claude-sonnet", Runs: 10, CostUSD: 1.0},
	}, nil
}

func (m *mockStore) TopTools(_ context.Context, _ analytics.TimeRange, limit int) ([]analytics.TopTool, error) {
	tools := []analytics.TopTool{
		{ToolName: "search", Executions: 50},
		{ToolName: "sql", Executions: 30},
	}
	if limit < len(tools) {
		tools = tools[:limit]
	}
	return tools, nil
}

func TestService_AgentUsage(t *testing.T) {
	svc := analytics.NewService(&mockStore{})
	agentID := uuid.New()
	result, err := svc.AgentUsage(context.Background(), agentID, analytics.TimeRange{})
	require.NoError(t, err)
	assert.Equal(t, 10, result.TotalRuns)
	assert.Equal(t, agentID, result.AgentID)
}

func TestService_AgentCosts(t *testing.T) {
	svc := analytics.NewService(&mockStore{})
	result, err := svc.AgentCosts(context.Background(), uuid.New(), analytics.TimeRange{})
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "anthropic", result[0].Provider)
}

func TestService_TopTools(t *testing.T) {
	svc := analytics.NewService(&mockStore{})
	result, err := svc.TopTools(context.Background(), analytics.TimeRange{}, 10)
	require.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestService_TopTools_DefaultLimit(t *testing.T) {
	svc := analytics.NewService(&mockStore{})
	result, err := svc.TopTools(context.Background(), analytics.TimeRange{}, 0)
	require.NoError(t, err)
	assert.Len(t, result, 2) // limit 0 defaults to 10 in service, mock returns 2
}
