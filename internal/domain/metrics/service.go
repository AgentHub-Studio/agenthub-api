package metrics

import (
	"context"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// MetricsRepository defines the persistence interface for AgentMetrics.
type MetricsRepository interface {
	ListByAgent(ctx context.Context, tenantID string, agentID uuid.UUID, pr pagination.PageRequest) ([]AgentMetrics, int, error)
	Record(ctx context.Context, tenantID string, m AgentMetrics) (AgentMetrics, error)
	GetAgentSummary(ctx context.Context, tenantID string, agentID uuid.UUID) (MetricsSummary, error)
	GetTenantSummary(ctx context.Context, tenantID string) (MetricsSummary, error)
	ListAll(ctx context.Context, tenantID string, pr pagination.PageRequest) ([]AgentMetrics, int, error)
}

// Service implements business logic for agent metrics.
type Service struct {
	repo MetricsRepository
}

// NewService creates a new Service.
func NewService(repo MetricsRepository) *Service {
	return &Service{repo: repo}
}

// ListByAgent returns paginated metrics for the given agent.
func (s *Service) ListByAgent(ctx context.Context, tenantID string, agentID uuid.UUID, pr pagination.PageRequest) ([]AgentMetrics, int, error) {
	return s.repo.ListByAgent(ctx, tenantID, agentID, pr)
}

// Record records a new metrics entry, auto-calculating cost when not supplied.
func (s *Service) Record(ctx context.Context, tenantID string, req RecordRequest) (AgentMetrics, error) {
	cost := req.EstimatedCostUSD
	if cost == 0 && req.TotalTokens > 0 {
		cost = EstimateCost(req.Provider, req.ModelName, req.TotalTokens)
	}
	m := AgentMetrics{
		AgentID:          req.AgentID,
		AgentExecutionID: req.AgentExecutionID,
		SessionID:        req.SessionID,
		ModelName:        req.ModelName,
		Provider:         req.Provider,
		PromptTokens:     req.PromptTokens,
		CompletionTokens: req.CompletionTokens,
		TotalTokens:      req.TotalTokens,
		EstimatedCostUSD: cost,
		LatencyMs:        req.LatencyMs,
	}
	return s.repo.Record(ctx, tenantID, m)
}

// GetAgentSummary returns aggregated metrics for an agent.
func (s *Service) GetAgentSummary(ctx context.Context, tenantID string, agentID uuid.UUID) (MetricsSummary, error) {
	return s.repo.GetAgentSummary(ctx, tenantID, agentID)
}

// GetTenantSummary returns aggregated metrics for the entire tenant.
func (s *Service) GetTenantSummary(ctx context.Context, tenantID string) (MetricsSummary, error) {
	return s.repo.GetTenantSummary(ctx, tenantID)
}

// TopAgents returns the top N agents ranked by total token usage.
func (s *Service) TopAgents(ctx context.Context, tenantID string, limit int) ([]AgentUsage, error) {
	if limit <= 0 {
		limit = 10
	}
	all, _, err := s.repo.ListAll(ctx, tenantID, pagination.PageRequest{Page: 0, Size: 10_000})
	if err != nil {
		return nil, err
	}

	agg := make(map[uuid.UUID]*AgentUsage)
	for _, m := range all {
		a, ok := agg[m.AgentID]
		if !ok {
			a = &AgentUsage{AgentID: m.AgentID}
			agg[m.AgentID] = a
		}
		a.Executions++
		a.TotalTokens += int64(m.TotalTokens)
		a.TotalCostUSD += m.EstimatedCostUSD
	}

	result := make([]AgentUsage, 0, len(agg))
	for _, a := range agg {
		result = append(result, *a)
	}
	// Sort descending by TotalTokens (insertion sort — small N expected).
	for i := 1; i < len(result); i++ {
		for j := i; j > 0 && result[j].TotalTokens > result[j-1].TotalTokens; j-- {
			result[j], result[j-1] = result[j-1], result[j]
		}
	}
	if limit < len(result) {
		result = result[:limit]
	}
	return result, nil
}

// CostBreakdown returns cost aggregated by provider and model.
func (s *Service) CostBreakdown(ctx context.Context, tenantID string) ([]CostBreakdownEntry, error) {
	all, _, err := s.repo.ListAll(ctx, tenantID, pagination.PageRequest{Page: 0, Size: 10_000})
	if err != nil {
		return nil, err
	}

	type key struct{ provider, model string }
	agg := make(map[key]*CostBreakdownEntry)
	for _, m := range all {
		k := key{provider: m.Provider, model: m.ModelName}
		e, ok := agg[k]
		if !ok {
			e = &CostBreakdownEntry{Provider: m.Provider, ModelName: m.ModelName}
			agg[k] = e
		}
		e.Executions++
		e.TotalTokens += int64(m.TotalTokens)
		e.TotalCostUSD += m.EstimatedCostUSD
	}

	result := make([]CostBreakdownEntry, 0, len(agg))
	for _, e := range agg {
		result = append(result, *e)
	}
	return result, nil
}
