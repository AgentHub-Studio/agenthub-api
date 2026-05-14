package agentic

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// CTX-003 — Memory hierarchy.
//
// PDF arXiv:2604.14228v1 §7.4 (memory hierarchy: layered facts with
// most-specific-wins resolution; per-layer staleness windows).
//
// Distinct from existing memory plumbing:
//   - memory.go = per-agent transient extraction (MemoryBridge).
//   - cross_session_memory.go (FUTURE-001) = durable cross-session
//     substrate keyed by (tenant + scope + subject).
//   - memory_hierarchy.go (this file) = LOOKUP HIERARCHY: same key
//     can have facts at multiple scopes; resolution returns the most-
//     specific live fact and exposes the trace for auditability.
//
// Scopes (narrow → wide): session → user → agent → tenant → global.
// Lookup walks narrow→wide so a session-level override takes precedence
// over user/agent/tenant defaults. Each fact carries max_age — stale
// facts are skipped during lookup so the agent always reads fresh data.

// MemoryHierarchyScope is the bounded set of scope levels in
// narrow→wide order. Index = specificity tier.
type MemoryHierarchyScope string

const (
	MemoryHierarchyScopeSession MemoryHierarchyScope = "session"
	MemoryHierarchyScopeUser    MemoryHierarchyScope = "user"
	MemoryHierarchyScopeAgent   MemoryHierarchyScope = "agent"
	MemoryHierarchyScopeTenant  MemoryHierarchyScope = "tenant"
	MemoryHierarchyScopeGlobal  MemoryHierarchyScope = "global"
)

// scopeOrder is narrow→wide; lower index = more specific.
var scopeOrder = []MemoryHierarchyScope{
	MemoryHierarchyScopeSession,
	MemoryHierarchyScopeUser,
	MemoryHierarchyScopeAgent,
	MemoryHierarchyScopeTenant,
	MemoryHierarchyScopeGlobal,
}

// IsValidMemoryHierarchyScope returns true for the bounded set.
func IsValidMemoryHierarchyScope(s MemoryHierarchyScope) bool {
	for _, v := range scopeOrder {
		if s == v {
			return true
		}
	}
	return false
}

// AllMemoryHierarchyScopes returns scopes narrow→wide (copy).
func AllMemoryHierarchyScopes() []MemoryHierarchyScope {
	out := make([]MemoryHierarchyScope, len(scopeOrder))
	copy(out, scopeOrder)
	return out
}

// scopeRank returns the specificity tier (0 = most specific).
func scopeRank(s MemoryHierarchyScope) int {
	for i, v := range scopeOrder {
		if v == s {
			return i
		}
	}
	return len(scopeOrder)
}

// HierarchicalFact is one layered fact.
type HierarchicalFact struct {
	Scope   MemoryHierarchyScope `json:"scope"`
	// ScopeID identifies the bound subject for this scope:
	//   session  → SessionID
	//   user     → UserID
	//   agent    → AgentID
	//   tenant   → TenantID
	//   global   → "" (no subject)
	ScopeID string        `json:"scopeId"`
	Key     string        `json:"key"`
	Value   string        `json:"value"`
	SetAt   time.Time     `json:"setAt"`
	// MaxAge is the staleness window. Zero = never expires.
	MaxAge   time.Duration `json:"maxAge"`
	Source   string        `json:"source"`
}

// IsStale returns true when the fact is older than MaxAge.
func (f HierarchicalFact) IsStale(now time.Time) bool {
	if f.MaxAge <= 0 {
		return false
	}
	return now.Sub(f.SetAt) > f.MaxAge
}

// LookupContext bundles the scope IDs to use during resolution. Empty
// scope IDs cause that scope to be skipped (e.g. queries without a
// session-id only walk user/agent/tenant/global).
type LookupContext struct {
	SessionID string
	UserID    string
	AgentID   string
	TenantID  string
}

