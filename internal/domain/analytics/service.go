package analytics

import (
	"context"

	"github.com/google/uuid"
)

// AnalyticsStore defines the query interface for analytics data.
// Implementations may use ClickHouse, PostgreSQL, or any other backend.
type AnalyticsStore interface {
	AgentUsage(ctx context.Context, agentID uuid.UUID, tr TimeRange) (UsageSummary, error)
	AgentCosts(ctx context.Context, agentID uuid.UUID, tr TimeRange) ([]CostSummary, error)
	TopTools(ctx context.Context, tr TimeRange, limit int) ([]TopTool, error)
}

// Service provides analytics query business logic.
type Service struct {
	store AnalyticsStore
}

// NewService creates a new analytics Service.
func NewService(store AnalyticsStore) *Service {
	return &Service{store: store}
}

// AgentUsage returns usage summary for an agent over the given time range.
func (s *Service) AgentUsage(ctx context.Context, agentID uuid.UUID, tr TimeRange) (UsageSummary, error) {
	return s.store.AgentUsage(ctx, agentID, tr)
}

// AgentCosts returns cost breakdown for an agent by provider/model.
func (s *Service) AgentCosts(ctx context.Context, agentID uuid.UUID, tr TimeRange) ([]CostSummary, error) {
	return s.store.AgentCosts(ctx, agentID, tr)
}

// TopTools returns the most used tools across all agents.
func (s *Service) TopTools(ctx context.Context, tr TimeRange, limit int) ([]TopTool, error) {
	if limit <= 0 {
		limit = 10
	}
	return s.store.TopTools(ctx, tr, limit)
}
