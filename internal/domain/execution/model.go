// Package execution manages agent execution tracking.
package execution

import (
	"time"

	"github.com/google/uuid"
)

// AgentExecution tracks a single agent run.
type AgentExecution struct {
	ID           uuid.UUID  `json:"id"`
	AgentID      uuid.UUID  `json:"agentId"`
	PipelineID   *uuid.UUID `json:"pipelineId,omitempty"`
	Status       string     `json:"status"` // RUNNING, SUCCESS, FAILED, CANCELLED
	Input        []byte     `json:"input"`
	Output       []byte     `json:"output,omitempty"`
	ErrorMessage *string    `json:"errorMessage,omitempty"`
	StartedAt    time.Time  `json:"startedAt"`
	FinishedAt   *time.Time `json:"finishedAt,omitempty"`
	DurationMs   *int64     `json:"durationMs,omitempty"`
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
