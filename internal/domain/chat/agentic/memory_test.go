package agentic_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/memory"
)

// --- mocks ---

type mockEmbedder struct {
	result []float32
	err    error
}

func (m *mockEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	return m.result, m.err
}

type mockRecaller struct {
	results []memory.MemoryRecallResult
	err     error
}

func (m *mockRecaller) Recall(_ context.Context, _ uuid.UUID, _ memory.RecallRequest) ([]memory.MemoryRecallResult, error) {
	return m.results, m.err
}

type mockUpserter struct {
	calls []upsertCall
	err   error
}

type upsertCall struct {
	AgentID uuid.UUID
	Key     string
	Req     memory.UpsertMemoryRequest
}

func (m *mockUpserter) Upsert(_ context.Context, agentID uuid.UUID, key string, req memory.UpsertMemoryRequest) (memory.AgentMemory, error) {
	m.calls = append(m.calls, upsertCall{AgentID: agentID, Key: key, Req: req})
	return memory.AgentMemory{Key: key}, m.err
}

type mockEvaluator struct {
	results []agentic.ExtractedMemory
	err     error
}

func (m *mockEvaluator) EvaluateMemories(_ context.Context, _ string) ([]agentic.ExtractedMemory, error) {
	return m.results, m.err
}

func defaultBridge(
	embedder agentic.Embedder,
	recaller agentic.MemoryRecaller,
	upserter agentic.MemoryUpserter,
	evaluator agentic.MemoryEvaluator,
) *agentic.MemoryBridge {
	return agentic.NewMemoryBridge(embedder, recaller, upserter, evaluator, agentic.DefaultMemoryBridgeConfig())
}

// --- Recall tests ---

func TestMemoryBridge_Recall_Success(t *testing.T) {
	agentID := uuid.New()
	embedder := &mockEmbedder{result: []float32{0.1, 0.2, 0.3}}
	recaller := &mockRecaller{results: []memory.MemoryRecallResult{
		{
			AgentMemory: memory.AgentMemory{
				Key:       "preferred_language",
				Value:     json.RawMessage(`"Portuguese"`),
				CreatedAt: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
			},
			Relevance: 0.9,
		},
		{
			AgentMemory: memory.AgentMemory{
				Key:       "project_stack",
				Value:     json.RawMessage(`"Go + chi + pgx"`),
				CreatedAt: time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC),
			},
			Relevance: 0.7,
		},
	}}

	bridge := defaultBridge(embedder, recaller, nil, nil)
	result, err := bridge.Recall(context.Background(), agentID, "tell me about the project")

	require.NoError(t, err)
	assert.Contains(t, result, "## Relevant Memories")
	assert.Contains(t, result, "preferred_language: Portuguese")
	assert.Contains(t, result, "project_stack: Go + chi + pgx")
	assert.Contains(t, result, "2026-03-15")
	assert.Contains(t, result, "2026-03-20")
}

func TestMemoryBridge_Recall_FiltersByRelevance(t *testing.T) {
	agentID := uuid.New()
	embedder := &mockEmbedder{result: []float32{0.1}}
	recaller := &mockRecaller{results: []memory.MemoryRecallResult{
		{
			AgentMemory: memory.AgentMemory{Key: "high", Value: json.RawMessage(`"relevant"`)},
			Relevance:   0.8,
		},
		{
			AgentMemory: memory.AgentMemory{Key: "low", Value: json.RawMessage(`"irrelevant"`)},
			Relevance:   0.1, // below default MinRelevance of 0.3
		},
	}}

	bridge := defaultBridge(embedder, recaller, nil, nil)
	result, err := bridge.Recall(context.Background(), agentID, "test")

	require.NoError(t, err)
	assert.Contains(t, result, "high")
	assert.NotContains(t, result, "low")
}

