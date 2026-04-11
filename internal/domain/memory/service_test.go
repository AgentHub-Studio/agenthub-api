package memory_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/memory"
)

// --- mock repository ---

type mockRepo struct {
	entries map[string]memory.AgentMemory
}

func newMockRepo() *mockRepo {
	return &mockRepo{entries: make(map[string]memory.AgentMemory)}
}

func entryKey(agentID uuid.UUID, key string) string {
	return agentID.String() + ":" + key
}

func (m *mockRepo) ListByAgent(_ context.Context, agentID uuid.UUID, _ *string) ([]memory.AgentMemory, error) {
	var out []memory.AgentMemory
	for _, e := range m.entries {
		if e.AgentID == agentID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (m *mockRepo) Upsert(_ context.Context, e memory.AgentMemory) (memory.AgentMemory, error) {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.LastAccessedAt.IsZero() {
		e.LastAccessedAt = time.Now()
	}
	m.entries[entryKey(e.AgentID, e.Key)] = e
	return e, nil
}

func (m *mockRepo) GetByKey(_ context.Context, agentID uuid.UUID, _ *string, key string) (memory.AgentMemory, error) {
	e, ok := m.entries[entryKey(agentID, key)]
	if !ok {
		return memory.AgentMemory{}, memory.ErrNotFound
	}
	return e, nil
}

func (m *mockRepo) DeleteByKey(_ context.Context, agentID uuid.UUID, _ *string, key string) error {
	k := entryKey(agentID, key)
	if _, ok := m.entries[k]; !ok {
		return memory.ErrNotFound
	}
	delete(m.entries, k)
	return nil
}

func (m *mockRepo) ClearByAgent(_ context.Context, agentID uuid.UUID) error {
	for k, e := range m.entries {
		if e.AgentID == agentID {
			delete(m.entries, k)
		}
	}
	return nil
}

func (m *mockRepo) Recall(_ context.Context, agentID uuid.UUID, _ *string, _ []float32, limit int, _ string, _ *uuid.UUID) ([]memory.AgentMemory, error) {
	var out []memory.AgentMemory
	for _, e := range m.entries {
		if e.AgentID == agentID && len(e.Embedding) > 0 {
			e.LastAccessedAt = time.Now()
			out = append(out, e)
		}
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (m *mockRepo) DistillExecutionMemories(_ context.Context, agentID uuid.UUID, executionID uuid.UUID) error {
	for k, e := range m.entries {
		if e.AgentID == agentID && e.ExecutionID != nil && *e.ExecutionID == executionID {
			e.Scope = memory.MemoryScopeWorkflow
			e.ExecutionID = nil
			m.entries[k] = e
		}
	}
	return nil
}

func (m *mockRepo) ListByAgentAndType(_ context.Context, agentID uuid.UUID, _ *string, memType memory.MemoryType) ([]memory.AgentMemory, error) {
	var out []memory.AgentMemory
	for _, e := range m.entries {
		if e.AgentID == agentID && e.MemoryType == memType {
			out = append(out, e)
		}
	}
	return out, nil
}

func (m *mockRepo) SearchByText(_ context.Context, agentID uuid.UUID, query string, limit int) ([]memory.AgentMemory, error) {
	var out []memory.AgentMemory
	for _, e := range m.entries {
		if e.AgentID == agentID {
			if strings.Contains(e.Key, query) || strings.Contains(string(e.Value), query) {
				out = append(out, e)
			}
		}
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (m *mockRepo) CountByType(_ context.Context, agentID uuid.UUID) (map[memory.MemoryType]int, error) {
	counts := make(map[memory.MemoryType]int)
	for _, e := range m.entries {
		if e.AgentID == agentID {
			counts[e.MemoryType]++
		}
	}
	return counts, nil
}

// --- tests ---

func TestMemoryService_UpsertAndGet(t *testing.T) {
	svc := memory.NewService(newMockRepo())
	agentID := uuid.New()

	val := json.RawMessage(`{"key":"hello"}`)
	m, err := svc.Upsert(context.Background(), agentID, "greeting", memory.UpsertMemoryRequest{Value: val})
	require.NoError(t, err)
	assert.Equal(t, "greeting", m.Key)
	assert.Equal(t, val, json.RawMessage(m.Value))
}

func TestMemoryService_Upsert_InvalidJSON_ReturnsError(t *testing.T) {
	svc := memory.NewService(newMockRepo())
	_, err := svc.Upsert(context.Background(), uuid.New(), "k", memory.UpsertMemoryRequest{
		Value: json.RawMessage(`not-json`),
	})
	require.Error(t, err)
}

func TestMemoryService_GetByKey_NotFound(t *testing.T) {
	svc := memory.NewService(newMockRepo())
	_, err := svc.GetByKey(context.Background(), uuid.New(), nil, "missing")
	require.ErrorIs(t, err, memory.ErrNotFound)
}

func TestMemoryService_DeleteByKey(t *testing.T) {
	svc := memory.NewService(newMockRepo())
	agentID := uuid.New()
	_, _ = svc.Upsert(context.Background(), agentID, "x", memory.UpsertMemoryRequest{Value: json.RawMessage(`1`)})

	require.NoError(t, svc.DeleteByKey(context.Background(), agentID, nil, "x"))
	_, err := svc.GetByKey(context.Background(), agentID, nil, "x")
	require.ErrorIs(t, err, memory.ErrNotFound)
}

func TestMemoryService_ClearByAgent(t *testing.T) {
	svc := memory.NewService(newMockRepo())
	agentID := uuid.New()
	for _, k := range []string{"a", "b", "c"} {
		_, _ = svc.Upsert(context.Background(), agentID, k, memory.UpsertMemoryRequest{Value: json.RawMessage(`1`)})
	}
	require.NoError(t, svc.ClearByAgent(context.Background(), agentID))

	items, err := svc.List(context.Background(), agentID, nil)
	require.NoError(t, err)
	assert.Empty(t, items)
}

func TestMemoryService_Recall_ReturnsResults(t *testing.T) {
	svc := memory.NewService(newMockRepo())
	agentID := uuid.New()

	embedding := []float32{0.1, 0.2, 0.3, 0.4}
	_, _ = svc.Upsert(context.Background(), agentID, "fact1", memory.UpsertMemoryRequest{
		Value:     json.RawMessage(`"Paris is the capital of France"`),
		Embedding: embedding,
	})

	results, err := svc.Recall(context.Background(), agentID, memory.RecallRequest{
		Embedding: embedding,
		Limit:     5,
	})
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.InDelta(t, 1.0, results[0].Relevance, 0.1, "recently accessed memory should have relevance near 1")
}

func TestMemoryService_Recall_EmptyEmbedding_ReturnsError(t *testing.T) {
	svc := memory.NewService(newMockRepo())
	_, err := svc.Recall(context.Background(), uuid.New(), memory.RecallRequest{Embedding: nil})
	require.Error(t, err)
}

func TestAgentMemory_RelevanceScore_Decays(t *testing.T) {
	recent := memory.AgentMemory{LastAccessedAt: time.Now()}
	old := memory.AgentMemory{LastAccessedAt: time.Now().Add(-72 * time.Hour)}

	assert.Greater(t, recent.RelevanceScore(), old.RelevanceScore())
	assert.InDelta(t, 1.0, recent.RelevanceScore(), 0.01)
}

func TestMemoryService_Upsert_ExecutionScope(t *testing.T) {
	svc := memory.NewService(newMockRepo())
	agentID := uuid.New()
	execID := uuid.New()

	val := json.RawMessage(`"execution-scoped value"`)
	m, err := svc.Upsert(context.Background(), agentID, "exec-fact", memory.UpsertMemoryRequest{
		Value:       val,
		Scope:       "execution",
		ExecutionID: &execID,
	})
	require.NoError(t, err)
	assert.Equal(t, memory.MemoryScopeExecution, m.Scope)
	assert.Equal(t, &execID, m.ExecutionID)
}

func TestMemoryService_DistillExecutionMemories(t *testing.T) {
	repo := newMockRepo()
	svc := memory.NewService(repo)
	agentID := uuid.New()
	execID := uuid.New()

	// Store an execution-scoped memory.
	_, err := svc.Upsert(context.Background(), agentID, "exec-insight", memory.UpsertMemoryRequest{
		Value:       json.RawMessage(`"discovered during run"`),
		Scope:       "execution",
		ExecutionID: &execID,
	})
	require.NoError(t, err)

	// Distill: execution → workflow scope.
	require.NoError(t, svc.DistillExecutionMemories(context.Background(), agentID, execID))

	// The entry should now be workflow-scoped with no executionID.
	e, err := svc.GetByKey(context.Background(), agentID, nil, "exec-insight")
	require.NoError(t, err)
	assert.Equal(t, memory.MemoryScopeWorkflow, e.Scope)
	assert.Nil(t, e.ExecutionID)
}
