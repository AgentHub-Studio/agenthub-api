package agentic

import (
	"errors"
	"sync"
)

// TOOL-010 — Tool invocation budget.
//
// PDF arXiv:2604.14228v1 §7 (Context management) discusses budgeting at
// multiple layers: token budget, tool-result budget, and implicitly a
// call-count budget that prevents runaway agentic loops from exhausting
// external APIs or incurring unbounded cost.
//
// Distinct from per-run tool retry limits (runner.go:337) — this budget
// is session-scoped and tracks the cumulative invocation count for the
// lifetime of a chat session, not just one run.

// ToolInvocationCategory classifies tools for budget purposes.
// The category is derived from the tool slug prefix.
type ToolInvocationCategory string

const (
	ToolCategoryRead    ToolInvocationCategory = "read"    // read-only tools (search, get, list)
	ToolCategoryMutate  ToolInvocationCategory = "mutate"  // write/delete tools (create, update, delete)
	ToolCategoryExternal ToolInvocationCategory = "external" // tools that call external APIs (HTTP, MCP)
	ToolCategoryInternal ToolInvocationCategory = "internal" // AgentHub-internal (skill, knowledge-base)
	ToolCategoryUnknown ToolInvocationCategory = "unknown"
)

// AllToolInvocationCategories lists every valid category.
var AllToolInvocationCategories = []ToolInvocationCategory{
	ToolCategoryRead,
	ToolCategoryMutate,
	ToolCategoryExternal,
	ToolCategoryInternal,
	ToolCategoryUnknown,
}

// IsValid returns true for recognised categories.
func (c ToolInvocationCategory) IsValid() bool {
	for _, v := range AllToolInvocationCategories {
		if c == v {
			return true
		}
	}
	return false
}

// ToolBudgetPolicy controls behaviour when a budget cap is hit.
type ToolBudgetPolicy string

const (
	// ToolBudgetPolicyDeny blocks the tool call and returns an error.
	ToolBudgetPolicyDeny ToolBudgetPolicy = "deny"
	// ToolBudgetPolicyWarn allows the call but records the breach event.
	ToolBudgetPolicyWarn ToolBudgetPolicy = "warn"
	// ToolBudgetPolicyReport allows the call and attaches a budget-exhausted
	// note to the tool result for LLM awareness.
	ToolBudgetPolicyReport ToolBudgetPolicy = "report"
)

// AllToolBudgetPolicies lists every valid policy.
var AllToolBudgetPolicies = []ToolBudgetPolicy{
	ToolBudgetPolicyDeny,
	ToolBudgetPolicyWarn,
	ToolBudgetPolicyReport,
}

// IsValid returns true for recognised policy values.
func (p ToolBudgetPolicy) IsValid() bool {
	for _, v := range AllToolBudgetPolicies {
		if p == v {
			return true
		}
	}
	return false
}

// Sentinel errors for ToolInvocationBudget.
var (
	ErrTIBTotalBudgetExhausted    = errors.New("tool_invocation_budget: session total call limit reached")
	ErrTIBCategoryBudgetExhausted = errors.New("tool_invocation_budget: category call limit reached")
	ErrTIBInvalidCategory         = errors.New("tool_invocation_budget: invalid category")
	ErrTIBInvalidPolicy           = errors.New("tool_invocation_budget: invalid policy")
	ErrTIBNegativeCap             = errors.New("tool_invocation_budget: cap must be non-negative (0 = unlimited)")
)

// ToolInvocationBudgetConfig holds the limits for one budget instance.
type ToolInvocationBudgetConfig struct {
	// TotalCap is the maximum cumulative tool calls for the session.
	// 0 means unlimited.
	TotalCap int
	// CategoryCaps maps category → per-category call limit. 0 = unlimited.
	CategoryCaps map[ToolInvocationCategory]int
	// Policy controls behaviour when any cap is exceeded.
	Policy ToolBudgetPolicy
}

// Validate returns an error when the config is inconsistent.
func (c *ToolInvocationBudgetConfig) Validate() error {
	if c.TotalCap < 0 {
		return ErrTIBNegativeCap
	}
	if !c.Policy.IsValid() {
		return ErrTIBInvalidPolicy
	}
	for cat, cap := range c.CategoryCaps {
		if !cat.IsValid() {
			return ErrTIBInvalidCategory
		}
		if cap < 0 {
			return ErrTIBNegativeCap
		}
	}
	return nil
}

// ToolInvocationBudgetSnapshot is a point-in-time view of the budget.
type ToolInvocationBudgetSnapshot struct {
	TotalCalled    int
	TotalCap       int
	CategoryCalled map[ToolInvocationCategory]int
	CategoryCaps   map[ToolInvocationCategory]int
	Policy         ToolBudgetPolicy
}