// scopeIDFor returns the bound subject ID for scope, or "" if missing.
func (c LookupContext) scopeIDFor(s MemoryHierarchyScope) string {
	switch s {
	case MemoryHierarchyScopeSession:
		return c.SessionID
	case MemoryHierarchyScopeUser:
		return c.UserID
	case MemoryHierarchyScopeAgent:
		return c.AgentID
	case MemoryHierarchyScopeTenant:
		return c.TenantID
	case MemoryHierarchyScopeGlobal:
		return ""
	}
	return ""
}

// LookupTrace records which scopes were checked and which matched.
type LookupTrace struct {
	Scopes  []ScopeProbe `json:"scopes"`
	Matched MemoryHierarchyScope `json:"matched,omitempty"` // "" if no match
}

// ScopeProbe is one entry in the resolution trace.
type ScopeProbe struct {
	Scope   MemoryHierarchyScope `json:"scope"`
	Skipped bool                 `json:"skipped,omitempty"`
	Reason  string               `json:"reason,omitempty"`
	Found   bool                 `json:"found"`
	Stale   bool                 `json:"stale,omitempty"`
}

// Sentinels.
var (
	ErrMemoryHierarchyInvalidScope = errors.New("memory hierarchy: invalid scope")
	ErrMemoryHierarchyKeyEmpty     = errors.New("memory hierarchy: key required")
	ErrMemoryHierarchyValueEmpty   = errors.New("memory hierarchy: value required")
	ErrMemoryHierarchyScopeIDEmpty = errors.New("memory hierarchy: scope_id required (except global)")
	ErrMemoryHierarchyNotFound     = errors.New("memory hierarchy: key not found at any layer")
)

// MemoryHierarchyStore is the persistence interface.
type MemoryHierarchyStore interface {
	Set(ctx context.Context, f HierarchicalFact) (HierarchicalFact, error)
	Lookup(ctx context.Context, lc LookupContext, key string) (HierarchicalFact, LookupTrace, error)
	LookupAll(ctx context.Context, lc LookupContext, key string) ([]HierarchicalFact, error)
	Delete(ctx context.Context, scope MemoryHierarchyScope, scopeID, key string) error
	ListByScope(ctx context.Context, scope MemoryHierarchyScope, scopeID string) ([]HierarchicalFact, error)
	PurgeStale(ctx context.Context) (int, error)
}

// validateFact checks structural invariants.
func validateFact(f HierarchicalFact) error {
	if !IsValidMemoryHierarchyScope(f.Scope) {
		return fmt.Errorf("%w: %q", ErrMemoryHierarchyInvalidScope, f.Scope)
	}
	if strings.TrimSpace(f.Key) == "" {
		return ErrMemoryHierarchyKeyEmpty
	}
	if strings.TrimSpace(f.Value) == "" {
		return ErrMemoryHierarchyValueEmpty
	}
	if f.Scope != MemoryHierarchyScopeGlobal && strings.TrimSpace(f.ScopeID) == "" {
		return ErrMemoryHierarchyScopeIDEmpty
	}
	if f.MaxAge < 0 {
		return errors.New("memory hierarchy: max_age cannot be negative")
	}
	return nil
}

// --- InMemoryMemoryHierarchyStore ---

type factKey struct {
	scope   MemoryHierarchyScope
	scopeID string
	key     string
}

type InMemoryMemoryHierarchyStore struct {
	mu    sync.Mutex
	facts map[factKey]HierarchicalFact
	now   func() time.Time // injectable for tests
}

// NewInMemoryMemoryHierarchyStore returns a concurrent-safe store.
func NewInMemoryMemoryHierarchyStore() *InMemoryMemoryHierarchyStore {
	return &InMemoryMemoryHierarchyStore{
		facts: map[factKey]HierarchicalFact{},
		now:   time.Now,
	}
}

