package agentic

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// HUMAN-003 — Decision records.
//
// PDF arXiv:2604.14228v1 §11 (agent-made runtime decisions need to be
// recorded as durable, queryable artifacts so future humans + future
// agents understand WHY past choices were made); §6.1 (decision
// provenance is part of the audit surface).
//
// Inspired by ADRs (Architecture Decision Records) but adapted for
// agent runtime: instead of human committee decisions about codebase
// architecture, these capture per-run / per-agent decisions like:
//   - "Chose REST API X over GraphQL Y because retries are simpler"
//   - "Decided not to retry the failed tool call because cost ceiling"
//   - "Picked openai/gpt-4o-mini over claude because user requested cost"
//
// Three goals:
//   1. AUDIT — compliance can ask "why did the agent do this?"
//   2. LEARNING — future runs can reference past decisions to stay consistent
//   3. SUPERSEDING — when context changes, mark old decisions as superseded
//      with link to the new one (immutable history with chains)
//
// Distinction from neighbouring abstractions:
//   - GOV-004 PermissionExplanation explains a runtime allow/deny.
//   - HUMAN-001 ReviewGuidance directs human attention.
//   - HUMAN-002 ExplainedDiff annotates code hunks.
//   - HUMAN-003 DecisionRecord captures WHY a CHOICE was made (broader than diff/permission).

// DecisionStatus bounded enum mirrors ADR lifecycle.
type DecisionStatus string

const (
	// DecisionStatusProposed — under consideration, not yet acted on.
	DecisionStatusProposed DecisionStatus = "proposed"
	// DecisionStatusAccepted — adopted; agent acted on this.
	DecisionStatusAccepted DecisionStatus = "accepted"
	// DecisionStatusSuperseded — replaced by a newer decision (link via SupersededBy).
	DecisionStatusSuperseded DecisionStatus = "superseded"
	// DecisionStatusRejected — explicitly NOT chosen (e.g. evaluator rejected).
	DecisionStatusRejected DecisionStatus = "rejected"
	// DecisionStatusDeprecated — context changed; no longer applicable.
	DecisionStatusDeprecated DecisionStatus = "deprecated"
)

// allDecisionStatuses is the closed bounded set.
var allDecisionStatuses = []DecisionStatus{
	DecisionStatusProposed,
	DecisionStatusAccepted,
	DecisionStatusSuperseded,
	DecisionStatusRejected,
	DecisionStatusDeprecated,
}

// IsValidDecisionStatus returns true for the bounded set.
func IsValidDecisionStatus(s DecisionStatus) bool {
	for _, v := range allDecisionStatuses {
		if s == v {
			return true
		}
	}
	return false
}

// AllDecisionStatuses returns a copy of the bounded set.
func AllDecisionStatuses() []DecisionStatus {
	out := make([]DecisionStatus, len(allDecisionStatuses))
	copy(out, allDecisionStatuses)
	return out
}

// DecisionImpact bounded enum classifies the scope of the decision.
// Stable strings — analytics aggregate by impact.
type DecisionImpact string

const (
	// DecisionImpactLocal — affects only this run / this turn.
	DecisionImpactLocal DecisionImpact = "local"
	// DecisionImpactModule — affects a module / skill / KB.
	DecisionImpactModule DecisionImpact = "module"
	// DecisionImpactSystem — affects the whole system / tenant.
	DecisionImpactSystem DecisionImpact = "system"
	// DecisionImpactBusiness — affects business outcomes / customer-facing surface.
	DecisionImpactBusiness DecisionImpact = "business"
)

// IsValidDecisionImpact returns true for the bounded set.
func IsValidDecisionImpact(i DecisionImpact) bool {
	switch i {
	case DecisionImpactLocal, DecisionImpactModule, DecisionImpactSystem, DecisionImpactBusiness:
		return true
	}
	return false
}

// AlternativeOption is one option that was considered + WHY rejected.
type AlternativeOption struct {
	// Title is the option name (e.g. "GraphQL", "Stripe webhook").
	Title string `json:"title"`
	// Pros is the list of advantages noted.
	Pros []string `json:"pros,omitempty"`
	// Cons is the list of disadvantages noted.
	Cons []string `json:"cons,omitempty"`
	// WhyRejected is the short reason this was NOT picked (≤ 300 chars).
	WhyRejected string `json:"whyRejected"`
}

