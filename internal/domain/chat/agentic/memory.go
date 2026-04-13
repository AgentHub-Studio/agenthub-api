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

// MemoryLister lists all memories for an agent. Used as a fallback when semantic
// recall yields no results but memories exist (e.g. exact-code recall, BUG-MEM7 fix).
type MemoryLister interface {
	List(ctx context.Context, agentID uuid.UUID, userID *string) ([]memory.AgentMemory, error)
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
	// RecentFallbackLimit is the max number of recent memories to include when semantic
	// recall returns nothing. Helps surface exact-code or low-entropy memories that
	// score poorly on cosine similarity (BUG-MEM7 fix). Zero disables the fallback.
	RecentFallbackLimit int
}

// DefaultMemoryBridgeConfig returns sensible defaults.
func DefaultMemoryBridgeConfig() MemoryBridgeConfig {
	return MemoryBridgeConfig{
		RecallLimit:         10,
		MinRelevance:        0.3,
		StoreTurnInterval:   3,
		RecentFallbackLimit: 5, // include up to 5 recent memories when semantic search yields nothing
	}
}

// MemoryDistiller promotes execution-scoped memories to workflow scope.
type MemoryDistiller interface {
	DistillExecutionMemories(ctx context.Context, agentID uuid.UUID, executionID uuid.UUID) error
}

// MemoryBridge connects the agentic loop with the memory subsystem.
// It handles recall (injecting relevant memories into the system prompt)
// and store (persisting new memories discovered during conversation).
type MemoryBridge struct {
	embedder    Embedder
	recaller    MemoryRecaller
	lister      MemoryLister  // optional: used for recent-memory fallback (BUG-MEM7)
	upserter    MemoryUpserter
	evaluator   MemoryEvaluator
	distiller   MemoryDistiller
	config      MemoryBridgeConfig
	executionID *uuid.UUID // when set, memories are scoped to this execution
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

// WithLister sets the MemoryLister used for recent-memory fallback when semantic
// recall returns no results (BUG-MEM7 fix).
func (mb *MemoryBridge) WithLister(l MemoryLister) *MemoryBridge {
	mb.lister = l
	return mb
}

// WithDistiller sets the distiller used to promote execution memories at run end.
func (mb *MemoryBridge) WithDistiller(d MemoryDistiller) *MemoryBridge {
	mb.distiller = d
	return mb
}

// WithExecutionID tags all memories written during this run as execution-scoped,
// enabling fine-grained distillation at the end of the execution.
func (mb *MemoryBridge) WithExecutionID(id uuid.UUID) *MemoryBridge {
	mb.executionID = &id
	return mb
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

	recallReq := memory.RecallRequest{
		Embedding:   embedding,
		Limit:       mb.config.RecallLimit,
		ExecutionID: mb.executionID,
	}
	// When running within an execution context, include both agent-scope memories
	// and execution-scope memories so the runner has full context.
	// (No explicit scope filter means all scopes are recalled.)
	results, err := mb.recaller.Recall(ctx, agentID, recallReq)
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

	if len(relevant) > 0 {
		return formatMemories(relevant), nil
	}

	// BUG-MEM7 fix: semantic recall yielded nothing. Fall back to the most recently
	// stored memories so that low-entropy values (codes, IDs, exact strings) that
	// score poorly on cosine similarity are still injected into the prompt.
	// This ensures the LLM has context to answer recall questions without calling ask_user.
	if mb.lister != nil && mb.config.RecentFallbackLimit > 0 {
		allMemories, listErr := mb.lister.List(ctx, agentID, nil)
		if listErr == nil && len(allMemories) > 0 {
			limit := mb.config.RecentFallbackLimit
			if limit > len(allMemories) {
				limit = len(allMemories)
			}
			recent := allMemories[:limit] // List returns newest-first
			var fallback []memory.MemoryRecallResult
			for _, m := range recent {
				fallback = append(fallback, memory.MemoryRecallResult{
					AgentMemory: m,
					Relevance:   0, // below threshold but included as recent fallback
				})
			}
			return formatMemories(fallback), nil
		}
	}

	return "", nil
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
	// P-MEM-2: normalize unknown memory types to "general" so they appear in
	// the output. Only the 5 canonical types are iterated in typeOrder; any
	// custom type (e.g. "preference") would otherwise be silently dropped.
	knownTypes := map[memory.MemoryType]bool{
		memory.MemoryTypeFeedback:  true,
		memory.MemoryTypeUser:      true,
		memory.MemoryTypeProject:   true,
		memory.MemoryTypeReference: true,
		memory.MemoryTypeGeneral:   true,
	}
	for _, r := range results {
		mt := r.MemoryType
		if mt == "" || !knownTypes[mt] {
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

		scope := "agent"
		if mb.executionID != nil {
			scope = "execution"
		}

		// P-C333-1 (ACT-F3-10): normalize key before persisting.
		_, err = mb.upserter.Upsert(ctx, agentID, normalizeKey(m.Key), memory.UpsertMemoryRequest{
			Value:       valueJSON,
			MemoryType:  memType,
			Scope:       scope,
			ExecutionID: mb.executionID,
			Embedding:   embedding,
		})
		if err != nil {
			continue
		}
		stored++
	}

	return stored, nil
}

// Store persists a single memory entry directly, without LLM evaluation.
// This is the low-level path invoked by the memory_store builtin tool when
// the LLM explicitly requests a value to be remembered.
// Returns a human-readable status message suitable for the tool result.
// normalizeKey converts a raw memory key to a canonical lowercase-with-hyphens form.
// P-C333-1 (ACT-F3-10): consistent key normalization prevents duplicate entries
// from case variations (e.g. "UserPrefs" vs "user-prefs").
func normalizeKey(k string) string {
	if k == "" {
		return "general"
	}
	// Lowercase, replace non-alphanumeric runs with a single hyphen.
	var b strings.Builder
	prevHyphen := true
	for _, r := range strings.ToLower(k) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevHyphen = false
		} else if !prevHyphen {
			b.WriteRune('-')
			prevHyphen = true
		}
	}
	result := strings.TrimRight(b.String(), "-")
	if result == "" {
		result = "general"
	}
	// Truncate to 100 chars max.
	if len(result) > 100 {
		result = result[:100]
	}
	return result
}

