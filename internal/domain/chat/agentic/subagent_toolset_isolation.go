package agentic

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// SUB-005 — Isolated toolsets for subagents.
//
// PDF arXiv:2604.14228v1 §8 (Subagents) — when a subagent is spawned,
// the parent should declare WHICH TOOLS the subagent may use, not just
// inherit the full parent pool. The PDF cites three drivers:
//   1. Capability isolation — a documentation subagent never needs Bash.
//   2. Recursion safety — a subagent must not see the Agent tool itself
//      at depth > N (otherwise spawn loops are possible).
//   3. Sensitive surface protection — admin/management tools should
//      only be available at depth 0 (the parent agent).
//
// This file implements the pure-domain SUBAGENT TOOLSET POLICY: given
// the parent's tool pool and the subagent's policy + current depth,
// produce the filtered tool list the subagent sees.
//
// Distinct from existing AgentHub plumbing:
//   - TOOL-003 ToolPoolAssembler.SetAllowlist = LOW-LEVEL allowlist on
//     entry names. SUB-005 is the HIGH-LEVEL declarative policy that
//     emits the allowlist + filter combo.
//   - SUB-006 ResolveSubagentPermissions = permission RULES composition
//     (allow/deny patterns). SUB-005 = tool POOL composition (which
//     tools exist). They are orthogonal — both feed the runtime.
//   - skill.AllowedTools (DB) = per-skill tool restriction at registry
//     time. SUB-005 = per-subagent at spawn time.

// SubagentToolsetIsolationMode bounded enum describes how the parent
// constrains the subagent's tool pool.
type SubagentToolsetIsolationMode string

const (
	// SubagentToolsetExplicitAllowlist — only the names in
	// AllowedToolNames are visible to the subagent. Parent pool is
	// otherwise ignored. Most restrictive mode.
	SubagentToolsetExplicitAllowlist SubagentToolsetIsolationMode = "explicit_allowlist"
	// SubagentToolsetParentMinusBlocklist — subagent sees parent pool
	// EXCEPT the names in BlockedToolNames. Use for "almost everything
	// the parent has, minus a few sharp edges".
	SubagentToolsetParentMinusBlocklist SubagentToolsetIsolationMode = "parent_minus_blocklist"
	// SubagentToolsetDepthFiltered — depth-aware filter: each tool
	// name in ExcludedAtDepthGreaterThan is removed when current depth
	// exceeds the threshold. Use for recursion safety (Agent tool
	// excluded at depth > 1).
	SubagentToolsetDepthFiltered SubagentToolsetIsolationMode = "depth_filtered"
	// SubagentToolsetCategoricalExclusion — names matching a category
	// prefix or suffix are excluded categorically (e.g., everything
	// matching "admin_*" or "*_dangerous").
	SubagentToolsetCategoricalExclusion SubagentToolsetIsolationMode = "categorical_exclusion"
)

var allSubagentToolsetIsolationModes = []SubagentToolsetIsolationMode{
	SubagentToolsetExplicitAllowlist,
	SubagentToolsetParentMinusBlocklist,
	SubagentToolsetDepthFiltered,
	SubagentToolsetCategoricalExclusion,
}

// IsValidSubagentToolsetIsolationMode returns true for the bounded set.
func IsValidSubagentToolsetIsolationMode(m SubagentToolsetIsolationMode) bool {
	for _, v := range allSubagentToolsetIsolationModes {
		if m == v {
			return true
		}
	}
	return false
}

