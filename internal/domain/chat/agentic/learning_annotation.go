package agentic

import (
	"errors"
	"fmt"
	"regexp"
	"sync"
	"time"
)

// HUMAN-007 — Learning-oriented documentation.
//
// PDF arXiv:2604.14228v1 §11 (agents should augment human understanding,
// not just complete tasks — surface teachable moments that accelerate
// the operator's mental model); §3.2 (progressive disclosure: reveal
// complexity only as the operator's confidence grows).
//
// When an agent completes a step it may emit a LearningAnnotation —
// a structured note explaining WHAT happened, WHY a particular approach
// was chosen, and WHAT the operator can learn from this interaction.
// Annotations are audience-tagged (beginner/intermediate/advanced)
// so UIs can apply progressive-disclosure filtering.
//
// Distinction from neighbouring HUMAN abstractions:
//   - HUMAN-001 ReviewGuidance   — directs attention AFTER an artifact.
//   - HUMAN-002 ExplainedDiff    — annotates AFTER a code change.
//   - HUMAN-003 DecisionRecord   — records WHY past decisions were made.
//   - HUMAN-004 UnderstandingCheckpoint — BEFORE-work comprehension.
//   - HUMAN-006 CapabilityMetrics — passive observation of capabilities.
//   - HUMAN-007 (this file)      — learning-oriented notes DURING execution.
//
// This is strictly OBSERVATIONAL: annotations never affect execution flow.
// They accumulate in a LearningAnnotationJournal (capped ring-buffer)
// and are rendered as a "learning feed" at session end.

// LearningAnnotationKind classifies the educational value of an annotation.
type LearningAnnotationKind string

const (
	// LearningAnnotationKindPattern records a successful approach worth repeating.
	LearningAnnotationKindPattern LearningAnnotationKind = "pattern"
	// LearningAnnotationKindAntiPattern flags a problematic approach to avoid.
	LearningAnnotationKindAntiPattern LearningAnnotationKind = "anti_pattern"
	// LearningAnnotationKindTip shares a shortcut or efficiency improvement.
	LearningAnnotationKindTip LearningAnnotationKind = "tip"
	// LearningAnnotationKindOptimization highlights a cost or performance gain.
	LearningAnnotationKindOptimization LearningAnnotationKind = "optimization"
	// LearningAnnotationKindKnowledgeGap signals a feature the operator seems unaware of.
	LearningAnnotationKindKnowledgeGap LearningAnnotationKind = "knowledge_gap"
)

// IsValid returns true when k is in the closed set.
func (k LearningAnnotationKind) IsValid() bool {
	switch k {
	case LearningAnnotationKindPattern, LearningAnnotationKindAntiPattern,
		LearningAnnotationKindTip, LearningAnnotationKindOptimization,
		LearningAnnotationKindKnowledgeGap:
		return true
	}
	return false
}

// AllLearningAnnotationKinds returns a defensive copy of every valid kind.
func AllLearningAnnotationKinds() []LearningAnnotationKind {
	return []LearningAnnotationKind{
		LearningAnnotationKindPattern,
		LearningAnnotationKindAntiPattern,
		LearningAnnotationKindTip,
		LearningAnnotationKindOptimization,
		LearningAnnotationKindKnowledgeGap,
	}
}

// LearningAnnotationAudience controls progressive-disclosure filtering.
// The hierarchy is beginner < intermediate < advanced: filtering for
// intermediate includes beginner annotations too.
type LearningAnnotationAudience string

const (
	// LearningAnnotationAudienceBeginner targets operators new to AgentHub.
	LearningAnnotationAudienceBeginner LearningAnnotationAudience = "beginner"
	// LearningAnnotationAudienceIntermediate targets operators with basic familiarity.
	LearningAnnotationAudienceIntermediate LearningAnnotationAudience = "intermediate"
	// LearningAnnotationAudienceAdvanced targets operators who understand the full stack.
	LearningAnnotationAudienceAdvanced LearningAnnotationAudience = "advanced"
)

// IsValid returns true when a is in the closed set.
func (a LearningAnnotationAudience) IsValid() bool {
	switch a {
	case LearningAnnotationAudienceBeginner,
		LearningAnnotationAudienceIntermediate,
		LearningAnnotationAudienceAdvanced:
		return true
	}
	return false
}

// AllLearningAnnotationAudiences returns a defensive copy of every valid audience.
func AllLearningAnnotationAudiences() []LearningAnnotationAudience {
	return []LearningAnnotationAudience{
		LearningAnnotationAudienceBeginner,
		LearningAnnotationAudienceIntermediate,
		LearningAnnotationAudienceAdvanced,
	}
}

// learningAudienceLevel maps audience to a numeric rank for >= comparisons.
func learningAudienceLevel(a LearningAnnotationAudience) int {
	switch a {
	case LearningAnnotationAudienceBeginner:
		return 0
	case LearningAnnotationAudienceIntermediate:
		return 1
	case LearningAnnotationAudienceAdvanced:
		return 2
	}
	return -1
}

// LearningAnnotationSlugRE is the kebab-case pattern annotations must satisfy.
var LearningAnnotationSlugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

// LearningAnnotation is a single teachable moment emitted during agent execution.
type LearningAnnotation struct {
	// AnnotationID is the unique identifier for this annotation (UUID string).
	AnnotationID string
	// SessionID ties the annotation to the agent session that produced it.
	SessionID string
	// AgentSlug identifies the agent that emitted the annotation.
	AgentSlug string
	// Kind classifies the educational value.
	Kind LearningAnnotationKind
	// Audience controls progressive disclosure; see FilterForAudience.
	Audience LearningAnnotationAudience
	// Title is a short headline (shown in collapsed list views).
	Title string
	// Content is the full learning note (shown in expanded view).
	Content string
	// RelatedFeatureSlugs links to AgentHub feature slugs relevant to this note.
	RelatedFeatureSlugs []string
	// EmittedAt is when the agent produced this annotation.
	EmittedAt time.Time
}

