package agentic

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// CTX-001 — Context assembly.
//
// PDF arXiv:2604.14228v1 §7 (Context window assembly) + §7.2 (canonical
// section ordering: system → memory → rules → skill catalog → tool
// catalog → KB summary → recent messages → aux prompt → scratchpad).
//
// Distinct from existing AgentHub plumbing:
//   - prompt.go = PromptBuilder (chat-coupled, builds the final string)
//   - context.go = ContextManager (CTX-010 compaction lifecycle)
//   - context_envelope.go = ENVELOPE pattern (this file): pure-domain
//     section list with per-kind caps + droppability + canonical order
//
// The envelope is the contract between assembler and renderer: any
// builder produces a ContextEnvelope; any LLM-call site consumes one.
// Per-section budgets prevent any single section from drowning out the
// rest (e.g. a 50-tool catalog squeezing out memory/KB/recent_messages).

// ContextSectionKind is the bounded set of canonical section kinds.
// Order in this slice IS the canonical render order.
type ContextSectionKind string

const (
	// ContextSectionSystem — system prompt (non-droppable).
	ContextSectionSystem ContextSectionKind = "system"
	// ContextSectionMemory — long-term user memory facts.
	ContextSectionMemory ContextSectionKind = "memory"
	// ContextSectionRules — global + path-scoped rules.
	ContextSectionRules ContextSectionKind = "rules"
	// ContextSectionSkillCatalog — available skills.
	ContextSectionSkillCatalog ContextSectionKind = "skill_catalog"
	// ContextSectionToolCatalog — available tools.
	ContextSectionToolCatalog ContextSectionKind = "tool_catalog"
	// ContextSectionKBSummary — knowledge-base headers/summaries.
	ContextSectionKBSummary ContextSectionKind = "kb_summary"
	// ContextSectionRecentMessages — recent conversation turns.
	ContextSectionRecentMessages ContextSectionKind = "recent_messages"
	// ContextSectionAuxPrompt — auxiliary prompts (tool-injected, hooks).
	ContextSectionAuxPrompt ContextSectionKind = "aux_prompt"
	// ContextSectionScratchpad — agent's working notes.
	ContextSectionScratchpad ContextSectionKind = "scratchpad"
)

// canonicalSectionOrder is the section render order. Index = priority.
var canonicalSectionOrder = []ContextSectionKind{
	ContextSectionSystem,
	ContextSectionMemory,
	ContextSectionRules,
	ContextSectionSkillCatalog,
	ContextSectionToolCatalog,
	ContextSectionKBSummary,
	ContextSectionRecentMessages,
	ContextSectionAuxPrompt,
	ContextSectionScratchpad,
}

// IsValidContextSectionKind returns true for the bounded set.
func IsValidContextSectionKind(k ContextSectionKind) bool {
	for _, v := range canonicalSectionOrder {
		if k == v {
			return true
		}
	}
	return false
}

// AllContextSectionKinds returns the canonical order (copy).
func AllContextSectionKinds() []ContextSectionKind {
	out := make([]ContextSectionKind, len(canonicalSectionOrder))
	copy(out, canonicalSectionOrder)
	return out
}

// canonicalRank returns the canonical-order index for sorting.
func canonicalRank(k ContextSectionKind) int {
	for i, v := range canonicalSectionOrder {
		if v == k {
			return i
		}
	}
	return len(canonicalSectionOrder)
}

// nonDroppable lists kinds that are never dropped by budget enforcement.
// System is the only one — every LLM call needs at least the system prompt.
var nonDroppable = map[ContextSectionKind]bool{
	ContextSectionSystem: true,
}

// IsNonDroppable returns true if a section kind cannot be dropped.
func IsNonDroppable(k ContextSectionKind) bool {
	return nonDroppable[k]
}

// estimateTokens uses the chars/4 heuristic shared with PromptBuilder.
func estimateTokens(s string) int {
	if s == "" {
		return 0
	}
	// Round up so even a 1-char body counts as 1 token.
	return (len(s) + 3) / 4
}

