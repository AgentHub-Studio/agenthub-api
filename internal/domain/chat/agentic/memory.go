package agentic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/memory"
)

// --- Interfaces ---

// Embedder generates a vector embedding for a text string.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// MemoryRecaller searches agent memory by embedding similarity.
type MemoryRecaller interface {
	Recall(ctx context.Context, agentID uuid.UUID, req memory.RecallRequest) ([]memory.MemoryRecallResult, error)
}

// MemoryUpserter stores a memory entry.
type MemoryUpserter interface {
	Upsert(ctx context.Context, agentID uuid.UUID, key string, req memory.UpsertMemoryRequest) (memory.AgentMemory, error)
}

// MemoryEvaluator asks the LLM whether turn messages contain memorable information.
// Returns a list of memory items to store, or nil/empty if nothing is worth remembering.
type MemoryEvaluator interface {
	EvaluateMemories(ctx context.Context, prompt string) ([]ExtractedMemory, error)
}

// ExtractedMemory represents a single memory item extracted by the LLM evaluator.
type ExtractedMemory struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// --- MemoryBridge ---

// MemoryBridgeConfig holds configurable parameters for the bridge.
type MemoryBridgeConfig struct {
	// RecallLimit is the max number of memories to retrieve (default 10).
	RecallLimit int
	// MinRelevance is the minimum relevance score to include a memory (default 0.3).
	MinRelevance float64
	// StoreTurnInterval controls how often MaybeStore evaluates (every N turns, default 3).
	StoreTurnInterval int
}

// DefaultMemoryBridgeConfig returns sensible defaults.
func DefaultMemoryBridgeConfig() MemoryBridgeConfig {
	return MemoryBridgeConfig{
		RecallLimit:       10,
		MinRelevance:      0.3,
		StoreTurnInterval: 3,
	}
}

// MemoryBridge connects the agentic loop with the memory subsystem.
// It handles recall (injecting relevant memories into the system prompt)
// and store (persisting new memories discovered during conversation).
type MemoryBridge struct {
	embedder  Embedder
	recaller  MemoryRecaller
	upserter  MemoryUpserter
	evaluator MemoryEvaluator
	config    MemoryBridgeConfig
}

// NewMemoryBridge creates a MemoryBridge with the given dependencies.
func NewMemoryBridge(
	embedder Embedder,
	recaller MemoryRecaller,
	upserter MemoryUpserter,
	evaluator MemoryEvaluator,
	config MemoryBridgeConfig,
) *MemoryBridge {
	return &MemoryBridge{
		embedder:  embedder,
		recaller:  recaller,
		upserter:  upserter,
		evaluator: evaluator,
		config:    config,
	}
}

// --- Recall ---

// Recall retrieves relevant memories for the given user message and formats
// them as a markdown section ready for injection into the system prompt.
// Returns an empty string if no relevant memories are found or if any
// dependency is nil.
func (mb *MemoryBridge) Recall(ctx context.Context, agentID uuid.UUID, userMessage string) (string, error) {
	if mb.embedder == nil || mb.recaller == nil {
		return "", nil
	}

	embedding, err := mb.embedder.Embed(ctx, userMessage)
	if err != nil {
		return "", fmt.Errorf("memory bridge: embed: %w", err)
	}

	results, err := mb.recaller.Recall(ctx, agentID, memory.RecallRequest{
		Embedding: embedding,
		Limit:     mb.config.RecallLimit,
	})
	if err != nil {
		return "", fmt.Errorf("memory bridge: recall: %w", err)
	}

	// Filter by minimum relevance.
	var relevant []memory.MemoryRecallResult
	for _, r := range results {
		if r.Relevance >= mb.config.MinRelevance {
			relevant = append(relevant, r)
		}
	}

	if len(relevant) == 0 {
		return "", nil
	}

	return formatMemories(relevant), nil
}

