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

// CTX-006 — Auto memory.
//
// PDF arXiv:2604.14228v1 §7.5 (auto-memory classifier: agent decides
// without admin prompting whether a turn contains memorable content,
// classifies it into the 4-type taxonomy, and stores it autonomously).
//
// Distinct from existing memory plumbing:
//   - memory.go = MemoryEvaluator interface for LLM-driven extraction
//     (ExtractedMemory carries Type/MemoryType field; legacy + new format).
//   - cross_session_memory.go (FUTURE-001) = durable substrate.
//   - memory_hierarchy.go (CTX-003) = layered lookup with most-specific-wins.
//   - auto_memory.go (this file) = AUTOMATED classifier + decision pipeline:
//     consumes a turn, emits AutoMemoryDecisions with confidence scores,
//     filters by min-confidence + admin-blocked-keys, stores survivors.
//
// HeuristicAutoMemoryClassifier is the deterministic fallback used when
// the LLM evaluator is unavailable (cold start, cost cap, eval failure).
// It matches a small set of well-known patterns to keep the agent
// learning even when the LLM path is offline.

// AutoMemoryType bounded enum mirrors the 4-type taxonomy from PDF
// §7.5 / existing ExtractedMemory.Type.
type AutoMemoryType string

const (
	// AutoMemoryTypeUser — facts about the user (preferences, role, etc.).
	AutoMemoryTypeUser AutoMemoryType = "user"
	// AutoMemoryTypeFeedback — corrections / praise about agent behavior.
	AutoMemoryTypeFeedback AutoMemoryType = "feedback"
	// AutoMemoryTypeProject — facts about the project / domain context.
	AutoMemoryTypeProject AutoMemoryType = "project"
	// AutoMemoryTypeReference — links / pointers to external resources.
	AutoMemoryTypeReference AutoMemoryType = "reference"
)

var allAutoMemoryTypes = []AutoMemoryType{
	AutoMemoryTypeUser, AutoMemoryTypeFeedback,
	AutoMemoryTypeProject, AutoMemoryTypeReference,
}

// IsValidAutoMemoryType returns true for the bounded set.
func IsValidAutoMemoryType(t AutoMemoryType) bool {
	for _, v := range allAutoMemoryTypes {
		if t == v {
			return true
		}
	}
	return false
}

// AllAutoMemoryTypes returns a copy.
func AllAutoMemoryTypes() []AutoMemoryType {
	out := make([]AutoMemoryType, len(allAutoMemoryTypes))
	copy(out, allAutoMemoryTypes)
	return out
}

// AutoMemoryDecision is a single classifier output.
type AutoMemoryDecision struct {
	Type        AutoMemoryType `json:"type"`
	Key         string         `json:"key"`
	Value       string         `json:"value"`
	// Confidence in [0, 1]. Decisions below MinConfidence are dropped.
	Confidence  float64        `json:"confidence"`
	// Rationale is a short human-readable why-this-was-classified note.
	Rationale   string         `json:"rationale"`
	// SourceMessage is the turn message that triggered the decision
	// (truncated for audit).
	SourceMessage string       `json:"sourceMessage,omitempty"`
}

// AutoMemoryRecord is the persisted form: a Decision plus an ID, tenant
// + user identity, and a stored-at timestamp.
type AutoMemoryRecord struct {
	ID         uuid.UUID         `json:"id"`
	TenantID   string            `json:"tenantId"`
	UserID     string            `json:"userId"`
	Decision   AutoMemoryDecision `json:"decision"`
	StoredAt   time.Time         `json:"storedAt"`
}

// AutoMemoryConfig tunes the classifier and storage pipeline.
type AutoMemoryConfig struct {
	// MinConfidence in [0, 1]. Decisions strictly below are dropped.
	MinConfidence float64
	// MaxPerTurn caps how many decisions one turn can produce. 0 = unlimited.
	MaxPerTurn int
	// AdminBlockedKeys is the closed set of keys never auto-stored
	// (e.g. things like "password", "credit_card" — sensitive).
	AdminBlockedKeys []string
}