// ContextSection is one logical chunk of context.
type ContextSection struct {
	Kind            ContextSectionKind `json:"kind"`
	Title           string             `json:"title,omitempty"`
	Body            string             `json:"body"`
	EstimatedTokens int                `json:"estimatedTokens"`
	Dropped         bool               `json:"dropped"`
	DropReason      string             `json:"dropReason,omitempty"`
	// Truncated is true when body was clipped to fit a per-kind cap
	// (vs Dropped which removes the section entirely).
	Truncated bool `json:"truncated"`
	// OriginalTokens is what EstimatedTokens was before truncation.
	OriginalTokens int `json:"originalTokens,omitempty"`
}

// ContextEnvelope is the assembled, ordered, budget-enforced bundle the
// renderer hands to the LLM.
type ContextEnvelope struct {
	Sections     []ContextSection `json:"sections"`
	TotalTokens  int              `json:"totalTokens"`
	BudgetLimit  int              `json:"budgetLimit"`
	BudgetUsed   int              `json:"budgetUsed"`
	Truncated    bool             `json:"truncated"`
	DroppedCount int              `json:"droppedCount"`
}

// FindSection returns the live (non-dropped) section by kind.
func (e ContextEnvelope) FindSection(kind ContextSectionKind) (ContextSection, bool) {
	for _, s := range e.Sections {
		if s.Kind == kind && !s.Dropped {
			return s, true
		}
	}
	return ContextSection{}, false
}

// LiveSections returns only the non-dropped sections.
func (e ContextEnvelope) LiveSections() []ContextSection {
	out := []ContextSection{}
	for _, s := range e.Sections {
		if !s.Dropped {
			out = append(out, s)
		}
	}
	return out
}

// AssemblerConfig tunes the assembler.
type AssemblerConfig struct {
	// BudgetLimit is the total token budget. 0 disables enforcement.
	BudgetLimit int
	// PerKindCap is an optional max tokens per kind (0 = unlimited).
	PerKindCap map[ContextSectionKind]int
}

// DefaultAssemblerConfig returns sensible defaults: 32k total, with
// per-kind caps preventing any one section from owning the budget.
func DefaultAssemblerConfig() AssemblerConfig {
	return AssemblerConfig{
		BudgetLimit: 32000,
		PerKindCap: map[ContextSectionKind]int{
			ContextSectionSkillCatalog:   2000,
			ContextSectionToolCatalog:    2000,
			ContextSectionKBSummary:      1500,
			ContextSectionRecentMessages: 16000,
			ContextSectionAuxPrompt:      1000,
			ContextSectionScratchpad:     1000,
		},
	}
}

// Sentinels.
var (
	ErrInvalidContextSectionKind = errors.New("context envelope: invalid section kind")
	ErrSystemSectionRequired     = errors.New("context envelope: system section required and non-droppable")
	ErrDuplicateSectionKind      = errors.New("context envelope: duplicate section kind")
)

// ContextAssembler is the pure-domain builder for ContextEnvelope.
// Append sections in any order; Assemble produces the canonically-
// ordered envelope with budget enforcement applied.
type ContextAssembler struct {
	config   AssemblerConfig
	sections []ContextSection
}

// NewContextAssembler returns an empty assembler.
func NewContextAssembler(cfg AssemblerConfig) *ContextAssembler {
	return &ContextAssembler{config: cfg}
}

// Append adds a section. Duplicate kinds are rejected — exactly one
// section per kind is allowed (composition happens before Append).
func (a *ContextAssembler) Append(s ContextSection) error {
	if !IsValidContextSectionKind(s.Kind) {
		return fmt.Errorf("%w: %q", ErrInvalidContextSectionKind, s.Kind)
	}
	for _, existing := range a.sections {
		if existing.Kind == s.Kind {
			return fmt.Errorf("%w: %q", ErrDuplicateSectionKind, s.Kind)
		}
	}
	if s.EstimatedTokens == 0 {
		s.EstimatedTokens = estimateTokens(s.Body)
	}
	a.sections = append(a.sections, s)
	return nil
}

