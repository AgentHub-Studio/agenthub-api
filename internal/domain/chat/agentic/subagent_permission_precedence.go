package agentic

import (
	"errors"
	"fmt"
	"sort"
)

// SUB-006 — Permission override precedence (subagent ↔ parent).
//
// PDF arXiv:2604.14228v1 §8 (Subagents) — when the harness spawns a
// subagent, the subagent inherits the parent's permission posture by
// default. But subagents often need:
//   - to be MORE restrictive (e.g., read-only investigator),
//   - to be DIFFERENT (e.g., a documentation generator that only
//     touches /docs paths),
//   - to ignore parent rules entirely (e.g., a sandbox-style worker).
//
// Without an explicit precedence policy, every spawn-time decision is
// ad-hoc and the audit trail loses meaning. SUB-006 makes the policy
// declarative.
//
// Distinct from existing AgentHub plumbing:
//   - permission.go EvaluatePermission = the engine that evaluates ONE
//     PermissionRules block. SUB-006 produces the rules to feed it.
//   - PERM-005 PermissionHookChain = wraps the engine with before/after
//     hooks. SUB-006 sits BEFORE the chain: it computes the effective
//     ruleset the chain will use for the subagent.
//   - PERM-004 PermissionPreFilter = pool-time deny screening. After
//     SUB-006 produces effective rules, PERM-004 uses them.
//   - SUB-001 (Agent tool) / forkedagent.go = the SPAWN MECHANICS;
//     SUB-006 declares the RULE COMPOSITION semantics.

// PermissionInheritanceMode bounded enum controls how parent rules
// combine with child rules.
type PermissionInheritanceMode string

const (
	// PermissionInheritAll — child rules overlay onto parent.
	// Allow: parent ∪ child. Deny: parent ∪ child (deny wins on conflict).
	// Confirm: parent ∪ child. Mode: child overrides if set, else parent.
	// Use case: subagent extends parent rights with extra allows.
	PermissionInheritAll PermissionInheritanceMode = "inherit_all"
	// PermissionInheritStrictOnly — only DENY rules inherit from
	// parent. Allow/Confirm/Mode come exclusively from child. Use case:
	// child is a sandbox worker that may not relax parent's deny set
	// but otherwise defines its own posture.
	PermissionInheritStrictOnly PermissionInheritanceMode = "inherit_strict_only"
	// PermissionOverrideReplace — child replaces parent entirely. Use
	// case: child is an isolated worker that the parent has reviewed
	// and explicitly de-coupled from itself.
	PermissionOverrideReplace PermissionInheritanceMode = "override_replace"
	// PermissionMergeIntersect — child allow ∩ parent allow (only what
	// BOTH permit survives). Deny: parent ∪ child (broadest deny).
	// Use case: child must be at least as restrictive as parent for
	// allows, but may add extra denies.
	PermissionMergeIntersect PermissionInheritanceMode = "merge_intersect"
)

var allPermissionInheritanceModes = []PermissionInheritanceMode{
	PermissionInheritAll, PermissionInheritStrictOnly,
	PermissionOverrideReplace, PermissionMergeIntersect,
}

// IsValidPermissionInheritanceMode returns true for the bounded set.
func IsValidPermissionInheritanceMode(m PermissionInheritanceMode) bool {
	for _, v := range allPermissionInheritanceModes {
		if m == v {
			return true
		}
	}
	return false
}

// SubagentPermissionResolution is the audit-shaped output of a merge.
type SubagentPermissionResolution struct {
	Mode           PermissionInheritanceMode
	ParentRules    *PermissionRules // copy of input parent (or nil)
	ChildRules     *PermissionRules // copy of input child (or nil)
	EffectiveRules *PermissionRules // result the chain will use
	AddedDenies    []string         // patterns added by the merge (audit)
	AddedAllows    []string         // patterns added by the merge (audit)
	DroppedAllows  []string         // allows removed by intersection
	ReasonSummary  string           // one-line audit summary
}

// Sentinel errors.
var (
	ErrSubagentPermBadMode = errors.New("subagent permission: invalid inheritance mode")
)