// DefaultAutoMemoryConfig returns sensible defaults.
func DefaultAutoMemoryConfig() AutoMemoryConfig {
	return AutoMemoryConfig{
		MinConfidence: 0.5,
		MaxPerTurn:    10,
		AdminBlockedKeys: []string{
			"password", "credit_card", "ssn", "api_key", "secret",
		},
	}
}

// Sentinels.
var (
	ErrAutoMemoryInvalidType    = errors.New("auto memory: invalid type")
	ErrAutoMemoryKeyEmpty       = errors.New("auto memory: key required")
	ErrAutoMemoryValueEmpty     = errors.New("auto memory: value required")
	ErrAutoMemoryConfidenceRange = errors.New("auto memory: confidence out of [0, 1]")
	ErrAutoMemoryBlocked        = errors.New("auto memory: key is in admin block list")
	ErrAutoMemoryRecordNotFound = errors.New("auto memory: record not found")
	ErrAutoMemoryTenantRequired = errors.New("auto memory: tenant_id required")
	ErrAutoMemoryUserRequired   = errors.New("auto memory: user_id required")
)

// AutoMemoryClassifier emits decisions for a turn message.
type AutoMemoryClassifier interface {
	Classify(ctx context.Context, message string) ([]AutoMemoryDecision, error)
}

// AutoMemoryStore persists decisions that pass the gate.
type AutoMemoryStore interface {
	Store(ctx context.Context, tenantID, userID string, d AutoMemoryDecision) (AutoMemoryRecord, error)
	Find(ctx context.Context, id uuid.UUID) (AutoMemoryRecord, error)
	ListByType(ctx context.Context, tenantID, userID string, mtype AutoMemoryType) ([]AutoMemoryRecord, error)
	ListByUser(ctx context.Context, tenantID, userID string) ([]AutoMemoryRecord, error)
}

// validateDecision checks structural invariants.
func validateDecision(d AutoMemoryDecision) error {
	if !IsValidAutoMemoryType(d.Type) {
		return fmt.Errorf("%w: %q", ErrAutoMemoryInvalidType, d.Type)
	}
	if strings.TrimSpace(d.Key) == "" {
		return ErrAutoMemoryKeyEmpty
	}
	if strings.TrimSpace(d.Value) == "" {
		return ErrAutoMemoryValueEmpty
	}
	if d.Confidence < 0 || d.Confidence > 1 {
		return fmt.Errorf("%w: %f", ErrAutoMemoryConfidenceRange, d.Confidence)
	}
	return nil
}

// FilterByConfig applies MinConfidence + AdminBlockedKeys + MaxPerTurn.
// Returns the surviving decisions in the same order, plus a slice of
// reasons keyed by the dropped indices for audit.
type DropReason struct {
	Index  int
	Reason string
}

func FilterByConfig(decisions []AutoMemoryDecision, cfg AutoMemoryConfig) ([]AutoMemoryDecision, []DropReason) {
	blocked := map[string]bool{}
	for _, k := range cfg.AdminBlockedKeys {
		blocked[strings.ToLower(k)] = true
	}
	survivors := []AutoMemoryDecision{}
	dropped := []DropReason{}
	for i, d := range decisions {
		if d.Confidence < cfg.MinConfidence {
			dropped = append(dropped, DropReason{
				Index: i,
				Reason: fmt.Sprintf("confidence %.2f < min %.2f", d.Confidence, cfg.MinConfidence),
			})
			continue
		}
		if blocked[strings.ToLower(d.Key)] {
			dropped = append(dropped, DropReason{
				Index:  i,
				Reason: fmt.Sprintf("key %q in admin block list", d.Key),
			})
			continue
		}
		if cfg.MaxPerTurn > 0 && len(survivors) >= cfg.MaxPerTurn {
			dropped = append(dropped, DropReason{
				Index:  i,
				Reason: fmt.Sprintf("max per turn (%d) reached", cfg.MaxPerTurn),
			})
			continue
		}
		survivors = append(survivors, d)
	}
	return survivors, dropped
}

