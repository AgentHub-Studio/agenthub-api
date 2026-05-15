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

// PERM-005 — Permission hooks.
//
// PDF arXiv:2604.14228v1 §4 (Permissions and Safety) — beyond static
// allow/deny/confirm rules, the harness can call user-defined hooks
// during permission evaluation. Hooks can:
//   - run BEFORE the engine and short-circuit (e.g., "always allow when
//     called from on-call rotation user", "always deny outside of
//     business hours");
//   - run AFTER the engine and override its decision (e.g., "engine
//     says Allow but this request includes a flagged customer ID →
//     escalate to Confirm").
//
// Distinct from existing AgentHub plumbing:
//   - permission.go EvaluatePermission = pure rule-engine decision.
//   - permission_prefilter.go PERM-004 = pool-time screening that
//     adapts the engine via filter contract.
//   - permission_hook.go (this file) = call-time hook chain that wraps
//     the engine. Hooks see the request + the engine decision and can
//     veto/escalate/promote. Result is the final decision actually
//     enforced by the call-time gate.
//   - hookregistry / hook_schema_registry = generic event hooks that
//     emit PermissionHookRequest/PermissionDenied notifications. PERM-005
//     hooks ARE part of the permission decision; event hooks observe it.

// PermissionHookPhase bounded enum tells the chain when to invoke a hook.
type PermissionHookPhase string

const (
	// PermissionHookBeforeEvaluate runs before the rule engine; can
	// short-circuit. If the hook returns OverrideXxx, the engine is
	// skipped for this evaluation.
	PermissionHookBeforeEvaluate PermissionHookPhase = "before_evaluate"
	// PermissionHookAfterEvaluate runs after the engine; receives the
	// engine's decision and can override it.
	PermissionHookAfterEvaluate PermissionHookPhase = "after_evaluate"
)

var allPermissionHookPhases = []PermissionHookPhase{
	PermissionHookBeforeEvaluate, PermissionHookAfterEvaluate,
}

// IsValidPermissionHookPhase returns true for the bounded set.
func IsValidPermissionHookPhase(p PermissionHookPhase) bool {
	for _, v := range allPermissionHookPhases {
		if p == v {
			return true
		}
	}
	return false
}

// PermissionHookOutcome is what a hook returns to the chain.
type PermissionHookOutcome string

const (
	// PermissionHookContinue — leave the decision as the chain sees it
	// (engine decision in After phase, "no decision yet" in Before).
	PermissionHookContinue PermissionHookOutcome = "continue"
	// PermissionHookOverrideAllow forces the final decision to Allow.
	PermissionHookOverrideAllow PermissionHookOutcome = "override_allow"
	// PermissionHookOverrideDeny forces the final decision to Deny.
	PermissionHookOverrideDeny PermissionHookOutcome = "override_deny"
	// PermissionHookOverrideConfirm escalates to Confirm. Used by After
	// hooks to gate an Allow decision behind human confirmation.
	PermissionHookOverrideConfirm PermissionHookOutcome = "override_confirm"
)

var allPermissionHookOutcomes = []PermissionHookOutcome{
	PermissionHookContinue, PermissionHookOverrideAllow,
	PermissionHookOverrideDeny, PermissionHookOverrideConfirm,
}

// IsValidPermissionHookOutcome returns true for the bounded set.
func IsValidPermissionHookOutcome(o PermissionHookOutcome) bool {
	for _, v := range allPermissionHookOutcomes {
		if o == v {
			return true
		}
	}
	return false
}

// outcomeToDecision maps an override outcome to a PermissionDecision.
// Returns ("", false) for Continue.
func outcomeToDecision(o PermissionHookOutcome) (PermissionDecision, bool) {
	switch o {
	case PermissionHookOverrideAllow:
		return PermissionAllow, true
	case PermissionHookOverrideDeny:
		return PermissionDeny, true
	case PermissionHookOverrideConfirm:
		return PermissionConfirm, true
	}
	return "", false
}

// PermissionHookRequest is the input observed by hooks. Mirrors the inputs
// of EvaluatePermission plus session-scoped context.
type PermissionHookRequest struct {
	TenantID  string
	SessionID string
	UserID    string
	ToolName  string
	ToolInput string
	At        time.Time
}

// PermissionHookHandle is the function signature implemented by hooks.
// Receives the request and (for After phase) the engine's current
// decision. Returns an outcome + optional reason for audit.
type PermissionHookHandle func(ctx context.Context, req PermissionHookRequest, engine PermissionDecision) (PermissionHookOutcome, string)

// PermissionHook is a single registered hook.
type PermissionHook struct {
	Name     string
	Phase    PermissionHookPhase
	Priority int // lower number = earlier; ties resolved by Name asc
	Enabled  bool
	Handle   PermissionHookHandle
}

// Validate enforces invariants.
func (h PermissionHook) Validate() error {
	if strings.TrimSpace(h.Name) == "" {
		return ErrPermissionHookNameRequired
	}
	if !IsValidPermissionHookPhase(h.Phase) {
		return fmt.Errorf("%w: %q", ErrPermissionHookBadPhase, h.Phase)
	}
	if h.Handle == nil {
		return ErrPermissionHookHandleRequired
	}
	return nil
}

// PermissionHookDecision is one audit record for a hook invocation.
type PermissionHookDecision struct {
	HookName string
	Phase    PermissionHookPhase
	Outcome  PermissionHookOutcome
	Reason   string
	At       time.Time
}

