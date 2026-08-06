package execution

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// Service handles business logic for agent executions.
type Service struct {
	repo ExecutionRepository
}

// NewService creates a new execution Service.
func NewService(repo ExecutionRepository) *Service {
	return &Service{repo: repo}
}

// List returns a page of executions.
func (s *Service) List(ctx context.Context, agentID *uuid.UUID, status *string, req pagination.PageRequest) (pagination.Page[AgentExecution], error) {
	items, total, err := s.repo.List(ctx, agentID, status, req)
	if err != nil {
		return pagination.Page[AgentExecution]{}, err
	}
	return pagination.NewPage(items, total, req), nil
}

// Start creates a new execution record in RUNNING status.
func (s *Service) Start(ctx context.Context, req StartExecutionRequest) (AgentExecution, error) {
	agentID, err := uuid.Parse(req.AgentID)
	if err != nil {
		return AgentExecution{}, fmt.Errorf("%w: invalid agentId: %s", ErrInvalidInput, err)
	}

	input := req.Input
	if len(input) == 0 {
		input = json.RawMessage("{}")
	}

	var pipelineID *uuid.UUID
	if req.PipelineID != nil {
		pid, err := uuid.Parse(*req.PipelineID)
		if err != nil {
			return AgentExecution{}, fmt.Errorf("%w: invalid pipelineId: %s", ErrInvalidInput, err)
		}
		pipelineID = &pid
	}

	e := AgentExecution{
		AgentID:    agentID,
		PipelineID: pipelineID,
		Status:     "RUNNING",
		Input:      input,
	}
	return s.repo.Create(ctx, e)
}

// GetByID returns a single execution.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (AgentExecution, error) {
	return s.repo.GetByID(ctx, id)
}

// Cancel cancels a running execution.
// Returns ErrNotFound if the execution does not exist.
// Returns ErrInvalidTransition if the execution is not in a cancellable state.
func (s *Service) Cancel(ctx context.Context, id uuid.UUID) error {
	e, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if !CanTransition(e.Status, StatusCancelled) {
		return fmt.Errorf("%w: %s → %s", ErrInvalidTransition, e.Status, StatusCancelled)
	}
	return s.repo.Cancel(ctx, id)
}

// ListNodes returns node executions for a given execution.
func (s *Service) ListNodes(ctx context.Context, executionID uuid.UUID) ([]AgentExecutionNode, error) {
	return s.repo.ListNodes(ctx, executionID)
}

// GetDetails returns a full execution with its nested nodes and tool executions.
func (s *Service) GetDetails(ctx context.Context, id uuid.UUID) (ExecutionDetails, error) {
	return s.repo.GetDetails(ctx, id)
}

// Complete marks a running execution as COMPLETED.
func (s *Service) Complete(ctx context.Context, id uuid.UUID, output []byte) error {
	e, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if !CanTransition(e.Status, StatusCompleted) {
		return fmt.Errorf("%w: %s → %s", ErrInvalidTransition, e.Status, StatusCompleted)
	}
	return s.repo.Transition(ctx, id, e.Status, StatusCompleted, output, nil)
}

// Fail marks a running execution as FAILED.
func (s *Service) Fail(ctx context.Context, id uuid.UUID, errMsg string) error {
	e, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if !CanTransition(e.Status, StatusFailed) {
		return fmt.Errorf("%w: %s → %s", ErrInvalidTransition, e.Status, StatusFailed)
	}
	return s.repo.Transition(ctx, id, e.Status, StatusFailed, nil, &errMsg)
}

// ListToolExecutions returns tool executions for a node within the requested execution.
func (s *Service) ListToolExecutions(ctx context.Context, executionID uuid.UUID, nodeExecutionID uuid.UUID) ([]ToolExecution, error) {
	return s.repo.ListToolExecutions(ctx, executionID, nodeExecutionID)
}