// formatMemories builds a markdown section from recalled memories.
func formatMemories(results []memory.MemoryRecallResult) string {
	var sb strings.Builder
	sb.WriteString("## Relevant Memories\n")
	for _, r := range results {
		date := r.CreatedAt.Format("2006-01-02")
		value := extractValueText(r.Value)
		if value != "" {
			fmt.Fprintf(&sb, "- [%s] %s: %s\n", date, r.Key, value)
		}
	}
	return sb.String()
}

// extractValueText attempts to extract a readable string from the JSONB value.
// If it's a JSON string, returns it unquoted. Otherwise returns the raw JSON.
func extractValueText(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
}

// --- MaybeStore ---

// MaybeStore evaluates the turn messages and persists any discovered memories.
// It only runs every StoreTurnInterval turns to avoid excessive LLM calls.
// Returns the number of memories stored.
func (mb *MemoryBridge) MaybeStore(ctx context.Context, agentID uuid.UUID, turnIndex int, turnMessages []TurnMessage) (int, error) {
	if mb.evaluator == nil || mb.upserter == nil || mb.embedder == nil {
		return 0, nil
	}

	// Rate limit: only evaluate every N turns.
	if turnIndex > 0 && turnIndex%mb.config.StoreTurnInterval != 0 {
		return 0, nil
	}

	if len(turnMessages) == 0 {
		return 0, nil
	}

	prompt := buildEvaluationPrompt(turnMessages)
	memories, err := mb.evaluator.EvaluateMemories(ctx, prompt)
	if err != nil {
		return 0, fmt.Errorf("memory bridge: evaluate: %w", err)
	}

	if len(memories) == 0 {
		return 0, nil
	}

	stored := 0
	for _, m := range memories {
		embedding, err := mb.embedder.Embed(ctx, m.Value)
		if err != nil {
			continue // skip this memory, don't fail the whole batch
		}

		valueJSON, err := json.Marshal(m.Value)
		if err != nil {
			continue
		}

		_, err = mb.upserter.Upsert(ctx, agentID, m.Key, memory.UpsertMemoryRequest{
			Value:     valueJSON,
			Embedding: embedding,
		})
		if err != nil {
			continue
		}
		stored++
	}

	return stored, nil
}

// TurnMessage is a simplified message representation for memory evaluation.
type TurnMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// buildEvaluationPrompt creates the prompt sent to the LLM for memory extraction.
func buildEvaluationPrompt(messages []TurnMessage) string {
	var sb strings.Builder
	sb.WriteString("Analyze the following conversation turn. Extract any information about the user, ")
	sb.WriteString("their preferences, project context, or important facts that would be useful to ")
	sb.WriteString("remember in future conversations.\n\n")
	sb.WriteString("For each memory, respond with a JSON array of objects with \"key\" and \"value\" fields.\n")
	sb.WriteString("The key should be a short snake_case identifier (e.g., \"preferred_language\", \"project_stack\").\n")
	sb.WriteString("The value should be a concise description.\n\n")
	sb.WriteString("If there is nothing worth remembering, respond with an empty array: []\n\n")
	sb.WriteString("---\n")

	for _, m := range messages {
		content := m.Content
		if len(content) > 500 {
			content = content[:500] + "..."
		}
		fmt.Fprintf(&sb, "[%s] %s\n", m.Role, content)
	}

	sb.WriteString("---\n\nMemories (JSON array):")
	return sb.String()
}

// ShouldEvaluateMemories returns true if the turn index warrants a memory evaluation.
func (mb *MemoryBridge) ShouldEvaluateMemories(turnIndex int) bool {
	if mb.evaluator == nil || mb.upserter == nil || mb.embedder == nil {
		return false
	}
	return turnIndex == 0 || turnIndex%mb.config.StoreTurnInterval == 0
}

// FormatRecallTimestamp formats a time for display in the memory section.
func FormatRecallTimestamp(t time.Time) string {
	return t.Format("2006-01-02")
}
