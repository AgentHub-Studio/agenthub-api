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

// PERM-003b — Bubble mode (subagent confirm escalation).
//
// PDF arXiv:2604.14228v1 §4 (Permissions and Safety) — when a subagent
// needs confirmation for a tool call, the natural surface to ask is
// often NOT the human user (who may not even know the subagent
// exists) but the PARENT agent that spawned it. The parent decides:
// approve and forward to its own posture, deny locally, or bubble
// further to its own parent.
//
// Distinct from existing AgentHub plumbing:
//   - permission.go PermissionMode (default/allow_edits/bypass/dont_ask/plan)
//     = 5 existing modes. This file adds "bubble" as the 6th.
//   - SUB-006 PermissionInheritanceMode = how parent rules COMBINE with
//     child rules at spawn time. PERM-003b = how DECISIONS escalate
//     at call time when no local rule definitively allows.
//   - PERM-005 PermissionHookChain = lateral hooks. Bubble mode is the
//     VERTICAL escalation pipeline up the subagent tree.

// PermissionModeBubble is the 6th PermissionMode value.
const PermissionModeBubble PermissionMode = "bubble"

// BubbleOutcome bounded enum classifies how a bubble-mode decision
// was resolved.
type BubbleOutcome string

const (
	// BubbleOutcomeHandledLocally — local rule definitively allowed or
	// the subagent's own engine resolved without needing parent input.
	BubbleOutcomeHandledLocally BubbleOutcome = "handled_locally"
	// BubbleOutcomeDeniedLocally — local deny rule fired; never bubbles.
	BubbleOutcomeDeniedLocally BubbleOutcome = "denied_locally"
	// BubbleOutcomeBubbledToParent — confirm or unhandled allow
	// bubbled up to the parent agent.
	BubbleOutcomeBubbledToParent BubbleOutcome = "bubbled_to_parent"
	// BubbleOutcomeMaxDepthReached — subagent past max bubble depth;
	// auto-denied (defensive: avoid runaway escalation loops).
	BubbleOutcomeMaxDepthReached BubbleOutcome = "max_depth_reached"
)

var allBubbleOutcomes = []BubbleOutcome{
	BubbleOutcomeHandledLocally, BubbleOutcomeDeniedLocally,
	BubbleOutcomeBubbledToParent, BubbleOutcomeMaxDepthReached,
}

// IsValidBubbleOutcome returns true for the bounded set.
func IsValidBubbleOutcome(o BubbleOutcome) bool {
	for _, v := range allBubbleOutcomes {
		if o == v {
			return true
		}
	}
	return false
}

// BubbleRequest is the input observed by the bubble resolver.
type BubbleRequest struct {
	SubagentID   string
	ParentID     string
	Depth        int // 0 = parent agent, 1+ = subagent
	ToolName     string
	ToolInput    string
	BubbledAt    time.Time
}

// Validate enforces invariants.
func (r BubbleRequest) Validate() error {
	if strings.TrimSpace(r.SubagentID) == "" {
		return ErrBubbleRequestEmptySubagent
	}
	if strings.TrimSpace(r.ToolName) == "" {
		return ErrBubbleRequestEmptyTool
	}
	if r.Depth < 0 {
		return ErrBubbleRequestNegativeDepth
	}
	return nil
}

// BubbleResolution is the audit-shaped output of one bubble decision.
type BubbleResolution struct {
	Request        BubbleRequest
	Outcome        BubbleOutcome
	FinalDecision  PermissionDecision
	LocalDecision  PermissionDecision // engine's first-pass before bubble
	ResolvedBy     string             // "local" / parent id / "max_depth"
	Reason         string
	BubbledThrough []string // chain of parent ids the request traveled
}

// ParentResolver abstracts the parent agent's permission response.
// Implementations are responsible for forwarding the request, applying
// their own posture, and returning a final decision.
type ParentResolver interface {
	Resolve(ctx context.Context, req BubbleRequest) (PermissionDecision, error)
}

// Sentinel errors.
var (
	ErrBubbleRequestEmptySubagent  = errors.New("bubble request: subagent id required")
	ErrBubbleRequestEmptyTool      = errors.New("bubble request: tool name required")
	ErrBubbleRequestNegativeDepth  = errors.New("bubble request: depth must be >= 0")
	ErrBubbleResolverNil           = errors.New("bubble resolver: parent resolver required when bubbling")
	ErrBubbleNegativeMaxDepth      = errors.New("bubble resolver: max depth must be >= 0")
)

// BubbleModeEvaluator dispatches permission decisions in bubble mode:
// local engine first; deny rules always fire locally; confirm or
// unresolved allow bubbles up to the parent.
type BubbleModeEvaluator struct {
	mu         sync.RWMutex
	rules      *PermissionRules
	parent     ParentResolver
	maxDepth   int
	now        func() time.Time
}

// NewBubbleModeEvaluator constructs an evaluator.
//
//   - rules: local PermissionRules (may be nil).
//   - parent: ParentResolver used when a request bubbles. Can be nil
//     ONLY if maxDepth is 0 (no bubbling allowed; every confirm is
//     auto-handled locally as max_depth_reached).
//   - maxDepth: how many parent hops are allowed. 0 = no bubbling.
func NewBubbleModeEvaluator(rules *PermissionRules, parent ParentResolver, maxDepth int) (*BubbleModeEvaluator, error) {
	if maxDepth < 0 {
		return nil, ErrBubbleNegativeMaxDepth
	}
	if maxDepth > 0 && parent == nil {
		return nil, ErrBubbleResolverNil
	}
	return &BubbleModeEvaluator{
		rules:    rules,
		parent:   parent,
		maxDepth: maxDepth,
		now:      time.Now,
	}, nil
}

