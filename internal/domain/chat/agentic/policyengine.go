package agentic

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// GOV-002 — External policy integration.
//
// PDF arXiv:2604.14228v1 Section 11 (governance — enterprises need to
// plug in their own policy systems: OPA / Cedar / custom rules engines /
// regulatory compliance frameworks); Section 5.3 (permission decisions
// must be observable by the audit layer regardless of who decides).
//
// This file introduces the EXTERNAL policy abstraction:
//
//   - PolicyEngine: pluggable interface — runner queries it BEFORE
//     executing a tool call. Different deployments wire different
//     engines (OPA HTTP backend, Cedar policy bundle, in-process
//     rule set, no-op for dev, etc).
//
//   - PolicyEvaluationRequest: carries the full audit context so the
//     engine decides on (tenant + user + agent + tool + operation).
//
//   - PolicyEvaluationResult: bounded outcome + reason + obligations
//     (e.g. "allowed but require evidence-of-consent header").
//
//   - NoOpPolicyEngine: dev/test default, allows everything.
//   - LimitsBackedPolicyEngine: adapter wrapping the existing
//     PolicyLimits fetcher (already in the codebase).
//   - ChainedPolicyEngine: composes engines, FIRST DENY WINS — lets
//     a tenant layer extra restrictions over enterprise defaults.

// PolicyOutcome is the bounded decision an engine returns.
type PolicyOutcome string

const (
	// PolicyAllow — proceed with the operation.
	PolicyAllow PolicyOutcome = "allow"
	// PolicyDeny — block the operation outright.
	PolicyDeny PolicyOutcome = "deny"
	// PolicyRequireApproval — block until human / out-of-band approval
	// is recorded. Different from PermissionConfirm: that's a runtime
	// UI prompt, this is a policy-level requirement that may need a
	// signed approval token before retry.
	PolicyRequireApproval PolicyOutcome = "require_approval"
	// PolicyIndeterminate — engine could not decide (network down,
	// timeout, malformed response). Caller decides fail-open vs
	// fail-closed per deployment policy.
	PolicyIndeterminate PolicyOutcome = "indeterminate"
)

// IsTerminalPolicyOutcome returns true for any of the 4 valid outcomes.
// Used by tracing layer to validate envelope shape.
func IsTerminalPolicyOutcome(o PolicyOutcome) bool {
	switch o {
	case PolicyAllow, PolicyDeny, PolicyRequireApproval, PolicyIndeterminate:
		return true
	}
	return false
}

// PolicyEvaluationRequest is the input the engine sees. Carries enough
// context that ANY backend (OPA, Cedar, in-process) can decide.
type PolicyEvaluationRequest struct {
	// TenantID scopes the request. Always set.
	TenantID string
	// UserID identifies the human user, if any (subject of the request).
	UserID string
	// AgentID identifies the running agent.
	AgentID string
	// ToolName is the tool the agent intends to invoke.
	ToolName string
	// Operation is the specific action within the tool (e.g. for an
	// agenthub_manage tool: "create_agent", "delete_kb"). Empty when
	// the tool itself is the granularity.
	Operation string
	// ResourceID identifies the resource being acted on (e.g. agent ID,
	// document ID). Empty for tool-level decisions.
	ResourceID string
	// Attributes is a free-form map for extra context (HTTP method, cost
	// estimate, classification, etc.). Engines that need it inspect it.
	Attributes map[string]string
	// Timestamp is when the runner asked. Engines may use this to
	// implement time-of-day restrictions.
	Timestamp time.Time
}

// PolicyEvaluationResult is the output the engine returns.
type PolicyEvaluationResult struct {
	// Outcome is the bounded decision.
	Outcome PolicyOutcome
	// Reason is a short human-readable explanation. REQUIRED for Deny
	// and RequireApproval — operators / users need to know WHY.
	Reason string
	// Obligations are extra requirements the caller must satisfy on
	// allow (e.g. "require_evidence_header", "log_to_compliance_sink").
	// Stable strings — runner inspects them post-decision.
	Obligations []string
	// EngineName identifies which engine produced the decision (audit
	// trail). For ChainedPolicyEngine, this is the FIRST engine that
	// returned a terminal (allow/deny) outcome.
	EngineName string
	// EvaluatedAt is the wall-clock timestamp.
	EvaluatedAt time.Time
	// LatencyMs is the engine's own latency (separate from runner).
	LatencyMs int64
}

// PolicyEngine is the pluggable external policy interface.
//
// Implementations MUST:
//   - Honor ctx cancellation (callers may set short timeouts).
//   - NOT mutate the request.
//   - Set Outcome, Reason (for Deny / RequireApproval), EngineName,
//     EvaluatedAt, and LatencyMs on every return.
//   - Return Outcome == PolicyIndeterminate (NOT an error) when they
//     cannot decide — preserves the caller's fail-open/fail-closed
//     choice. Errors are reserved for caller bugs (nil request).
type PolicyEngine interface {
	// Name returns a stable identifier for this engine instance.
	// Used in audit / tracing / chained decision attribution.
	Name() string

	// Evaluate produces a decision for the given request.
	Evaluate(ctx context.Context, req PolicyEvaluationRequest) (PolicyEvaluationResult, error)
}

