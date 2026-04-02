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
	return EdgeResponse(e)
}

// GraphNodeResponse is the frontend-compatible node format used by the /graph endpoint.
// Uses "type" (not "nodeType") and a nested "position" object.
type GraphNodeResponse struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Label    string         `json:"label,omitempty"`
	Position struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	} `json:"position"`
	Config any `json:"config"`
}

// GraphEdgeResponse is the frontend-compatible edge format.
type GraphEdgeResponse struct {
	ID           string `json:"id"`
	SourceNodeID string `json:"sourceNodeId"`
	TargetNodeID string `json:"targetNodeId"`
	Label        string `json:"label,omitempty"`
}

// GraphResponse is the payload returned by GET /api/pipelines/{id}/graph.
type GraphResponse struct {
	Nodes        []GraphNodeResponse `json:"nodes"`
	Edges        []GraphEdgeResponse `json:"edges"`
	BlocklyState json.RawMessage     `json:"blocklyState,omitempty"`
}

// GraphRequest is the payload accepted by PUT /api/pipelines/{id}/graph.
type GraphRequest struct {
	Nodes        []GraphNodeRequest `json:"nodes"`
	Edges        []GraphEdgeRequest `json:"edges"`
	BlocklyState json.RawMessage    `json:"blocklyState,omitempty"`
}

// GraphNodeRequest is the frontend-compatible node format for graph updates.
type GraphNodeRequest struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Label    string `json:"label,omitempty"`
	Position struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	} `json:"position"`
	Config json.RawMessage `json:"config"`
}

// GraphEdgeRequest is the frontend-compatible edge format for graph updates.
type GraphEdgeRequest struct {
	ID           string `json:"id"`
	SourceNodeID string `json:"sourceNodeId"`
	TargetNodeID string `json:"targetNodeId"`
	Label        string `json:"label,omitempty"`
}

// GraphResponseFrom converts NodeResponse/EdgeResponse slices to GraphResponse.
func GraphResponseFrom(nodes []NodeResponse, edges []EdgeResponse, blocklyState json.RawMessage) GraphResponse {
	gNodes := make([]GraphNodeResponse, len(nodes))
	for i, n := range nodes {
		gn := GraphNodeResponse{
			ID:     n.ID.String(),
			Type:   n.NodeType,
			Label:  n.Name,
			Config: n.Config,
		}
		gn.Position.X = n.PositionX
		gn.Position.Y = n.PositionY
		gNodes[i] = gn
	}

	gEdges := make([]GraphEdgeResponse, len(edges))
	for i, e := range edges {
		gEdges[i] = GraphEdgeResponse{
			ID:           e.ID.String(),
			SourceNodeID: e.SourceNodeID.String(),
			TargetNodeID: e.TargetNodeID.String(),
			Label:        e.Label,
		}
	}

	return GraphResponse{Nodes: gNodes, Edges: gEdges, BlocklyState: blocklyState}
}
