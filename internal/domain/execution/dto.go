package execution

import "encoding/json"

// StartExecutionRequest is the request body for POST /api/executions.
type StartExecutionRequest struct {
	AgentID    string          `json:"agentId"`
	PipelineID *string         `json:"pipelineId,omitempty"`
	Input      json.RawMessage `json:"input"`
}
