package agentic

import (
	"strings"
	"sync"
)

// Intelligent command queue processing with mode-aware batching.
//
// Inspired by Claude Code's queueProcessor.ts — processes queued
// commands with three strategies: slash commands individually,
// bash-mode individually, and other commands batched by mode.

// ProcessingStrategy determines how a command is processed.
type ProcessingStrategy string

const (
	// StrategyIndividual processes the command alone.
	StrategyIndividual ProcessingStrategy = "individual"
	// StrategyBatch combines the command with same-mode commands.
	StrategyBatch ProcessingStrategy = "batch"
)

// CommandBatch represents a group of commands to process together.
type CommandBatch struct {
	// Commands are the queued commands in this batch.
	Commands []QueuedCommand
	// Strategy is how this batch was formed.
	Strategy ProcessingStrategy
}

// CommandClassifier determines the processing strategy for a command.
type CommandClassifier func(cmd QueuedCommand) ProcessingStrategy

// DefaultClassifier classifies commands using standard rules:
// - Slash commands (starting with /) → individual
// - Commands with Source "bash" → individual
// - Everything else → batch
func DefaultClassifier(cmd QueuedCommand) ProcessingStrategy {
	if strings.HasPrefix(cmd.Content, "/") {
		return StrategyIndividual
	}
	if cmd.Source == "bash" {
		return StrategyIndividual
	}
	return StrategyBatch
}

// QueueProcessor drains commands from a CommandQueue in intelligent batches.
type QueueProcessor struct {
	mu         sync.Mutex
	queue      *CommandQueue
	classifier CommandClassifier
}

// NewQueueProcessor creates a processor for the given queue.
func NewQueueProcessor(queue *CommandQueue, classifier CommandClassifier) *QueueProcessor {
	if classifier == nil {
		classifier = DefaultClassifier
	}
	return &QueueProcessor{
		queue:      queue,
		classifier: classifier,
	}
}

// Next returns the next batch to process.
// Returns nil if the queue is empty.
func (p *QueueProcessor) Next() *CommandBatch {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Peek at the highest-priority command.
	top, ok := p.queue.Peek()
	if !ok {
		return nil
	}

	strategy := p.classifier(top)

	if strategy == StrategyIndividual {
		cmd, ok := p.queue.Dequeue()
		if !ok {
			return nil
		}
		return &CommandBatch{
			Commands: []QueuedCommand{cmd},
			Strategy: StrategyIndividual,
		}
	}

	// Batch strategy: drain all commands with the same source/mode
	// that are also classified as batch.
	topSource := top.Source
	var batch []QueuedCommand

	matching := p.queue.DequeueAllMatching(func(cmd QueuedCommand) bool {
		if p.classifier(cmd) != StrategyBatch {
			return false
		}
		return cmd.Source == topSource
	})

	batch = append(batch, matching...)

	if len(batch) == 0 {
		// Fallback: just dequeue the top command.
		cmd, ok := p.queue.Dequeue()
		if !ok {
			return nil
		}
		return &CommandBatch{
			Commands: []QueuedCommand{cmd},
			Strategy: StrategyIndividual,
		}
	}

	return &CommandBatch{
		Commands: batch,
		Strategy: StrategyBatch,
	}
}

// DrainAll processes the entire queue into batches.
func (p *QueueProcessor) DrainAll() []CommandBatch {
	var batches []CommandBatch
	for {
		batch := p.Next()
		if batch == nil {
			break
		}
		batches = append(batches, *batch)
	}
	return batches
}

// HasPending returns true if the queue has commands.
func (p *QueueProcessor) HasPending() bool {
	return !p.queue.IsEmpty()
}
