package agentic

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// EXT-010 — Extension context-cost policy.
//
// PDF arXiv:2604.14228v1 §6.4 (Extensions declare their context cost
// so the platform can budget across loaded extensions and refuse
// configurations that would blow past the session window).
//
// Distinct from neighbouring abstractions:
//   - CTX-008 ToolResultBudget = runtime per-tool-result truncation.
//   - CTX-001 ContextSectionBudget = section-level token allocation.
//   - EXT-010 (this) = STATIC declaration per extension: "loading me
//     costs X tokens of header + Y per turn + Z per tool call." The
//     platform sums these at load time and decides whether to admit.

// ExtensionContextCostCategory bounded enum classifies the magnitude
// band of an extension's per-turn token cost.
type ExtensionContextCostCategory string

const (
	ExtensionContextCostMicro  ExtensionContextCostCategory = "micro"
	ExtensionContextCostSmall  ExtensionContextCostCategory = "small"
	ExtensionContextCostMedium ExtensionContextCostCategory = "medium"
	ExtensionContextCostLarge  ExtensionContextCostCategory = "large"
	ExtensionContextCostHeavy  ExtensionContextCostCategory = "heavy"
)

var allExtensionContextCostCategories = []ExtensionContextCostCategory{
	ExtensionContextCostMicro,
	ExtensionContextCostSmall,
	ExtensionContextCostMedium,
	ExtensionContextCostLarge,
	ExtensionContextCostHeavy,
}

// IsValidExtensionContextCostCategory returns true for the bounded set.
func IsValidExtensionContextCostCategory(c ExtensionContextCostCategory) bool {
	for _, v := range allExtensionContextCostCategories {
		if c == v {
			return true
		}
	}
	return false
}

// AllExtensionContextCostCategories returns a defensive copy.
func AllExtensionContextCostCategories() []ExtensionContextCostCategory {
	out := make([]ExtensionContextCostCategory, len(allExtensionContextCostCategories))
	copy(out, allExtensionContextCostCategories)
	return out
}

// Band boundaries for PerTurnTokens → Category classification.
const (
	ExtensionContextCostMicroMaxPerTurn  = 100
	ExtensionContextCostSmallMaxPerTurn  = 500
	ExtensionContextCostMediumMaxPerTurn = 2000
	ExtensionContextCostLargeMaxPerTurn  = 8000
	// Heavy = anything above ExtensionContextCostLargeMaxPerTurn.
)

// ClassifyByPerTurnTokens returns the canonical category for a given
// PerTurnTokens value.
func ClassifyByPerTurnTokens(perTurn int) ExtensionContextCostCategory {
	switch {
	case perTurn <= ExtensionContextCostMicroMaxPerTurn:
		return ExtensionContextCostMicro
	case perTurn <= ExtensionContextCostSmallMaxPerTurn:
		return ExtensionContextCostSmall
	case perTurn <= ExtensionContextCostMediumMaxPerTurn:
		return ExtensionContextCostMedium
	case perTurn <= ExtensionContextCostLargeMaxPerTurn:
		return ExtensionContextCostLarge
	default:
		return ExtensionContextCostHeavy
	}
}

// ExtensionContextCostPolicy declares one extension's context cost
// budget. All token fields are non-negative.
type ExtensionContextCostPolicy struct {
	ExtensionSlug         string
	StaticHeaderTokens    int // one-time injection at session start
	PerTurnTokens         int // tokens added every turn (system prompt fragments)
	PerToolCallTokens     int // tokens added each time one of this extension's tools is called
	MaxBudgetPerSession   int // hard cap; 0 = unbounded (NOT recommended)
	Category              ExtensionContextCostCategory
}

var extensionContextCostSlugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

// Validate enforces invariants. Category must match the band implied
// by PerTurnTokens (no drift between declared and observed cost).
func (p ExtensionContextCostPolicy) Validate() error {
	if !extensionContextCostSlugRE.MatchString(p.ExtensionSlug) {
		return fmt.Errorf("%w: %q must be kebab-case", ErrExtensionContextCostBadSlug, p.ExtensionSlug)
	}
	if p.StaticHeaderTokens < 0 {
		return fmt.Errorf("%w: static_header_tokens %d", ErrExtensionContextCostNegative, p.StaticHeaderTokens)
	}
	if p.PerTurnTokens < 0 {
		return fmt.Errorf("%w: per_turn_tokens %d", ErrExtensionContextCostNegative, p.PerTurnTokens)
	}
	if p.PerToolCallTokens < 0 {
		return fmt.Errorf("%w: per_tool_call_tokens %d", ErrExtensionContextCostNegative, p.PerToolCallTokens)
	}
	if p.MaxBudgetPerSession < 0 {
		return fmt.Errorf("%w: max_budget_per_session %d", ErrExtensionContextCostNegative, p.MaxBudgetPerSession)
	}
	if !IsValidExtensionContextCostCategory(p.Category) {
		return fmt.Errorf("%w: %q", ErrExtensionContextCostBadCategory, p.Category)
	}
	expectedCategory := ClassifyByPerTurnTokens(p.PerTurnTokens)
	if p.Category != expectedCategory {
		return fmt.Errorf("%w: declared %q but per_turn_tokens=%d implies %q",
			ErrExtensionContextCostCategoryDrift, p.Category, p.PerTurnTokens, expectedCategory)
	}
	return nil
}

