package workflow_test

import (
	"encoding/json"
	"testing"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/workflow"
)

func TestWorkflowJSONRoundTrip(t *testing.T) {
	wf := workflow.Workflow{
		Name:  "approve",
		Start: "s1",
		Steps: []workflow.Step{
			{ID: "s1", Kind: workflow.StepKindAgent, Next: "s2"},
			{ID: "s2", Kind: workflow.StepKindBranch, Cases: map[string]string{"yes": "s3", "no": "s4"}},
			{ID: "s3", Kind: workflow.StepKindTool},
			{ID: "s4", Kind: workflow.StepKindTool},
		},
	}
	data, err := json.Marshal(wf)
	if err != nil {
		t.Fatal(err)
	}
	var back workflow.Workflow
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if len(back.Steps) != 4 || back.Steps[1].Cases["yes"] != "s3" {
		t.Fatalf("lost data: %+v", back)
	}
}
