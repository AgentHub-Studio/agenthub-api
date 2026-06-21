package analytics

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	tenantctx "github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// ClickHouseStore implements AnalyticsStore with ClickHouse metric tables.
type ClickHouseStore struct {
	client *ClickHouseClient
}

// NewClickHouseStore creates an AnalyticsStore backed by ClickHouse.
func NewClickHouseStore(client *ClickHouseClient) AnalyticsStore {
	return &ClickHouseStore{client: client}
}

// AgentUsage aggregates run counts, tokens, cost, latency, and turns for an agent.
func (s *ClickHouseStore) AgentUsage(ctx context.Context, agentID uuid.UUID, tr TimeRange) (UsageSummary, error) {
	query := fmt.Sprintf(`
		SELECT
			count() AS totalRuns,
			toInt64(ifNull(sum(total_tokens), 0)) AS totalTokens,
			toInt64(ifNull(sum(prompt_tokens), 0)) AS promptTokens,
			toInt64(ifNull(sum(completion_tokens), 0)) AS completionTokens,
			ifNull(sum(cost_usd), 0) AS totalCostUsd,
			ifNull(avg(duration_ms), 0) AS avgDurationMs,
			ifNull(avg(total_turns), 0) AS avgTurns
		FROM agent_run_metrics
		WHERE agent_id = %s
		  AND started_at >= %s
		  AND started_at <= %s%s`,
		uuidExpr(agentID), timeExpr(tr.From), timeExpr(tr.To), tenantPredicate(ctx))

	var rows []struct {
		TotalRuns        int     `json:"totalRuns"`
		TotalTokens      int64   `json:"totalTokens"`
		PromptTokens     int64   `json:"promptTokens"`
		CompletionTokens int64   `json:"completionTokens"`
		TotalCostUSD     float64 `json:"totalCostUsd"`
		AvgDurationMs    float64 `json:"avgDurationMs"`
		AvgTurns         float64 `json:"avgTurns"`
	}
	if err := queryJSONEachRow(ctx, s.client, query, &rows); err != nil {
		return UsageSummary{}, fmt.Errorf("analytics: clickhouse usage: %w", err)
	}
	out := UsageSummary{AgentID: agentID}
	if len(rows) > 0 {
		out.TotalRuns = rows[0].TotalRuns
		out.TotalTokens = rows[0].TotalTokens
		out.PromptTokens = rows[0].PromptTokens
		out.CompletionTokens = rows[0].CompletionTokens
		out.TotalCostUSD = rows[0].TotalCostUSD
		out.AvgDurationMs = rows[0].AvgDurationMs
		out.AvgTurns = rows[0].AvgTurns
	}
	return out, nil
}

// AgentCosts breaks down cost by provider + model for an agent.
func (s *ClickHouseStore) AgentCosts(ctx context.Context, agentID uuid.UUID, tr TimeRange) ([]CostSummary, error) {
	query := fmt.Sprintf(`
		SELECT
			provider AS provider,
			model AS model,
			count() AS runs,
			toInt64(sum(total_tokens)) AS totalTokens,
			sum(cost_usd) AS costUsd
		FROM agent_run_metrics
		WHERE agent_id = %s
		  AND started_at >= %s
		  AND started_at <= %s%s
		GROUP BY provider, model
		ORDER BY costUsd DESC`,
		uuidExpr(agentID), timeExpr(tr.From), timeExpr(tr.To), tenantPredicate(ctx))

	var out []CostSummary
	if err := queryJSONEachRow(ctx, s.client, query, &out); err != nil {
		return nil, fmt.Errorf("analytics: clickhouse costs: %w", err)
	}
	return out, nil
}

// TopTools returns the most used tools from ClickHouse tool metrics.
func (s *ClickHouseStore) TopTools(ctx context.Context, tr TimeRange, limit int) ([]TopTool, error) {
	query := fmt.Sprintf(`
		SELECT
			tool_name AS toolName,
			count() AS executions,
			avg(duration_ms) AS avgDurationMs,
			toFloat64(sum(if(state = 'failed' OR error IS NOT NULL, 1, 0))) / count() AS errorRate
		FROM tool_execution_metrics
		WHERE started_at >= %s
		  AND started_at <= %s%s
		GROUP BY tool_name
		ORDER BY executions DESC
		LIMIT %d`,
		timeExpr(tr.From), timeExpr(tr.To), tenantPredicate(ctx), limit)

	var out []TopTool
	if err := queryJSONEachRow(ctx, s.client, query, &out); err != nil {
		return nil, fmt.Errorf("analytics: clickhouse top tools: %w", err)
	}
	return out, nil
}

func tenantPredicate(ctx context.Context) string {
	tenantID := strings.TrimPrefix(strings.TrimSpace(tenantctx.FromContext(ctx)), "ah_")
	if tenantID == "" {
		return ""
	}
	return " AND tenant_id = " + quoteString(tenantID)
}

func queryJSONEachRow[T any](ctx context.Context, client *ClickHouseClient, query string, dest *[]T) error {
	body, err := client.do(ctx, client.database, []byte(query+"\nFORMAT JSONEachRow"))
	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(bytes.NewReader(body))
	for scanner.Scan() {
		if len(bytes.TrimSpace(scanner.Bytes())) == 0 {
			continue
		}
		var item T
		if err := json.Unmarshal(scanner.Bytes(), &item); err != nil {
			return err
		}
		*dest = append(*dest, item)
	}
	return scanner.Err()
}

var _ AnalyticsStore = (*ClickHouseStore)(nil)
