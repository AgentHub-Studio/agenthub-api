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

// PERM-010 — Permission decision audit (query / aggregate / retention).
//
// PDF arXiv:2604.14228v1 §4 (Permissions and Safety) — compliance and
// incident response need to ANSWER three questions about permission
// decisions, not just LOG them:
//   1. "Which tool calls did agent X have denied last week?" (query).
//   2. "What's the top-denied tool across the tenant?" (aggregate).
//   3. "Drop old audit rows but preserve denies for 7 years" (retention).
//
// The existing permission_audit.go is write-only (Logger interface +
// PermissionAuditEntry). This file adds the READ + AGGREGATE +
// RETENTION + REDACTION layers, all pure-domain.
//
// Distinct from existing AgentHub plumbing:
//   - permission_audit.go = WRITE pipeline (Logger).
//   - permission_audit_repository.go = WRITE side persistence.
//   - permission_audit_query.go (this file) = READ side + analytics +
//     lifecycle policy.

// PermissionAuditQuery filters entries for read operations.
//
// All filter fields are AND'd: an entry must match every non-zero
// filter to be included. Zero values mean "no filter on that field".
type PermissionAuditQuery struct {
	SessionID    *uuid.UUID
	RunID        *uuid.UUID
	ToolName     string
	Decision     PermissionAuditDecision
	FromInclusive time.Time
	ToExclusive   time.Time
	Limit         int // 0 = unlimited
}

// Matches returns true if the entry passes every active filter.
func (q PermissionAuditQuery) Matches(e PermissionAuditEntry) bool {
	if q.SessionID != nil && e.SessionID != *q.SessionID {
		return false
	}
	if q.RunID != nil {
		if e.RunID == nil || *e.RunID != *q.RunID {
			return false
		}
	}
	if q.ToolName != "" && e.ToolName != q.ToolName {
		return false
	}
	if q.Decision != "" && e.Decision != q.Decision {
		return false
	}
	if !q.FromInclusive.IsZero() && e.CreatedAt.Before(q.FromInclusive) {
		return false
	}
	if !q.ToExclusive.IsZero() && !e.CreatedAt.Before(q.ToExclusive) {
		return false
	}
	return true
}

// PermissionAuditCountByTool is one bucket of aggregate counts.
type PermissionAuditCountByTool struct {
	ToolName string
	Count    int
}

// PermissionAuditAggregate is the analytics output of a query.
type PermissionAuditAggregate struct {
	TotalEntries        int
	CountByDecision     map[PermissionAuditDecision]int
	CountByTool         []PermissionAuditCountByTool // sorted by count desc, then name
	TopDeniedTools      []PermissionAuditCountByTool // sorted by deny count desc
	UniqueSessions      int
	WindowFromInclusive time.Time
	WindowToExclusive   time.Time
}

// PermissionAuditReader is the READ side of the audit pipeline.
// Implementations must be safe for concurrent use.
type PermissionAuditReader interface {
	List(ctx context.Context, query PermissionAuditQuery) ([]PermissionAuditEntry, error)
	Aggregate(ctx context.Context, query PermissionAuditQuery) (PermissionAuditAggregate, error)
}

// InMemoryPermissionAuditStore implements both PermissionAuditLogger
// AND PermissionAuditReader for tests/dev. Thread-safe.
type InMemoryPermissionAuditStore struct {
	mu      sync.RWMutex
	entries []PermissionAuditEntry
}

// NewInMemoryPermissionAuditStore builds an empty store.
func NewInMemoryPermissionAuditStore() *InMemoryPermissionAuditStore {
	return &InMemoryPermissionAuditStore{}
}

// LogDecision satisfies PermissionAuditLogger.
func (s *InMemoryPermissionAuditStore) LogDecision(_ context.Context, e PermissionAuditEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, e)
	return nil
}

// List returns matching entries, oldest first, capped by Limit.
func (s *InMemoryPermissionAuditStore) List(_ context.Context, query PermissionAuditQuery) ([]PermissionAuditEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	matched := []PermissionAuditEntry{}
	for _, e := range s.entries {
		if query.Matches(e) {
			matched = append(matched, e)
		}
	}
	sort.SliceStable(matched, func(i, j int) bool {
		return matched[i].CreatedAt.Before(matched[j].CreatedAt)
	})
	if query.Limit > 0 && len(matched) > query.Limit {
		matched = matched[:query.Limit]
	}
	return matched, nil
}

// Aggregate computes counts and per-tool/per-decision buckets.
func (s *InMemoryPermissionAuditStore) Aggregate(ctx context.Context, query PermissionAuditQuery) (PermissionAuditAggregate, error) {
	// Aggregate ignores Limit — analytics must see every match.
	q := query
	q.Limit = 0
	entries, err := s.List(ctx, q)
	if err != nil {
		return PermissionAuditAggregate{}, err
	}
	return buildAggregate(entries, q), nil
}

// Size returns the total number of stored entries (test helper).
func (s *InMemoryPermissionAuditStore) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

// Snapshot returns a defensive copy of stored entries (test helper).
func (s *InMemoryPermissionAuditStore) Snapshot() []PermissionAuditEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]PermissionAuditEntry(nil), s.entries...)
}

// ApplyRetention deletes entries that the policy considers expired.
// Returns the number of entries removed.
func (s *InMemoryPermissionAuditStore) ApplyRetention(policy PermissionAuditRetentionPolicy, now time.Time) (int, error) {
	if err := policy.Validate(); err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.entries[:0]
	dropped := 0
	for _, e := range s.entries {
		if policy.shouldExpire(e, now) {
			dropped++
			continue
		}
		kept = append(kept, e)
	}
	s.entries = kept
	return dropped, nil
}

