package workflow

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeRepository struct {
	createdWorkflow Workflow
	workflow        Workflow
	createdExec     Execution
}

func (r *fakeRepository) Create(_ context.Context, wf Workflow) (Workflow, error) {
	r.createdWorkflow = wf
	wf.CreatedAt = time.Now()
	wf.UpdatedAt = time.Now()
	return wf, nil
}

func (r *fakeRepository) GetBySlug(_ context.Context, slug string) (Workflow, error) {
	if r.workflow.Slug != slug {
		return Workflow{}, ErrNotFound
	}
	return r.workflow, nil
}

func (r *fakeRepository) CreateExecution(_ context.Context, ex Execution) (Execution, error) {
	r.createdExec = ex
	return ex, nil
}

func (r *fakeRepository) GetExecution(_ context.Context, id uuid.UUID) (Execution, error) {
	return Execution{ID: id, State: ExecutionStateSuspended}, nil
}

func (r *fakeRepository) ResumeExecution(_ context.Context, id uuid.UUID, data map[string]any) (Execution, error) {
	return Execution{ID: id, State: ExecutionStateCompleted, ResumeData: data}, nil
}

func TestServiceCreateNormalizesSlugStartAndBranchCases(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewService(repo)

	wf, err := svc.Create(context.Background(), CreateRequest{
		Name: "Aprovar Pedido",
		Steps: []Step{
			{ID: "s1", Kind: StepKindAgent, Next: "s2"},
			{ID: "s2", Kind: StepKindBranch, OnTrue: "s_approve", OnFalse: "s_done"},
			{ID: "s_approve", Kind: StepKindTool},
		},
	})

	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if wf.Slug != "aprovar-pedido" {
		t.Fatalf("expected slug aprovar-pedido, got %q", wf.Slug)
	}
	if wf.Start != "s1" {
		t.Fatalf("expected start s1, got %q", wf.Start)
	}
	if wf.Steps[1].Cases["true"] != "s_approve" {
		t.Fatalf("branch cases were not normalized: %+v", wf.Steps[1].Cases)
	}
}

func TestServiceExecuteSuspendsAtBranch(t *testing.T) {
	workflowID := uuid.New()
	repo := &fakeRepository{workflow: Workflow{
		ID:    workflowID,
		Slug:  "aprovar-pedido",
		Name:  "Aprovar Pedido",
		Start: "s1",
		Steps: []Step{
			{ID: "s1", Kind: StepKindAgent},
			{ID: "s2", Kind: StepKindBranch},
			{ID: "s_approve", Kind: StepKindTool},
		},
	}}
	svc := NewService(repo)

	ex, err := svc.Execute(context.Background(), "aprovar-pedido", ExecuteRequest{Input: map[string]any{"amount": 10000}})

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if ex.State != ExecutionStateSuspended {
		t.Fatalf("expected SUSPENDED, got %s", ex.State)
	}
	if ex.CurrentStepID == nil || *ex.CurrentStepID != "s2" {
		t.Fatalf("expected current step s2, got %#v", ex.CurrentStepID)
	}
	if repo.createdExec.WorkflowID != workflowID {
		t.Fatalf("expected execution workflow id %s, got %s", workflowID, repo.createdExec.WorkflowID)
	}
}

func TestServiceCreateRejectsInvalidStepType(t *testing.T) {
	svc := NewService(&fakeRepository{})
	if _, err := svc.Create(context.Background(), CreateRequest{
		Name:  "Invalid",
		Steps: []Step{{ID: "s1", Kind: "dag"}},
	}); err == nil {
		t.Fatal("expected validation error")
	}
}