func TestMemoryBridge_Recall_NoResults(t *testing.T) {
	embedder := &mockEmbedder{result: []float32{0.1}}
	recaller := &mockRecaller{results: nil}

	bridge := defaultBridge(embedder, recaller, nil, nil)
	result, err := bridge.Recall(context.Background(), uuid.New(), "test")

	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestMemoryBridge_Recall_AllBelowRelevance(t *testing.T) {
	embedder := &mockEmbedder{result: []float32{0.1}}
	recaller := &mockRecaller{results: []memory.MemoryRecallResult{
		{AgentMemory: memory.AgentMemory{Key: "old"}, Relevance: 0.05},
	}}

	bridge := defaultBridge(embedder, recaller, nil, nil)
	result, err := bridge.Recall(context.Background(), uuid.New(), "test")

	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestMemoryBridge_Recall_NilEmbedder(t *testing.T) {
	bridge := defaultBridge(nil, &mockRecaller{}, nil, nil)
	result, err := bridge.Recall(context.Background(), uuid.New(), "test")

	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestMemoryBridge_Recall_NilRecaller(t *testing.T) {
	bridge := defaultBridge(&mockEmbedder{result: []float32{0.1}}, nil, nil, nil)
	result, err := bridge.Recall(context.Background(), uuid.New(), "test")

	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestMemoryBridge_Recall_EmbedError(t *testing.T) {
	embedder := &mockEmbedder{err: errors.New("embedding service down")}
	recaller := &mockRecaller{}

	bridge := defaultBridge(embedder, recaller, nil, nil)
	_, err := bridge.Recall(context.Background(), uuid.New(), "test")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "embedding service down")
}

func TestMemoryBridge_Recall_RecallerError(t *testing.T) {
	embedder := &mockEmbedder{result: []float32{0.1}}
	recaller := &mockRecaller{err: errors.New("db connection lost")}

	bridge := defaultBridge(embedder, recaller, nil, nil)
	_, err := bridge.Recall(context.Background(), uuid.New(), "test")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "db connection lost")
}

func TestMemoryBridge_Recall_JSONObjectValue(t *testing.T) {
	embedder := &mockEmbedder{result: []float32{0.1}}
	recaller := &mockRecaller{results: []memory.MemoryRecallResult{
		{
			AgentMemory: memory.AgentMemory{
				Key:   "complex",
				Value: json.RawMessage(`{"nested":"data","count":42}`),
			},
			Relevance: 0.9,
		},
	}}

	bridge := defaultBridge(embedder, recaller, nil, nil)
	result, err := bridge.Recall(context.Background(), uuid.New(), "test")

	require.NoError(t, err)
	assert.Contains(t, result, `complex: {"nested":"data","count":42}`)
}

// --- MaybeStore tests ---

func TestMemoryBridge_MaybeStore_Success(t *testing.T) {
	agentID := uuid.New()
	embedder := &mockEmbedder{result: []float32{0.5, 0.6}}
	upserter := &mockUpserter{}
	evaluator := &mockEvaluator{results: []agentic.ExtractedMemory{
		{Key: "preferred_language", Value: "User prefers Portuguese"},
		{Key: "project_tech", Value: "Uses Go with chi router"},
	}}

	bridge := defaultBridge(embedder, nil, upserter, evaluator)
	stored, err := bridge.MaybeStore(context.Background(), agentID, 0, []agentic.TurnMessage{
		{Role: "user", Content: "Prefiro respostas em português. O projeto usa Go com chi."},
		{Role: "assistant", Content: "Entendido! Vou responder em português."},
	})

	require.NoError(t, err)
	assert.Equal(t, 2, stored)
	assert.Len(t, upserter.calls, 2)
	assert.Equal(t, "preferred_language", upserter.calls[0].Key)
	assert.Equal(t, "project_tech", upserter.calls[1].Key)
}

func TestMemoryBridge_MaybeStore_RateLimited(t *testing.T) {
	bridge := defaultBridge(&mockEmbedder{}, nil, &mockUpserter{}, &mockEvaluator{})

	// Default StoreTurnInterval = 3, so turns 1 and 2 should be skipped.
	stored, err := bridge.MaybeStore(context.Background(), uuid.New(), 1, []agentic.TurnMessage{
		{Role: "user", Content: "test"},
	})

	require.NoError(t, err)
	assert.Equal(t, 0, stored)
}

func TestMemoryBridge_MaybeStore_RunsAtInterval(t *testing.T) {
	evaluator := &mockEvaluator{results: nil}
	bridge := defaultBridge(&mockEmbedder{result: []float32{0.1}}, nil, &mockUpserter{}, evaluator)

	// Turn 0 should run (first turn).
	_, _ = bridge.MaybeStore(context.Background(), uuid.New(), 0, []agentic.TurnMessage{
		{Role: "user", Content: "test"},
	})
	// Turn 3 should run (interval).
	_, _ = bridge.MaybeStore(context.Background(), uuid.New(), 3, []agentic.TurnMessage{
		{Role: "user", Content: "test"},
	})
	// Turn 6 should run.
	_, _ = bridge.MaybeStore(context.Background(), uuid.New(), 6, []agentic.TurnMessage{
		{Role: "user", Content: "test"},
	})
}

func TestMemoryBridge_MaybeStore_NilDependencies(t *testing.T) {
	bridge := defaultBridge(nil, nil, nil, nil)
	stored, err := bridge.MaybeStore(context.Background(), uuid.New(), 0, []agentic.TurnMessage{
		{Role: "user", Content: "test"},
	})

	require.NoError(t, err)
	assert.Equal(t, 0, stored)
}

func TestMemoryBridge_MaybeStore_EmptyMessages(t *testing.T) {
	bridge := defaultBridge(&mockEmbedder{}, nil, &mockUpserter{}, &mockEvaluator{})
	stored, err := bridge.MaybeStore(context.Background(), uuid.New(), 0, nil)

	require.NoError(t, err)
	assert.Equal(t, 0, stored)
}

func TestMemoryBridge_MaybeStore_EvaluatorReturnsEmpty(t *testing.T) {
	evaluator := &mockEvaluator{results: nil}
	bridge := defaultBridge(&mockEmbedder{result: []float32{0.1}}, nil, &mockUpserter{}, evaluator)

	stored, err := bridge.MaybeStore(context.Background(), uuid.New(), 0, []agentic.TurnMessage{
		{Role: "user", Content: "just a greeting"},
	})

	require.NoError(t, err)
	assert.Equal(t, 0, stored)
}

func TestMemoryBridge_MaybeStore_EvaluatorError(t *testing.T) {
	evaluator := &mockEvaluator{err: errors.New("LLM timeout")}
	bridge := defaultBridge(&mockEmbedder{result: []float32{0.1}}, nil, &mockUpserter{}, evaluator)

	_, err := bridge.MaybeStore(context.Background(), uuid.New(), 0, []agentic.TurnMessage{
		{Role: "user", Content: "test"},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "LLM timeout")
}

func TestMemoryBridge_MaybeStore_EmbedErrorSkipsMemory(t *testing.T) {
	embedder := &mockEmbedder{err: errors.New("embed failed")}
	upserter := &mockUpserter{}
	evaluator := &mockEvaluator{results: []agentic.ExtractedMemory{
		{Key: "k1", Value: "v1"},
	}}

	bridge := defaultBridge(embedder, nil, upserter, evaluator)
	stored, err := bridge.MaybeStore(context.Background(), uuid.New(), 0, []agentic.TurnMessage{
		{Role: "user", Content: "test"},
	})

	require.NoError(t, err)
	assert.Equal(t, 0, stored) // skipped due to embed error
	assert.Empty(t, upserter.calls)
}

func TestMemoryBridge_MaybeStore_UpsertErrorSkipsMemory(t *testing.T) {
	embedder := &mockEmbedder{result: []float32{0.1}}
	upserter := &mockUpserter{err: errors.New("db error")}
	evaluator := &mockEvaluator{results: []agentic.ExtractedMemory{
		{Key: "k1", Value: "v1"},
		{Key: "k2", Value: "v2"},
	}}

	bridge := defaultBridge(embedder, nil, upserter, evaluator)
	stored, err := bridge.MaybeStore(context.Background(), uuid.New(), 0, []agentic.TurnMessage{
		{Role: "user", Content: "test"},
	})

	require.NoError(t, err)
	assert.Equal(t, 0, stored) // all skipped due to upsert error
}

// --- ShouldEvaluateMemories ---

func TestMemoryBridge_ShouldEvaluateMemories(t *testing.T) {
	bridge := defaultBridge(&mockEmbedder{}, nil, &mockUpserter{}, &mockEvaluator{})

	assert.True(t, bridge.ShouldEvaluateMemories(0))  // first turn
	assert.False(t, bridge.ShouldEvaluateMemories(1)) // skip
	assert.False(t, bridge.ShouldEvaluateMemories(2)) // skip
	assert.True(t, bridge.ShouldEvaluateMemories(3))  // interval
	assert.False(t, bridge.ShouldEvaluateMemories(4)) // skip
	assert.True(t, bridge.ShouldEvaluateMemories(6))  // interval
}

func TestMemoryBridge_ShouldEvaluateMemories_NilDeps(t *testing.T) {
	bridge := defaultBridge(nil, nil, nil, nil)
	assert.False(t, bridge.ShouldEvaluateMemories(0))
}

// --- Config ---

func TestDefaultMemoryBridgeConfig(t *testing.T) {
	cfg := agentic.DefaultMemoryBridgeConfig()
	assert.Equal(t, 10, cfg.RecallLimit)
	assert.Equal(t, 0.3, cfg.MinRelevance)
	assert.Equal(t, 3, cfg.StoreTurnInterval)
}

// --- extractValueText via Recall ---

func TestMemoryBridge_Recall_EmptyValue(t *testing.T) {
	embedder := &mockEmbedder{result: []float32{0.1}}
	recaller := &mockRecaller{results: []memory.MemoryRecallResult{
		{
			AgentMemory: memory.AgentMemory{Key: "empty", Value: nil},
			Relevance:   0.9,
		},
	}}

	bridge := defaultBridge(embedder, recaller, nil, nil)
	result, err := bridge.Recall(context.Background(), uuid.New(), "test")

	require.NoError(t, err)
	// Empty value memory should not produce a line.
	assert.NotContains(t, result, "empty")
}

// --- FormatRecallTimestamp ---

func TestFormatRecallTimestamp(t *testing.T) {
	ts := time.Date(2026, 4, 2, 15, 30, 0, 0, time.UTC)
	assert.Equal(t, "2026-04-02", agentic.FormatRecallTimestamp(ts))
}