// DecisionRecord captures a single agent-made decision.
type DecisionRecord struct {
	// ID is the storage-layer identifier (assigned at Save).
	ID uuid.UUID `json:"id"`
	// TenantID scopes the record. Required.
	TenantID string `json:"tenantId"`
	// AgentID identifies the agent that made the decision.
	AgentID string `json:"agentId"`
	// RunID correlates back to the originating run trace.
	RunID string `json:"runId"`
	// Title is a one-line summary (≤ 200 chars).
	Title string `json:"title"`
	// Context describes the situation that prompted the decision (≤ 1000 chars).
	Context string `json:"context"`
	// Alternatives is the list of options considered (the chosen one is
	// represented by Choice — alternatives are everything else).
	Alternatives []AlternativeOption `json:"alternatives,omitempty"`
	// Choice is the option that was chosen.
	Choice string `json:"choice"`
	// Rationale is the WHY for the choice (≤ 1000 chars). Required.
	Rationale string `json:"rationale"`
	// Consequences are the foreseen consequences of the choice
	// (positive + negative). Each ≤ 200 chars.
	Consequences []string `json:"consequences,omitempty"`
	// Status tracks lifecycle. Default = accepted (the agent already acted).
	Status DecisionStatus `json:"status"`
	// Impact classifies the scope.
	Impact DecisionImpact `json:"impact"`
	// SupersededBy links to a newer decision (when Status=superseded).
	SupersededBy *uuid.UUID `json:"supersededBy,omitempty"`
	// RecordedAt is the wall-clock timestamp.
	RecordedAt time.Time `json:"recordedAt"`
}

// ErrInvalidDecisionStatus — defensive guard.
var ErrInvalidDecisionStatus = errors.New("decision: invalid status")

// ErrInvalidDecisionImpact — defensive guard.
var ErrInvalidDecisionImpact = errors.New("decision: invalid impact")

// ErrDecisionNotFound — sentinel.
var ErrDecisionNotFound = errors.New("decision: not found")

// ErrDecisionAlreadySuperseded — supersede attempted on a record
// that's already superseded. First-supersede-wins (matches checkpoint
// + quality report patterns).
var ErrDecisionAlreadySuperseded = errors.New("decision: already superseded")

// validateRecord checks required fields + bounded enums.
func validateRecord(r DecisionRecord) error {
	if r.TenantID == "" {
		return errors.New("decision: tenantId required")
	}
	if r.Title == "" {
		return errors.New("decision: title required")
	}
	if r.Choice == "" {
		return errors.New("decision: choice required")
	}
	if r.Rationale == "" {
		return errors.New("decision: rationale required")
	}
	if r.Status == "" {
		r.Status = DecisionStatusAccepted
	}
	if !IsValidDecisionStatus(r.Status) {
		return fmt.Errorf("%w: %q", ErrInvalidDecisionStatus, r.Status)
	}
	if r.Impact == "" {
		r.Impact = DecisionImpactLocal
	}
	if !IsValidDecisionImpact(r.Impact) {
		return fmt.Errorf("%w: %q", ErrInvalidDecisionImpact, r.Impact)
	}
	return nil
}

// DecisionRecordStore is the persistence interface. Implementations must
// be concurrent-safe and append-only (records are immutable after Save;
// only Status transitions allowed via Supersede / Deprecate).
type DecisionRecordStore interface {
	// Save persists a record. ID + RecordedAt are assigned by the store.
	// Status defaults to accepted, Impact defaults to local.
	Save(ctx context.Context, r DecisionRecord) (DecisionRecord, error)

	// FindByID returns a record by ID.
	FindByID(ctx context.Context, id uuid.UUID) (DecisionRecord, error)

	// ListByRun returns records for a run, newest-first.
	ListByRun(ctx context.Context, tenantID, runID string) ([]DecisionRecord, error)

	// ListByAgent returns records for an agent, newest-first, capped by limit.
	ListByAgent(ctx context.Context, tenantID, agentID string, limit int) ([]DecisionRecord, error)

	// Supersede transitions a record to Status=Superseded and links it
	// to the newer record. First-supersede-wins.
	Supersede(ctx context.Context, oldID, newID uuid.UUID) error

	// CountByStatus returns histogram of records by status (always
	// returns all 5 statuses with 0 default — stable axes).
	CountByStatus(ctx context.Context, tenantID string) (map[DecisionStatus]int, error)
}

