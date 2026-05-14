package agentic

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// PERM-009 — Permission restore policy on resume / fork.
//
// PDF arXiv:2604.14228v1 §4 (Permissions and Safety) — when a session
// is RESUMED (saved transcript replayed) or FORKED (new session id
// branched from an existing one), previously-granted permissions must
// NOT be silently restored. The user who granted "allow Bash for this
// session" at 10:00 cannot be presumed to still consent at 17:00 the
// next day. Defaulting to restore-everything turns saved sessions into
// permission-escalation time bombs.
//
// Distinct from existing AgentHub plumbing:
//   - PERSIST-010 = SERIALISATION boundary: session-scoped permissions
//     are not written to disk in the first place.
//   - PERM-009 (this file) = RESTORE boundary: even if a grant was
//     persisted, the resume/fork pipeline filters it before
//     re-hydrating the runtime.
//   - PERSIST-004 (Resume) / PERSIST-005 (Fork) = the operations that
//     trigger this check; this file is the policy they consult.
//   - PERM-005 PermissionHookChain + PERM-004 PermissionPreFilter =
//     the runtime that will re-evaluate any non-restored grant.

// PermissionRestorePolicy bounded enum declares which grants survive
// the resume/fork boundary.
type PermissionRestorePolicy string

const (
	// PermissionRestoreDiscardAll — drop every grant; force the LLM
	// to re-request each one. Most conservative; default for fresh
	// tenants without explicit admin opt-in.
	PermissionRestoreDiscardAll PermissionRestorePolicy = "discard_all"
	// PermissionRestorePreserveDurableOnly — keep grants with
	// GrantDurabilityPersisted or higher (explicit_admin); drop
	// session_scoped and one_shot. Default for routine tenants.
	PermissionRestorePreserveDurableOnly PermissionRestorePolicy = "preserve_durable_only"
	// PermissionRestorePreserveExplicitGrants — keep only grants
	// originating from explicit admin action (GrantDurabilityExplicitAdmin).
	// Used for compliance-strict tenants where any non-admin grant
	// must be re-confirmed.
	PermissionRestorePreserveExplicitGrants PermissionRestorePolicy = "preserve_explicit_grants"
	// PermissionRestoreStrictReRequest — drop all and additionally
	// flag the runtime to require user confirmation on the FIRST
	// invocation of every tool, regardless of static rules. Used for
	// audit-strict tenants resuming a session after a long gap.
	PermissionRestoreStrictReRequest PermissionRestorePolicy = "strict_re_request"
)

var allPermissionRestorePolicies = []PermissionRestorePolicy{
	PermissionRestoreDiscardAll,
	PermissionRestorePreserveDurableOnly,
	PermissionRestorePreserveExplicitGrants,
	PermissionRestoreStrictReRequest,
}

// IsValidPermissionRestorePolicy returns true for the bounded set.
func IsValidPermissionRestorePolicy(p PermissionRestorePolicy) bool {
	for _, v := range allPermissionRestorePolicies {
		if p == v {
			return true
		}
	}
	return false
}

// GrantDurability bounded enum tags how durable a previously-granted
// permission is.
type GrantDurability string

const (
	// GrantDurabilityOneShot — single tool call grant; never survives.
	GrantDurabilityOneShot GrantDurability = "one_shot"
	// GrantDurabilitySessionScoped — survived the original session
	// only; PERSIST-010 already filters this out at write-time, but
	// this enum still classifies for in-memory transfer cases.
	GrantDurabilitySessionScoped GrantDurability = "session_scoped"
	// GrantDurabilityPersisted — written to durable storage (DB) with
	// an expiry but no admin sign-off.
	GrantDurabilityPersisted GrantDurability = "persisted"
	// GrantDurabilityExplicitAdmin — admin granted via the management
	// surface; survives until admin revokes.
	GrantDurabilityExplicitAdmin GrantDurability = "explicit_admin"
)

var allGrantDurabilities = []GrantDurability{
	GrantDurabilityOneShot, GrantDurabilitySessionScoped,
	GrantDurabilityPersisted, GrantDurabilityExplicitAdmin,
}

// IsValidGrantDurability returns true for the bounded set.
func IsValidGrantDurability(d GrantDurability) bool {
	for _, v := range allGrantDurabilities {
		if d == v {
			return true
		}
	}
	return false
}

// RestoreOperation bounded enum identifies which operation triggered
// the filter (audit information).
type RestoreOperation string

const (
	RestoreOperationResume RestoreOperation = "resume"
	RestoreOperationFork   RestoreOperation = "fork"
)

var allRestoreOperations = []RestoreOperation{
	RestoreOperationResume, RestoreOperationFork,
}

// IsValidRestoreOperation returns true for the bounded set.
func IsValidRestoreOperation(o RestoreOperation) bool {
	for _, v := range allRestoreOperations {
		if o == v {
			return true
		}
	}
	return false
}

// PreviousGrant is one permission record from the saved session that
// the resume/fork pipeline considers restoring.
type PreviousGrant struct {
	ToolName    string
	InputPrefix string // optional: input pattern that originally matched
	Decision    PermissionDecision
	Durability  GrantDurability
	GrantedAt   time.Time
	GrantedBy   string // user id or "admin" or "auto"
}