// PermissionEvaluation is the chain's full audit output for one
// permission decision (one tool call).
type PermissionEvaluation struct {
	Request        PermissionHookRequest
	EngineDecision PermissionDecision // "" if Before-hook short-circuited
	FinalDecision  PermissionDecision
	HookDecisions  []PermissionHookDecision
	ShortCircuited bool
	EvaluatedAt    time.Time
}

// Sentinel errors.
var (
	ErrPermissionHookNameRequired   = errors.New("permission hook: name required")
	ErrPermissionHookBadPhase       = errors.New("permission hook: invalid phase")
	ErrPermissionHookHandleRequired = errors.New("permission hook: handle required")
	ErrPermissionHookDuplicate      = errors.New("permission hook: duplicate name in same phase")
	ErrPermissionHookNotFound       = errors.New("permission hook: not found")
	ErrPermissionHookBadOutcome     = errors.New("permission hook: invalid outcome")
)

// PermissionHookChain wraps EvaluatePermission with before/after hooks.
// Concurrent-safe.
type PermissionHookChain struct {
	mu     sync.RWMutex
	hooks  []PermissionHook
	rules  *PermissionRules
	now    func() time.Time
}

// NewPermissionHookChain builds an empty chain.
func NewPermissionHookChain(rules *PermissionRules) *PermissionHookChain {
	return &PermissionHookChain{rules: rules, now: time.Now}
}

// SetClock injects a clock for deterministic tests.
func (c *PermissionHookChain) SetClock(fn func() time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = fn
}

// Register adds a hook. Returns error if a hook with the same name +
// phase already exists.
func (c *PermissionHookChain) Register(h PermissionHook) error {
	if err := h.Validate(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, existing := range c.hooks {
		if existing.Name == h.Name && existing.Phase == h.Phase {
			return fmt.Errorf("%w: %q in %q", ErrPermissionHookDuplicate, h.Name, h.Phase)
		}
	}
	c.hooks = append(c.hooks, h)
	return nil
}

// Remove deletes a hook by name + phase.
func (c *PermissionHookChain) Remove(name string, phase PermissionHookPhase) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, h := range c.hooks {
		if h.Name == name && h.Phase == phase {
			c.hooks = append(c.hooks[:i], c.hooks[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("%w: %q in %q", ErrPermissionHookNotFound, name, phase)
}

// Disable marks a hook disabled without removing it. Idempotent.
func (c *PermissionHookChain) Disable(name string, phase PermissionHookPhase) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.hooks {
		if c.hooks[i].Name == name && c.hooks[i].Phase == phase {
			c.hooks[i].Enabled = false
			return
		}
	}
}

// Enable marks a hook enabled. Idempotent.
func (c *PermissionHookChain) Enable(name string, phase PermissionHookPhase) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.hooks {
		if c.hooks[i].Name == name && c.hooks[i].Phase == phase {
			c.hooks[i].Enabled = true
			return
		}
	}
}

// hooksFor returns enabled hooks for a phase, sorted by priority then name.
func (c *PermissionHookChain) hooksFor(phase PermissionHookPhase) []PermissionHook {
	out := []PermissionHook{}
	for _, h := range c.hooks {
		if h.Enabled && h.Phase == phase {
			out = append(out, h)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority < out[j].Priority
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// Evaluate runs the chain: Before hooks → engine (if no short-circuit)
// → After hooks. Returns a complete PermissionEvaluation audit record.
func (c *PermissionHookChain) Evaluate(ctx context.Context, req PermissionHookRequest) PermissionEvaluation {
	c.mu.RLock()
	beforeHooks := c.hooksFor(PermissionHookBeforeEvaluate)
	afterHooks := c.hooksFor(PermissionHookAfterEvaluate)
	rules := c.rules
	now := c.now
	c.mu.RUnlock()

	eval := PermissionEvaluation{
		Request:     req,
		EvaluatedAt: now(),
	}

	// Before phase — first override short-circuits the engine.
	for _, h := range beforeHooks {
		outcome, reason := h.Handle(ctx, req, "")
		eval.HookDecisions = append(eval.HookDecisions, PermissionHookDecision{
			HookName: h.Name,
			Phase:    PermissionHookBeforeEvaluate,
			Outcome:  outcome,
			Reason:   reason,
			At:       now(),
		})
		if d, ok := outcomeToDecision(outcome); ok {
			eval.FinalDecision = d
			eval.ShortCircuited = true
			return eval
		}
	}

	// Engine evaluation.
	eval.EngineDecision = EvaluatePermission(rules, req.ToolName, req.ToolInput)
	eval.FinalDecision = eval.EngineDecision

	// After phase — override possibly mutates the final decision.
	for _, h := range afterHooks {
		outcome, reason := h.Handle(ctx, req, eval.FinalDecision)
		eval.HookDecisions = append(eval.HookDecisions, PermissionHookDecision{
			HookName: h.Name,
			Phase:    PermissionHookAfterEvaluate,
			Outcome:  outcome,
			Reason:   reason,
			At:       now(),
		})
		if d, ok := outcomeToDecision(outcome); ok {
			eval.FinalDecision = d
		}
	}

	return eval
}

// SetRules replaces the engine rules.
func (c *PermissionHookChain) SetRules(rules *PermissionRules) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rules = rules
}

// HookCount returns the number of registered hooks (enabled + disabled).
func (c *PermissionHookChain) HookCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.hooks)
}
