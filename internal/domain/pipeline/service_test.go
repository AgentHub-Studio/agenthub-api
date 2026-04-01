package pipeline_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/pipeline"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

type mockPipelineRepo struct {
	data  map[uuid.UUID]pipeline.Pipeline
	nodes map[uuid.UUID][]pipeline.Node
	edges map[uuid.UUID][]pipeline.Edge
}

func newMockRepo() *mockPipelineRepo {
	return &mockPipelineRepo{
		data:  make(map[uuid.UUID]pipeline.Pipeline),
		nodes: make(map[uuid.UUID][]pipeline.Node),
		edges: make(map[uuid.UUID][]pipeline.Edge),
	}
}

func (m *mockPipelineRepo) List(_ context.Context, _ pagination.PageRequest) ([]pipeline.Pipeline, int64, error) {
	out := make([]pipeline.Pipeline, 0, len(m.data))
	for _, p := range m.data {
		out = append(out, p)
	}
	return out, int64(len(out)), nil
}

func (m *mockPipelineRepo) Create(_ context.Context, p pipeline.Pipeline) (pipeline.Pipeline, error) {
	p.ID = uuid.New()
	m.data[p.ID] = p
	return p, nil
}

func (m *mockPipelineRepo) GetByID(_ context.Context, id uuid.UUID) (pipeline.Pipeline, []pipeline.Node, []pipeline.Edge, error) {
	p, ok := m.data[id]
	if !ok {
		return pipeline.Pipeline{}, nil, nil, pipeline.ErrNotFound
	}
	return p, m.nodes[id], m.edges[id], nil
}

func (m *mockPipelineRepo) Update(_ context.Context, id uuid.UUID, req pipeline.UpdateRequest) (pipeline.Pipeline, error) {
	p, ok := m.data[id]
	if !ok {
		return pipeline.Pipeline{}, pipeline.ErrNotFound
	}
	p.Name = req.Name
	p.Description = req.Description
	m.data[id] = p
	return p, nil
}

func (m *mockPipelineRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := m.data[id]; !ok {
		return pipeline.ErrNotFound
	}
	delete(m.data, id)
	return nil
}

func (m *mockPipelineRepo) ReplaceNodes(_ context.Context, pipelineID uuid.UUID, nodes []pipeline.NodeRequest) ([]pipeline.Node, error) {
	out := make([]pipeline.Node, len(nodes))
	for i, n := range nodes {
		out[i] = pipeline.Node{
			ID:         uuid.New(),
			PipelineID: pipelineID,
			NodeType:   n.NodeType,
			Name:       n.Name,
		}
	}
	m.nodes[pipelineID] = out
	return out, nil
}

func (m *mockPipelineRepo) ReplaceEdges(_ context.Context, pipelineID uuid.UUID, edges []pipeline.EdgeRequest) ([]pipeline.Edge, error) {
	out := make([]pipeline.Edge, len(edges))
	for i, e := range edges {
		out[i] = pipeline.Edge{
			ID:           uuid.New(),
			PipelineID:   pipelineID,
			SourceNodeID: e.SourceNodeID,
			TargetNodeID: e.TargetNodeID,
		}
	}
	m.edges[pipelineID] = out
	return out, nil
}

func TestPipelineService_Create_Success(t *testing.T) {
	svc := pipeline.NewService(newMockRepo())
	p, err := svc.Create(context.Background(), pipeline.CreateRequest{
		Name:    "RAG Pipeline",
		AgentID: uuid.New(),
	})
	require.NoError(t, err)
	assert.Equal(t, "RAG Pipeline", p.Name)
	assert.NotEqual(t, uuid.Nil, p.ID)
}

func TestPipelineService_GetByID_NotFound(t *testing.T) {
	svc := pipeline.NewService(newMockRepo())
	_, err := svc.GetByID(context.Background(), uuid.New())
	require.ErrorIs(t, err, pipeline.ErrNotFound)
}

func TestPipelineService_Delete_NotFound(t *testing.T) {
	svc := pipeline.NewService(newMockRepo())
	err := svc.Delete(context.Background(), uuid.New())
	require.ErrorIs(t, err, pipeline.ErrNotFound)
}

func TestPipelineService_ReplaceNodes(t *testing.T) {
	svc := pipeline.NewService(newMockRepo())
	p, err := svc.Create(context.Background(), pipeline.CreateRequest{Name: "Pipeline", AgentID: uuid.New()})
	require.NoError(t, err)
	nodes, err := svc.ReplaceNodes(context.Background(), p.ID, []pipeline.NodeRequest{
		{NodeType: "INPUT", Name: "Start"},
		{NodeType: "LLM", Name: "Generate"},
		{NodeType: "OUTPUT", Name: "End"},
	})
	require.NoError(t, err)
	assert.Len(t, nodes, 3)
}

func TestPipelineService_ReplaceNodes_DuplicateName(t *testing.T) {
	svc := pipeline.NewService(newMockRepo())
	p, err := svc.Create(context.Background(), pipeline.CreateRequest{Name: "Pipeline", AgentID: uuid.New()})
	require.NoError(t, err)

	_, err = svc.ReplaceNodes(context.Background(), p.ID, []pipeline.NodeRequest{
		{NodeType: "INPUT", Name: "Start"},
		{NodeType: "LLM", Name: "Start"}, // duplicate
	})
	require.ErrorIs(t, err, pipeline.ErrDuplicateNodeName)
}

