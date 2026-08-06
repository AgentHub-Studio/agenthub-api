package workflow

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestServiceExecuteUsesTrimmedTransitionIdentifiers(t *testing.T) {
	repo := &fakeRepository{workflow: Workflow{
		ID:    uuid.New(),
		Slug:  "trimmed-transition",
		Name:  "Trimmed transition",
		Start: " entry ",
		Steps: []Step{
			{ID: "entry", Kind: StepKindAgent, Next: " approval "},
			{ID: "approval", Kind: StepKindBranch},
			{ID: "finish", Kind: StepKindTool},
		},
	}}

	execution, err := NewService(repo).Execute(context.Background(), "trimmed-transition", ExecuteRequest{})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if execution.State != ExecutionStateSuspended {
		t.Fatalf("expected SUSPENDED at the reachable branch, got %s", execution.State)
	}
	if execution.CurrentStepID == nil || *execution.CurrentStepID != "approval" {
		t.Fatalf("expected approval as current step, got %#v", execution.CurrentStepID)
	}
}

func TestServiceCreateCanonicalizesTransitionReferencesWithoutMutatingRequest(t *testing.T) {
	cases := map[string]string{"manual": " finish "}
	req := CreateRequest{
		Name:  "Canonical transitions",
		Start: " entry ",
		Steps: []Step{
			{ID: " entry ", Kind: StepKindAgent, Next: " approval "},
			{
				ID:      " approval ",
				Kind:    StepKindBranch,
				Cases:   cases,
				OnTrue:  " approve ",
				OnFalse: " finish ",
			},
			{ID: " approve ", Kind: StepKindTool},
			{ID: " finish ", Kind: StepKindTool},
		},
	}

	wf, err := NewService(&fakeRepository{}).Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if wf.Start != "entry" || wf.Steps[0].Next != "approval" {
		t.Fatalf("expected canonical start/next, got start=%q next=%q", wf.Start, wf.Steps[0].Next)
	}
	branch := wf.Steps[1]
	if branch.OnTrue != "approve" || branch.OnFalse != "finish" || branch.Cases["manual"] != "finish" {
		t.Fatalf("expected canonical branch transitions, got %+v", branch)
	}
	if cases["manual"] != " finish " {
		t.Fatalf("Create mutated caller cases: %+v", cases)
	}
}