// ErrNilPolicyRequest is returned when callers pass a zero-value
// request that lacks TenantID — defensive caller-bug guard.
var ErrNilPolicyRequest = errors.New("policyengine: request missing TenantID")

// --- NoOpPolicyEngine ---

// NoOpPolicyEngine allows everything. Default for dev / tests / installs
// where no external policy backend is configured.
type NoOpPolicyEngine struct{}

// NewNoOpPolicyEngine creates a NoOpPolicyEngine.
func NewNoOpPolicyEngine() *NoOpPolicyEngine { return &NoOpPolicyEngine{} }

// Name returns "noop".
func (e *NoOpPolicyEngine) Name() string { return "noop" }

// Evaluate always returns PolicyAllow.
func (e *NoOpPolicyEngine) Evaluate(ctx context.Context, req PolicyEvaluationRequest) (PolicyEvaluationResult, error) {
	if err := ctx.Err(); err != nil {
		return PolicyEvaluationResult{}, fmt.Errorf("policyengine: ctx cancelled: %w", err)
	}
	if req.TenantID == "" {
		return PolicyEvaluationResult{}, ErrNilPolicyRequest
	}
	start := time.Now()
	return PolicyEvaluationResult{
		Outcome:     PolicyAllow,
		EngineName:  e.Name(),
		EvaluatedAt: time.Now(),
		LatencyMs:   time.Since(start).Milliseconds(),
	}, nil
}

// --- LimitsBackedPolicyEngine ---

// PolicyLimitsProvider is the minimal interface LimitsBackedPolicyEngine
// needs from a fetcher. Existing PolicyLimits in policylimits.go satisfies
// it — this seam keeps the engine independent of HTTP fetching.
type PolicyLimitsProvider interface {
	// CurrentLimits returns the most recently fetched limits, or nil
	// when no limits have been fetched yet.
	CurrentLimits() *PolicyLimits
}

// LimitsBackedPolicyEngine adapts a PolicyLimitsProvider into a
// PolicyEngine. The mapping: if Limits.Restrictions[ToolName] exists
// AND Allowed=false → PolicyDeny. Otherwise → PolicyAllow.
type LimitsBackedPolicyEngine struct {
	provider PolicyLimitsProvider
}

// NewLimitsBackedPolicyEngine creates the adapter.
func NewLimitsBackedPolicyEngine(provider PolicyLimitsProvider) *LimitsBackedPolicyEngine {
	return &LimitsBackedPolicyEngine{provider: provider}
}

// Name returns "policy-limits".
func (e *LimitsBackedPolicyEngine) Name() string { return "policy-limits" }

// Evaluate maps PolicyLimits to a PolicyDecision.
func (e *LimitsBackedPolicyEngine) Evaluate(ctx context.Context, req PolicyEvaluationRequest) (PolicyEvaluationResult, error) {
	if err := ctx.Err(); err != nil {
		return PolicyEvaluationResult{}, fmt.Errorf("policyengine: ctx cancelled: %w", err)
	}
	if req.TenantID == "" {
		return PolicyEvaluationResult{}, ErrNilPolicyRequest
	}
	start := time.Now()
	limits := e.provider.CurrentLimits()
	// No limits fetched yet → indeterminate. Caller decides fail-open
	// vs fail-closed (e.g. ChainedPolicyEngine treats indeterminate
	// as "let next engine decide").
	if limits == nil {
		return PolicyEvaluationResult{
			Outcome:     PolicyIndeterminate,
			Reason:      "no policy limits fetched yet",
			EngineName:  e.Name(),
			EvaluatedAt: time.Now(),
			LatencyMs:   time.Since(start).Milliseconds(),
		}, nil
	}
	if r, ok := limits.Restrictions[req.ToolName]; ok && !r.Allowed {
		return PolicyEvaluationResult{
			Outcome:     PolicyDeny,
			Reason:      fmt.Sprintf("policy limits restrict tool %q for this enterprise", req.ToolName),
			EngineName:  e.Name(),
			EvaluatedAt: time.Now(),
			LatencyMs:   time.Since(start).Milliseconds(),
		}, nil
	}
	return PolicyEvaluationResult{
		Outcome:     PolicyAllow,
		EngineName:  e.Name(),
		EvaluatedAt: time.Now(),
		LatencyMs:   time.Since(start).Milliseconds(),
	}, nil
}

// --- ChainedPolicyEngine ---

// ChainedPolicyEngine composes multiple engines in priority order.
//
// Decision semantics:
//   - First engine that returns Deny → DENY (and stop).
//   - First engine that returns RequireApproval (and no prior Deny)
//     → REQUIRE_APPROVAL (and stop).
//   - All engines return Allow / Indeterminate → ALLOW.
//   - All engines return Indeterminate (no Allow at all) → INDETERMINATE.
//
// Obligations from ALL engines that returned Allow are unioned.
//
// EngineName on the result is "chain:<first-terminal-name>" so the
// audit trail records who the deciding voice was.
type ChainedPolicyEngine struct {
	name    string
	engines []PolicyEngine
}