// EstimateForTurns returns the total token cost for a given session
// with N turns and M tool calls.
func (p ExtensionContextCostPolicy) EstimateForTurns(turns, toolCalls int) int {
	return p.StaticHeaderTokens + p.PerTurnTokens*turns + p.PerToolCallTokens*toolCalls
}

// ExtensionContextCostRegistry holds per-extension policies and
// answers "can we afford to load all these extensions in one session?"
type ExtensionContextCostRegistry struct {
	mu       sync.RWMutex
	policies map[string]ExtensionContextCostPolicy
}

// NewExtensionContextCostRegistry creates an empty registry.
func NewExtensionContextCostRegistry() *ExtensionContextCostRegistry {
	return &ExtensionContextCostRegistry{
		policies: map[string]ExtensionContextCostPolicy{},
	}
}

// Register adds a policy. Rejects bad descriptors and duplicate slugs.
func (r *ExtensionContextCostRegistry) Register(p ExtensionContextCostPolicy) error {
	if err := p.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.policies[p.ExtensionSlug]; exists {
		return fmt.Errorf("%w: %q", ErrExtensionContextCostDuplicate, p.ExtensionSlug)
	}
	r.policies[p.ExtensionSlug] = p
	return nil
}

// Lookup returns one policy by slug.
func (r *ExtensionContextCostRegistry) Lookup(slug string) (ExtensionContextCostPolicy, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.policies[slug]
	return p, ok
}

// ListAll returns all policies sorted by slug.
func (r *ExtensionContextCostRegistry) ListAll() []ExtensionContextCostPolicy {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ExtensionContextCostPolicy, 0, len(r.policies))
	for _, p := range r.policies {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ExtensionSlug < out[j].ExtensionSlug })
	return out
}

// ListByCategory returns policies in a given category sorted by slug.
func (r *ExtensionContextCostRegistry) ListByCategory(c ExtensionContextCostCategory) []ExtensionContextCostPolicy {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []ExtensionContextCostPolicy{}
	for _, p := range r.policies {
		if p.Category == c {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ExtensionSlug < out[j].ExtensionSlug })
	return out
}

// EstimateLoadedExtensions sums the estimated cost across the named
// extensions for a session with `turns` turns and `toolCalls` total
// tool calls. Returns (estimated, missingExtensions). Missing slugs do
// NOT contribute to the estimate.
func (r *ExtensionContextCostRegistry) EstimateLoadedExtensions(
	slugs []string, turns, toolCalls int,
) (int, []string) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	total := 0
	missing := []string{}
	for _, slug := range slugs {
		p, ok := r.policies[slug]
		if !ok {
			missing = append(missing, slug)
			continue
		}
		total += p.EstimateForTurns(turns, toolCalls)
	}
	sort.Strings(missing)
	return total, missing
}

// CanAffordSessionWindow reports whether loading the given extensions
// would fit inside contextWindowTokens (the LLM's context budget).
// Missing extensions are treated as zero-cost — caller should inspect
// missing list separately. minHeadroom is the number of tokens the
// caller wants to reserve for user content + responses (e.g., 8000).
func (r *ExtensionContextCostRegistry) CanAffordSessionWindow(
	slugs []string, turns, toolCalls, contextWindowTokens, minHeadroom int,
) (bool, int, []string) {
	estimated, missing := r.EstimateLoadedExtensions(slugs, turns, toolCalls)
	budget := contextWindowTokens - minHeadroom
	return estimated <= budget, estimated, missing
}

// Size returns the registry count.
func (r *ExtensionContextCostRegistry) Size() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.policies)
}

// Sentinel errors.
var (
	ErrExtensionContextCostBadSlug         = errors.New("extension context cost: slug must be kebab-case")
	ErrExtensionContextCostNegative        = errors.New("extension context cost: token field must be >= 0")
	ErrExtensionContextCostBadCategory     = errors.New("extension context cost: invalid category")
	ErrExtensionContextCostCategoryDrift   = errors.New("extension context cost: declared category does not match per_turn_tokens band")
	ErrExtensionContextCostDuplicate       = errors.New("extension context cost: duplicate extension slug in registry")
)

// Compile-time check that strings package is imported (used for
// validation messages elsewhere if needed in the future).
var _ = strings.TrimSpace