// IsExhausted returns true when the total cap has been reached.
func (s ToolInvocationBudgetSnapshot) IsExhausted() bool {
	return s.TotalCap > 0 && s.TotalCalled >= s.TotalCap
}

// PercentUsed returns the percentage of the total budget consumed (0-100).
// Returns 0 when TotalCap is 0 (unlimited).
func (s ToolInvocationBudgetSnapshot) PercentUsed() int {
	if s.TotalCap <= 0 {
		return 0
	}
	pct := s.TotalCalled * 100 / s.TotalCap
	if pct > 100 {
		return 100
	}
	return pct
}

// ToolInvocationBudget tracks session-scoped tool invocations against
// configurable caps. Thread-safe.
type ToolInvocationBudget struct {
	mu           sync.Mutex
	cfg          ToolInvocationBudgetConfig
	totalCalled  int
	byCat        map[ToolInvocationCategory]int
}

// NewToolInvocationBudget constructs a budget from a validated config.
func NewToolInvocationBudget(cfg ToolInvocationBudgetConfig) (*ToolInvocationBudget, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	cats := make(map[ToolInvocationCategory]int, len(AllToolInvocationCategories))
	for _, c := range AllToolInvocationCategories {
		cats[c] = 0
	}
	return &ToolInvocationBudget{cfg: cfg, byCat: cats}, nil
}

// DefaultUnlimitedBudget returns a budget with no caps and warn-on-breach policy.
func DefaultUnlimitedBudget() *ToolInvocationBudget {
	b, _ := NewToolInvocationBudget(ToolInvocationBudgetConfig{
		TotalCap: 0,
		Policy:   ToolBudgetPolicyWarn,
	})
	return b
}

// Record increments the invocation counters for the given category and
// returns an error when either the category cap or total cap is breached
// AND the policy is Deny. When policy is Warn or Report the counters
// still increment but no error is returned.
func (b *ToolInvocationBudget) Record(cat ToolInvocationCategory) error {
	if !cat.IsValid() {
		return ErrTIBInvalidCategory
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// Check category cap BEFORE incrementing.
	if catCap, ok := b.cfg.CategoryCaps[cat]; ok && catCap > 0 {
		if b.byCat[cat] >= catCap && b.cfg.Policy == ToolBudgetPolicyDeny {
			return ErrTIBCategoryBudgetExhausted
		}
	}
	// Check total cap BEFORE incrementing.
	if b.cfg.TotalCap > 0 && b.totalCalled >= b.cfg.TotalCap {
		if b.cfg.Policy == ToolBudgetPolicyDeny {
			return ErrTIBTotalBudgetExhausted
		}
	}

	b.totalCalled++
	b.byCat[cat]++
	return nil
}

// Snapshot returns a point-in-time copy of the current budget state.
func (b *ToolInvocationBudget) Snapshot() ToolInvocationBudgetSnapshot {
	b.mu.Lock()
	defer b.mu.Unlock()

	catCalled := make(map[ToolInvocationCategory]int, len(b.byCat))
	for k, v := range b.byCat {
		catCalled[k] = v
	}
	catCaps := make(map[ToolInvocationCategory]int, len(b.cfg.CategoryCaps))
	for k, v := range b.cfg.CategoryCaps {
		catCaps[k] = v
	}
	return ToolInvocationBudgetSnapshot{
		TotalCalled:    b.totalCalled,
		TotalCap:       b.cfg.TotalCap,
		CategoryCalled: catCalled,
		CategoryCaps:   catCaps,
		Policy:         b.cfg.Policy,
	}
}

// Reset clears all counters (useful for test teardown or session restart).
func (b *ToolInvocationBudget) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.totalCalled = 0
	for k := range b.byCat {
		b.byCat[k] = 0
	}
}

// ClassifyToolForBudget returns the ToolInvocationCategory for a tool slug.
// Classification is prefix-based and heuristic — callers can override.
func ClassifyToolForBudget(slug string) ToolInvocationCategory {
	if len(slug) == 0 {
		return ToolCategoryUnknown
	}
	// MCP tools are external.
	if len(slug) >= 5 && slug[:5] == "mcp__" {
		return ToolCategoryExternal
	}
	// HTTP tools: any slug containing "http" prefix.
	for _, prefix := range []string{"http-get", "http-post", "http-put", "http-delete", "fetch", "search-web"} {
		if len(slug) >= len(prefix) && slug[:len(prefix)] == prefix {
			return ToolCategoryExternal
		}
	}
	// Mutating tool slugs.
	for _, keyword := range []string{"create", "update", "delete", "publish", "upload", "upsert", "patch", "remove"} {
		if containsWholeWord(slug, keyword) {
			return ToolCategoryMutate
		}
	}
	// Read tools.
	for _, keyword := range []string{"list", "get", "search", "find", "read", "export", "download"} {
		if containsWholeWord(slug, keyword) {
			return ToolCategoryRead
		}
	}
	return ToolCategoryInternal
}
