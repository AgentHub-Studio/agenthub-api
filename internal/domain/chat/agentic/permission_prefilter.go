package agentic

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// PERM-004 — Pre-filtering of denied tools.
//
// PDF arXiv:2604.14228v1 §4 (Permissions and Safety) — denied tools
// should not even be advertised to the LLM. Letting the model attempt
// a denied call and then rejecting at call time wastes tokens, leaks
// information about the deny set ("this tenant can't run Bash, so it
// must be in safe mode"), and burns prompt budget on hallucinated
// arguments. The pre-filter intercepts during pool assembly (TOOL-003)
// and drops Deny outcomes upfront. Confirm/Allow outcomes pass through:
// the LLM still sees them; the call-time check remains the gate.
//
// Distinct from existing AgentHub plumbing:
//   - permission.go EvaluatePermission = CALL-TIME decision for one
//     toolName + toolInput at execution.
//   - permission_prefilter.go (this file) = POOL-TIME decision applied
//     across every ToolPoolEntry the assembler is about to expose.
//   - PermissionPreFilter is a `ToolPoolFilter` adapter — feeds
//     ToolPoolAssembler.AddFilter() so the pool snapshot is already
//     screened. DroppedNames audit trail surfaces what was filtered.

// PrefilterStance bounded enum controls how Confirm-tier tools are
// treated by the pre-filter.
type PrefilterStance string

const (
	// PrefilterStanceShowConfirm — Confirm decisions remain visible to
	// the LLM; call-time prompt asks the user. Default for interactive
	// sessions.
	PrefilterStanceShowConfirm PrefilterStance = "show_confirm"
	// PrefilterStanceHideConfirm — Confirm decisions are dropped from
	// the pool. Used for unattended/batch runs where there is no human
	// to confirm.
	PrefilterStanceHideConfirm PrefilterStance = "hide_confirm"
)

var allPrefilterStances = []PrefilterStance{
	PrefilterStanceShowConfirm, PrefilterStanceHideConfirm,
}

// IsValidPrefilterStance returns true for the bounded set.
func IsValidPrefilterStance(s PrefilterStance) bool {
	for _, v := range allPrefilterStances {
		if s == v {
			return true
		}
	}
	return false
}

// PrefilterConfig wires the rules + stance + a sample input function.
//
// SampleInputFor lets the caller produce a representative input for a
// tool when the pool-time evaluation needs to test against patterns
// like `execute-sql(DROP TABLE)`. If nil, the empty string is used —
// rules of the form `tool(*)` still apply (match-all), but specific
// argument patterns only match when caller supplies a probe.
type PrefilterConfig struct {
	Rules          *PermissionRules
	Stance         PrefilterStance
	SampleInputFor func(entry ToolPoolEntry) string
}

// Validate ensures the config is internally consistent.
func (c PrefilterConfig) Validate() error {
	if c.Rules == nil {
		return ErrPrefilterRulesRequired
	}
	if c.Stance != "" && !IsValidPrefilterStance(c.Stance) {
		return fmt.Errorf("%w: %q", ErrPrefilterBadStance, c.Stance)
	}
	return nil
}

// PrefilterEntryDecision is the per-entry outcome.
type PrefilterEntryDecision struct {
	Entry    ToolPoolEntry
	Decision PermissionDecision
	Kept     bool
	At       time.Time
}

// PrefilterAudit accumulates decisions in order for OBS-003 alignment.
// Implementations must be concurrent-safe (the assembler fan-outs).
type PrefilterAudit interface {
	Record(d PrefilterEntryDecision)
	Snapshot() []PrefilterEntryDecision
}

// InMemoryPrefilterAudit is the default audit sink.
type InMemoryPrefilterAudit struct {
	mu  sync.Mutex
	log []PrefilterEntryDecision
}

// NewInMemoryPrefilterAudit builds an empty audit.
func NewInMemoryPrefilterAudit() *InMemoryPrefilterAudit {
	return &InMemoryPrefilterAudit{}
}

