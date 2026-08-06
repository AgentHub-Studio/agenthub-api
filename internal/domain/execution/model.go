// Package execution manages agent execution tracking.
package execution

import (
	"encoding/json"
	"errors"
	"regexp"
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

// ErrInvalidInput is returned when the request payload contains an invalid field value.
var ErrInvalidInput = errors.New("execution: invalid input")

// validTransitions maps each status to the set of statuses it may transition to.
var validTransitions = map[string]map[string]bool{
	StatusPending:   {StatusRunning: true, StatusCancelled: true},
	StatusRunning:   {StatusCompleted: true, StatusFailed: true, StatusCancelled: true},
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
	ID           uuid.UUID       `json:"id"`
	AgentID      uuid.UUID       `json:"agentId"`
	PipelineID   *uuid.UUID      `json:"pipelineId,omitempty"`
	Status       string          `json:"status"` // PENDING, RUNNING, COMPLETED, FAILED, CANCELLED
	Input        json.RawMessage `json:"input"`
	Output       json.RawMessage `json:"output,omitempty"`
	ErrorMessage *string         `json:"errorMessage,omitempty"`
	StartedAt    time.Time       `json:"startedAt"`
	FinishedAt   *time.Time      `json:"finishedAt,omitempty"`
	DurationMs   *int64          `json:"durationMs,omitempty"`
}

var sensitiveExecutionDiagnosticLinePattern = regexp.MustCompile(`(?im)(^|:[\t ]+)[\t ]*(?:authorization|proxy-authorization|cookie|set-cookie|x-api-key|api-key|x-auth-token|password|api[_-]?key|secret|client[_-]?secret)[\t ]*[:=][^\r\n]*`)

// PublicExecutionFrom returns a copy safe to serialize in execution listings.
func PublicExecutionFrom(execution AgentExecution) AgentExecution {
	public := execution
	if execution.ErrorMessage != nil {
		redacted := sensitiveExecutionDiagnosticLinePattern.ReplaceAllString(*execution.ErrorMessage, "$1[REDACTED]")
		public.ErrorMessage = &redacted
	}
	return public
}

// PublicExecutionNodeFrom returns a node copy safe to serialize in execution details.
func PublicExecutionNodeFrom(node AgentExecutionNode) AgentExecutionNode {
	public := node
	if node.ErrorMessage != nil {
		redacted := sensitiveExecutionDiagnosticLinePattern.ReplaceAllString(*node.ErrorMessage, "$1[REDACTED]")
		public.ErrorMessage = &redacted
	}
	return public
}

// PublicToolExecutionFrom returns a tool execution copy safe to serialize.
func PublicToolExecutionFrom(tool ToolExecution) ToolExecution {
	public := tool
	if tool.ErrorMessage != nil {
		redacted := sensitiveExecutionDiagnosticLinePattern.ReplaceAllString(*tool.ErrorMessage, "$1[REDACTED]")
		public.ErrorMessage = &redacted
	}
	return public
}

// PublicExecutionDetailsFrom returns a sanitized copy of an execution hierarchy.
func PublicExecutionDetailsFrom(details ExecutionDetails) ExecutionDetails {
	public := details
	public.AgentExecution = PublicExecutionFrom(details.AgentExecution)
	public.Nodes = make([]NodeDetails, len(details.Nodes))
	for index, node := range details.Nodes {
		public.Nodes[index] = node
		public.Nodes[index].AgentExecutionNode = PublicExecutionNodeFrom(node.AgentExecutionNode)
		public.Nodes[index].Tools = make([]ToolExecution, len(node.Tools))
		for toolIndex, tool := range node.Tools {
			public.Nodes[index].Tools[toolIndex] = PublicToolExecutionFrom(tool)
		}
	}
	return public
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
	ID           uuid.UUID       `json:"id"`
	ExecutionID  uuid.UUID       `json:"executionId"`
	NodeID       uuid.UUID       `json:"nodeId"`
	NodeType     string          `json:"nodeType"`
	Status       string          `json:"status"` // PENDING, RUNNING, SUCCESS, FAILED, SKIPPED
	Input        json.RawMessage `json:"input"`
	Output       json.RawMessage `json:"output,omitempty"`
	ErrorMessage *string         `json:"errorMessage,omitempty"`
	StartedAt    *time.Time      `json:"startedAt,omitempty"`
	FinishedAt   *time.Time      `json:"finishedAt,omitempty"`
	DurationMs   *int64          `json:"durationMs,omitempty"`
}

// ToolExecution tracks a single tool call within a node execution.
type ToolExecution struct {
	ID              uuid.UUID       `json:"id"`
	NodeExecutionID *uuid.UUID      `json:"nodeExecutionId,omitempty"`
	ToolID          uuid.UUID       `json:"toolId"`
	Status          string          `json:"status"` // RUNNING, SUCCESS, FAILED
	Input           json.RawMessage `json:"input"`
	Output          json.RawMessage `json:"output,omitempty"`
	ErrorMessage    *string         `json:"errorMessage,omitempty"`
	StartedAt       time.Time       `json:"startedAt"`
	FinishedAt      *time.Time      `json:"finishedAt,omitempty"`
	DurationMs      *int64          `json:"durationMs,omitempty"`
}