func TestPipelineService_ReplaceEdges_Cycle(t *testing.T) {
	svc := pipeline.NewService(newMockRepo())
	p, err := svc.Create(context.Background(), pipeline.CreateRequest{Name: "Pipeline", AgentID: uuid.New()})
	require.NoError(t, err)

	a, b, c := uuid.New(), uuid.New(), uuid.New()
	_, err = svc.ReplaceEdges(context.Background(), p.ID, []pipeline.EdgeRequest{
		{SourceNodeID: a, TargetNodeID: b},
		{SourceNodeID: b, TargetNodeID: c},
		{SourceNodeID: c, TargetNodeID: a}, // creates cycle a→b→c→a
	})
	require.ErrorIs(t, err, pipeline.ErrCyclicDependency)
}

func TestPipelineService_ReplaceEdges_NoCycle(t *testing.T) {
	svc := pipeline.NewService(newMockRepo())
	p, err := svc.Create(context.Background(), pipeline.CreateRequest{Name: "Pipeline", AgentID: uuid.New()})
	require.NoError(t, err)

	a, b, c := uuid.New(), uuid.New(), uuid.New()
	edges, err := svc.ReplaceEdges(context.Background(), p.ID, []pipeline.EdgeRequest{
		{SourceNodeID: a, TargetNodeID: b},
		{SourceNodeID: b, TargetNodeID: c},
	})
	require.NoError(t, err)
	assert.Len(t, edges, 2)
}

func TestPipelineService_List(t *testing.T) {
	svc := pipeline.NewService(newMockRepo())
	for i := 0; i < 3; i++ {
		_, err := svc.Create(context.Background(), pipeline.CreateRequest{Name: "Pipeline", AgentID: uuid.New()})
		require.NoError(t, err)
	}
	page, err := svc.List(context.Background(), pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(3), page.TotalElements)
}

// --- Cycle detection tests ---

func makeEdge(src, tgt uuid.UUID) pipeline.EdgeRequest {
	return pipeline.EdgeRequest{SourceNodeID: src, TargetNodeID: tgt}
}

func TestReplaceEdges_NoCycle(t *testing.T) {
	repo := newMockRepo()
	svc := pipeline.NewService(repo)
	pid := uuid.New()
	repo.data[pid] = pipeline.Pipeline{ID: pid}

	a, b, c := uuid.New(), uuid.New(), uuid.New()
	edges := []pipeline.EdgeRequest{makeEdge(a, b), makeEdge(b, c)}
	_, err := svc.ReplaceEdges(context.Background(), pid, edges)
	require.NoError(t, err)
}

func TestReplaceEdges_DirectCycle(t *testing.T) {
	repo := newMockRepo()
	svc := pipeline.NewService(repo)
	pid := uuid.New()
	repo.data[pid] = pipeline.Pipeline{ID: pid}

	a, b := uuid.New(), uuid.New()
	edges := []pipeline.EdgeRequest{makeEdge(a, b), makeEdge(b, a)}
	_, err := svc.ReplaceEdges(context.Background(), pid, edges)
	require.Error(t, err)
	assert.ErrorIs(t, err, pipeline.ErrCyclicGraph)
}

func TestReplaceEdges_IndirectCycle(t *testing.T) {
	repo := newMockRepo()
	svc := pipeline.NewService(repo)
	pid := uuid.New()
	repo.data[pid] = pipeline.Pipeline{ID: pid}

	a, b, c := uuid.New(), uuid.New(), uuid.New()
	// A→B→C→A
	edges := []pipeline.EdgeRequest{makeEdge(a, b), makeEdge(b, c), makeEdge(c, a)}
	_, err := svc.ReplaceEdges(context.Background(), pid, edges)
	require.Error(t, err)
	assert.ErrorIs(t, err, pipeline.ErrCyclicGraph)
}

func TestReplaceEdges_EmptyEdges(t *testing.T) {
	repo := newMockRepo()
	svc := pipeline.NewService(repo)
	pid := uuid.New()
	repo.data[pid] = pipeline.Pipeline{ID: pid}

	_, err := svc.ReplaceEdges(context.Background(), pid, nil)
	require.NoError(t, err)
}

// --- Duplicate node name tests ---

func TestReplaceNodes_UniqueNames(t *testing.T) {
	repo := newMockRepo()
	svc := pipeline.NewService(repo)
	pid := uuid.New()
	repo.data[pid] = pipeline.Pipeline{ID: pid}

	nodes := []pipeline.NodeRequest{
		{Name: "input", NodeType: "INPUT"},
		{Name: "llm", NodeType: "LLM"},
	}
	_, err := svc.ReplaceNodes(context.Background(), pid, nodes)
	require.NoError(t, err)
}

func TestReplaceNodes_DuplicateNames(t *testing.T) {
	repo := newMockRepo()
	svc := pipeline.NewService(repo)
	pid := uuid.New()
	repo.data[pid] = pipeline.Pipeline{ID: pid}

	nodes := []pipeline.NodeRequest{
		{Name: "node", NodeType: "INPUT"},
		{Name: "node", NodeType: "OUTPUT"}, // duplicate
	}
	_, err := svc.ReplaceNodes(context.Background(), pid, nodes)
	require.Error(t, err)
	assert.ErrorIs(t, err, pipeline.ErrDuplicateNodeName)
}
