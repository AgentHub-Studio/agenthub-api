package trigger

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// CronParser computes the next run time from a cron expression.
type CronParser interface {
	NextRun(cronExpr string, from time.Time) (time.Time, error)
	Validate(cronExpr string) error
}

// Service provides business logic for agent triggers.
type Service struct {
	repo TriggerRepository
	cron CronParser
}

// NewService creates a new trigger Service.
func NewService(repo TriggerRepository, cron CronParser) *Service {
	return &Service{repo: repo, cron: cron}
}

// Create creates a new trigger and computes the initial next_run_at.
func (s *Service) Create(ctx context.Context, agentID uuid.UUID, req CreateTriggerRequest) (AgentTrigger, error) {
	if req.Name == "" {
		return AgentTrigger{}, fmt.Errorf("trigger: name is required")
	}
	// Bug 132: name varchar(255) — gate length antes do INSERT.
	if len(req.Name) > 255 {
		return AgentTrigger{}, fmt.Errorf("trigger: name exceeds maximum length of 255 chars (got %d)", len(req.Name))
	}
	if req.CronExpression == "" {
		return AgentTrigger{}, fmt.Errorf("trigger: cron expression is required")
	}
	if err := s.cron.Validate(req.CronExpression); err != nil {
		return AgentTrigger{}, fmt.Errorf("trigger: invalid cron expression: %w", err)
	}
	// Bug 161: cap inputTemplate em 32KB. Sem isso, trigger com 200KB+
	// payload é renderizado a cada cron tick — perf hit + storage waste.
	if len(req.InputTemplate) > 32000 {
		return AgentTrigger{}, fmt.Errorf("trigger: inputTemplate exceeds maximum length of 32000 chars (got %d)", len(req.InputTemplate))
	}

	exists, err := s.repo.ExistsByName(ctx, agentID, req.Name)
	if err != nil {
		return AgentTrigger{}, err
	}
	if exists {
		return AgentTrigger{}, ErrDuplicateName
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	var nextRun *time.Time
	if enabled {
		t, err := s.cron.NextRun(req.CronExpression, time.Now())
		if err == nil {
			nextRun = &t
		}
	}

	trigger := AgentTrigger{
		AgentID:        agentID,
		Name:           req.Name,
		CronExpression: req.CronExpression,
		Enabled:        enabled,
		InputTemplate:  req.InputTemplate,
		NextRunAt:      nextRun,
	}

	return s.repo.Create(ctx, trigger)
}

// GetByID returns a trigger by ID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (AgentTrigger, error) {
	return s.repo.GetByID(ctx, id)
}

// List returns paginated triggers for an agent.
func (s *Service) List(ctx context.Context, agentID uuid.UUID, page pagination.PageRequest) (pagination.Page[AgentTrigger], error) {
	return s.repo.ListByAgent(ctx, agentID, page)
}

// Update patches a trigger and recomputes next_run_at if cron changed.
func (s *Service) Update(ctx context.Context, id uuid.UUID, req UpdateTriggerRequest) (AgentTrigger, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return AgentTrigger{}, err
	}

	if req.Name != nil {
		// Bug 140: name varchar(255) — gate length em Update (sem isso
		// 422 vazava SQL 22001 message para o cliente).
		if len(*req.Name) > 255 {
			return AgentTrigger{}, fmt.Errorf("trigger: name exceeds maximum length of 255 chars (got %d)", len(*req.Name))
		}
		existing.Name = *req.Name
	}
	if req.CronExpression != nil {
		if err := s.cron.Validate(*req.CronExpression); err != nil {
			return AgentTrigger{}, fmt.Errorf("trigger: invalid cron expression: %w", err)
		}
		existing.CronExpression = *req.CronExpression
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if req.InputTemplate != nil {
		existing.InputTemplate = *req.InputTemplate
	}

	// Recompute next run.
	if existing.Enabled {
		t, err := s.cron.NextRun(existing.CronExpression, time.Now())
		if err == nil {
			existing.NextRunAt = &t
		}
	} else {
		existing.NextRunAt = nil
	}

	return s.repo.Update(ctx, existing)
}

// Delete removes a trigger.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// ListRuns returns paginated trigger runs.
func (s *Service) ListRuns(ctx context.Context, triggerID uuid.UUID, page pagination.PageRequest) (pagination.Page[AgentTriggerRun], error) {
	return s.repo.ListRuns(ctx, triggerID, page)
}

// StartRun creates a new run record and returns it.
func (s *Service) StartRun(ctx context.Context, triggerID, sessionID uuid.UUID) (AgentTriggerRun, error) {
	run := AgentTriggerRun{
		TriggerID: triggerID,
		SessionID: sessionID,
		Status:    RunStatusRunning,
	}
	return s.repo.CreateRun(ctx, run)
}

// CompleteRun marks a run as completed.
func (s *Service) CompleteRun(ctx context.Context, runID uuid.UUID, turns, tokens int) error {
	return s.repo.CompleteRun(ctx, runID, RunStatusCompleted, &turns, &tokens, nil)
}

// FailRun marks a run as failed.
func (s *Service) FailRun(ctx context.Context, runID uuid.UUID, errMsg string) error {
	return s.repo.CompleteRun(ctx, runID, RunStatusFailed, nil, nil, &errMsg)
}
