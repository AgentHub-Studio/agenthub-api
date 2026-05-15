package agentic

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// FUTURE-002 — Longitudinal user-agent relationship state.
//
// PDF arXiv:2604.14228v1 §12 (Future Directions — agent maintains
// long-term relationship state with each user beyond memory facts);
// §11 (relationship signals feed adaptive behavior).
//
// Distinct from FUTURE-001 cross-session memory:
//   - FUTURE-001 = cross-session FACTS the agent remembers (preferences, etc.).
//   - FUTURE-002 = META RELATIONSHIP state: trust level, rapport score,
//     communication style, interaction history.
//
// The agent uses this state to:
//   - Pick appropriate tone (chatty for established, formal for probationary).
//   - Calibrate confidence (trusted users get more autonomy).
//   - Detect deteriorating rapport (escalate to human review).
//   - Personalize responses over time (learn the user's preferred verbosity).

// TrustLevel bounded enum tracks accumulated trust signal.
type TrustLevel string

const (
	// TrustLevelUnknown — first interaction; no signal yet.
	TrustLevelUnknown TrustLevel = "unknown"
	// TrustLevelProbationary — early interactions; agent verifies before acting.
	TrustLevelProbationary TrustLevel = "probationary"
	// TrustLevelEstablished — sufficient history; default trust applies.
	TrustLevelEstablished TrustLevel = "established"
	// TrustLevelTrusted — extended positive history; agent grants more autonomy.
	TrustLevelTrusted TrustLevel = "trusted"
	// TrustLevelMistrusted — past negative signals (jailbreak attempts,
	// repeated abuse); agent applies stricter scrutiny.
	TrustLevelMistrusted TrustLevel = "mistrusted"
)

// allTrustLevels is the closed bounded set in low→high order
// (mistrusted is its own band, not on the ladder).
var allTrustLevels = []TrustLevel{
	TrustLevelUnknown, TrustLevelProbationary, TrustLevelEstablished,
	TrustLevelTrusted, TrustLevelMistrusted,
}

// IsValidTrustLevel returns true for the bounded set.
func IsValidTrustLevel(l TrustLevel) bool {
	for _, v := range allTrustLevels {
		if l == v {
			return true
		}
	}
	return false
}

// AllTrustLevels returns a copy.
func AllTrustLevels() []TrustLevel {
	out := make([]TrustLevel, len(allTrustLevels))
	copy(out, allTrustLevels)
	return out
}

// RapportEvent bounded enum classifies single interaction outcomes
// that move rapport.
type RapportEvent string

const (
	// RapportEventPositive — user expressed satisfaction (thanks, accept).
	RapportEventPositive RapportEvent = "positive"
	// RapportEventNeutral — no signal either way.
	RapportEventNeutral RapportEvent = "neutral"
	// RapportEventNegative — user expressed dissatisfaction.
	RapportEventNegative RapportEvent = "negative"
	// RapportEventConflictResolved — past negative resolved positively.
	RapportEventConflictResolved RapportEvent = "conflict_resolved"
	// RapportEventEscalation — interaction escalated to human / abandoned.
	RapportEventEscalation RapportEvent = "escalation"
)

var allRapportEvents = []RapportEvent{
	RapportEventPositive, RapportEventNeutral, RapportEventNegative,
	RapportEventConflictResolved, RapportEventEscalation,
}

// IsValidRapportEvent returns true for the bounded set.
func IsValidRapportEvent(e RapportEvent) bool {
	for _, v := range allRapportEvents {
		if e == v {
			return true
		}
	}
	return false
}

// AllRapportEvents returns a copy.
func AllRapportEvents() []RapportEvent {
	out := make([]RapportEvent, len(allRapportEvents))
	copy(out, allRapportEvents)
	return out
}

// rapportDelta returns the score adjustment per event.
// Range is [-1, +1] cumulative; clamped at boundaries.
func rapportDelta(e RapportEvent) float64 {
	switch e {
	case RapportEventPositive:
		return 0.05
	case RapportEventNeutral:
		return 0
	case RapportEventNegative:
		return -0.10
	case RapportEventConflictResolved:
		return 0.10
	case RapportEventEscalation:
		return -0.20
	}
	return 0
}

// CommunicationStyle classifies the user's preferred style. Stable
// strings — analytics aggregate by style.
type CommunicationStyle string