// Record appends a decision.
func (a *InMemoryPrefilterAudit) Record(d PrefilterEntryDecision) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.log = append(a.log, d)
}

// Snapshot returns a stable copy of recorded decisions.
func (a *InMemoryPrefilterAudit) Snapshot() []PrefilterEntryDecision {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := append([]PrefilterEntryDecision(nil), a.log...)
	return out
}

// DroppedEntries returns only the decisions where Kept = false.
func (a *InMemoryPrefilterAudit) DroppedEntries() []PrefilterEntryDecision {
	all := a.Snapshot()
	out := make([]PrefilterEntryDecision, 0, len(all))
	for _, d := range all {
		if !d.Kept {
			out = append(out, d)
		}
	}
	return out
}

// SortedDecisions returns the snapshot sorted by tool name then
// decision time; useful for deterministic audit emission.
func (a *InMemoryPrefilterAudit) SortedDecisions() []PrefilterEntryDecision {
	out := a.Snapshot()
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Entry.Name != out[j].Entry.Name {
			return out[i].Entry.Name < out[j].Entry.Name
		}
		return out[i].At.Before(out[j].At)
	})
	return out
}

// Sentinel errors.
var (
	ErrPrefilterRulesRequired = errors.New("permission prefilter: rules required")
	ErrPrefilterBadStance     = errors.New("permission prefilter: invalid stance")
)

// PermissionPreFilter is the PERM-004 pool-time screening device.
type PermissionPreFilter struct {
	mu     sync.RWMutex
	cfg    PrefilterConfig
	audit  PrefilterAudit
	now    func() time.Time
}

// NewPermissionPreFilter builds a pre-filter. Validates config eagerly.
// The audit can be nil — decisions still happen but no record is kept.
func NewPermissionPreFilter(cfg PrefilterConfig, audit PrefilterAudit) (*PermissionPreFilter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg.Stance == "" {
		cfg.Stance = PrefilterStanceShowConfirm
	}
	return &PermissionPreFilter{
		cfg:   cfg,
		audit: audit,
		now:   time.Now,
	}, nil
}

// SetClock injects a clock for deterministic tests.
func (p *PermissionPreFilter) SetClock(c func() time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.now = c
}

// Decide returns the per-entry decision without recording it. Used by
// callers that want to inspect outcomes without applying the filter.
func (p *PermissionPreFilter) Decide(entry ToolPoolEntry) PermissionDecision {
	p.mu.RLock()
	defer p.mu.RUnlock()
	input := ""
	if p.cfg.SampleInputFor != nil {
		input = p.cfg.SampleInputFor(entry)
	}
	return EvaluatePermission(p.cfg.Rules, entry.Name, input)
}

// AsToolPoolFilter returns the TOOL-003 ToolPoolFilter adapter. The
// returned closure is safe for concurrent use because PermissionPreFilter
// guards its state under a read lock.
func (p *PermissionPreFilter) AsToolPoolFilter() ToolPoolFilter {
	return func(e ToolPoolEntry) bool {
		decision := p.Decide(e)
		p.mu.RLock()
		stance := p.cfg.Stance
		audit := p.audit
		now := p.now
		p.mu.RUnlock()
		kept := true
		switch decision {
		case PermissionDeny:
			kept = false
		case PermissionConfirm:
			if stance == PrefilterStanceHideConfirm {
				kept = false
			}
		}
		if audit != nil {
			audit.Record(PrefilterEntryDecision{
				Entry:    e,
				Decision: decision,
				Kept:     kept,
				At:       now(),
			})
		}
		return kept
	}
}

// ApplyAll runs the filter against a batch of entries and returns the
// kept slice. Convenience helper for tests or callers that want batch
// evaluation without going through the assembler.
func (p *PermissionPreFilter) ApplyAll(entries []ToolPoolEntry) []ToolPoolEntry {
	f := p.AsToolPoolFilter()
	out := make([]ToolPoolEntry, 0, len(entries))
	for _, e := range entries {
		if f(e) {
			out = append(out, e)
		}
	}
	return out
}
