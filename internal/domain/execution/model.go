// Package execution manages agent execution tracking.
package execution

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Status constants for AgentExecution.
const (
	StatusPending   = "PENDING"
	StatusRunning   = "RUNNING"
	StatusCompleted = "COMPLETED"
	StatusFailed    = "FAILED"
	StatusCancelled = "CANCELLED"
)

// ErrInvalidTransition is returned when a status transition is not allowed.
var ErrInvalidTransition = errors.New("execution: invalid status transition")

// validTransitions maps each status to the set of statuses it may transition to.
var validTransitions = map[string]map[string]bool{
	StatusPending:  {StatusRunning: true, StatusCancelled: true},
	StatusRunning:  {StatusCompleted: true, StatusFailed: true, StatusCancelled: true},
	StatusCompleted: {},
	StatusFailed:    {},
	StatusCancelled: {},
}

// CanTransition returns true if moving from 'from' to 'to' is a valid state-machine step.
func CanTransition(from, to string) bool {
	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}
	return allowed[to]
}

// AgentExecution tracks a single agent run.
type AgentExecution struct {
	ID           uuid.UUID  `json:"id"`
	AgentID      uuid.UUID  `json:"agentId"`
	PipelineID   *uuid.UUID `json:"pipelineId,omitempty"`
	Status       string     `json:"status"` // PENDING, RUNNING, COMPLETED, FAILED, CANCELLED
	Input        []byte     `json:"input"`
	Output       []byte     `json:"output,omitempty"`
	ErrorMessage *string    `json:"errorMessage,omitempty"`
	StartedAt    time.Time  `json:"startedAt"`
	FinishedAt   *time.Time `json:"finishedAt,omitempty"`
	DurationMs   *int64     `json:"durationMs,omitempty"`
}

// ExecutionDetails groups an execution with its nested nodes and tool executions.
type ExecutionDetails struct {
	AgentExecution
	Nodes []NodeDetails `json:"nodes"`
}

// NodeDetails groups a node execution with its tool executions.
type NodeDetails struct {
	AgentExecutionNode
	Tools []ToolExecution `json:"tools"`
}

// AgentExecutionNode tracks a single node execution within an agent run.
type AgentExecutionNode struct {
	ID           uuid.UUID  `json:"id"`
	ExecutionID  uuid.UUID  `json:"executionId"`
	NodeID       uuid.UUID  `json:"nodeId"`
	NodeType     string     `json:"nodeType"`
	Status       string     `json:"status"` // PENDING, RUNNING, SUCCESS, FAILED, SKIPPED
	Input        []byte     `json:"input"`
	Output       []byte     `json:"output,omitempty"`
	ErrorMessage *string    `json:"errorMessage,omitempty"`
	StartedAt    *time.Time `json:"startedAt,omitempty"`
	FinishedAt   *time.Time `json:"finishedAt,omitempty"`
	DurationMs   *int64     `json:"durationMs,omitempty"`
}

// ToolExecution tracks a single tool call within a node execution.
type ToolExecution struct {
	ID              uuid.UUID  `json:"id"`
	NodeExecutionID *uuid.UUID `json:"nodeExecutionId,omitempty"`
	ToolID          uuid.UUID  `json:"toolId"`
	Status          string     `json:"status"` // RUNNING, SUCCESS, FAILED
	Input           []byte     `json:"input"`
	Output          []byte     `json:"output,omitempty"`
	ErrorMessage    *string    `json:"errorMessage,omitempty"`
	StartedAt       time.Time  `json:"startedAt"`
	FinishedAt      *time.Time `json:"finishedAt,omitempty"`
	DurationMs      *int64     `json:"durationMs,omitempty"`
}