// NewChainedPolicyEngine creates a chain. `name` identifies the chain
// in audit logs (e.g. "enterprise-then-tenant"); `engines` is the
// ordered list — first deny wins.
func NewChainedPolicyEngine(name string, engines ...PolicyEngine) *ChainedPolicyEngine {
	return &ChainedPolicyEngine{name: name, engines: engines}
}

// Name returns the chain's identifier.
func (c *ChainedPolicyEngine) Name() string { return c.name }

// Evaluate runs the engines in order with first-deny-wins semantics.
func (c *ChainedPolicyEngine) Evaluate(ctx context.Context, req PolicyEvaluationRequest) (PolicyEvaluationResult, error) {
	if err := ctx.Err(); err != nil {
		return PolicyEvaluationResult{}, fmt.Errorf("policyengine: ctx cancelled: %w", err)
	}
	if req.TenantID == "" {
		return PolicyEvaluationResult{}, ErrNilPolicyRequest
	}
	start := time.Now()

	allObligations := []string{}
	sawAllow := false
	var firstApproval *PolicyEvaluationResult

	for _, eng := range c.engines {
		if err := ctx.Err(); err != nil {
			return PolicyEvaluationResult{}, fmt.Errorf("policyengine: ctx cancelled mid-chain: %w", err)
		}
		res, err := eng.Evaluate(ctx, req)
		if err != nil {
			// Underlying engine errored — surface it; caller decides
			// retry/fallback. Do NOT silently allow.
			return PolicyEvaluationResult{}, fmt.Errorf("policyengine: chain engine %q failed: %w", eng.Name(), err)
		}
		switch res.Outcome {
		case PolicyDeny:
			res.EngineName = c.name + ":" + eng.Name()
			res.LatencyMs = time.Since(start).Milliseconds()
			return res, nil
		case PolicyRequireApproval:
			if firstApproval == nil {
				copy := res
				copy.EngineName = c.name + ":" + eng.Name()
				firstApproval = &copy
			}
		case PolicyAllow:
			sawAllow = true
			allObligations = append(allObligations, res.Obligations...)
		case PolicyIndeterminate:
			// Keep going.
		}
	}

	if firstApproval != nil {
		firstApproval.LatencyMs = time.Since(start).Milliseconds()
		return *firstApproval, nil
	}
	if sawAllow {
		return PolicyEvaluationResult{
			Outcome:     PolicyAllow,
			Obligations: allObligations,
			EngineName:  c.name,
			EvaluatedAt: time.Now(),
			LatencyMs:   time.Since(start).Milliseconds(),
		}, nil
	}
	// All engines returned Indeterminate.
	return PolicyEvaluationResult{
		Outcome:     PolicyIndeterminate,
		Reason:      "all engines returned indeterminate",
		EngineName:  c.name,
		EvaluatedAt: time.Now(),
		LatencyMs:   time.Since(start).Milliseconds(),
	}, nil
}

// --- StaticDenyPolicyEngine — useful for tests and deny-list configs ---

// StaticDenyPolicyEngine denies a fixed set of tool names. Useful for
// hardcoded blocklists and test fixtures. Concurrent-safe (RWMutex).
type StaticDenyPolicyEngine struct {
	name string
	mu   sync.RWMutex
	deny map[string]string // tool name → reason
}

// NewStaticDenyPolicyEngine creates the engine with the initial deny set.
func NewStaticDenyPolicyEngine(name string, deny map[string]string) *StaticDenyPolicyEngine {
	cp := map[string]string{}
	for k, v := range deny {
		cp[k] = v
	}
	return &StaticDenyPolicyEngine{name: name, deny: cp}
}

// Name returns the engine's identifier.
func (s *StaticDenyPolicyEngine) Name() string { return s.name }

// Evaluate denies tools in the set, allows others.
func (s *StaticDenyPolicyEngine) Evaluate(ctx context.Context, req PolicyEvaluationRequest) (PolicyEvaluationResult, error) {
	if err := ctx.Err(); err != nil {
		return PolicyEvaluationResult{}, fmt.Errorf("policyengine: ctx cancelled: %w", err)
	}
	if req.TenantID == "" {
		return PolicyEvaluationResult{}, ErrNilPolicyRequest
	}
	start := time.Now()
	s.mu.RLock()
	reason, denied := s.deny[req.ToolName]
	s.mu.RUnlock()
	if denied {
		return PolicyEvaluationResult{
			Outcome:     PolicyDeny,
			Reason:      reason,
			EngineName:  s.Name(),
			EvaluatedAt: time.Now(),
			LatencyMs:   time.Since(start).Milliseconds(),
		}, nil
	}
	return PolicyEvaluationResult{
		Outcome:     PolicyAllow,
		EngineName:  s.Name(),
		EvaluatedAt: time.Now(),
		LatencyMs:   time.Since(start).Milliseconds(),
	}, nil
}

// AddDeny adds (or overwrites) a deny entry. Concurrent-safe.
func (s *StaticDenyPolicyEngine) AddDeny(toolName, reason string) {
	s.mu.Lock()
	s.deny[toolName] = reason
	s.mu.Unlock()
}