// --- HeuristicAutoMemoryClassifier ---
//
// Deterministic rule-based classifier used when the LLM evaluator is
// unavailable. Catches well-known patterns: explicit "remember that..."
// statements, name/role declarations, URLs (reference type), corrective
// language ("don't ...", "stop ...").

type HeuristicAutoMemoryClassifier struct {
	// MaxBodyLength truncates Value to this many characters (defensive
	// against pathological turn lengths).
	MaxBodyLength int
}

// NewHeuristicAutoMemoryClassifier returns a deterministic classifier.
func NewHeuristicAutoMemoryClassifier() *HeuristicAutoMemoryClassifier {
	return &HeuristicAutoMemoryClassifier{MaxBodyLength: 500}
}

// Classify applies heuristic rules. Returns decisions ordered by
// confidence descending.
func (c *HeuristicAutoMemoryClassifier) Classify(ctx context.Context, message string) ([]AutoMemoryDecision, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out := []AutoMemoryDecision{}
	lower := strings.ToLower(message)
	maxLen := c.MaxBodyLength
	if maxLen <= 0 {
		maxLen = 500
	}

	truncate := func(s string) string {
		if len(s) <= maxLen {
			return s
		}
		return s[:maxLen-3] + "..."
	}

	// Rule 1: explicit "remember that X" → high confidence project fact.
	for _, prefix := range []string{"remember that ", "remember: ", "lembre-se que ", "lembre que "} {
		if idx := strings.Index(lower, prefix); idx >= 0 {
			val := strings.TrimSpace(message[idx+len(prefix):])
			if val != "" {
				out = append(out, AutoMemoryDecision{
					Type: AutoMemoryTypeProject, Key: "explicit_note",
					Value: truncate(val), Confidence: 0.95,
					Rationale: "explicit user remember-statement",
					SourceMessage: truncate(message),
				})
			}
		}
	}

	// Rule 2: name declaration "my name is X" / "i'm X" / "meu nome é X".
	for _, prefix := range []string{"my name is ", "i'm ", "i am ", "meu nome é ", "meu nome e ", "me chamo "} {
		if idx := strings.Index(lower, prefix); idx >= 0 {
			val := strings.TrimSpace(message[idx+len(prefix):])
			// Take first token as the name.
			if val != "" {
				name := strings.SplitN(val, " ", 2)[0]
				name = strings.TrimRight(name, ".,!?")
				if name != "" {
					out = append(out, AutoMemoryDecision{
						Type: AutoMemoryTypeUser, Key: "name",
						Value: name, Confidence: 0.85,
						Rationale: "name declaration heuristic",
						SourceMessage: truncate(message),
					})
				}
			}
		}
	}

	// Rule 3: corrective "don't" / "stop" → feedback.
	for _, marker := range []string{"don't ", "do not ", "stop ", "não faça ", "nao faca "} {
		if idx := strings.Index(lower, marker); idx >= 0 {
			val := strings.TrimSpace(message[idx:])
			out = append(out, AutoMemoryDecision{
				Type: AutoMemoryTypeFeedback, Key: "correction",
				Value: truncate(val), Confidence: 0.7,
				Rationale: "corrective-language heuristic",
				SourceMessage: truncate(message),
			})
			break
		}
	}

	// Rule 4: URLs → reference.
	for _, scheme := range []string{"http://", "https://"} {
		if idx := strings.Index(lower, scheme); idx >= 0 {
			rest := message[idx:]
			end := strings.IndexAny(rest, " \t\n\r,;)")
			if end < 0 {
				end = len(rest)
			}
			url := rest[:end]
			out = append(out, AutoMemoryDecision{
				Type: AutoMemoryTypeReference, Key: "url",
				Value: truncate(url), Confidence: 0.8,
				Rationale: "URL reference heuristic",
				SourceMessage: truncate(message),
			})
			break
		}
	}

	// Sort by confidence descending so highest-priority decisions
	// reach the gate first (matters when MaxPerTurn is small).
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Confidence > out[j].Confidence
	})
	return out, nil
}

