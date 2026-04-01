package experiment

import (
	"context"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// ExperimentRepository defines the persistence interface for PromptExperiment.
type ExperimentRepository interface {
	ListAll(ctx context.Context, tenantID string, pr pagination.PageRequest) ([]PromptExperiment, int, error)
	GetByID(ctx context.Context, tenantID string, id uuid.UUID) (PromptExperiment, error)
	Create(ctx context.Context, tenantID string, e PromptExperiment) (PromptExperiment, error)
	Update(ctx context.Context, tenantID string, id uuid.UUID, e PromptExperiment) (PromptExperiment, error)
	UpdateStatus(ctx context.Context, tenantID string, id uuid.UUID, status ExperimentStatus) (PromptExperiment, error)
	Delete(ctx context.Context, tenantID string, id uuid.UUID) error
	RecordResult(ctx context.Context, tenantID string, res ExperimentResult) (ExperimentResult, error)
	GetResults(ctx context.Context, tenantID string, experimentID uuid.UUID, pr pagination.PageRequest) ([]ExperimentResult, int, error)
}

// Service implements business logic for prompt experiments.
type Service struct {
	repo ExperimentRepository
}

// NewService creates a new Service.
func NewService(repo ExperimentRepository) *Service {
	return &Service{repo: repo}
}

// ListAll returns a paginated list of experiments.
func (s *Service) ListAll(ctx context.Context, tenantID string, pr pagination.PageRequest) ([]PromptExperiment, int, error) {
	return s.repo.ListAll(ctx, tenantID, pr)
}

// GetByID retrieves an experiment by ID.
func (s *Service) GetByID(ctx context.Context, tenantID string, id uuid.UUID) (PromptExperiment, error) {
	return s.repo.GetByID(ctx, tenantID, id)
}

// Create creates a new experiment in DRAFT status.
func (s *Service) Create(ctx context.Context, tenantID string, req CreateRequest) (PromptExperiment, error) {
	e := PromptExperiment{
		AgentID:      req.AgentID,
		Name:         req.Name,
		Status:       ExperimentStatusDraft,
		TrafficSplit: req.TrafficSplit,
		Variants:     req.Variants,
		StartDate:    req.StartDate,
		EndDate:      req.EndDate,
	}
	return s.repo.Create(ctx, tenantID, e)
}

// Update updates an existing experiment (fields only, not status).
func (s *Service) Update(ctx context.Context, tenantID string, id uuid.UUID, req CreateRequest) (PromptExperiment, error) {
	e := PromptExperiment{
		AgentID:      req.AgentID,
		Name:         req.Name,
		TrafficSplit: req.TrafficSplit,
		Variants:     req.Variants,
		StartDate:    req.StartDate,
		EndDate:      req.EndDate,
	}
	return s.repo.Update(ctx, tenantID, id, e)
}

// Delete removes an experiment.
func (s *Service) Delete(ctx context.Context, tenantID string, id uuid.UUID) error {
	return s.repo.Delete(ctx, tenantID, id)
}

// Activate transitions an experiment to ACTIVE.
func (s *Service) Activate(ctx context.Context, tenantID string, id uuid.UUID) (PromptExperiment, error) {
	return s.repo.UpdateStatus(ctx, tenantID, id, ExperimentStatusActive)
}

// Pause transitions an experiment to PAUSED.
func (s *Service) Pause(ctx context.Context, tenantID string, id uuid.UUID) (PromptExperiment, error) {
	return s.repo.UpdateStatus(ctx, tenantID, id, ExperimentStatusPaused)
}

// Complete transitions an experiment to COMPLETED.
func (s *Service) Complete(ctx context.Context, tenantID string, id uuid.UUID) (PromptExperiment, error) {
	return s.repo.UpdateStatus(ctx, tenantID, id, ExperimentStatusCompleted)
}

// RecordResult records an observation for an experiment variant.
func (s *Service) RecordResult(ctx context.Context, tenantID string, experimentID uuid.UUID, req RecordResultRequest) (ExperimentResult, error) {
	res := ExperimentResult{
		ExperimentID: experimentID,
		VariantKey:   req.VariantKey,
		SessionID:    req.SessionID,
		UserFeedback: req.UserFeedback,
		LatencyMs:    req.LatencyMs,
		TokenCount:   req.TokenCount,
	}
	return s.repo.RecordResult(ctx, tenantID, res)
}

// GetResults returns paginated results for an experiment.
func (s *Service) GetResults(ctx context.Context, tenantID string, experimentID uuid.UUID, pr pagination.PageRequest) ([]ExperimentResult, int, error) {
	return s.repo.GetResults(ctx, tenantID, experimentID, pr)
}
