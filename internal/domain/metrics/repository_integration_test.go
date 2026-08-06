//go:build integration

package metrics_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/metrics"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

const metricsIntegrationTenant = "metricsnullsession"

func TestIntegration_MetricsAggregationsTolerateNullSessionID(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	ctx := tenant.NewContext(context.Background(), metricsIntegrationTenant)
	schema := "ah_" + metricsIntegrationTenant
	testutil.MustExec(t, pool, "CREATE SCHEMA IF NOT EXISTS "+schema)
	testutil.MustExec(t, pool, `
		CREATE TABLE `+schema+`.agent_metrics (
			id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			agent_id            UUID NOT NULL,
			agent_execution_id  UUID,
			session_id          UUID,
			model_name          VARCHAR(255) NOT NULL,
			provider            VARCHAR(100) NOT NULL,
			prompt_tokens       INTEGER NOT NULL DEFAULT 0,
			completion_tokens   INTEGER NOT NULL DEFAULT 0,
			total_tokens        INTEGER NOT NULL DEFAULT 0,
			estimated_cost_usd  NUMERIC(10,6) NOT NULL DEFAULT 0,
			latency_ms          BIGINT NOT NULL DEFAULT 0,
			created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`)

	agentID := uuid.New()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO `+schema+`.agent_metrics
			(agent_id, agent_execution_id, session_id, model_name, provider,
			 prompt_tokens, completion_tokens, total_tokens, estimated_cost_usd, latency_ms)
		VALUES ($1, NULL, NULL, 'gpt-4o-mini', 'openai', 10, 5, 15, 0.0001, 123)`,
		agentID,
	)
	require.NoError(t, err)

	svc := metrics.NewService(metrics.NewRepository(pool))

	topAgents, err := svc.TopAgents(ctx, metricsIntegrationTenant, 10)
	require.NoError(t, err)
	require.Len(t, topAgents, 1)
	assert.Equal(t, agentID, topAgents[0].AgentID)

	breakdown, err := svc.CostBreakdown(ctx, metricsIntegrationTenant)
	require.NoError(t, err)
	require.Len(t, breakdown, 1)
	assert.Equal(t, "openai", breakdown[0].Provider)
	assert.Equal(t, "gpt-4o-mini", breakdown[0].ModelName)
}
