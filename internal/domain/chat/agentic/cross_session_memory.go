package agentic

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// FUTURE-001 — Cross-session memory substrate.
//
// PDF arXiv:2604.14228v1 §12 (Future Directions — persistent memory
// that outlives a single session; agent recall across runs); §11
// (memory must be auditable and decay-managed).
//
// Existing in AgentHub:
//   - memory.go MemoryEvaluator + MemoryBridge — per-agent recall.
//   - tenant agent_memory table — typically scoped to one agent.
//
// What FUTURE-001 introduces:
//   - CrossSessionMemorySubstrate interface — long-lived facts keyed
//     by (TenantID + Scope + Subject) with optional cross-agent sharing.
//   - MemoryScope bounded enum: user / agent / tenant_global (broadcast).
//   - Decay / eviction model: memories carry LastRecalledAt; substrate
//     can archive entries not recalled in N months (retention policy).
//   - Conflict resolution: when newer evidence contradicts older,
//     supersede chain (matches DecisionRecord pattern).
//
// Distinction from existing MemoryBridge:
//   - MemoryBridge = per-run extraction + recall (transient lifecycle).
//   - CrossSessionMemorySubstrate = the durable layer underneath. The
//     bridge writes to the substrate; future runs query the substrate.

// MemoryScope bounded enum.
type MemoryScope string

const (
	// MemoryScopeUser — fact about a specific user, keyed by UserID.
	// Recallable by ANY agent the user interacts with (subject to tenant policy).
	MemoryScopeUser MemoryScope = "user"
	// MemoryScopeAgent — fact about a specific agent's behavior /
	// learned heuristics. Per-agent, cross-session.
	MemoryScopeAgent MemoryScope = "agent"
	// MemoryScopeTenantGlobal — tenant-wide fact (e.g. "tenant uses
	// fiscal-year Q4 = Jul-Sep"). Available to all agents + users.
	MemoryScopeTenantGlobal MemoryScope = "tenant_global"
)

// allMemoryScopes is the closed bounded set.
var allMemoryScopes = []MemoryScope{
	MemoryScopeUser, MemoryScopeAgent, MemoryScopeTenantGlobal,
}

// IsValidMemoryScope returns true for the bounded set.
func IsValidMemoryScope(s MemoryScope) bool {
	for _, v := range allMemoryScopes {
		if s == v {
			return true
		}
	}
	return false
}

// AllMemoryScopes returns a copy of the bounded set.
func AllMemoryScopes() []MemoryScope {
	out := make([]MemoryScope, len(allMemoryScopes))
	copy(out, allMemoryScopes)
	return out
}

// MemoryKind bounded enum (mirrors auto-memory taxonomy from CLAUDE.md).
type MemoryKind string

const (
	MemoryKindFact       MemoryKind = "fact"       // factual statement about subject
	MemoryKindPreference MemoryKind = "preference" // user/agent preference
	MemoryKindFeedback   MemoryKind = "feedback"   // correction/guidance from human
	MemoryKindReference  MemoryKind = "reference"  // pointer to external resource
	MemoryKindRelationship MemoryKind = "relationship" // connection between entities
)

var allMemoryKinds = []MemoryKind{
	MemoryKindFact, MemoryKindPreference, MemoryKindFeedback,
	MemoryKindReference, MemoryKindRelationship,
}

// IsValidMemoryKind returns true for the bounded set.
func IsValidMemoryKind(k MemoryKind) bool {
	for _, v := range allMemoryKinds {
		if k == v {
			return true
		}
	}
	return false
}

// AllMemoryKinds returns a copy.
func AllMemoryKinds() []MemoryKind {
	out := make([]MemoryKind, len(allMemoryKinds))
	copy(out, allMemoryKinds)
	return out
}

// CrossSessionMemoryEntry is one long-lived memory fact.
type CrossSessionMemoryEntry struct {
	ID uuid.UUID `json:"id"`
	// TenantID scopes everything. Required.
	TenantID string `json:"tenantId"`
	// Scope classifies who the memory belongs to.
	Scope MemoryScope `json:"scope"`
	// Subject identifies the bearer:
	//   Scope=user → UserID
	//   Scope=agent → AgentID
	//   Scope=tenant_global → "" (empty by convention)
	Subject string `json:"subject,omitempty"`
	// Kind classifies the memory.
	Kind MemoryKind `json:"kind"`
	// Topic is a short label for grouping (e.g. "preferred_language",
	// "fiscal_calendar", "favorite_tool").
	Topic string `json:"topic"`
	// Content is the actual memory text (≤ 1000 chars).
	Content string `json:"content"`
	// SourceRunID identifies the run that created this memory.
	SourceRunID string `json:"sourceRunId,omitempty"`
	// Confidence is the agent's certainty (0..1). Defaults 1.0.
	Confidence float64 `json:"confidence"`
	// CreatedAt is wall-clock at insertion.
	CreatedAt time.Time `json:"createdAt"`
	// LastRecalledAt updates each time the memory is read.
	LastRecalledAt time.Time `json:"lastRecalledAt,omitempty"`
	// SupersededBy links to a newer memory that replaced this one.
	SupersededBy *uuid.UUID `json:"supersededBy,omitempty"`
	// Archived marks an entry past retention without deleting (audit trail).
	Archived bool `json:"archived"`
}