const (
	CommunicationStyleUnspecified CommunicationStyle = "unspecified"
	CommunicationStyleFormal      CommunicationStyle = "formal"
	CommunicationStyleCasual      CommunicationStyle = "casual"
	CommunicationStyleTerse       CommunicationStyle = "terse"
	CommunicationStyleVerbose     CommunicationStyle = "verbose"
)

var allCommunicationStyles = []CommunicationStyle{
	CommunicationStyleUnspecified, CommunicationStyleFormal, CommunicationStyleCasual,
	CommunicationStyleTerse, CommunicationStyleVerbose,
}

// IsValidCommunicationStyle returns true for the bounded set.
func IsValidCommunicationStyle(s CommunicationStyle) bool {
	for _, v := range allCommunicationStyles {
		if s == v {
			return true
		}
	}
	return false
}

// AllCommunicationStyles returns a copy.
func AllCommunicationStyles() []CommunicationStyle {
	out := make([]CommunicationStyle, len(allCommunicationStyles))
	copy(out, allCommunicationStyles)
	return out
}

// UserAgentRelationship is the longitudinal state per (tenant + user + agent).
type UserAgentRelationship struct {
	ID               uuid.UUID          `json:"id"`
	TenantID         string             `json:"tenantId"`
	UserID           string             `json:"userId"`
	AgentID          string             `json:"agentId"`
	TrustLevel       TrustLevel         `json:"trustLevel"`
	InteractionCount int                `json:"interactionCount"`
	// RapportScore in [-1, +1]. Defaults 0 (neutral).
	RapportScore       float64            `json:"rapportScore"`
	CommunicationStyle CommunicationStyle `json:"communicationStyle"`
	StartedAt          time.Time          `json:"startedAt"`
	LastInteractionAt  time.Time          `json:"lastInteractionAt,omitempty"`
}

// Sentinels.
var (
	ErrRelationshipNotFound      = errors.New("user-agent relationship: not found")
	ErrInvalidTrustLevel         = errors.New("user-agent relationship: invalid trust level")
	ErrInvalidRapportEvent       = errors.New("user-agent relationship: invalid rapport event")
	ErrInvalidCommunicationStyle = errors.New("user-agent relationship: invalid communication style")
)

func validateRelationship(r UserAgentRelationship) error {
	if r.TenantID == "" {
		return errors.New("user-agent relationship: tenantId required")
	}
	if r.UserID == "" {
		return errors.New("user-agent relationship: userId required")
	}
	if r.AgentID == "" {
		return errors.New("user-agent relationship: agentId required")
	}
	if r.TrustLevel == "" {
		r.TrustLevel = TrustLevelUnknown
	}
	if !IsValidTrustLevel(r.TrustLevel) {
		return fmt.Errorf("%w: %q", ErrInvalidTrustLevel, r.TrustLevel)
	}
	if r.CommunicationStyle == "" {
		r.CommunicationStyle = CommunicationStyleUnspecified
	}
	if !IsValidCommunicationStyle(r.CommunicationStyle) {
		return fmt.Errorf("%w: %q", ErrInvalidCommunicationStyle, r.CommunicationStyle)
	}
	return nil
}

// promoteTrust returns the next trust level after N positive interactions.
// Demotion to mistrusted requires explicit signal (not just count).
func promoteTrust(current TrustLevel, count int, rapport float64) TrustLevel {
	if current == TrustLevelMistrusted {
		return current
	}
	switch {
	case count >= 50 && rapport >= 0.5:
		return TrustLevelTrusted
	case count >= 10 && rapport >= 0:
		return TrustLevelEstablished
	case count >= 1:
		return TrustLevelProbationary
	}
	return TrustLevelUnknown
}

// applyEvent mutates a relationship in place, applying the event delta
// to RapportScore (clamped) and incrementing InteractionCount. Updates
// TrustLevel via promotion rules, with explicit demotion on escalation.
func applyEvent(r *UserAgentRelationship, e RapportEvent) {
	r.InteractionCount++
	r.RapportScore += rapportDelta(e)
	if r.RapportScore > 1.0 {
		r.RapportScore = 1.0
	}
	if r.RapportScore < -1.0 {
		r.RapportScore = -1.0
	}
	// Escalation alone can demote to mistrusted if score goes below -0.5.
	if e == RapportEventEscalation && r.RapportScore <= -0.5 {
		r.TrustLevel = TrustLevelMistrusted
	} else {
		r.TrustLevel = promoteTrust(r.TrustLevel, r.InteractionCount, r.RapportScore)
	}
	r.LastInteractionAt = time.Now()
}

