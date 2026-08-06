// Package workflow manages hybrid linear workflows (LLM + deterministic steps).
// ADR-012 deprecated DAG pipelines; workflows here are ordered step sequences
// with simple branch suspend/resume semantics.
package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// StepKind identifies how the step is executed.
type StepKind string

const (
	StepKindTool   StepKind = "tool"
	StepKindAgent  StepKind = "agent"
	StepKindBranch StepKind = "branch"
)

// ExecutionState identifies an in-flight workflow execution state.
type ExecutionState string

const (
	ExecutionStateRunning   ExecutionState = "RUNNING"
	ExecutionStateSuspended ExecutionState = "SUSPENDED"
	ExecutionStateCompleted ExecutionState = "COMPLETED"
	ExecutionStateFailed    ExecutionState = "FAILED"
)

var (
	ErrNotFound        = errors.New("workflow: not found")
	ErrValidation      = errors.New("workflow: validation failed")
	ErrConflict        = errors.New("workflow: conflict")
	ErrAlreadyResolved = errors.New("workflow: execution already resolved")
)

// Step is one node in the linear workflow.
type Step struct {
	ID        string         `json:"id"`
	Kind      StepKind       `json:"type"`
	AgentID   string         `json:"agentId,omitempty"`
	ToolID    string         `json:"toolId,omitempty"`
	Condition string         `json:"condition,omitempty"`
	Config    map[string]any `json:"config,omitempty"`
	// Next is the default next step ID. Branches override this per case.
	Next string `json:"next,omitempty"`
	// Cases maps a branch label → next step ID. Only used when Kind=branch.
	Cases   map[string]string `json:"cases,omitempty"`
	OnTrue  string            `json:"onTrue,omitempty"`
	OnFalse string            `json:"onFalse,omitempty"`
}

// Workflow is the persisted definition.
type Workflow struct {
	ID          uuid.UUID `json:"id" db:"id"`
	Slug        string    `json:"slug" db:"slug"`
	Name        string    `json:"name" db:"name"`
	Description *string   `json:"description,omitempty" db:"description"`
	Start       string    `json:"start,omitempty" db:"start_step_id"`
	Steps       []Step    `json:"steps" db:"steps"`
	CreatedAt   time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt   time.Time `json:"updatedAt" db:"updated_at"`
}

// Execution is a workflow run snapshot.
type Execution struct {
	ID            uuid.UUID      `json:"id" db:"id"`
	WorkflowID    uuid.UUID      `json:"workflowId" db:"workflow_id"`
	WorkflowSlug  string         `json:"workflowSlug" db:"workflow_slug"`
	State         ExecutionState `json:"state" db:"state"`
	Input         map[string]any `json:"input" db:"input"`
	CurrentStepID *string        `json:"currentStepId,omitempty" db:"current_step_id"`
	SuspendedAt   *time.Time     `json:"suspendedAt,omitempty" db:"suspended_at"`
	ResumedAt     *time.Time     `json:"resumedAt,omitempty" db:"resumed_at"`
	ResumeData    map[string]any `json:"resumeData,omitempty" db:"resume_data"`
	Output        map[string]any `json:"output,omitempty" db:"output"`
	CreatedAt     time.Time      `json:"createdAt" db:"created_at"`
	UpdatedAt     time.Time      `json:"updatedAt" db:"updated_at"`
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

type stepJSON struct {
	ID        string            `json:"id"`
	Type      StepKind          `json:"type,omitempty"`
	Kind      StepKind          `json:"kind,omitempty"`
	AgentID   string            `json:"agentId,omitempty"`
	ToolID    string            `json:"toolId,omitempty"`
	Condition string            `json:"condition,omitempty"`
	Config    map[string]any    `json:"config,omitempty"`
	Next      string            `json:"next,omitempty"`
	Cases     map[string]string `json:"cases,omitempty"`
	OnTrue    string            `json:"onTrue,omitempty"`
	OnFalse   string            `json:"onFalse,omitempty"`
}

// UnmarshalJSON accepts both the new "type" field and the older "kind" stub field.
func (s *Step) UnmarshalJSON(data []byte) error {
	var raw stepJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw.Type != "" && raw.Kind != "" && raw.Type != raw.Kind {
		return fmt.Errorf("workflow: conflicting step type aliases")
	}
	kind := raw.Type
	if kind == "" {
		kind = raw.Kind
	}
	s.ID = raw.ID
	s.Kind = kind
	s.AgentID = raw.AgentID
	s.ToolID = raw.ToolID
	s.Condition = raw.Condition
	s.Config = raw.Config
	s.Next = raw.Next
	s.Cases = raw.Cases
	s.OnTrue = raw.OnTrue
	s.OnFalse = raw.OnFalse
	return nil
}

// MarshalJSON emits the MA-04 API contract field "type".
func (s Step) MarshalJSON() ([]byte, error) {
	return json.Marshal(stepJSON{
		ID:        s.ID,
		Type:      s.Kind,
		AgentID:   s.AgentID,
		ToolID:    s.ToolID,
		Condition: s.Condition,
		Config:    s.Config,
		Next:      s.Next,
		Cases:     s.Cases,
		OnTrue:    s.OnTrue,
		OnFalse:   s.OnFalse,
	})
}