// SubagentToolsetPolicy declares how the subagent's pool is built.
type SubagentToolsetPolicy struct {
	Mode SubagentToolsetIsolationMode
	// AllowedToolNames is used by ExplicitAllowlist. Empty in other modes.
	AllowedToolNames []string
	// BlockedToolNames is used by ParentMinusBlocklist. Empty otherwise.
	BlockedToolNames []string
	// ExcludedAtDepthGreaterThan: a tool name → max depth threshold.
	// When current depth > threshold, the tool is removed. Example:
	// {"Agent": 1} keeps the Agent tool only at depth 0 and 1.
	// Used by DepthFiltered mode.
	ExcludedAtDepthGreaterThan map[string]int
	// CategoryPrefixesExcluded: tool names starting with any of these
	// strings are dropped. Used by CategoricalExclusion mode.
	CategoryPrefixesExcluded []string
	// CategorySuffixesExcluded: tool names ending with any of these
	// strings are dropped. Used by CategoricalExclusion mode.
	CategorySuffixesExcluded []string
}

// Validate enforces invariants per mode.
func (p SubagentToolsetPolicy) Validate() error {
	if !IsValidSubagentToolsetIsolationMode(p.Mode) {
		return fmt.Errorf("%w: %q", ErrSubagentToolsetBadMode, p.Mode)
	}
	switch p.Mode {
	case SubagentToolsetExplicitAllowlist:
		if len(p.AllowedToolNames) == 0 {
			return ErrSubagentToolsetAllowlistEmpty
		}
	case SubagentToolsetDepthFiltered:
		for name, threshold := range p.ExcludedAtDepthGreaterThan {
			if strings.TrimSpace(name) == "" {
				return ErrSubagentToolsetEmptyName
			}
			if threshold < 0 {
				return fmt.Errorf("%w: tool %q threshold negative", ErrSubagentToolsetBadDepth, name)
			}
		}
	case SubagentToolsetCategoricalExclusion:
		if len(p.CategoryPrefixesExcluded) == 0 && len(p.CategorySuffixesExcluded) == 0 {
			return ErrSubagentToolsetCategoryEmpty
		}
	}
	return nil
}

// SubagentToolsetResolution is the audit-shaped output.
type SubagentToolsetResolution struct {
	Mode           SubagentToolsetIsolationMode
	CurrentDepth   int
	KeptToolNames  []string
	DroppedToolNames []string
	Reason         string
}

// Sentinel errors.
var (
	ErrSubagentToolsetBadMode        = errors.New("subagent toolset: invalid isolation mode")
	ErrSubagentToolsetAllowlistEmpty = errors.New("subagent toolset: explicit_allowlist requires AllowedToolNames")
	ErrSubagentToolsetCategoryEmpty  = errors.New("subagent toolset: categorical_exclusion requires prefix or suffix patterns")
	ErrSubagentToolsetEmptyName      = errors.New("subagent toolset: empty tool name in policy")
	ErrSubagentToolsetBadDepth       = errors.New("subagent toolset: invalid depth threshold")
	ErrSubagentToolsetBadDepthInput  = errors.New("subagent toolset: depth must be >= 0")
)

