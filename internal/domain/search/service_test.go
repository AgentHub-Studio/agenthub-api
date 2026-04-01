package search_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/search"
)

// mockSearchRepo implements search.Repository for unit tests.
type mockSearchRepo struct {
	agents []search.SearchResult
	skills []search.SearchResult
	tools  []search.SearchResult
	kbs    []search.SearchResult
	failOn string // entity type to fail on, e.g. "skill"

	// counters to verify which methods are called
	agentCalls int
	skillCalls int
	toolCalls  int
	kbCalls    int
}

func (m *mockSearchRepo) SearchAgents(_ context.Context, _, _ string, _ int) ([]search.SearchResult, error) {
	m.agentCalls++
	if m.failOn == "agent" {
		return nil, errors.New("agent search failed")
	}
	return m.agents, nil
}

func (m *mockSearchRepo) SearchSkills(_ context.Context, _, _ string, _ int) ([]search.SearchResult, error) {
	m.skillCalls++
	if m.failOn == "skill" {
		return nil, errors.New("skill search failed")
	}
	return m.skills, nil
}

func (m *mockSearchRepo) SearchTools(_ context.Context, _, _ string, _ int) ([]search.SearchResult, error) {
	m.toolCalls++
	if m.failOn == "tool" {
		return nil, errors.New("tool search failed")
	}
	return m.tools, nil
}

func (m *mockSearchRepo) SearchKnowledgeBases(_ context.Context, _, _ string, _ int) ([]search.SearchResult, error) {
	m.kbCalls++
	if m.failOn == "kb" {
		return nil, errors.New("kb search failed")
	}
	return m.kbs, nil
}

func TestSearchService_Success(t *testing.T) {
	repo := &mockSearchRepo{
		agents: []search.SearchResult{{ID: "a1", Name: "RAG Agent", Type: "agent"}},
		skills: []search.SearchResult{{ID: "s1", Name: "Email Skill", Type: "skill"}},
		tools:  []search.SearchResult{},
		kbs:    []search.SearchResult{},
	}
	svc := search.NewService(repo)
	res, err := svc.Search(context.Background(), "tenant-1", "ag", "", 10)
	require.NoError(t, err)
	assert.Equal(t, "ag", res.Query)
	assert.Len(t, res.Agents, 1)
	assert.Equal(t, "RAG Agent", res.Agents[0].Name)
	assert.Len(t, res.Skills, 1)
	assert.Empty(t, res.Tools)
	assert.Empty(t, res.KnowledgeBases)
}

func TestSearchService_DefaultLimit(t *testing.T) {
	repo := &mockSearchRepo{}
	svc := search.NewService(repo)
	_, err := svc.Search(context.Background(), "t1", "q", "", 0)
	require.NoError(t, err)
}

func TestSearchService_ErrorPropagates(t *testing.T) {
	repo := &mockSearchRepo{failOn: "skill"}
	svc := search.NewService(repo)
	_, err := svc.Search(context.Background(), "t1", "q", "", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "skill search failed")
}

func TestSearchService_EmptyQuery_ReturnsEmpty(t *testing.T) {
	repo := &mockSearchRepo{
		agents: []search.SearchResult{},
		skills: []search.SearchResult{},
		tools:  []search.SearchResult{},
		kbs:    []search.SearchResult{},
	}
	svc := search.NewService(repo)
	res, err := svc.Search(context.Background(), "t1", "", "", 5)
	require.NoError(t, err)
	assert.Empty(t, res.Agents)
	assert.Empty(t, res.Skills)
}

func TestSearchService_EntityTypeFilter_OnlySearchesAgent(t *testing.T) {
	repo := &mockSearchRepo{
		agents: []search.SearchResult{{ID: "a1", Name: "My Agent", Type: "agent"}},
	}
	svc := search.NewService(repo)
	res, err := svc.Search(context.Background(), "t1", "agent query", search.EntityAgent, 5)
	require.NoError(t, err)
	assert.Len(t, res.Agents, 1)
	assert.Empty(t, res.Skills)
	assert.Empty(t, res.Tools)
	assert.Empty(t, res.KnowledgeBases)
	// Only agent search should have been called.
	assert.Equal(t, 1, repo.agentCalls)
	assert.Equal(t, 0, repo.skillCalls)
	assert.Equal(t, 0, repo.toolCalls)
	assert.Equal(t, 0, repo.kbCalls)
}

func TestSearchService_EntityTypeFilter_OnlySearchesSkill(t *testing.T) {
	repo := &mockSearchRepo{
		skills: []search.SearchResult{{ID: "s1", Name: "Email Skill", Type: "skill"}},
	}
	svc := search.NewService(repo)
	res, err := svc.Search(context.Background(), "t1", "email", search.EntitySkill, 5)
	require.NoError(t, err)
	assert.Empty(t, res.Agents)
	assert.Len(t, res.Skills, 1)
	assert.Equal(t, 0, repo.agentCalls)
	assert.Equal(t, 1, repo.skillCalls)
}

func TestSearchService_TotalResults(t *testing.T) {
	repo := &mockSearchRepo{
		agents: []search.SearchResult{{ID: "a1", Name: "Agent 1"}, {ID: "a2", Name: "Agent 2"}},
		skills: []search.SearchResult{{ID: "s1", Name: "Skill 1"}},
		tools:  []search.SearchResult{},
		kbs:    []search.SearchResult{},
	}
	svc := search.NewService(repo)
	res, err := svc.Search(context.Background(), "t1", "query", "", 10)
	require.NoError(t, err)
	assert.Equal(t, 3, res.TotalResults, "TotalResults should sum all result counts")
}


func TestSearchService_EntityTypeFilter_Empty_SearchesAll(t *testing.T) {
	repo := &mockSearchRepo{
		agents: []search.SearchResult{{ID: "a1", Name: "Agent"}},
		skills: []search.SearchResult{{ID: "s1", Name: "Skill"}},
	}
	svc := search.NewService(repo)
	_, err := svc.Search(context.Background(), "t1", "query", "", 5)
	require.NoError(t, err)
	// All entity types should be searched when filter is empty.
	assert.Equal(t, 1, repo.agentCalls)
	assert.Equal(t, 1, repo.skillCalls)
	assert.Equal(t, 1, repo.toolCalls)
	assert.Equal(t, 1, repo.kbCalls)
}