// Sentinel errors for LearningAnnotation operations.
var (
	ErrLearningAnnotationNil             = errors.New("learning annotation: annotation must not be nil")
	ErrLearningAnnotationIDEmpty         = errors.New("learning annotation: annotation_id must not be empty")
	ErrLearningAnnotationSessionEmpty    = errors.New("learning annotation: session_id must not be empty")
	ErrLearningAnnotationAgentEmpty      = errors.New("learning annotation: agent_slug must not be empty")
	ErrLearningAnnotationKindInvalid     = errors.New("learning annotation: kind not in closed set")
	ErrLearningAnnotationAudienceInvalid = errors.New("learning annotation: audience not in closed set")
	ErrLearningAnnotationTitleEmpty      = errors.New("learning annotation: title must not be empty")
	ErrLearningAnnotationContentEmpty    = errors.New("learning annotation: content must not be empty")
	ErrLearningAnnotationJournalFull     = errors.New("learning annotation: journal at capacity — oldest entry dropped")
	ErrLearningAnnotationSessionMismatch = errors.New("learning annotation: session_id mismatch")
)

// Validate returns the first structural violation found, or nil.
func (a *LearningAnnotation) Validate() error {
	if a == nil {
		return ErrLearningAnnotationNil
	}
	if a.AnnotationID == "" {
		return ErrLearningAnnotationIDEmpty
	}
	if a.SessionID == "" {
		return ErrLearningAnnotationSessionEmpty
	}
	if a.AgentSlug == "" {
		return ErrLearningAnnotationAgentEmpty
	}
	if !a.Kind.IsValid() {
		return ErrLearningAnnotationKindInvalid
	}
	if !a.Audience.IsValid() {
		return ErrLearningAnnotationAudienceInvalid
	}
	if a.Title == "" {
		return ErrLearningAnnotationTitleEmpty
	}
	if a.Content == "" {
		return ErrLearningAnnotationContentEmpty
	}
	return nil
}

// LearningAnnotationJournalMaxSize is the maximum entries held before oldest is dropped.
const LearningAnnotationJournalMaxSize = 50

// LearningAnnotationJournal accumulates teachable moments for one agent session.
// It is a capped ring-buffer: when full the oldest entry is dropped on Add.
// All methods are safe for concurrent use.
type LearningAnnotationJournal struct {
	mu        sync.RWMutex
	sessionID string
	entries   []*LearningAnnotation
}

// NewLearningAnnotationJournal creates a journal bound to sessionID.
// Returns ErrLearningAnnotationSessionEmpty when sessionID is blank.
func NewLearningAnnotationJournal(sessionID string) (*LearningAnnotationJournal, error) {
	if sessionID == "" {
		return nil, ErrLearningAnnotationSessionEmpty
	}
	return &LearningAnnotationJournal{sessionID: sessionID}, nil
}

// SessionID returns the session this journal is bound to.
func (j *LearningAnnotationJournal) SessionID() string { return j.sessionID }

// Add validates and appends a. When the journal is at capacity the oldest
// entry is silently dropped (ring-buffer semantic) before appending.
// Returns ErrLearningAnnotationSessionMismatch when a.SessionID != j.sessionID.
func (j *LearningAnnotationJournal) Add(a *LearningAnnotation) error {
	if a == nil {
		return ErrLearningAnnotationNil
	}
	if err := a.Validate(); err != nil {
		return err
	}
	if a.SessionID != j.sessionID {
		return fmt.Errorf("%w: got %q want %q",
			ErrLearningAnnotationSessionMismatch, a.SessionID, j.sessionID)
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if len(j.entries) >= LearningAnnotationJournalMaxSize {
		j.entries = j.entries[1:]
	}
	cp := *a
	j.entries = append(j.entries, &cp)
	return nil
}

// ListAll returns a defensive copy of all accumulated annotations in
// insertion order.
func (j *LearningAnnotationJournal) ListAll() []*LearningAnnotation {
	j.mu.RLock()
	defer j.mu.RUnlock()
	out := make([]*LearningAnnotation, len(j.entries))
	copy(out, j.entries)
	return out
}

// ListByKind returns all annotations whose Kind equals kind, in insertion order.
func (j *LearningAnnotationJournal) ListByKind(kind LearningAnnotationKind) []*LearningAnnotation {
	j.mu.RLock()
	defer j.mu.RUnlock()
	var out []*LearningAnnotation
	for _, e := range j.entries {
		if e.Kind == kind {
			out = append(out, e)
		}
	}
	return out
}

// FilterForAudience returns annotations whose Audience level is ≤ the requested
// level, implementing progressive disclosure:
//   - beginner    → only beginner annotations
//   - intermediate → beginner + intermediate
//   - advanced     → all annotations
func (j *LearningAnnotationJournal) FilterForAudience(audience LearningAnnotationAudience) []*LearningAnnotation {
	cap := learningAudienceLevel(audience)
	j.mu.RLock()
	defer j.mu.RUnlock()
	var out []*LearningAnnotation
	for _, e := range j.entries {
		if learningAudienceLevel(e.Audience) <= cap {
			out = append(out, e)
		}
	}
	return out
}

// Size returns the current number of annotations in the journal.
func (j *LearningAnnotationJournal) Size() int {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return len(j.entries)
}

// Clear removes all annotations from the journal.
func (j *LearningAnnotationJournal) Clear() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.entries = j.entries[:0]
}
