package workflow

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/sanitize"
)

// Repository defines workflow persistence operations.
type Repository interface {
	Create(ctx context.Context, wf Workflow) (Workflow, error)
	GetBySlug(ctx context.Context, slug string) (Workflow, error)
	CreateExecution(ctx context.Context, ex Execution) (Execution, error)
	GetExecution(ctx context.Context, id uuid.UUID) (Execution, error)
	ResumeExecution(ctx context.Context, id uuid.UUID, data map[string]any) (Execution, error)
}

// Service defines workflow business operations.
type Service interface {
	Create(ctx context.Context, req CreateRequest) (Workflow, error)
	GetBySlug(ctx context.Context, slug string) (Workflow, error)
	Execute(ctx context.Context, slug string, req ExecuteRequest) (Execution, error)
	GetExecution(ctx context.Context, id uuid.UUID) (Execution, error)
	Resume(ctx context.Context, id uuid.UUID, req ResumeRequest) (Execution, error)
}

type service struct {
	repo Repository
}

// NewService creates a workflow service.
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) Create(ctx context.Context, req CreateRequest) (Workflow, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return Workflow{}, fmt.Errorf("%w: name is required", ErrValidation)
	}
	if len(name) > 255 {
		return Workflow{}, fmt.Errorf("%w: name exceeds maximum length of 255 chars", ErrValidation)
	}
	slug := strings.TrimSpace(req.Slug)
	if slug == "" {
		slug = sanitize.ToSlug(name, "workflow")
	}
	if !sanitize.ValidSlug(slug) {
		return Workflow{}, fmt.Errorf("%w: slug must match %s", ErrValidation, sanitize.CanonicalSlugPattern)
	}
	steps, start, err := normalizeSteps(req.Steps, req.Start)
	if err != nil {
		return Workflow{}, err
	}
	return s.repo.Create(ctx, Workflow{
		ID:          uuid.New(),
		Slug:        slug,
		Name:        name,
		Description: req.Description,
		Start:       start,
		Steps:       steps,
	})
}

func (s *service) GetBySlug(ctx context.Context, slug string) (Workflow, error) {
	if strings.TrimSpace(slug) == "" {
		return Workflow{}, fmt.Errorf("%w: slug is required", ErrValidation)
	}
	return s.repo.GetBySlug(ctx, slug)
}

func (s *service) Execute(ctx context.Context, slug string, req ExecuteRequest) (Execution, error) {
	wf, err := s.GetBySlug(ctx, slug)
	if err != nil {
		return Execution{}, err
	}
	state := ExecutionStateCompleted
	var currentStepID *string
	if branchID := firstReachableBranchStepID(wf); branchID != "" {
		state = ExecutionStateSuspended
		currentStepID = &branchID
	}
	input := req.Input
	if input == nil {
		input = map[string]any{}
	}
	return s.repo.CreateExecution(ctx, Execution{
		ID:            uuid.New(),
		WorkflowID:    wf.ID,
		WorkflowSlug:  wf.Slug,
		State:         state,
		Input:         input,
		CurrentStepID: currentStepID,
		ResumeData:    map[string]any{},
		Output:        map[string]any{},
	})
}

func (s *service) GetExecution(ctx context.Context, id uuid.UUID) (Execution, error) {
	return s.repo.GetExecution(ctx, id)
}

func (s *service) Resume(ctx context.Context, id uuid.UUID, req ResumeRequest) (Execution, error) {
	data := map[string]any(req)
	if data == nil {
		data = map[string]any{}
	}
	return s.repo.ResumeExecution(ctx, id, data)
}

func normalizeSteps(steps []Step, start string) ([]Step, string, error) {
	if len(steps) == 0 {
		return nil, "", fmt.Errorf("%w: steps are required", ErrValidation)
	}
	seen := map[string]bool{}
	normalized := make([]Step, len(steps))
	for i, step := range steps {
		step.ID = strings.TrimSpace(step.ID)
		step.AgentID = strings.TrimSpace(step.AgentID)
		step.ToolID = strings.TrimSpace(step.ToolID)
		step.Next = strings.TrimSpace(step.Next)
		step.OnTrue = strings.TrimSpace(step.OnTrue)
		step.OnFalse = strings.TrimSpace(step.OnFalse)
		if step.Cases != nil {
			cases := make(map[string]string, len(step.Cases))
			for label, target := range step.Cases {
				cases[label] = strings.TrimSpace(target)
			}
			step.Cases = cases
		}
		if step.ID == "" {
			return nil, "", fmt.Errorf("%w: step id is required", ErrValidation)
		}
		if seen[step.ID] {
			return nil, "", fmt.Errorf("%w: duplicate step id %q", ErrValidation, step.ID)
		}
		seen[step.ID] = true
		switch step.Kind {
		case StepKindAgent, StepKindBranch, StepKindTool:
		default:
			return nil, "", fmt.Errorf("%w: invalid step type %q", ErrValidation, step.Kind)
		}
		if step.Kind == StepKindBranch {
			if step.Cases == nil {
				step.Cases = map[string]string{}
			}
			if step.OnTrue != "" {
				step.Cases["true"] = step.OnTrue
			}
			if step.OnFalse != "" {
				step.Cases["false"] = step.OnFalse
			}
		}
		normalized[i] = step
	}
	start = strings.TrimSpace(start)
	if start == "" {
		start = normalized[0].ID
	}
	if !seen[start] {
		return nil, "", fmt.Errorf("%w: start step %q does not exist", ErrValidation, start)
	}
	return normalized, start, nil
}

// firstReachableBranchStepID follows the deterministic linear path from Start.
// An explicit Next wins; otherwise the next declared step is the linear default.
// Branches outside that path must not suspend an execution.
func firstReachableBranchStepID(wf Workflow) string {
	if len(wf.Steps) == 0 {
		return ""
	}

	indexes := make(map[string]int, len(wf.Steps))
	for index, step := range wf.Steps {
		indexes[step.ID] = index
	}

	currentID := strings.TrimSpace(wf.Start)
	if currentID == "" {
		currentID = wf.Steps[0].ID
	}
	visited := make(map[string]struct{}, len(wf.Steps))
	for currentID != "" {
		if _, seen := visited[currentID]; seen {
			return ""
		}
		visited[currentID] = struct{}{}

		index, ok := indexes[currentID]
		if !ok {
			return ""
		}
		step := wf.Steps[index]
		if step.Kind == StepKindBranch {
			return step.ID
		}
		if nextID := strings.TrimSpace(step.Next); nextID != "" {
			currentID = nextID
			continue
		}
		if index+1 >= len(wf.Steps) {
			return ""
		}
		currentID = wf.Steps[index+1].ID
	}
	return ""
}