func (mb *MemoryBridge) Store(ctx context.Context, agentID uuid.UUID, content, category string) string {
	if mb.upserter == nil {
		return "Memory storage is not available for this agent."
	}
	if content == "" {
		return "Nothing to store: content is empty."
	}

	// P-MEM-5: use content-derived key so different facts under the same category
	// get distinct DB rows and don't overwrite each other. Category is used only as
	// MemoryType. The normalizeKey function limits to 100 chars, so two long strings
	// differing only past 100 chars would collide — acceptable for this use case.
	// (If category were the key, storing "favorite language: Rust" and
	// "favorite language: Go" under category="preference" would collapse to 1 row.)
	key := normalizeKey(content)

	valueJSON, err := json.Marshal(content)
	if err != nil {
		return fmt.Sprintf("Failed to encode memory: %v", err)
	}

	var embedding []float32
	if mb.embedder != nil {
		if emb, embedErr := mb.embedder.Embed(ctx, content); embedErr == nil {
			embedding = emb
		}
	}

	scope := "agent"
	if mb.executionID != nil {
		scope = "execution"
	}

	_, err = mb.upserter.Upsert(ctx, agentID, key, memory.UpsertMemoryRequest{
		Value:       valueJSON,
		MemoryType:  category,
		Scope:       scope,
		ExecutionID: mb.executionID,
		Embedding:   embedding,
	})
	if err != nil {
		return fmt.Sprintf("Failed to store memory: %v", err)
	}
	// BUG-MEM-STRESS1 fix: removed "respond to user now" directive — that instruction
	// caused the LLM to stop after storing the first fact instead of looping through
	// all remaining facts. The per-key deduplication in runner.go now handles retries.
	return "Memory stored successfully. Proceed to store the next fact if there are more."
}

// StoreBulk stores multiple facts at once, returning a summary of successes/failures.
// BUG-MEM-STRESS1: gpt-oss-120b cannot reliably execute N sequential tool calls —
// it loops on the first item. StoreBulk lets the LLM make ONE call with all facts.
func (mb *MemoryBridge) StoreBulk(ctx context.Context, agentID uuid.UUID, facts []struct {
	Content  string `json:"content"`
	Category string `json:"category"`
}) string {
	if mb.upserter == nil {
		return "Memory storage is not available for this agent."
	}
	if len(facts) == 0 {
		return "Nothing to store: facts list is empty."
	}

	stored := 0
	skipped := 0
	var errs []string
	for _, fact := range facts {
		if fact.Content == "" {
			skipped++
			continue
		}
		result := mb.Store(ctx, agentID, fact.Content, fact.Category)
		if strings.HasPrefix(result, "Memory stored successfully") {
			stored++
		} else if strings.HasPrefix(result, "Memory already stored") {
			skipped++
		} else {
			errs = append(errs, fmt.Sprintf("%q: %s", fact.Content, result))
		}
	}

	if len(errs) > 0 {
		return fmt.Sprintf("Stored %d facts; %d already existed; %d errors: %s",
			stored, skipped, len(errs), strings.Join(errs, "; "))
	}
	return fmt.Sprintf("Stored %d facts successfully. %d were already stored (skipped).", stored, skipped)
}

// DistillExecution promotes all execution-scoped memories to workflow scope.
// Should be called at the end of a successful execution run.
// No-ops if no distiller is configured or no executionID was set.
func (mb *MemoryBridge) DistillExecution(ctx context.Context, agentID uuid.UUID) error {
	if mb.distiller == nil || mb.executionID == nil {
		return nil
	}
	return mb.distiller.DistillExecutionMemories(ctx, agentID, *mb.executionID)
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
