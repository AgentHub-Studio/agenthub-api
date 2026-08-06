package workflow

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestServiceExecuteDoesNotSuspendAtUnreachableBranch(t *testing.T) {
	repo := &fakeRepository{workflow: Workflow{
		ID:    uuid.New(),
		Slug:  "skip-approval",
		Name:  "Skip approval",
		Start: "start",
		Steps: []Step{
			{ID: "orphan-approval", Kind: StepKindBranch},
			{ID: "start", Kind: StepKindAgent, Next: "finish"},
			{ID: "finish", Kind: StepKindTool},
		},
	}}

	execution, err := NewService(repo).Execute(context.Background(), "skip-approval", ExecuteRequest{})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if execution.State != ExecutionStateCompleted {
		t.Fatalf("expected COMPLETED when the branch is unreachable, got %s", execution.State)
	}
	if execution.CurrentStepID != nil {
		t.Fatalf("expected no suspended step, got %q", *execution.CurrentStepID)
	}
}