// Assemble produces the canonically-ordered, budget-enforced envelope.
//
// Algorithm:
//  1. Apply per-kind cap (truncate body, mark Truncated).
//  2. Sort sections by canonical rank.
//  3. If total ≤ budget, done.
//  4. Otherwise drop sections from LOWEST priority (highest canonical
//     rank index) until total ≤ budget; never drop non-droppable kinds.
//  5. If even after dropping all droppables the system section alone
//     exceeds budget, return the envelope as Truncated=true with system
//     present (caller decides whether to fail or proceed).
func (a *ContextAssembler) Assemble() (ContextEnvelope, error) {
	// Defensive: ensure system is present (LLM calls require it).
	hasSystem := false
	for _, s := range a.sections {
		if s.Kind == ContextSectionSystem {
			hasSystem = true
			break
		}
	}
	if !hasSystem {
		return ContextEnvelope{}, ErrSystemSectionRequired
	}

	// Step 1: per-kind cap.
	processed := make([]ContextSection, len(a.sections))
	copy(processed, a.sections)
	for i, s := range processed {
		cap, ok := a.config.PerKindCap[s.Kind]
		if !ok || cap <= 0 {
			continue
		}
		if s.EstimatedTokens > cap {
			origTokens := s.EstimatedTokens
			// Truncate body to roughly cap tokens (chars = cap*4).
			maxChars := cap * 4
			if len(s.Body) > maxChars {
				s.Body = s.Body[:maxChars-3] + "..."
			}
			s.EstimatedTokens = estimateTokens(s.Body)
			s.Truncated = true
			s.OriginalTokens = origTokens
			processed[i] = s
		}
	}

	// Step 2: canonical sort.
	sort.SliceStable(processed, func(i, j int) bool {
		return canonicalRank(processed[i].Kind) < canonicalRank(processed[j].Kind)
	})

	// Step 3-4: budget enforcement.
	envelope := ContextEnvelope{
		Sections:    processed,
		BudgetLimit: a.config.BudgetLimit,
	}
	envelope.TotalTokens = 0
	for _, s := range envelope.Sections {
		envelope.TotalTokens += s.EstimatedTokens
	}

	if a.config.BudgetLimit > 0 && envelope.TotalTokens > a.config.BudgetLimit {
		// Drop from the lowest-priority (highest canonical rank) downward.
		// Iterate in REVERSE canonical order.
		for i := len(envelope.Sections) - 1; i >= 0 && envelope.TotalTokens > a.config.BudgetLimit; i-- {
			s := envelope.Sections[i]
			if IsNonDroppable(s.Kind) {
				continue
			}
			if s.Dropped {
				continue
			}
			s.Dropped = true
			s.DropReason = fmt.Sprintf("budget exceeded: total=%d limit=%d", envelope.TotalTokens, a.config.BudgetLimit)
			envelope.TotalTokens -= s.EstimatedTokens
			envelope.DroppedCount++
			envelope.Sections[i] = s
		}
		envelope.Truncated = envelope.DroppedCount > 0 || envelope.TotalTokens > a.config.BudgetLimit
	}

	envelope.BudgetUsed = envelope.TotalTokens
	return envelope, nil
}

// Render returns a deterministic flat string of all live sections in
// canonical order. Each section is preceded by its title (or kind) as
// a markdown-style header.
func (e ContextEnvelope) Render() string {
	var b strings.Builder
	first := true
	for _, s := range e.Sections {
		if s.Dropped {
			continue
		}
		if !first {
			b.WriteString("\n\n")
		}
		first = false
		title := s.Title
		if title == "" {
			title = string(s.Kind)
		}
		b.WriteString("# ")
		b.WriteString(title)
		b.WriteString("\n\n")
		b.WriteString(s.Body)
	}
	return b.String()
}