// Set adds or updates a fact at its scope. Set-at is stamped now if zero.
func (s *InMemoryMemoryHierarchyStore) Set(ctx context.Context, f HierarchicalFact) (HierarchicalFact, error) {
	if err := ctx.Err(); err != nil {
		return HierarchicalFact{}, err
	}
	if err := validateFact(f); err != nil {
		return HierarchicalFact{}, err
	}
	if f.SetAt.IsZero() {
		f.SetAt = s.now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.facts[factKey{f.Scope, f.ScopeID, f.Key}] = f
	return f, nil
}

// Lookup walks narrow→wide. Returns the first non-stale match and a
// trace of scopes probed. Skipped scopes (missing scope ID) are still
// traced for auditability.
func (s *InMemoryMemoryHierarchyStore) Lookup(ctx context.Context, lc LookupContext, key string) (HierarchicalFact, LookupTrace, error) {
	if err := ctx.Err(); err != nil {
		return HierarchicalFact{}, LookupTrace{}, err
	}
	if strings.TrimSpace(key) == "" {
		return HierarchicalFact{}, LookupTrace{}, ErrMemoryHierarchyKeyEmpty
	}
	now := s.now()
	trace := LookupTrace{Scopes: make([]ScopeProbe, 0, len(scopeOrder))}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, scope := range scopeOrder {
		probe := ScopeProbe{Scope: scope}
		scopeID := lc.scopeIDFor(scope)
		if scope != MemoryHierarchyScopeGlobal && scopeID == "" {
			probe.Skipped = true
			probe.Reason = "no scope_id supplied"
			trace.Scopes = append(trace.Scopes, probe)
			continue
		}
		f, ok := s.facts[factKey{scope, scopeID, key}]
		if !ok {
			probe.Found = false
			trace.Scopes = append(trace.Scopes, probe)
			continue
		}
		if f.IsStale(now) {
			probe.Found = true
			probe.Stale = true
			trace.Scopes = append(trace.Scopes, probe)
			continue
		}
		probe.Found = true
		trace.Scopes = append(trace.Scopes, probe)
		trace.Matched = scope
		return f, trace, nil
	}
	return HierarchicalFact{}, trace, ErrMemoryHierarchyNotFound
}

// LookupAll returns all live facts for key across all matching scopes,
// ordered narrow→wide. Useful for "show me every layer that mentions this key".
func (s *InMemoryMemoryHierarchyStore) LookupAll(ctx context.Context, lc LookupContext, key string) ([]HierarchicalFact, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(key) == "" {
		return nil, ErrMemoryHierarchyKeyEmpty
	}
	now := s.now()
	out := []HierarchicalFact{}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, scope := range scopeOrder {
		scopeID := lc.scopeIDFor(scope)
		if scope != MemoryHierarchyScopeGlobal && scopeID == "" {
			continue
		}
		f, ok := s.facts[factKey{scope, scopeID, key}]
		if !ok || f.IsStale(now) {
			continue
		}
		out = append(out, f)
	}
	return out, nil
}

// Delete removes one fact.
func (s *InMemoryMemoryHierarchyStore) Delete(ctx context.Context, scope MemoryHierarchyScope, scopeID, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !IsValidMemoryHierarchyScope(scope) {
		return fmt.Errorf("%w: %q", ErrMemoryHierarchyInvalidScope, scope)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	k := factKey{scope, scopeID, key}
	if _, ok := s.facts[k]; !ok {
		return ErrMemoryHierarchyNotFound
	}
	delete(s.facts, k)
	return nil
}

// ListByScope returns all facts at one scope/scopeID, sorted by key.
func (s *InMemoryMemoryHierarchyStore) ListByScope(ctx context.Context, scope MemoryHierarchyScope, scopeID string) ([]HierarchicalFact, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !IsValidMemoryHierarchyScope(scope) {
		return nil, fmt.Errorf("%w: %q", ErrMemoryHierarchyInvalidScope, scope)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []HierarchicalFact{}
	for k, f := range s.facts {
		if k.scope != scope || k.scopeID != scopeID {
			continue
		}
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

// PurgeStale removes all stale facts. Returns the count removed.
func (s *InMemoryMemoryHierarchyStore) PurgeStale(ctx context.Context) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	removed := 0
	for k, f := range s.facts {
		if f.IsStale(now) {
			delete(s.facts, k)
			removed++
		}
	}
	return removed, nil
}

// SetClock allows tests to inject a deterministic clock.
func (s *InMemoryMemoryHierarchyStore) SetClock(clock func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.now = clock
}