// --- InMemoryAutoMemoryStore ---

type InMemoryAutoMemoryStore struct {
	mu      sync.Mutex
	records map[uuid.UUID]AutoMemoryRecord
}

// NewInMemoryAutoMemoryStore returns a concurrent-safe store.
func NewInMemoryAutoMemoryStore() *InMemoryAutoMemoryStore {
	return &InMemoryAutoMemoryStore{records: map[uuid.UUID]AutoMemoryRecord{}}
}

// Store persists a decision after validating it.
func (s *InMemoryAutoMemoryStore) Store(ctx context.Context, tenantID, userID string, d AutoMemoryDecision) (AutoMemoryRecord, error) {
	if err := ctx.Err(); err != nil {
		return AutoMemoryRecord{}, err
	}
	if strings.TrimSpace(tenantID) == "" {
		return AutoMemoryRecord{}, ErrAutoMemoryTenantRequired
	}
	if strings.TrimSpace(userID) == "" {
		return AutoMemoryRecord{}, ErrAutoMemoryUserRequired
	}
	if err := validateDecision(d); err != nil {
		return AutoMemoryRecord{}, err
	}
	r := AutoMemoryRecord{
		ID: uuid.New(), TenantID: tenantID, UserID: userID,
		Decision: d, StoredAt: time.Now(),
	}
	s.mu.Lock()
	s.records[r.ID] = r
	s.mu.Unlock()
	return r, nil
}

// Find returns one record by ID.
func (s *InMemoryAutoMemoryStore) Find(ctx context.Context, id uuid.UUID) (AutoMemoryRecord, error) {
	if err := ctx.Err(); err != nil {
		return AutoMemoryRecord{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.records[id]
	if !ok {
		return AutoMemoryRecord{}, ErrAutoMemoryRecordNotFound
	}
	return r, nil
}

// ListByType returns records of one type for one user, newest first.
func (s *InMemoryAutoMemoryStore) ListByType(ctx context.Context, tenantID, userID string, mtype AutoMemoryType) ([]AutoMemoryRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []AutoMemoryRecord{}
	for _, r := range s.records {
		if r.TenantID != tenantID || r.UserID != userID || r.Decision.Type != mtype {
			continue
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].StoredAt.After(out[j].StoredAt)
	})
	return out, nil
}

// ListByUser returns all records for one user, newest first.
func (s *InMemoryAutoMemoryStore) ListByUser(ctx context.Context, tenantID, userID string) ([]AutoMemoryRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []AutoMemoryRecord{}
	for _, r := range s.records {
		if r.TenantID != tenantID || r.UserID != userID {
			continue
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].StoredAt.After(out[j].StoredAt)
	})
	return out, nil
}

// AutoMemoryPipeline composes Classifier + Filter + Store. Returns
// stored records and the dropped reasons.
type AutoMemoryPipelineResult struct {
	Stored  []AutoMemoryRecord `json:"stored"`
	Dropped []DropReason       `json:"dropped"`
}

// RunAutoMemoryPipeline classifies the message, filters by config, and
// stores survivors. Returns details for both stored and dropped sides.
func RunAutoMemoryPipeline(
	ctx context.Context,
	classifier AutoMemoryClassifier,
	store AutoMemoryStore,
	cfg AutoMemoryConfig,
	tenantID, userID, message string,
) (AutoMemoryPipelineResult, error) {
	decisions, err := classifier.Classify(ctx, message)
	if err != nil {
		return AutoMemoryPipelineResult{}, err
	}
	survivors, dropped := FilterByConfig(decisions, cfg)
	stored := []AutoMemoryRecord{}
	for _, d := range survivors {
		r, err := store.Store(ctx, tenantID, userID, d)
		if err != nil {
			return AutoMemoryPipelineResult{}, err
		}
		stored = append(stored, r)
	}
	return AutoMemoryPipelineResult{Stored: stored, Dropped: dropped}, nil
}
