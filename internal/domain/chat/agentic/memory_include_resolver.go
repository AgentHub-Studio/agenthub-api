package agentic

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// CTX-007 — @include em memória.
//
// PDF arXiv:2604.14228v1 §7.10 (memory facts can reference other facts
// via @include{key} markers; resolver expands them recursively with
// cycle detection and max-depth guard).
//
// Distinct from existing AgentHub plumbing:
//   - memory_hierarchy.go (CTX-003) = layered fact lookup.
//   - content_reference.go (CTX-009) = opaque @ref tokens with content
//     stored in registry (hash-keyed).
//   - memory_include_resolver.go (this file) = TEXT-BASED include
//     expansion: source memory contains literal @include{key}; resolver
//     pulls value from a lookup, recursively expands nested @includes,
//     guards against cycles + max depth.

// MemoryIncludeMissingPolicy bounded enum controls behavior when
// an @include{key} references a key that isn't in the lookup.
type MemoryIncludeMissingPolicy string

const (
	// MemoryIncludeLeaveAsIs — leave the @include{key} literal in output.
	MemoryIncludeLeaveAsIs MemoryIncludeMissingPolicy = "leave_as_is"
	// MemoryIncludeReplaceWithEmpty — replace with empty string.
	MemoryIncludeReplaceWithEmpty MemoryIncludeMissingPolicy = "replace_with_empty"
	// MemoryIncludeErrorOnMissing — abort resolution with ErrMemoryIncludeMissing.
	MemoryIncludeErrorOnMissing MemoryIncludeMissingPolicy = "error"
)

var allMemoryIncludeMissingPolicies = []MemoryIncludeMissingPolicy{
	MemoryIncludeLeaveAsIs, MemoryIncludeReplaceWithEmpty, MemoryIncludeErrorOnMissing,
}

// IsValidMemoryIncludeMissingPolicy returns true for the bounded set.
func IsValidMemoryIncludeMissingPolicy(p MemoryIncludeMissingPolicy) bool {
	for _, v := range allMemoryIncludeMissingPolicies {
		if p == v {
			return true
		}
	}
	return false
}

// AllMemoryIncludeMissingPolicies returns a copy.
func AllMemoryIncludeMissingPolicies() []MemoryIncludeMissingPolicy {
	out := make([]MemoryIncludeMissingPolicy, len(allMemoryIncludeMissingPolicies))
	copy(out, allMemoryIncludeMissingPolicies)
	return out
}

// includePattern matches @include{key} markers. Key allows
// alphanumeric, dot, dash, underscore.
var includePattern = regexp.MustCompile(`@include\{([a-zA-Z0-9._-]+)\}`)

// MemoryIncludeResolverConfig tunes the resolver.
type MemoryIncludeResolverConfig struct {
	// MaxDepth caps recursive include depth. 0 = unlimited (NOT recommended).
	MaxDepth int
	// MissingPolicy decides behavior on unresolved @include{key}.
	MissingPolicy MemoryIncludeMissingPolicy
}

// DefaultMemoryIncludeResolverConfig returns sensible defaults.
func DefaultMemoryIncludeResolverConfig() MemoryIncludeResolverConfig {
	return MemoryIncludeResolverConfig{
		MaxDepth:      8,
		MissingPolicy: MemoryIncludeLeaveAsIs,
	}
}

// MemoryIncludeResolveTrace records resolution details for audit/debug.
type MemoryIncludeResolveTrace struct {
	// ExpandedKeys is the list of keys expanded (ordered).
	ExpandedKeys []string `json:"expandedKeys"`
	// MissingKeys is the list of @include{key} not in lookup.
	MissingKeys []string `json:"missingKeys"`
	// MaxDepthReached is the deepest recursion the resolver hit.
	MaxDepthReached int `json:"maxDepthReached"`
	// CycleDetected lists keys that triggered cycle detection.
	CycleDetected []string `json:"cycleDetected,omitempty"`
}

// Sentinels.
var (
	ErrMemoryIncludeInvalidPolicy = errors.New("memory include resolver: invalid missing policy")
	ErrMemoryIncludeCycle         = errors.New("memory include resolver: cycle detected")
	ErrMemoryIncludeMaxDepth      = errors.New("memory include resolver: max depth exceeded")
	ErrMemoryIncludeMissing       = errors.New("memory include resolver: include key not found")
)

// MemoryIncludeResolver expands @include{key} markers.
type MemoryIncludeResolver struct {
	mu     sync.Mutex
	config MemoryIncludeResolverConfig
	lookup map[string]string
}

