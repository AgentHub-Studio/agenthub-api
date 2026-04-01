package pipeline

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// Service holds business logic for pipelines.
type Service struct {
	repo PipelineRepository
}

// NewService creates a new Service.
func NewService(repo PipelineRepository) *Service {
	return &Service{repo: repo}
}

// List returns a paginated list of pipelines.
func (s *Service) List(ctx context.Context, req pagination.PageRequest) (pagination.Page[Response], error) {
	pipelines, total, err := s.repo.List(ctx, req)
	if err != nil {
		return pagination.Page[Response]{}, err
	}
	content := make([]Response, len(pipelines))
	for i, p := range pipelines {
		content[i] = ResponseFrom(p)
	}
	return pagination.NewPage(content, total, req), nil
}

// Create creates a new pipeline.
func (s *Service) Create(ctx context.Context, req CreateRequest) (Response, error) {
	cfg := []byte("{}")
	if len(req.Config) > 0 {
		cfg = req.Config
	}
	p := Pipeline{
		Name:        req.Name,
		Description: req.Description,
		AgentID:     req.AgentID,
		Status:      "DRAFT",
		Config:      cfg,
	}
	created, err := s.repo.Create(ctx, p)
	if err != nil {
		return Response{}, err
	}
	return ResponseFrom(created), nil
}

// GetByID returns a pipeline with nodes and edges.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (Response, error) {
	p, nodes, edges, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Response{}, err
	}
	resp := ResponseFrom(p)
	resp.Nodes = make([]NodeResponse, len(nodes))
	for i, n := range nodes {
		resp.Nodes[i] = NodeResponseFrom(n)
	}
	resp.Edges = make([]EdgeResponse, len(edges))
	for i, e := range edges {
		resp.Edges[i] = EdgeResponseFrom(e)
	}
	return resp, nil
}

// Update updates a pipeline.
func (s *Service) Update(ctx context.Context, id uuid.UUID, req UpdateRequest) (Response, error) {
	updated, err := s.repo.Update(ctx, id, req)
	if err != nil {
		return Response{}, err
	}
	return ResponseFrom(updated), nil
}

// Delete deletes a pipeline.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// ReplaceNodes replaces all nodes for a pipeline.
// Validates that node names are unique within the pipeline.
func (s *Service) ReplaceNodes(ctx context.Context, pipelineID uuid.UUID, nodes []NodeRequest) ([]NodeResponse, error) {
	if err := validateUniqueNodeNames(nodes); err != nil {
		return nil, err
	}
	result, err := s.repo.ReplaceNodes(ctx, pipelineID, nodes)
	if err != nil {
		return nil, err
	}
	resp := make([]NodeResponse, len(result))
	for i, n := range result {
		resp[i] = NodeResponseFrom(n)
	}
	return resp, nil
}

// ReplaceEdges replaces all edges for a pipeline.
// Validates that the resulting graph has no cycles.
func (s *Service) ReplaceEdges(ctx context.Context, pipelineID uuid.UUID, edges []EdgeRequest) ([]EdgeResponse, error) {
	if hasCycle(edges) {
		return nil, fmt.Errorf("%w: edges form a cycle", ErrCyclicGraph)
	}
	result, err := s.repo.ReplaceEdges(ctx, pipelineID, edges)
	if err != nil {
		return nil, err
	}
	resp := make([]EdgeResponse, len(result))
	for i, e := range result {
		resp[i] = EdgeResponseFrom(e)
	}
	return resp, nil
}

// validateUniqueNodeNames returns ErrDuplicateNodeName if any two nodes share the same name.
func validateUniqueNodeNames(nodes []NodeRequest) error {
	seen := make(map[string]struct{}, len(nodes))
	for _, n := range nodes {
		if _, exists := seen[n.Name]; exists {
			return fmt.Errorf("%w: %q", ErrDuplicateNodeName, n.Name)
		}
		seen[n.Name] = struct{}{}
	}
	return nil
}

// hasCycle detects cycles in a directed graph represented by EdgeRequests.
// Uses DFS with three-color marking: white (unvisited), gray (in stack), black (done).
func hasCycle(edges []EdgeRequest) bool {
	// Build adjacency list.
	adj := make(map[uuid.UUID][]uuid.UUID)
	nodes := make(map[uuid.UUID]struct{})
	for _, e := range edges {
		adj[e.SourceNodeID] = append(adj[e.SourceNodeID], e.TargetNodeID)
		nodes[e.SourceNodeID] = struct{}{}
		nodes[e.TargetNodeID] = struct{}{}
	}

	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[uuid.UUID]int, len(nodes))

	var dfs func(n uuid.UUID) bool
	dfs = func(n uuid.UUID) bool {
		color[n] = gray
		for _, next := range adj[n] {
			if color[next] == gray {
				return true // back edge → cycle
			}
			if color[next] == white && dfs(next) {
				return true
			}
		}
		color[n] = black
		return false
	}

	for n := range nodes {
		if color[n] == white && dfs(n) {
			return true
		}
	}
	return false
}
