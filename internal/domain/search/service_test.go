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
	agents  []search.SearchResult
	skills  []search.SearchResult
	tools   []search.SearchResult
	kbs     []search.SearchResult
	failOn  string // entity type to fail on, e.g. "skill"
}

func (m *mockSearchRepo) SearchAgents(_ context.Context, _, _ string, _ int) ([]search.SearchResult, error) {
	if m.failOn == "agent" {
		return nil, errors.New("agent search failed")
	}
	return m.agents, nil
}

func (m *mockSearchRepo) SearchSkills(_ context.Context, _, _ string, _ int) ([]search.SearchResult, error) {
	if m.failOn == "skill" {
		return nil, errors.New("skill search failed")
	}
	return m.skills, nil
}

func (m *mockSearchRepo) SearchTools(_ context.Context, _, _ string, _ int) ([]search.SearchResult, error) {
	if m.failOn == "tool" {
		return nil, errors.New("tool search failed")
	}
	return m.tools, nil
}

func (m *mockSearchRepo) SearchKnowledgeBases(_ context.Context, _, _ string, _ int) ([]search.SearchResult, error) {
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
	res, err := svc.Search(context.Background(), "tenant-1", "ag", 10)
	require.NoError(t, err)
	assert.Equal(t, "ag", res.Query)
	assert.Len(t, res.Agents, 1)
	assert.Equal(t, "RAG Agent", res.Agents[0].Name)
	assert.Len(t, res.Skills, 1)
	assert.Empty(t, res.Tools)
	assert.Empty(t, res.KnowledgeBases)
}

func TestSearchService_DefaultLimit(t *testing.T) {
	called := false
	repo := &mockSearchRepo{}
	// Just verify it doesn't error when limit=0.
	svc := search.NewService(repo)
	_, err := svc.Search(context.Background(), "t1", "q", 0)
	require.NoError(t, err)
	_ = called
}

func TestSearchService_ErrorPropagates(t *testing.T) {
	repo := &mockSearchRepo{failOn: "skill"}
	svc := search.NewService(repo)
	_, err := svc.Search(context.Background(), "t1", "q", 5)
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
	res, err := svc.Search(context.Background(), "t1", "", 5)
	require.NoError(t, err)
	assert.Empty(t, res.Agents)
	assert.Empty(t, res.Skills)
}