// Validate enforces invariants.
func (g PreviousGrant) Validate() error {
	if strings.TrimSpace(g.ToolName) == "" {
		return ErrPermissionRestoreEmptyToolName
	}
	if !IsValidGrantDurability(g.Durability) {
		return fmt.Errorf("%w: %q", ErrPermissionRestoreBadDurability, g.Durability)
	}
	return nil
}

// RestoreContext carries the inputs to FilterRestorableGrants.
type RestoreContext struct {
	Operation       RestoreOperation
	OriginalSession string
	NewSession      string
	GapSeconds      int // seconds between original last-activity and resume
}

// Validate enforces invariants.
func (c RestoreContext) Validate() error {
	if !IsValidRestoreOperation(c.Operation) {
		return fmt.Errorf("%w: %q", ErrPermissionRestoreBadOperation, c.Operation)
	}
	if c.GapSeconds < 0 {
		return ErrPermissionRestoreNegativeGap
	}
	return nil
}

// RestoreDecision is the per-grant audit record.
type RestoreDecision struct {
	Grant   PreviousGrant
	Kept    bool
	Reason  string
}

// RestoreResolution is the audit-shaped output of FilterRestorableGrants.
type RestoreResolution struct {
	Policy                     PermissionRestorePolicy
	Context                    RestoreContext
	GrantsConsidered           int
	GrantsKept                 []PreviousGrant
	GrantsDropped              []PreviousGrant
	Decisions                  []RestoreDecision
	RequireFirstUseConfirmation bool // true when policy is StrictReRequest
}

// Sentinel errors.
var (
	ErrPermissionRestoreBadPolicy       = errors.New("permission restore: invalid policy")
	ErrPermissionRestoreBadDurability   = errors.New("permission restore: invalid grant durability")
	ErrPermissionRestoreBadOperation    = errors.New("permission restore: invalid operation")
	ErrPermissionRestoreEmptyToolName   = errors.New("permission restore: grant tool name required")
	ErrPermissionRestoreNegativeGap     = errors.New("permission restore: gap seconds must be >= 0")
)

// FilterRestorableGrants applies the policy to the saved-session
// grants and returns the kept set + audit. Pure function.
//
// Invariants regardless of policy:
//   - One-shot grants NEVER survive (single-call by definition).
//   - Deny grants are always preserved — the user who said NO must
//     not be silently overruled by a resume.
//   - The audit records every grant decision so compliance can answer
//     "why did the runtime not re-grant X".
func FilterRestorableGrants(policy PermissionRestorePolicy, ctx RestoreContext, grants []PreviousGrant) (RestoreResolution, error) {
	if !IsValidPermissionRestorePolicy(policy) {
		return RestoreResolution{}, fmt.Errorf("%w: %q", ErrPermissionRestoreBadPolicy, policy)
	}
	if err := ctx.Validate(); err != nil {
		return RestoreResolution{}, err
	}
	for _, g := range grants {
		if err := g.Validate(); err != nil {
			return RestoreResolution{}, err
		}
	}

	res := RestoreResolution{
		Policy:           policy,
		Context:          ctx,
		GrantsConsidered: len(grants),
		RequireFirstUseConfirmation: policy == PermissionRestoreStrictReRequest,
	}

	for _, g := range grants {
		kept, reason := decideRestore(policy, g)
		if kept {
			res.GrantsKept = append(res.GrantsKept, g)
		} else {
			res.GrantsDropped = append(res.GrantsDropped, g)
		}
		res.Decisions = append(res.Decisions, RestoreDecision{
			Grant: g, Kept: kept, Reason: reason,
		})
	}

	sortGrants(res.GrantsKept)
	sortGrants(res.GrantsDropped)
	return res, nil
}

// decideRestore returns (keep, reason) for a single grant under the
// policy.
func decideRestore(policy PermissionRestorePolicy, g PreviousGrant) (bool, string) {
	// Deny grants always survive — never silently overrule a user's
	// past refusal.
	if g.Decision == PermissionDeny {
		return true, "deny grants always preserved"
	}
	// One-shot grants never survive regardless of policy.
	if g.Durability == GrantDurabilityOneShot {
		return false, "one_shot grant cannot survive resume/fork"
	}

	switch policy {
	case PermissionRestoreDiscardAll:
		return false, "policy discard_all: every Allow/Confirm grant dropped"

	case PermissionRestorePreserveDurableOnly:
		if g.Durability == GrantDurabilityPersisted || g.Durability == GrantDurabilityExplicitAdmin {
			return true, fmt.Sprintf("policy preserve_durable_only: %s grant survives", g.Durability)
		}
		return false, fmt.Sprintf("policy preserve_durable_only: %s grant dropped", g.Durability)

	case PermissionRestorePreserveExplicitGrants:
		if g.Durability == GrantDurabilityExplicitAdmin {
			return true, "policy preserve_explicit_grants: admin grant survives"
		}
		return false, fmt.Sprintf("policy preserve_explicit_grants: %s grant dropped", g.Durability)

	case PermissionRestoreStrictReRequest:
		return false, "policy strict_re_request: every Allow/Confirm grant dropped + first-use confirmation required"
	}
	return false, "unreachable"
}

func sortGrants(in []PreviousGrant) {
	sort.SliceStable(in, func(i, j int) bool {
		if in[i].ToolName != in[j].ToolName {
			return in[i].ToolName < in[j].ToolName
		}
		return in[i].GrantedAt.Before(in[j].GrantedAt)
	})
}
