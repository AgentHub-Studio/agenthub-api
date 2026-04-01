package experiment

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"

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

// Create creates a new experiment in DRAFT status after validating the traffic split.
func (s *Service) Create(ctx context.Context, tenantID string, req CreateRequest) (PromptExperiment, error) {
	if req.TrafficSplit != "" {
		if err := validateTrafficSplit(req.TrafficSplit); err != nil {
			return PromptExperiment{}, err
		}
	}
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
	if req.TrafficSplit != "" {
		if err := validateTrafficSplit(req.TrafficSplit); err != nil {
			return PromptExperiment{}, err
		}
	}
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

// transition enforces the state machine before delegating to the repo.
func (s *Service) transition(ctx context.Context, tenantID string, id uuid.UUID, to ExperimentStatus) (PromptExperiment, error) {
	current, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return PromptExperiment{}, err
	}
	if !CanTransition(current.Status, to) {
		return PromptExperiment{}, fmt.Errorf("%w: %s → %s", ErrInvalidTransition, current.Status, to)
	}
	return s.repo.UpdateStatus(ctx, tenantID, id, to)
}

// Activate transitions an experiment from DRAFT or PAUSED to ACTIVE.
func (s *Service) Activate(ctx context.Context, tenantID string, id uuid.UUID) (PromptExperiment, error) {
	return s.transition(ctx, tenantID, id, ExperimentStatusActive)
}

// Pause transitions an experiment from ACTIVE to PAUSED.
func (s *Service) Pause(ctx context.Context, tenantID string, id uuid.UUID) (PromptExperiment, error) {
	return s.transition(ctx, tenantID, id, ExperimentStatusPaused)
}

// Complete transitions an experiment from ACTIVE or PAUSED to COMPLETED.
func (s *Service) Complete(ctx context.Context, tenantID string, id uuid.UUID) (PromptExperiment, error) {
	return s.transition(ctx, tenantID, id, ExperimentStatusCompleted)
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

// SelectVariant deterministically picks a variant key for a given session.
// The session ID is hashed to [0, 100) using FNV-1a; the cumulative traffic split
// weights determine which bucket the session falls into.
func (s *Service) SelectVariant(ctx context.Context, tenantID string, id uuid.UUID, sessionID string) (string, error) {
	exp, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return "", err
	}
	if exp.TrafficSplit == "" {
		return "", fmt.Errorf("experiment: no traffic split configured")
	}

	var split map[string]int
	if err := json.Unmarshal([]byte(exp.TrafficSplit), &split); err != nil {
		return "", fmt.Errorf("experiment: invalid traffic split JSON: %w", err)
	}

	// FNV-1a hash of (sessionID + experimentID) → [0, 100)
	h := fnv.New32a()
	h.Write([]byte(sessionID + id.String()))
	bucket := int(h.Sum32() % 100)

	keys := sortedKeys(split)
	cumulative := 0
	for _, k := range keys {
		cumulative += split[k]
		if bucket < cumulative {
			return k, nil
		}
	}
	// Fallback to last key (handles rounding)
	if len(keys) > 0 {
		return keys[len(keys)-1], nil
	}
	return "", fmt.Errorf("experiment: empty traffic split")
}

// GetSummary aggregates experiment results per variant.
func (s *Service) GetSummary(ctx context.Context, tenantID string, id uuid.UUID) (ExperimentSummary, error) {
	results, _, err := s.repo.GetResults(ctx, tenantID, id, pagination.PageRequest{Page: 0, Size: 10_000})
	if err != nil {
		return ExperimentSummary{}, err
	}

	type agg struct {
		count       int
		feedbackSum int
		latencySum  int64
		tokenSum    int64
	}
	aggregates := make(map[string]*agg)
	for _, r := range results {
		a, ok := aggregates[r.VariantKey]
		if !ok {
			a = &agg{}
			aggregates[r.VariantKey] = a
		}
		a.count++
		a.feedbackSum += r.UserFeedback
		a.latencySum += r.LatencyMs
		a.tokenSum += int64(r.TokenCount)
	}

	summaries := make([]VariantSummary, 0, len(aggregates))
	for k, a := range aggregates {
		vs := VariantSummary{
			VariantKey:      k,
			Count:           a.count,
			TotalTokenCount: a.tokenSum,
		}
		if a.count > 0 {
			vs.AvgFeedback = float64(a.feedbackSum) / float64(a.count)
			vs.AvgLatencyMs = float64(a.latencySum) / float64(a.count)
		}
		summaries = append(summaries, vs)
	}
	return ExperimentSummary{ExperimentID: id, Variants: summaries}, nil
}

// validateTrafficSplit parses a JSON map and checks that all values sum to 100.
func validateTrafficSplit(raw string) error {
	var split map[string]int
	if err := json.Unmarshal([]byte(raw), &split); err != nil {
		return fmt.Errorf("experiment: invalid traffic split JSON: %w", err)
	}
	sum := 0
	for _, v := range split {
		sum += v
	}
	if sum != 100 {
		return fmt.Errorf("%w: got %d", ErrInvalidTrafficSplit, sum)
	}
	return nil
}

// sortedKeys returns the map keys sorted alphabetically using insertion sort.
func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}
