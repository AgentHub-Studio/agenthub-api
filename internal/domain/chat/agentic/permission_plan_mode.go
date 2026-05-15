package agentic

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// PERM-003a — Plan mode permission evaluation.
//
// PDF arXiv:2604.14228v1 §4 (Permissions and Safety) — Plan mode is a
// distinguished posture where the LLM proposes mutating actions but
// the harness records them as PLANNED rather than executing. The user
// then approves or rejects the plan as a batch. This separates
// REASONING (what the agent would do) from EXECUTION (what actually
// happens), enabling human-in-the-loop approval for high-stakes work.
//
// Distinct from existing AgentHub plumbing:
//   - permission.go PermissionMode (default/allow_edits/bypass/dont_ask) =
//     4 existing modes; this file adds the 5th VALUE plus its evaluator.
//   - Plan mode evaluation is a WRAPPER around EvaluatePermission: the
//     base engine still classifies, but Plan mode intercepts the
//     final decision and converts Allow → PlannedRecord for mutating
//     tools while passing read-only through.
//   - PERM-010 PermissionAuditEntry = post-execution audit. PlanRecord =
//     pre-execution intent (different lifecycle).

// PermissionModePlan is the 5th PermissionMode value. Added here (not
// in permission.go) to keep the additive scope contained. Callers
// detect it via the same PermissionRules.Mode field.
const PermissionModePlan PermissionMode = "plan"

// PlanModeDecision is the per-call outcome under plan mode.
type PlanModeDecision string

const (
	// PlanModeAllowReadOnly — tool is read-only; allowed to execute now.
	PlanModeAllowReadOnly PlanModeDecision = "allow_read_only"
	// PlanModePlanned — tool would mutate; recorded as planned, NOT executed.
	PlanModePlanned PlanModeDecision = "planned"
	// PlanModeDeniedByRule — base engine deny rule still applies even
	// in plan mode (deny is sticky).
	PlanModeDeniedByRule PlanModeDecision = "denied_by_rule"
)

var allPlanModeDecisions = []PlanModeDecision{
	PlanModeAllowReadOnly, PlanModePlanned, PlanModeDeniedByRule,
}

// IsValidPlanModeDecision returns true for the bounded set.
func IsValidPlanModeDecision(d PlanModeDecision) bool {
	for _, v := range allPlanModeDecisions {
		if d == v {
			return true
		}
	}
	return false
}

// PlanRecord is one planned mutating tool call captured during plan
// mode. The user reviews these as a batch before any execution.
type PlanRecord struct {
	ToolName    string
	ToolInput   string
	RecordedAt  time.Time
	Sequence    int // 1-based position in the plan
	Rationale   string // optional explanation captured at record time
}

// Validate enforces invariants.
func (r PlanRecord) Validate() error {
	if strings.TrimSpace(r.ToolName) == "" {
		return ErrPlanRecordEmptyTool
	}
	if r.Sequence < 1 {
		return ErrPlanRecordBadSequence
	}
	return nil
}

// PlanSession accumulates records during a plan-mode run. Thread-safe.
type PlanSession struct {
	mu       sync.Mutex
	records  []PlanRecord
	approved bool
	sealed   bool
	now      func() time.Time
}

// NewPlanSession builds an empty session.
func NewPlanSession() *PlanSession {
	return &PlanSession{now: time.Now}
}

// SetClock injects a clock for deterministic tests.
func (s *PlanSession) SetClock(fn func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.now = fn
}

// AddRecord appends a planned action. Returns the assigned sequence
// number. Errors if the session is sealed.
func (s *PlanSession) AddRecord(toolName, toolInput, rationale string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sealed {
		return 0, ErrPlanSessionSealed
	}
	seq := len(s.records) + 1
	r := PlanRecord{
		ToolName:   toolName,
		ToolInput:  toolInput,
		RecordedAt: s.now(),
		Sequence:   seq,
		Rationale:  rationale,
	}
	if err := r.Validate(); err != nil {
		return 0, err
	}
	s.records = append(s.records, r)
	return seq, nil
}