// ResolveSubagentPermissions merges parent + child PermissionRules
// according to the inheritance mode. The output is the EFFECTIVE rules
// block to feed to EvaluatePermission for the subagent.
//
// Invariants regardless of mode:
//   - Deny rules from the parent ALWAYS survive in some form (safe
//     default — never relax parent's deny set, except in
//     OverrideReplace which is explicit).
//   - The function never mutates inputs; it returns new slices.
//   - Output PermissionRules.Mode falls back to parent when child is
//     unset, except in OverrideReplace.
//   - Order within slices is deterministic (sorted) for reproducible
//     audit emission.
func ResolveSubagentPermissions(parent, child *PermissionRules, mode PermissionInheritanceMode) (SubagentPermissionResolution, error) {
	if !IsValidPermissionInheritanceMode(mode) {
		return SubagentPermissionResolution{}, fmt.Errorf("%w: %q", ErrSubagentPermBadMode, mode)
	}

	res := SubagentPermissionResolution{
		Mode:        mode,
		ParentRules: copyRules(parent),
		ChildRules:  copyRules(child),
	}

	switch mode {
	case PermissionInheritAll:
		res.EffectiveRules = mergeAll(parent, child)
		res.AddedDenies = sortedDiff(rulesDeny(child), rulesDeny(parent))
		res.AddedAllows = sortedDiff(rulesAllow(child), rulesAllow(parent))
		res.ReasonSummary = "inherit_all: parent ∪ child for all categories"

	case PermissionInheritStrictOnly:
		res.EffectiveRules = mergeStrictOnly(parent, child)
		res.AddedDenies = sortedDiff(rulesDeny(parent), rulesDeny(child))
		res.AddedAllows = nil
		res.ReasonSummary = "inherit_strict_only: parent denies inherited; child defines the rest"

	case PermissionOverrideReplace:
		res.EffectiveRules = copyRules(child)
		if res.EffectiveRules == nil {
			res.EffectiveRules = &PermissionRules{}
		}
		res.AddedDenies = nil
		res.AddedAllows = nil
		res.ReasonSummary = "override_replace: child rules used verbatim; parent rules ignored"

	case PermissionMergeIntersect:
		intersected, dropped := mergeIntersect(parent, child)
		res.EffectiveRules = intersected
		res.AddedDenies = sortedDiff(rulesDeny(child), rulesDeny(parent))
		res.AddedAllows = nil
		res.DroppedAllows = dropped
		res.ReasonSummary = "merge_intersect: allow=intersect, deny=union"
	}

	return res, nil
}

// --- mode implementations ---

func mergeAll(parent, child *PermissionRules) *PermissionRules {
	if parent == nil && child == nil {
		return nil
	}
	out := &PermissionRules{
		Allow:   sortedUnion(rulesAllow(parent), rulesAllow(child)),
		Deny:    sortedUnion(rulesDeny(parent), rulesDeny(child)),
		Confirm: sortedUnion(rulesConfirm(parent), rulesConfirm(child)),
		Mode:    pickMode(parent, child),
	}
	return out
}

func mergeStrictOnly(parent, child *PermissionRules) *PermissionRules {
	if parent == nil && child == nil {
		return nil
	}
	return &PermissionRules{
		Allow:   sortedCopy(rulesAllow(child)),
		Deny:    sortedUnion(rulesDeny(parent), rulesDeny(child)),
		Confirm: sortedCopy(rulesConfirm(child)),
		Mode:    pickMode(parent, child),
	}
}

func mergeIntersect(parent, child *PermissionRules) (*PermissionRules, []string) {
	if parent == nil && child == nil {
		return nil, nil
	}
	parentAllow := rulesAllow(parent)
	childAllow := rulesAllow(child)
	intersectAllow, droppedAllow := intersectAndDifference(parentAllow, childAllow)
	return &PermissionRules{
		Allow:   intersectAllow,
		Deny:    sortedUnion(rulesDeny(parent), rulesDeny(child)),
		Confirm: sortedUnion(rulesConfirm(parent), rulesConfirm(child)),
		Mode:    pickMode(parent, child),
	}, droppedAllow
}

// --- helpers ---

func rulesAllow(r *PermissionRules) []string {
	if r == nil {
		return nil
	}
	return r.Allow
}

func rulesDeny(r *PermissionRules) []string {
	if r == nil {
		return nil
	}
	return r.Deny
}

func rulesConfirm(r *PermissionRules) []string {
	if r == nil {
		return nil
	}
	return r.Confirm
}

func pickMode(parent, child *PermissionRules) PermissionMode {
	if child != nil && child.Mode != "" {
		return child.Mode
	}
	if parent != nil {
		return parent.Mode
	}
	return ""
}

func copyRules(r *PermissionRules) *PermissionRules {
	if r == nil {
		return nil
	}
	return &PermissionRules{
		Allow:   sortedCopy(r.Allow),
		Deny:    sortedCopy(r.Deny),
		Confirm: sortedCopy(r.Confirm),
		Mode:    r.Mode,
	}
}

func sortedCopy(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func sortedUnion(a, b []string) []string {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := []string{}
	for _, v := range a {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	for _, v := range b {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}

// sortedDiff returns elements in a that are NOT in b. Sorted output.
func sortedDiff(a, b []string) []string {
	if len(a) == 0 {
		return nil
	}
	exclude := map[string]bool{}
	for _, v := range b {
		exclude[v] = true
	}
	out := []string{}
	seen := map[string]bool{}
	for _, v := range a {
		if exclude[v] || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// intersectAndDifference returns (intersect, difference). difference
// is elements in parentAllow that are NOT in childAllow (i.e., allows
// dropped because the child didn't grant them).
func intersectAndDifference(parentAllow, childAllow []string) ([]string, []string) {
	childSet := map[string]bool{}
	for _, v := range childAllow {
		childSet[v] = true
	}
	intersectSeen := map[string]bool{}
	differenceSeen := map[string]bool{}
	intersect := []string{}
	difference := []string{}
	for _, v := range parentAllow {
		if childSet[v] {
			if !intersectSeen[v] {
				intersectSeen[v] = true
				intersect = append(intersect, v)
			}
		} else {
			if !differenceSeen[v] {
				differenceSeen[v] = true
				difference = append(difference, v)
			}
		}
	}
	sort.Strings(intersect)
	sort.Strings(difference)
	if len(intersect) == 0 {
		intersect = nil
	}
	if len(difference) == 0 {
		difference = nil
	}
	return intersect, difference
}