func buildAggregate(entries []PermissionAuditEntry, q PermissionAuditQuery) PermissionAuditAggregate {
	agg := PermissionAuditAggregate{
		TotalEntries:        len(entries),
		CountByDecision:     map[PermissionAuditDecision]int{},
		WindowFromInclusive: q.FromInclusive,
		WindowToExclusive:   q.ToExclusive,
	}
	toolCount := map[string]int{}
	denyByTool := map[string]int{}
	sessionSet := map[uuid.UUID]bool{}
	for _, e := range entries {
		agg.CountByDecision[e.Decision]++
		toolCount[e.ToolName]++
		if e.Decision == AuditDecisionDeny || e.Decision == AuditDecisionConfirmDenied {
			denyByTool[e.ToolName]++
		}
		sessionSet[e.SessionID] = true
	}
	agg.CountByTool = mapToSortedBuckets(toolCount)
	agg.TopDeniedTools = mapToSortedBuckets(denyByTool)
	agg.UniqueSessions = len(sessionSet)
	return agg
}

func mapToSortedBuckets(m map[string]int) []PermissionAuditCountByTool {
	buckets := make([]PermissionAuditCountByTool, 0, len(m))
	for name, count := range m {
		buckets = append(buckets, PermissionAuditCountByTool{ToolName: name, Count: count})
	}
	sort.SliceStable(buckets, func(i, j int) bool {
		if buckets[i].Count != buckets[j].Count {
			return buckets[i].Count > buckets[j].Count
		}
		return buckets[i].ToolName < buckets[j].ToolName
	})
	return buckets
}

// PermissionAuditRetentionPolicy declares how long each decision tier
// is kept.
type PermissionAuditRetentionPolicy struct {
	// AllowTTL — how long Allow decisions are kept. 0 disables (kept forever).
	AllowTTL time.Duration
	// DenyTTL — how long Deny decisions are kept. Typically the longest.
	DenyTTL time.Duration
	// ConfirmApprovedTTL — how long approved confirm decisions are kept.
	ConfirmApprovedTTL time.Duration
	// ConfirmDeniedTTL — how long user-denied confirm decisions are kept.
	ConfirmDeniedTTL time.Duration
	// ConfirmEscalatedTTL — how long escalated confirms are kept.
	ConfirmEscalatedTTL time.Duration
}

// Validate enforces non-negative TTLs.
func (p PermissionAuditRetentionPolicy) Validate() error {
	if p.AllowTTL < 0 || p.DenyTTL < 0 || p.ConfirmApprovedTTL < 0 ||
		p.ConfirmDeniedTTL < 0 || p.ConfirmEscalatedTTL < 0 {
		return ErrPermissionAuditNegativeTTL
	}
	return nil
}

// shouldExpire returns true if the entry has aged past its tier TTL.
func (p PermissionAuditRetentionPolicy) shouldExpire(e PermissionAuditEntry, now time.Time) bool {
	ttl := p.ttlFor(e.Decision)
	if ttl == 0 {
		return false // 0 disables retention for that tier (kept forever)
	}
	return now.Sub(e.CreatedAt) >= ttl
}

func (p PermissionAuditRetentionPolicy) ttlFor(d PermissionAuditDecision) time.Duration {
	switch d {
	case AuditDecisionAllow:
		return p.AllowTTL
	case AuditDecisionDeny:
		return p.DenyTTL
	case AuditDecisionConfirmApproved:
		return p.ConfirmApprovedTTL
	case AuditDecisionConfirmDenied:
		return p.ConfirmDeniedTTL
	case AuditDecisionConfirmEscalated:
		return p.ConfirmEscalatedTTL
	}
	return 0
}

// PermissionAuditRedactionPolicy declares which fields to scrub for
// export (e.g., compliance dump that goes to an external auditor).
type PermissionAuditRedactionPolicy struct {
	// RedactInputSnippet — replace InputSnippet with "[REDACTED]".
	RedactInputSnippet bool
	// RedactMatchedRule — replace MatchedRule with "[REDACTED]".
	RedactMatchedRule bool
	// RedactRunID — clear RunID pointer.
	RedactRunID bool
}

// Redact applies the policy to a copy of the entry. Original is
// untouched.
func (p PermissionAuditRedactionPolicy) Redact(e PermissionAuditEntry) PermissionAuditEntry {
	out := e
	if p.RedactInputSnippet && out.InputSnippet != "" {
		out.InputSnippet = redactedMarker
	}
	if p.RedactMatchedRule && out.MatchedRule != "" {
		out.MatchedRule = redactedMarker
	}
	if p.RedactRunID {
		out.RunID = nil
	}
	return out
}

// RedactAll applies Redact to every entry in the slice; returns a new
// slice with redacted entries.
func (p PermissionAuditRedactionPolicy) RedactAll(in []PermissionAuditEntry) []PermissionAuditEntry {
	out := make([]PermissionAuditEntry, len(in))
	for i, e := range in {
		out[i] = p.Redact(e)
	}
	return out
}

const redactedMarker = "[REDACTED]"

// Sentinel errors.
var (
	ErrPermissionAuditNegativeTTL = errors.New("permission audit: retention TTL must be >= 0")
)

// FormatAggregateSummary renders a one-line human-readable summary of
// the aggregate (audit dashboards, log lines).
func FormatAggregateSummary(agg PermissionAuditAggregate) string {
	parts := []string{
		fmt.Sprintf("total=%d", agg.TotalEntries),
		fmt.Sprintf("sessions=%d", agg.UniqueSessions),
	}
	keys := make([]string, 0, len(agg.CountByDecision))
	for k := range agg.CountByDecision {
		keys = append(keys, string(k))
	}
	sort.Strings(keys)
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, agg.CountByDecision[PermissionAuditDecision(k)]))
	}
	return strings.Join(parts, " ")
}
