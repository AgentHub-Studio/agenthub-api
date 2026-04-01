package pipeline

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// CreateRequest is the payload for creating a pipeline.
type CreateRequest struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	AgentID     uuid.UUID       `json:"agentId"`
	Config      json.RawMessage `json:"config"`
}

// UpdateRequest is the payload for updating a pipeline.
type UpdateRequest struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Status      string          `json:"status"`
	Config      json.RawMessage `json:"config"`
}

// NodeRequest is used when replacing pipeline nodes.
type NodeRequest struct {
	NodeType  string          `json:"nodeType"`
	Name      string          `json:"name"`
	Config    json.RawMessage `json:"config"`
	PositionX float64         `json:"positionX"`
	PositionY float64         `json:"positionY"`
}

// EdgeRequest is used when replacing pipeline edges.
type EdgeRequest struct {
	SourceNodeID uuid.UUID `json:"sourceNodeId"`
	TargetNodeID uuid.UUID `json:"targetNodeId"`
	Label        string    `json:"label"`
}

// Response is the JSON representation of a Pipeline.
type Response struct {
	ID          uuid.UUID      `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	AgentID     uuid.UUID      `json:"agentId"`
	Status      string         `json:"status"`
	Config      any            `json:"config"`
	Nodes       []NodeResponse `json:"nodes,omitempty"`
	Edges       []EdgeResponse `json:"edges,omitempty"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

// NodeResponse is the JSON representation of a Node.
type NodeResponse struct {
	ID         uuid.UUID `json:"id"`
	PipelineID uuid.UUID `json:"pipelineId"`
	NodeType   string    `json:"nodeType"`
	Name       string    `json:"name"`
	Config     any       `json:"config"`
	PositionX  float64   `json:"positionX"`
	PositionY  float64   `json:"positionY"`
	CreatedAt  time.Time `json:"createdAt"`
}

// EdgeResponse is the JSON representation of an Edge.
type EdgeResponse struct {
	ID           uuid.UUID `json:"id"`
	PipelineID   uuid.UUID `json:"pipelineId"`
	SourceNodeID uuid.UUID `json:"sourceNodeId"`
	TargetNodeID uuid.UUID `json:"targetNodeId"`
	Label        string    `json:"label"`
	CreatedAt    time.Time `json:"createdAt"`
}

// ResponseFrom converts a Pipeline to a Response.
func ResponseFrom(p Pipeline) Response {
	var config any
	_ = json.Unmarshal(p.Config, &config)
	return Response{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		AgentID:     p.AgentID,
		Status:      p.Status,
		Config:      config,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

// NodeResponseFrom converts a Node to a NodeResponse.
func NodeResponseFrom(n Node) NodeResponse {
	var config any
	_ = json.Unmarshal(n.Config, &config)
	return NodeResponse{
		ID:         n.ID,
		PipelineID: n.PipelineID,
		NodeType:   n.NodeType,
		Name:       n.Name,
		Config:     config,
		PositionX:  n.PositionX,
		PositionY:  n.PositionY,
		CreatedAt:  n.CreatedAt,
	}
}

// EdgeResponseFrom converts an Edge to an EdgeResponse.
func EdgeResponseFrom(e Edge) EdgeResponse {
	return EdgeResponse{
		ID:           e.ID,
		PipelineID:   e.PipelineID,
		SourceNodeID: e.SourceNodeID,
		TargetNodeID: e.TargetNodeID,
		Label:        e.Label,
		CreatedAt:    e.CreatedAt,
	}
}