// SetClock injects a clock for deterministic tests.
func (e *BubbleModeEvaluator) SetClock(fn func() time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.now = fn
}

// Evaluate runs bubble-mode evaluation for one request. Pure-ish: it
// calls into ParentResolver but never mutates its own state aside
// from clock.
func (e *BubbleModeEvaluator) Evaluate(ctx context.Context, req BubbleRequest) (BubbleResolution, error) {
	if err := req.Validate(); err != nil {
		return BubbleResolution{}, err
	}
	e.mu.RLock()
	rules := e.rules
	parent := e.parent
	maxDepth := e.maxDepth
	now := e.now
	e.mu.RUnlock()

	if req.BubbledAt.IsZero() {
		req.BubbledAt = now()
	}

	res := BubbleResolution{Request: req}
	local := EvaluatePermission(rules, req.ToolName, req.ToolInput)
	res.LocalDecision = local

	switch local {
	case PermissionDeny:
		res.Outcome = BubbleOutcomeDeniedLocally
		res.FinalDecision = PermissionDeny
		res.ResolvedBy = "local"
		res.Reason = "local deny rule matched"
		return res, nil

	case PermissionAllow:
		// Definitively allowed locally — no need to bubble.
		res.Outcome = BubbleOutcomeHandledLocally
		res.FinalDecision = PermissionAllow
		res.ResolvedBy = "local"
		res.Reason = "local allow rule matched (no bubble needed)"
		return res, nil

	case PermissionConfirm:
		// This is the bubble case. Check depth.
		if req.Depth >= maxDepth {
			res.Outcome = BubbleOutcomeMaxDepthReached
			res.FinalDecision = PermissionDeny
			res.ResolvedBy = "max_depth"
			res.Reason = fmt.Sprintf("depth %d >= max %d: auto-deny to prevent escalation loop", req.Depth, maxDepth)
			return res, nil
		}
		if parent == nil {
			// Should be unreachable given constructor validation, but
			// defensive.
			res.Outcome = BubbleOutcomeMaxDepthReached
			res.FinalDecision = PermissionDeny
			res.ResolvedBy = "no_parent"
			res.Reason = "no parent resolver configured: auto-deny"
			return res, nil
		}
		parentDecision, err := parent.Resolve(ctx, req)
		if err != nil {
			return BubbleResolution{}, fmt.Errorf("bubble: parent resolver failed: %w", err)
		}
		res.Outcome = BubbleOutcomeBubbledToParent
		res.FinalDecision = parentDecision
		res.ResolvedBy = req.ParentID
		res.Reason = fmt.Sprintf("bubbled to parent %s at depth %d", req.ParentID, req.Depth)
		res.BubbledThrough = []string{req.ParentID}
		return res, nil
	}

	// Engine returned an unbounded value (should not happen with the
	// fixed enum). Defensive: deny.
	res.Outcome = BubbleOutcomeDeniedLocally
	res.FinalDecision = PermissionDeny
	res.ResolvedBy = "unknown"
	res.Reason = fmt.Sprintf("unexpected local decision %q: defensive deny", local)
	return res, nil
}

// StaticParentResolver is a test/dev parent that always returns the
// same decision regardless of input.
type StaticParentResolver struct {
	Decision PermissionDecision
	Err      error
}

// Resolve satisfies ParentResolver.
func (p StaticParentResolver) Resolve(_ context.Context, _ BubbleRequest) (PermissionDecision, error) {
	if p.Err != nil {
		return "", p.Err
	}
	return p.Decision, nil
}

// ChainedParentResolver records the chain of bubbles for audit. Each
// call appends the request to History before delegating to Inner.
type ChainedParentResolver struct {
	mu      sync.Mutex
	Inner   ParentResolver
	History []BubbleRequest
}

// Resolve satisfies ParentResolver while recording history.
func (p *ChainedParentResolver) Resolve(ctx context.Context, req BubbleRequest) (PermissionDecision, error) {
	p.mu.Lock()
	p.History = append(p.History, req)
	p.mu.Unlock()
	if p.Inner == nil {
		return PermissionDeny, nil
	}
	return p.Inner.Resolve(ctx, req)
}

// HistorySnapshot returns a defensive copy of the recorded chain.
func (p *ChainedParentResolver) HistorySnapshot() []BubbleRequest {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]BubbleRequest(nil), p.History...)
}

// SortedBubbleResolutions sorts a slice deterministically by
// (Subagent, Tool, BubbledAt). Useful for audit emission.
func SortedBubbleResolutions(in []BubbleResolution) []BubbleResolution {
	out := append([]BubbleResolution(nil), in...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Request.SubagentID != out[j].Request.SubagentID {
			return out[i].Request.SubagentID < out[j].Request.SubagentID
		}
		if out[i].Request.ToolName != out[j].Request.ToolName {
			return out[i].Request.ToolName < out[j].Request.ToolName
		}
		return out[i].Request.BubbledAt.Before(out[j].Request.BubbledAt)
	})
	return out
}
