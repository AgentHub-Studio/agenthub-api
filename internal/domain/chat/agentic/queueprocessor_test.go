package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- DefaultClassifier ---

func TestDefaultClassifier_SlashCommand(t *testing.T) {
	cmd := agentic.QueuedCommand{Content: "/help"}
	assert.Equal(t, agentic.StrategyIndividual, agentic.DefaultClassifier(cmd))
}

func TestDefaultClassifier_BashSource(t *testing.T) {
	cmd := agentic.QueuedCommand{Content: "ls -la", Source: "bash"}
	assert.Equal(t, agentic.StrategyIndividual, agentic.DefaultClassifier(cmd))
}

func TestDefaultClassifier_Normal(t *testing.T) {
	cmd := agentic.QueuedCommand{Content: "explain this code", Source: "user"}
	assert.Equal(t, agentic.StrategyBatch, agentic.DefaultClassifier(cmd))
}

// --- NewQueueProcessor ---

func TestNewQueueProcessor(t *testing.T) {
	q := agentic.NewCommandQueue()
	p := agentic.NewQueueProcessor(q, nil)
	assert.False(t, p.HasPending())
}

// --- Next ---

func TestQueueProcessor_Next_Empty(t *testing.T) {
	q := agentic.NewCommandQueue()
	p := agentic.NewQueueProcessor(q, nil)
	assert.Nil(t, p.Next())
}

func TestQueueProcessor_Next_SlashCommandIndividual(t *testing.T) {
	q := agentic.NewCommandQueue()
	q.Enqueue(agentic.QueuedCommand{ID: "1", Content: "/commit"})
	q.Enqueue(agentic.QueuedCommand{ID: "2", Content: "/help"})

	p := agentic.NewQueueProcessor(q, nil)

	batch := p.Next()
	require.NotNil(t, batch)
	assert.Equal(t, agentic.StrategyIndividual, batch.Strategy)
	assert.Len(t, batch.Commands, 1)
	assert.Equal(t, "/commit", batch.Commands[0].Content)

	batch = p.Next()
	require.NotNil(t, batch)
	assert.Equal(t, "/help", batch.Commands[0].Content)
}

func TestQueueProcessor_Next_BatchSameSource(t *testing.T) {
	q := agentic.NewCommandQueue()
	q.Enqueue(agentic.QueuedCommand{ID: "1", Content: "fix auth", Source: "user"})
	q.Enqueue(agentic.QueuedCommand{ID: "2", Content: "add tests", Source: "user"})
	q.Enqueue(agentic.QueuedCommand{ID: "3", Content: "deploy", Source: "system"})

	p := agentic.NewQueueProcessor(q, nil)

	batch := p.Next()
	require.NotNil(t, batch)
	assert.Equal(t, agentic.StrategyBatch, batch.Strategy)
	assert.Len(t, batch.Commands, 2, "should batch same-source commands")
	assert.Equal(t, "fix auth", batch.Commands[0].Content)
	assert.Equal(t, "add tests", batch.Commands[1].Content)

	// Next batch should be the system command (different source, but still batchable).
	batch = p.Next()
	require.NotNil(t, batch)
	assert.Len(t, batch.Commands, 1)
	assert.Equal(t, "deploy", batch.Commands[0].Content)
}

func TestQueueProcessor_Next_MixedStrategies(t *testing.T) {
	q := agentic.NewCommandQueue()
	q.Enqueue(agentic.QueuedCommand{ID: "1", Content: "explain code", Source: "user", Priority: agentic.PriorityNow})
	q.Enqueue(agentic.QueuedCommand{ID: "2", Content: "/commit", Priority: agentic.PriorityLater})

	p := agentic.NewQueueProcessor(q, nil)

	// First: the highest-priority batchable command.
	batch := p.Next()
	require.NotNil(t, batch)
	assert.Equal(t, "explain code", batch.Commands[0].Content)

	// Second: the slash command.
	batch = p.Next()
	require.NotNil(t, batch)
	assert.Equal(t, agentic.StrategyIndividual, batch.Strategy)
	assert.Equal(t, "/commit", batch.Commands[0].Content)
}

// --- DrainAll ---

func TestQueueProcessor_DrainAll(t *testing.T) {
	q := agentic.NewCommandQueue()
	q.Enqueue(agentic.QueuedCommand{ID: "1", Content: "/help"})
	q.Enqueue(agentic.QueuedCommand{ID: "2", Content: "do A", Source: "user"})
	q.Enqueue(agentic.QueuedCommand{ID: "3", Content: "do B", Source: "user"})

	p := agentic.NewQueueProcessor(q, nil)
	batches := p.DrainAll()

	assert.GreaterOrEqual(t, len(batches), 2, "should have at least 2 batches")
	assert.True(t, q.IsEmpty())
}

func TestQueueProcessor_DrainAll_Empty(t *testing.T) {
	q := agentic.NewCommandQueue()
	p := agentic.NewQueueProcessor(q, nil)
	assert.Nil(t, p.DrainAll())
}

// --- Custom classifier ---

func TestQueueProcessor_CustomClassifier(t *testing.T) {
	q := agentic.NewCommandQueue()
	q.Enqueue(agentic.QueuedCommand{ID: "1", Content: "task-a", Source: "task"})
	q.Enqueue(agentic.QueuedCommand{ID: "2", Content: "task-b", Source: "task"})

	// Classify everything as individual.
	classifier := func(cmd agentic.QueuedCommand) agentic.ProcessingStrategy {
		return agentic.StrategyIndividual
	}

	p := agentic.NewQueueProcessor(q, classifier)
	batch := p.Next()
	require.NotNil(t, batch)
	assert.Len(t, batch.Commands, 1, "custom classifier forces individual processing")

	batch = p.Next()
	require.NotNil(t, batch)
	assert.Len(t, batch.Commands, 1)
}

// --- HasPending ---

func TestQueueProcessor_HasPending(t *testing.T) {
	q := agentic.NewCommandQueue()
	p := agentic.NewQueueProcessor(q, nil)
	assert.False(t, p.HasPending())

	q.Enqueue(agentic.QueuedCommand{ID: "1", Content: "hello"})
	assert.True(t, p.HasPending())
}