// NewMemoryIncludeResolver creates a resolver with the given config + lookup.
func NewMemoryIncludeResolver(cfg MemoryIncludeResolverConfig, lookup map[string]string) (*MemoryIncludeResolver, error) {
	if !IsValidMemoryIncludeMissingPolicy(cfg.MissingPolicy) {
		return nil, fmt.Errorf("%w: %q", ErrMemoryIncludeInvalidPolicy, cfg.MissingPolicy)
	}
	if cfg.MaxDepth < 0 {
		return nil, errors.New("memory include resolver: max_depth must be ≥ 0")
	}
	// Defensive copy of lookup to avoid external mutation.
	internalLookup := make(map[string]string, len(lookup))
	for k, v := range lookup {
		internalLookup[k] = v
	}
	return &MemoryIncludeResolver{
		config: cfg,
		lookup: internalLookup,
	}, nil
}

// SetLookup replaces the resolver's lookup table (e.g. when memory
// hierarchy changes).
func (r *MemoryIncludeResolver) SetLookup(lookup map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	internalLookup := make(map[string]string, len(lookup))
	for k, v := range lookup {
		internalLookup[k] = v
	}
	r.lookup = internalLookup
}

// Resolve recursively expands @include{key} markers in text. Returns
// the expanded text + a trace for audit. Cycles and depth-exceeded
// surface as errors.
func (r *MemoryIncludeResolver) Resolve(ctx context.Context, text string) (string, MemoryIncludeResolveTrace, error) {
	if err := ctx.Err(); err != nil {
		return "", MemoryIncludeResolveTrace{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	trace := &MemoryIncludeResolveTrace{}
	visited := map[string]bool{}
	expanded, err := r.expand(ctx, text, 0, visited, trace)
	if err != nil {
		return "", *trace, err
	}
	return expanded, *trace, nil
}

// expand is the recursive core. visited tracks the current resolution
// path to detect cycles.
func (r *MemoryIncludeResolver) expand(ctx context.Context, text string, depth int, visited map[string]bool, trace *MemoryIncludeResolveTrace) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if depth > trace.MaxDepthReached {
		trace.MaxDepthReached = depth
	}
	if r.config.MaxDepth > 0 && depth > r.config.MaxDepth {
		return "", fmt.Errorf("%w: %d > %d", ErrMemoryIncludeMaxDepth, depth, r.config.MaxDepth)
	}

	matches := includePattern.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return text, nil
	}

	var b strings.Builder
	cursor := 0
	for _, m := range matches {
		// m[0]=full start, m[1]=full end, m[2]=key start, m[3]=key end.
		fullStart, fullEnd, keyStart, keyEnd := m[0], m[1], m[2], m[3]
		key := text[keyStart:keyEnd]

		// Cycle detection.
		if visited[key] {
			trace.CycleDetected = append(trace.CycleDetected, key)
			return "", fmt.Errorf("%w: %q", ErrMemoryIncludeCycle, key)
		}

		// Append literal text before this match.
		b.WriteString(text[cursor:fullStart])
		cursor = fullEnd

		val, ok := r.lookup[key]
		if !ok {
			// Apply missing policy.
			switch r.config.MissingPolicy {
			case MemoryIncludeErrorOnMissing:
				trace.MissingKeys = append(trace.MissingKeys, key)
				return "", fmt.Errorf("%w: %q", ErrMemoryIncludeMissing, key)
			case MemoryIncludeReplaceWithEmpty:
				trace.MissingKeys = append(trace.MissingKeys, key)
				// Append empty (nothing to write).
			case MemoryIncludeLeaveAsIs:
				trace.MissingKeys = append(trace.MissingKeys, key)
				b.WriteString(text[fullStart:fullEnd])
			}
			continue
		}

		// Resolve recursively with cycle-tracking.
		visited[key] = true
		nested, err := r.expand(ctx, val, depth+1, visited, trace)
		delete(visited, key)
		if err != nil {
			return "", err
		}
		trace.ExpandedKeys = append(trace.ExpandedKeys, key)
		b.WriteString(nested)
	}
	b.WriteString(text[cursor:])
	return b.String(), nil
}

// CountIncludes returns the number of @include{key} markers in text.
func CountIncludes(text string) int {
	return len(includePattern.FindAllString(text, -1))
}

// ExtractIncludeKeys returns a sorted list of unique keys referenced in text.
func ExtractIncludeKeys(text string) []string {
	matches := includePattern.FindAllStringSubmatch(text, -1)
	seen := map[string]bool{}
	for _, m := range matches {
		if len(m) >= 2 {
			seen[m[1]] = true
		}
	}
	out := []string{}
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