// --- InMemoryDecisionRecordStore ---

// InMemoryDecisionRecordStore is the default in-memory implementation.
// Concurrent-safe via sync.RWMutex. Append-only enforced via value-copy.
type InMemoryDecisionRecordStore struct {
	mu      sync.RWMutex
	records map[uuid.UUID]DecisionRecord
	order   []uuid.UUID // insertion order, newest at end
}

// NewInMemoryDecisionRecordStore creates an empty store.
func NewInMemoryDecisionRecordStore() *InMemoryDecisionRecordStore {
	return &InMemoryDecisionRecordStore{
		records: map[uuid.UUID]DecisionRecord{},
	}
}

// Save persists a record. Defaults applied + validation enforced.
func (s *InMemoryDecisionRecordStore) Save(ctx context.Context, r DecisionRecord) (DecisionRecord, error) {
	if err := ctx.Err(); err != nil {
		return DecisionRecord{}, err
	}
	if r.Status == "" {
		r.Status = DecisionStatusAccepted
	}
	if r.Impact == "" {
		r.Impact = DecisionImpactLocal
	}
	if err := validateRecord(r); err != nil {
		return DecisionRecord{}, err
	}
	if len(r.Title) > 200 {
		r.Title = r.Title[:197] + "..."
	}
	if len(r.Context) > 1000 {
		r.Context = r.Context[:997] + "..."
	}
	if len(r.Rationale) > 1000 {
		r.Rationale = r.Rationale[:997] + "..."
	}
	r.ID = uuid.New()
	r.RecordedAt = time.Now()

	s.mu.Lock()
	s.records[r.ID] = r
	s.order = append(s.order, r.ID)
	s.mu.Unlock()
	return r, nil
}

// FindByID returns a record by ID.
func (s *InMemoryDecisionRecordStore) FindByID(ctx context.Context, id uuid.UUID) (DecisionRecord, error) {
	if err := ctx.Err(); err != nil {
		return DecisionRecord{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.records[id]
	if !ok {
		return DecisionRecord{}, ErrDecisionNotFound
	}
	return r, nil
}

// ListByRun returns records for a run, newest-first.
func (s *InMemoryDecisionRecordStore) ListByRun(ctx context.Context, tenantID, runID string) ([]DecisionRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	matched := []DecisionRecord{}
	for _, r := range s.records {
		if r.TenantID == tenantID && r.RunID == runID {
			matched = append(matched, r)
		}
	}
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].RecordedAt.After(matched[j].RecordedAt)
	})
	return matched, nil
}

// ListByAgent returns records for an agent, newest-first, capped.
func (s *InMemoryDecisionRecordStore) ListByAgent(ctx context.Context, tenantID, agentID string, limit int) ([]DecisionRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	matched := []DecisionRecord{}
	for _, r := range s.records {
		if r.TenantID == tenantID && r.AgentID == agentID {
			matched = append(matched, r)
		}
	}
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].RecordedAt.After(matched[j].RecordedAt)
	})
	if limit > 0 && len(matched) > limit {
		matched = matched[:limit]
	}
	return matched, nil
}

// Supersede transitions oldID to Status=Superseded and links to newID.
// First-supersede-wins; subsequent attempts return ErrDecisionAlreadySuperseded.
func (s *InMemoryDecisionRecordStore) Supersede(ctx context.Context, oldID, newID uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.records[oldID]
	if !ok {
		return ErrDecisionNotFound
	}
	if old.Status == DecisionStatusSuperseded {
		return ErrDecisionAlreadySuperseded
	}
	// Verify new record exists too — defensive.
	if _, ok := s.records[newID]; !ok {
		return fmt.Errorf("decision: new record %s does not exist", newID)
	}
	old.Status = DecisionStatusSuperseded
	old.SupersededBy = &newID
	s.records[oldID] = old
	return nil
}

// CountByStatus returns histogram with all 5 statuses (0 default).
func (s *InMemoryDecisionRecordStore) CountByStatus(ctx context.Context, tenantID string) (map[DecisionStatus]int, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	hist := map[DecisionStatus]int{}
	for _, st := range allDecisionStatuses {
		hist[st] = 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, r := range s.records {
		if r.TenantID == tenantID {
			hist[r.Status]++
		}
	}
	return hist, nil
}
