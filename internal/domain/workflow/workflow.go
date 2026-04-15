// Package workflow is a stub module for hybrid workflows (LLM + deterministic
// steps) — see MA-04 in the Mastra adoption plan.
//
// ADR-012 deprecated the previous DAG pipeline. This module introduces a
// linear sequence of Steps with simple branches and suspend/resume support.
// The full implementation lands in the agenthub-orchestrator; this file
// ships only the type vocabulary so callers have a stable import path.
package workflow

import (
	"context"

	"github.com/google/uuid"
)

// StepKind identifies how the step is executed.
type StepKind string

const (
	StepKindTool   StepKind = "tool"
	StepKindAgent  StepKind = "agent"
	StepKindBranch StepKind = "branch"
)

// Step is one node in the linear workflow.
type Step struct {
	ID     string         `json:"id"`
	Kind   StepKind       `json:"kind"`
	Config map[string]any `json:"config,omitempty"`
	// Next is the default next step ID. Branches override this per case.
	Next string `json:"next,omitempty"`
	// Cases maps a branch label → next step ID. Only used when Kind=branch.
	Cases map[string]string `json:"cases,omitempty"`
}

// Workflow is the persisted definition.
type Workflow struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Start string    `json:"start"`
	Steps []Step    `json:"steps"`
}

// State is an in-flight execution snapshot.
type State struct {
	WorkflowID  uuid.UUID
	ExecutionID uuid.UUID
	Current     string
	Vars        map[string]any
	Suspended   bool
}

// Executor is the runtime surface. Implementation lands with the
// orchestrator; the signature is fixed here so callers can wire against it.
type Executor interface {
	Start(ctx context.Context, wf Workflow, vars map[string]any) (State, error)
	Resume(ctx context.Context, executionID uuid.UUID, data map[string]any) (State, error)
}