// ResolveSubagentToolset applies the policy to the parent pool and
// returns the entries the subagent should see, plus an audit record.
//
// currentDepth is the subagent's depth (parent = 0, child = 1, etc.).
// Pure function — never mutates inputs.
func ResolveSubagentToolset(parentPool []ToolPoolEntry, policy SubagentToolsetPolicy, currentDepth int) ([]ToolPoolEntry, SubagentToolsetResolution, error) {
	if err := policy.Validate(); err != nil {
		return nil, SubagentToolsetResolution{}, err
	}
	if currentDepth < 0 {
		return nil, SubagentToolsetResolution{}, ErrSubagentToolsetBadDepthInput
	}

	res := SubagentToolsetResolution{
		Mode:         policy.Mode,
		CurrentDepth: currentDepth,
	}

	var kept []ToolPoolEntry
	dropped := []string{}

	switch policy.Mode {
	case SubagentToolsetExplicitAllowlist:
		allowed := map[string]bool{}
		for _, n := range policy.AllowedToolNames {
			allowed[n] = true
		}
		for _, e := range parentPool {
			if allowed[e.Name] {
				kept = append(kept, e)
			} else {
				dropped = append(dropped, e.Name)
			}
		}
		res.Reason = fmt.Sprintf("explicit_allowlist: %d allowed, %d dropped from parent pool",
			len(kept), len(dropped))

	case SubagentToolsetParentMinusBlocklist:
		blocked := map[string]bool{}
		for _, n := range policy.BlockedToolNames {
			blocked[n] = true
		}
		for _, e := range parentPool {
			if blocked[e.Name] {
				dropped = append(dropped, e.Name)
				continue
			}
			kept = append(kept, e)
		}
		res.Reason = fmt.Sprintf("parent_minus_blocklist: %d kept, %d blocked",
			len(kept), len(dropped))

	case SubagentToolsetDepthFiltered:
		for _, e := range parentPool {
			threshold, hasRule := policy.ExcludedAtDepthGreaterThan[e.Name]
			if hasRule && currentDepth > threshold {
				dropped = append(dropped, e.Name)
				continue
			}
			kept = append(kept, e)
		}
		res.Reason = fmt.Sprintf("depth_filtered at depth=%d: %d kept, %d excluded",
			currentDepth, len(kept), len(dropped))

	case SubagentToolsetCategoricalExclusion:
		for _, e := range parentPool {
			if matchesCategoryFilter(e.Name, policy.CategoryPrefixesExcluded, policy.CategorySuffixesExcluded) {
				dropped = append(dropped, e.Name)
				continue
			}
			kept = append(kept, e)
		}
		res.Reason = fmt.Sprintf("categorical_exclusion: %d kept, %d category-dropped",
			len(kept), len(dropped))
	}

	// Deterministic audit output.
	sort.Strings(dropped)
	res.KeptToolNames = entryNames(kept)
	sort.Strings(res.KeptToolNames)
	res.DroppedToolNames = dropped
	return kept, res, nil
}

// AsToolPoolFilter returns a ToolPoolFilter that the parent harness
// can hand to a child ToolPoolAssembler. The closure captures the
// policy + depth and drops disallowed entries when the child pool is
// being assembled.
//
// This is the integration point with TOOL-003: subagent assemblers
// AddFilter(policy.AsToolPoolFilter(depth)) before Assemble.
func (p SubagentToolsetPolicy) AsToolPoolFilter(currentDepth int) (ToolPoolFilter, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if currentDepth < 0 {
		return nil, ErrSubagentToolsetBadDepthInput
	}

	// Index policy fields once so the closure doesn't re-scan slices.
	allowSet := map[string]bool{}
	for _, n := range p.AllowedToolNames {
		allowSet[n] = true
	}
	blockSet := map[string]bool{}
	for _, n := range p.BlockedToolNames {
		blockSet[n] = true
	}
	depthRules := p.ExcludedAtDepthGreaterThan
	prefixes := append([]string(nil), p.CategoryPrefixesExcluded...)
	suffixes := append([]string(nil), p.CategorySuffixesExcluded...)
	mode := p.Mode

	return func(entry ToolPoolEntry) bool {
		switch mode {
		case SubagentToolsetExplicitAllowlist:
			return allowSet[entry.Name]
		case SubagentToolsetParentMinusBlocklist:
			return !blockSet[entry.Name]
		case SubagentToolsetDepthFiltered:
			threshold, hasRule := depthRules[entry.Name]
			if hasRule && currentDepth > threshold {
				return false
			}
			return true
		case SubagentToolsetCategoricalExclusion:
			return !matchesCategoryFilter(entry.Name, prefixes, suffixes)
		}
		return true
	}, nil
}

func entryNames(entries []ToolPoolEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Name)
	}
	return out
}

func matchesCategoryFilter(name string, prefixes, suffixes []string) bool {
	for _, p := range prefixes {
		if p == "" {
			continue
		}
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	for _, s := range suffixes {
		if s == "" {
			continue
		}
		if strings.HasSuffix(name, s) {
			return true
		}
	}
	return false
}
