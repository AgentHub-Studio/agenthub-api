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
// Supports both the new four-type taxonomy format (type/key/value) and the
// legacy format (memoryType/key/value). When both are set, Type takes precedence.
// Inspired by Claude Code's four-type taxonomy: user|feedback|project|reference.
type ExtractedMemory struct {
	Key        string `json:"key"`
	Value      string `json:"value"`
	// Type is the canonical field name matching the memory_eval_prompt template output.
	Type string `json:"type,omitempty"` // user|feedback|project|reference
	// MemoryType is the legacy field name kept for backwards compatibility.
	MemoryType string `json:"memoryType,omitempty"` // deprecated: use Type
}

// resolvedType returns the effective memory type, preferring Type over MemoryType.
func (e ExtractedMemory) resolvedType() string {
	if e.Type != "" {
		return e.Type
	}
	if e.MemoryType != "" {
		return e.MemoryType
	}
	return "general"
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

// memoryTypeLabel returns a human-readable section header for a memory type.
func memoryTypeLabel(mt memory.MemoryType) string {
	switch mt {
	case memory.MemoryTypeUser:
		return "User Profile"
	case memory.MemoryTypeFeedback:
		return "Behavioral Guidance"
	case memory.MemoryTypeProject:
		return "Project Context"
	case memory.MemoryTypeReference:
		return "External References"
	default:
		return "General"
	}
}

// formatMemories builds a markdown section from recalled memories, grouped by type.
func formatMemories(results []memory.MemoryRecallResult) string {
	// Group by type for structured output.
	grouped := map[memory.MemoryType][]memory.MemoryRecallResult{}
	typeOrder := []memory.MemoryType{
		memory.MemoryTypeFeedback,
		memory.MemoryTypeUser,
		memory.MemoryTypeProject,
		memory.MemoryTypeReference,
		memory.MemoryTypeGeneral,
	}
	for _, r := range results {
		mt := r.MemoryType
		if mt == "" {
			mt = memory.MemoryTypeGeneral
		}
		grouped[mt] = append(grouped[mt], r)
	}

	var sb strings.Builder
	sb.WriteString("## Relevant Memories\n")

	for _, mt := range typeOrder {
		items, ok := grouped[mt]
		if !ok || len(items) == 0 {
			continue
		}
		fmt.Fprintf(&sb, "\n### %s\n", memoryTypeLabel(mt))
		for _, r := range items {
			date := r.CreatedAt.Format("2006-01-02")
			value := extractValueText(r.Value)
			if value != "" {
				// Truncate long memories to 200 chars.
				if len(value) > 200 {
					value = value[:200] + "..."
				}
				fmt.Fprintf(&sb, "- [%s] %s: %s\n", date, r.Key, value)
			}
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

		memType := m.resolvedType()

		_, err = mb.upserter.Upsert(ctx, agentID, m.Key, memory.UpsertMemoryRequest{
			Value:      valueJSON,
			MemoryType: memType,
			Embedding:  embedding,
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

// buildEvaluationPrompt creates the prompt for memory extraction by injecting
// the conversation into the memory_eval_prompt template's {{conversation}} slot.
// The template uses CC's four-type taxonomy (user|feedback|project|reference).
func buildEvaluationPrompt(messages []TurnMessage) string {
	var conv strings.Builder
	for _, m := range messages {
		content := m.Content
		if len(content) > 500 {
			content = content[:500] + "..."
		}
		fmt.Fprintf(&conv, "[%s] %s\n", m.Role, content)
	}
	// The template store is embedded; load it lazily via the package-level store.
	// Fall back to inline prompt if the store is unavailable (e.g. in unit tests).
	if tmpl, ok := loadMemoryEvalTemplate(); ok {
		return strings.ReplaceAll(tmpl, "{{conversation}}", conv.String())
	}

	// Fallback: minimal inline prompt (used only when embedded FS is unavailable).
	var sb strings.Builder
	sb.WriteString("Extract memorable information from this conversation. ")
	sb.WriteString("Respond with a JSON array of {\"type\",\"key\",\"value\"} objects ")
	sb.WriteString("(type: user|feedback|project|reference). Return [] if nothing is memorable.\n\n---\n")
	sb.WriteString(conv.String())
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

// loadMemoryEvalTemplate returns the embedded memory_eval_prompt template content.
// Returns (content, true) on success, ("", false) if the embedded FS is unavailable.
func loadMemoryEvalTemplate() (string, bool) {
	data, err := templateFS.ReadFile("templates/memory_eval_prompt.txt")
	if err != nil {
		return "", false
	}
	return string(data), true
}