// RelationshipStore is the persistence interface.
type RelationshipStore interface {
	// Save persists a relationship. ID + StartedAt assigned at first save.
	Save(ctx context.Context, r UserAgentRelationship) (UserAgentRelationship, error)
	// FindByPair returns the relationship for (tenant, user, agent).
	FindByPair(ctx context.Context, tenantID, userID, agentID string) (UserAgentRelationship, error)
	// RecordEvent fetches the relationship, applies the event, persists.
	// Auto-creates the relationship on first event.
	RecordEvent(ctx context.Context, tenantID, userID, agentID string, event RapportEvent) (UserAgentRelationship, error)
	// CountByTrust returns histogram of trust levels for a tenant.
	CountByTrust(ctx context.Context, tenantID string) (map[TrustLevel]int, error)
}

// --- InMemoryRelationshipStore ---

// InMemoryRelationshipStore is the default in-memory impl.
type InMemoryRelationshipStore struct {
	mu    sync.Mutex
	byKey map[string]UserAgentRelationship // key = tenant|user|agent
}

// NewInMemoryRelationshipStore creates an empty store.
func NewInMemoryRelationshipStore() *InMemoryRelationshipStore {
	return &InMemoryRelationshipStore{
		byKey: map[string]UserAgentRelationship{},
	}
}

func relationshipKey(tenantID, userID, agentID string) string {
	return tenantID + "|" + userID + "|" + agentID
}

// Save persists a relationship. Defaults applied + validation enforced.
func (s *InMemoryRelationshipStore) Save(ctx context.Context, r UserAgentRelationship) (UserAgentRelationship, error) {
	if err := ctx.Err(); err != nil {
		return UserAgentRelationship{}, err
	}
	if r.TrustLevel == "" {
		r.TrustLevel = TrustLevelUnknown
	}
	if r.CommunicationStyle == "" {
		r.CommunicationStyle = CommunicationStyleUnspecified
	}
	if err := validateRelationship(r); err != nil {
		return UserAgentRelationship{}, err
	}
	key := relationshipKey(r.TenantID, r.UserID, r.AgentID)
	s.mu.Lock()
	existing, exists := s.byKey[key]
	if exists {
		// Preserve ID + StartedAt on update.
		r.ID = existing.ID
		r.StartedAt = existing.StartedAt
	} else {
		r.ID = uuid.New()
		r.StartedAt = time.Now()
	}
	s.byKey[key] = r
	s.mu.Unlock()
	return r, nil
}

// FindByPair returns the relationship for (tenant, user, agent).
func (s *InMemoryRelationshipStore) FindByPair(ctx context.Context, tenantID, userID, agentID string) (UserAgentRelationship, error) {
	if err := ctx.Err(); err != nil {
		return UserAgentRelationship{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.byKey[relationshipKey(tenantID, userID, agentID)]
	if !ok {
		return UserAgentRelationship{}, ErrRelationshipNotFound
	}
	return r, nil
}

// RecordEvent applies an event to the relationship (auto-creating if missing).
func (s *InMemoryRelationshipStore) RecordEvent(ctx context.Context, tenantID, userID, agentID string, event RapportEvent) (UserAgentRelationship, error) {
	if err := ctx.Err(); err != nil {
		return UserAgentRelationship{}, err
	}
	if !IsValidRapportEvent(event) {
		return UserAgentRelationship{}, fmt.Errorf("%w: %q", ErrInvalidRapportEvent, event)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := relationshipKey(tenantID, userID, agentID)
	r, exists := s.byKey[key]
	if !exists {
		r = UserAgentRelationship{
			ID: uuid.New(), TenantID: tenantID, UserID: userID, AgentID: agentID,
			TrustLevel: TrustLevelUnknown, CommunicationStyle: CommunicationStyleUnspecified,
			StartedAt: time.Now(),
		}
	}
	applyEvent(&r, event)
	s.byKey[key] = r
	return r, nil
}

// CountByTrust returns histogram (always 5 keys with 0 default).
func (s *InMemoryRelationshipStore) CountByTrust(ctx context.Context, tenantID string) (map[TrustLevel]int, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	hist := map[TrustLevel]int{}
	for _, l := range allTrustLevels {
		hist[l] = 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.byKey {
		if r.TenantID == tenantID {
			hist[r.TrustLevel]++
		}
	}
	return hist, nil
}