// Records returns a defensive copy of accumulated records.
func (s *PlanSession) Records() []PlanRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]PlanRecord(nil), s.records...)
}

// Size returns the count of recorded actions.
func (s *PlanSession) Size() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.records)
}

// Approve seals the session as approved. Idempotent within the same
// state. Errors if already rejected.
func (s *PlanSession) Approve() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sealed && !s.approved {
		return ErrPlanSessionAlreadyRejected
	}
	s.sealed = true
	s.approved = true
	return nil
}

// Reject seals the session as rejected. Idempotent. Errors if already
// approved.
func (s *PlanSession) Reject() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sealed && s.approved {
		return ErrPlanSessionAlreadyApproved
	}
	s.sealed = true
	s.approved = false
	return nil
}

// IsApproved returns true when the session was sealed via Approve.
func (s *PlanSession) IsApproved() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sealed && s.approved
}

// IsSealed returns true when the session is in a final state.
func (s *PlanSession) IsSealed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sealed
}

// Render produces a human-readable plan summary suitable for an
// approval UI prompt.
func (s *PlanSession) Render() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.records) == 0 {
		return "[plan empty — no mutating actions proposed]"
	}
	lines := []string{
		fmt.Sprintf("[plan with %d action(s)]", len(s.records)),
	}
	for _, r := range s.records {
		line := fmt.Sprintf("%d. %s", r.Sequence, r.ToolName)
		if r.ToolInput != "" {
			line += fmt.Sprintf(" — %s", r.ToolInput)
		}
		if r.Rationale != "" {
			line += fmt.Sprintf(" (%s)", r.Rationale)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// PlanModeReadOnlyClassifier decides whether a tool is safe to execute
// during plan mode. Returns true when the tool has no side effects.
type PlanModeReadOnlyClassifier func(toolName, toolInput string) bool

// DefaultPlanModeReadOnlyClassifier classifies via TOOL-003's existing
// ClassifyTool helper. Read-only tools execute; everything else is
// captured as planned.
func DefaultPlanModeReadOnlyClassifier(toolName, toolInput string) bool {
	return ClassifyTool(toolName, toolInput, nil) == ToolEffectReadOnly
}

// EvaluatePermissionInPlanMode wraps EvaluatePermission with plan mode
// semantics. The base engine runs first; deny rules still bite. If
// the base says Allow/Confirm, we then classify: read-only → execute;
// mutating → record in the plan session and return PlanModePlanned.
func EvaluatePermissionInPlanMode(
	rules *PermissionRules,
	classifier PlanModeReadOnlyClassifier,
	session *PlanSession,
	toolName, toolInput, rationale string,
) (PlanModeDecision, error) {
	if session == nil {
		return "", ErrPlanSessionNil
	}
	if classifier == nil {
		classifier = DefaultPlanModeReadOnlyClassifier
	}
	base := EvaluatePermission(rules, toolName, toolInput)
	if base == PermissionDeny {
		return PlanModeDeniedByRule, nil
	}
	if classifier(toolName, toolInput) {
		return PlanModeAllowReadOnly, nil
	}
	if _, err := session.AddRecord(toolName, toolInput, rationale); err != nil {
		return "", err
	}
	return PlanModePlanned, nil
}

// SortedRecords returns the session records sorted by sequence (which
// is already insertion order). Useful for audit emission that wants a
// reproducible buffer.
func SortedRecords(records []PlanRecord) []PlanRecord {
	out := append([]PlanRecord(nil), records...)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Sequence < out[j].Sequence
	})
	return out
}

// Sentinel errors.
var (
	ErrPlanRecordEmptyTool       = errors.New("plan record: tool name required")
	ErrPlanRecordBadSequence     = errors.New("plan record: sequence must be >= 1")
	ErrPlanSessionSealed         = errors.New("plan session: cannot add to sealed session")
	ErrPlanSessionAlreadyApproved = errors.New("plan session: already approved, cannot reject")
	ErrPlanSessionAlreadyRejected = errors.New("plan session: already rejected, cannot approve")
	ErrPlanSessionNil            = errors.New("plan session: nil session not allowed")
)