// MemoryRecallQuery scopes a recall request.
type MemoryRecallQuery struct {
	TenantID string
	Scope    MemoryScope
	Subject  string // matches entry.Subject; empty means "any subject in scope"
	Kind     MemoryKind // empty means "any kind"
	Topic    string // empty means "any topic"
	Limit    int    // 0 means "all"
}

// Sentinels.
var (
	ErrMemoryNotFound        = errors.New("cross-session memory: not found")
	ErrMemoryAlreadySuperseded = errors.New("cross-session memory: already superseded")
	ErrInvalidMemoryScope    = errors.New("cross-session memory: invalid scope")
	ErrInvalidMemoryKind     = errors.New("cross-session memory: invalid kind")
)

func validateEntry(e CrossSessionMemoryEntry) error {
	if e.TenantID == "" {
		return errors.New("cross-session memory: tenantId required")
	}
	if !IsValidMemoryScope(e.Scope) {
		return fmt.Errorf("%w: %q", ErrInvalidMemoryScope, e.Scope)
	}
	if !IsValidMemoryKind(e.Kind) {
		return fmt.Errorf("%w: %q", ErrInvalidMemoryKind, e.Kind)
	}
	if e.Topic == "" {
		return errors.New("cross-session memory: topic required")
	}
	if e.Content == "" {
		return errors.New("cross-session memory: content required")
	}
	// Scope-specific subject requirements:
	switch e.Scope {
	case MemoryScopeUser, MemoryScopeAgent:
		if e.Subject == "" {
			return fmt.Errorf("cross-session memory: scope %q requires subject", e.Scope)
		}
	case MemoryScopeTenantGlobal:
		if e.Subject != "" {
			return fmt.Errorf("cross-session memory: scope tenant_global must NOT have subject")
		}
	}
	return nil
}

// CrossSessionMemorySubstrate is the persistence + recall interface.
type CrossSessionMemorySubstrate interface {
	Save(ctx context.Context, e CrossSessionMemoryEntry) (CrossSessionMemoryEntry, error)
	FindByID(ctx context.Context, id uuid.UUID) (CrossSessionMemoryEntry, error)
	Recall(ctx context.Context, q MemoryRecallQuery) ([]CrossSessionMemoryEntry, error)
	Supersede(ctx context.Context, oldID, newID uuid.UUID) error
	// ArchiveExpired marks entries with LastRecalledAt before cutoff
	// as Archived (without deleting — audit trail intact). Returns
	// the number archived.
	ArchiveExpired(ctx context.Context, tenantID string, cutoff time.Time) (int, error)
	CountByScope(ctx context.Context, tenantID string) (map[MemoryScope]int, error)
}

// --- InMemoryCrossSessionMemorySubstrate ---

// InMemoryCrossSessionMemorySubstrate is the default in-memory impl.
type InMemoryCrossSessionMemorySubstrate struct {
	mu      sync.RWMutex
	entries map[uuid.UUID]CrossSessionMemoryEntry
}

// NewInMemoryCrossSessionMemorySubstrate creates an empty substrate.
func NewInMemoryCrossSessionMemorySubstrate() *InMemoryCrossSessionMemorySubstrate {
	return &InMemoryCrossSessionMemorySubstrate{
		entries: map[uuid.UUID]CrossSessionMemoryEntry{},
	}
}

// Save persists an entry. Defaults applied + validation enforced.
func (s *InMemoryCrossSessionMemorySubstrate) Save(ctx context.Context, e CrossSessionMemoryEntry) (CrossSessionMemoryEntry, error) {
	if err := ctx.Err(); err != nil {
		return CrossSessionMemoryEntry{}, err
	}
	if e.Confidence == 0 {
		e.Confidence = 1.0
	}
	if err := validateEntry(e); err != nil {
		return CrossSessionMemoryEntry{}, err
	}
	if len(e.Content) > 1000 {
		e.Content = e.Content[:997] + "..."
	}
	if len(e.Topic) > 100 {
		e.Topic = e.Topic[:97] + "..."
	}
	e.ID = uuid.New()
	e.CreatedAt = time.Now()
	e.Archived = false
	s.mu.Lock()
	s.entries[e.ID] = e
	s.mu.Unlock()
	return e, nil
}

