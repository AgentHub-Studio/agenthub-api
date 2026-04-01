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

// Record records a new metrics entry.
func (s *Service) Record(ctx context.Context, tenantID string, req RecordRequest) (AgentMetrics, error) {
	m := AgentMetrics{
		AgentID:          req.AgentID,
		AgentExecutionID: req.AgentExecutionID,
		SessionID:        req.SessionID,
		ModelName:        req.ModelName,
		Provider:         req.Provider,
		PromptTokens:     req.PromptTokens,
		CompletionTokens: req.CompletionTokens,
		TotalTokens:      req.TotalTokens,
		EstimatedCostUSD: req.EstimatedCostUSD,
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