// FindByID returns an entry by ID.
func (s *InMemoryCrossSessionMemorySubstrate) FindByID(ctx context.Context, id uuid.UUID) (CrossSessionMemoryEntry, error) {
	if err := ctx.Err(); err != nil {
		return CrossSessionMemoryEntry{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.entries[id]
	if !ok {
		return CrossSessionMemoryEntry{}, ErrMemoryNotFound
	}
	return e, nil
}

// Recall returns entries matching the query, newest-first by CreatedAt.
// Updates LastRecalledAt on returned entries (recall touches the timestamp).
// Excludes superseded and archived entries by default.
func (s *InMemoryCrossSessionMemorySubstrate) Recall(ctx context.Context, q MemoryRecallQuery) ([]CrossSessionMemoryEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if q.TenantID == "" {
		return nil, errors.New("cross-session memory: recall requires tenantID")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	matched := []CrossSessionMemoryEntry{}
	now := time.Now()
	for id, e := range s.entries {
		if e.TenantID != q.TenantID {
			continue
		}
		if e.Archived {
			continue
		}
		if e.SupersededBy != nil {
			continue
		}
		if q.Scope != "" && e.Scope != q.Scope {
			continue
		}
		if q.Subject != "" && e.Subject != q.Subject {
			continue
		}
		if q.Kind != "" && e.Kind != q.Kind {
			continue
		}
		if q.Topic != "" && e.Topic != q.Topic {
			continue
		}
		// Touch LastRecalledAt.
		e.LastRecalledAt = now
		s.entries[id] = e
		matched = append(matched, e)
	}
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].CreatedAt.After(matched[j].CreatedAt)
	})
	if q.Limit > 0 && len(matched) > q.Limit {
		matched = matched[:q.Limit]
	}
	return matched, nil
}

// Supersede chains old → new. First-supersede-wins.
func (s *InMemoryCrossSessionMemorySubstrate) Supersede(ctx context.Context, oldID, newID uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.entries[oldID]
	if !ok {
		return ErrMemoryNotFound
	}
	if old.SupersededBy != nil {
		return ErrMemoryAlreadySuperseded
	}
	if _, ok := s.entries[newID]; !ok {
		return fmt.Errorf("cross-session memory: new id %s does not exist", newID)
	}
	old.SupersededBy = &newID
	s.entries[oldID] = old
	return nil
}

// ArchiveExpired marks entries not recalled since cutoff as Archived.
// Entries that were never recalled use CreatedAt as fallback timestamp.
func (s *InMemoryCrossSessionMemorySubstrate) ArchiveExpired(ctx context.Context, tenantID string, cutoff time.Time) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for id, e := range s.entries {
		if e.TenantID != tenantID {
			continue
		}
		if e.Archived {
			continue
		}
		// Use LastRecalledAt if set, else CreatedAt.
		ref := e.LastRecalledAt
		if ref.IsZero() {
			ref = e.CreatedAt
		}
		if ref.Before(cutoff) {
			e.Archived = true
			s.entries[id] = e
			count++
		}
	}
	return count, nil
}

// CountByScope returns histogram (always 3 keys with 0 default).
// Counts only NON-archived, NON-superseded entries.
func (s *InMemoryCrossSessionMemorySubstrate) CountByScope(ctx context.Context, tenantID string) (map[MemoryScope]int, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	hist := map[MemoryScope]int{}
	for _, sc := range allMemoryScopes {
		hist[sc] = 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, e := range s.entries {
		if e.TenantID != tenantID || e.Archived || e.SupersededBy != nil {
			continue
		}
		hist[e.Scope]++
	}
	return hist, nil
}

// --- Helpers ---

// CompactRecallSummary renders a short summary suitable for prepending
// to an agent's system prompt during a fresh session.
//
// Format: "Cross-session memory:\n- [scope] topic: content\n- ..."
func CompactRecallSummary(entries []CrossSessionMemoryEntry, max int) string {
	if max <= 0 {
		max = 10
	}
	if len(entries) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Cross-session memory:\n")
	limit := len(entries)
	if limit > max {
		limit = max
	}
	for _, e := range entries[:limit] {
		fmt.Fprintf(&b, "- [%s/%s] %s: %s\n", e.Scope, e.Kind, e.Topic, e.Content)
	}
	if len(entries) > limit {
		fmt.Fprintf(&b, "  (... %d more entries elided)\n", len(entries)-limit)
	}
	return b.String()
}
