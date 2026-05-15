package agentic

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// CTX-004 — Path-scoped rules.
//
// PDF arXiv:2604.14228v1 §7.8 (rules can apply globally or to a path
// glob — e.g. "rules under tools/execute_sql/**" or "**/*.go").
// Specificity: more concrete globs win over wildcards.
//
// Distinct from existing AgentHub plumbing:
//   - permruleparser.go = permission rule parser (action-on-resource)
//   - rule_loader.go = catalog loader for ah_core seeded rules
//   - shellrulematch.go = shell command matching
//   - path_scoped_rule.go (this file) = pure-domain registry of
//     PATH-INDEXED rules + matcher returning rules sorted by specificity.

// PathScopedRuleScope bounded enum identifies the rule's audience.
type PathScopedRuleScope string

const (
	// PathRuleScopeGlobal — applies regardless of path (path glob "*").
	PathRuleScopeGlobal PathScopedRuleScope = "global"
	// PathRuleScopeTool — applies to specific tool slug paths.
	PathRuleScopeTool PathScopedRuleScope = "tool"
	// PathRuleScopeFile — applies to file paths (e.g. **/*.go).
	PathRuleScopeFile PathScopedRuleScope = "file"
	// PathRuleScopeDirectory — applies to directory paths.
	PathRuleScopeDirectory PathScopedRuleScope = "directory"
	// PathRuleScopeAgent — applies to agent-execution paths.
	PathRuleScopeAgent PathScopedRuleScope = "agent"
)

var allPathScopedRuleScopes = []PathScopedRuleScope{
	PathRuleScopeGlobal, PathRuleScopeTool, PathRuleScopeFile,
	PathRuleScopeDirectory, PathRuleScopeAgent,
}

// IsValidPathScopedRuleScope returns true for the bounded set.
func IsValidPathScopedRuleScope(s PathScopedRuleScope) bool {
	for _, v := range allPathScopedRuleScopes {
		if s == v {
			return true
		}
	}
	return false
}

// AllPathScopedRuleScopes returns a copy.
func AllPathScopedRuleScopes() []PathScopedRuleScope {
	out := make([]PathScopedRuleScope, len(allPathScopedRuleScopes))
	copy(out, allPathScopedRuleScopes)
	return out
}

// PathScopedRule is one rule indexed by a path glob.
type PathScopedRule struct {
	ID         uuid.UUID            `json:"id"`
	TenantID   string               `json:"tenantId"`
	Slug       string               `json:"slug"`
	Scope      PathScopedRuleScope  `json:"scope"`
	// PathGlob uses simple glob syntax: "*" wildcards, "**" recursive.
	// Examples: "**/*.go", "tools/execute_sql/**", "agent/researcher/*".
	PathGlob   string               `json:"pathGlob"`
	Content    string               `json:"content"`
	Priority   int                  `json:"priority"`
	Enabled    bool                 `json:"enabled"`
	CreatedAt  time.Time            `json:"createdAt"`
}

// Sentinels.
var (
	ErrPathScopedRuleInvalidScope = errors.New("path scoped rule: invalid scope")
	ErrPathScopedRuleSlugEmpty    = errors.New("path scoped rule: slug required")
	ErrPathScopedRuleGlobEmpty    = errors.New("path scoped rule: path_glob required")
	ErrPathScopedRuleContentEmpty = errors.New("path scoped rule: content required")
	ErrPathScopedRuleTenantEmpty  = errors.New("path scoped rule: tenant_id required")
	ErrPathScopedRuleNotFound     = errors.New("path scoped rule: not found")
	ErrPathScopedRuleDuplicate    = errors.New("path scoped rule: slug already registered for tenant")
)

// matchPathGlob returns true if `path` matches the `glob` pattern.
// Supports "*" (matches anything within a path segment, no /),
// "**" (recursive — matches anything including /), and "?" (single char).
//
// Conversion rules:
//   "**" → ".*"      (any chars including /)
//   "*"  → "[^/]*"   (any chars within a segment)
//   "?"  → "[^/]"    (single char)
//   regex metachars escaped.
func matchPathGlob(glob, p string) bool {
	if glob == "" {
		return false
	}
	glob = strings.TrimPrefix(glob, "/")
	p = strings.TrimPrefix(p, "/")
	pattern := globToRegex(glob)
	re, err := regexp.Compile("^" + pattern + "$")
	if err != nil {
		return false
	}
	return re.MatchString(p)
}

// globToRegex converts a glob pattern to a regex pattern.
func globToRegex(glob string) string {
	var b strings.Builder
	i := 0
	for i < len(glob) {
		c := glob[i]
		switch c {
		case '*':
			if i+1 < len(glob) && glob[i+1] == '*' {
				// "**" — recursive. If followed by /, eat the slash and
				// emit ".*?" so "**/foo" matches "foo" too (zero segments).
				i += 2
				if i < len(glob) && glob[i] == '/' {
					i++
					b.WriteString("(?:.*/)?")
				} else {
					b.WriteString(".*")
				}
			} else {
				b.WriteString("[^/]*")
				i++
			}
		case '?':
			b.WriteString("[^/]")
			i++
		case '.', '+', '(', ')', '|', '^', '$', '{', '}', '[', ']', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
			i++
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}

// specificityScore returns higher value for more concrete globs:
//   - exact path: 1000
//   - path with single * wildcards: 500 - count(*)*10
//   - path with ** wildcards: 100 - count(**)*5
//   - "*" or "**" alone: 1
func specificityScore(glob string) int {
	if glob == "*" || glob == "**" {
		return 1
	}
	if !strings.ContainsAny(glob, "*") {
		return 1000
	}
	doubleStarCount := strings.Count(glob, "**")
	singleStarCount := strings.Count(glob, "*") - doubleStarCount*2
	if doubleStarCount > 0 {
		return 100 - doubleStarCount*5
	}
	return 500 - singleStarCount*10
}

// PathScopedRuleRegistry is the persistence interface.
type PathScopedRuleRegistry interface {
	Register(ctx context.Context, r PathScopedRule) (PathScopedRule, error)
	Find(ctx context.Context, tenantID, slug string) (PathScopedRule, error)
	List(ctx context.Context, tenantID string) ([]PathScopedRule, error)
	ListByScope(ctx context.Context, tenantID string, scope PathScopedRuleScope) ([]PathScopedRule, error)
	Match(ctx context.Context, tenantID, queryPath string) ([]PathScopedRule, error)
	Delete(ctx context.Context, tenantID, slug string) error
}

// validateRule checks structural invariants.
func validateRule(r PathScopedRule) error {
	if strings.TrimSpace(r.TenantID) == "" {
		return ErrPathScopedRuleTenantEmpty
	}
	if strings.TrimSpace(r.Slug) == "" {
		return ErrPathScopedRuleSlugEmpty
	}
	if !IsValidPathScopedRuleScope(r.Scope) {
		return fmt.Errorf("%w: %q", ErrPathScopedRuleInvalidScope, r.Scope)
	}
	if strings.TrimSpace(r.PathGlob) == "" {
		return ErrPathScopedRuleGlobEmpty
	}
	if strings.TrimSpace(r.Content) == "" {
		return ErrPathScopedRuleContentEmpty
	}
	return nil
}

// --- InMemoryPathScopedRuleRegistry ---

type pathRuleKey struct {
	tenant string
	slug   string
}

type InMemoryPathScopedRuleRegistry struct {
	mu    sync.Mutex
	rules map[pathRuleKey]PathScopedRule
	now   func() time.Time
}

// NewInMemoryPathScopedRuleRegistry returns a concurrent-safe registry.
func NewInMemoryPathScopedRuleRegistry() *InMemoryPathScopedRuleRegistry {
	return &InMemoryPathScopedRuleRegistry{
		rules: map[pathRuleKey]PathScopedRule{},
		now:   time.Now,
	}
}

// SetClock allows tests to inject a deterministic clock.
func (r *InMemoryPathScopedRuleRegistry) SetClock(clock func() time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.now = clock
}

// Register persists a rule. Rejects duplicates per tenant.
func (r *InMemoryPathScopedRuleRegistry) Register(ctx context.Context, rule PathScopedRule) (PathScopedRule, error) {
	if err := ctx.Err(); err != nil {
		return PathScopedRule{}, err
	}
	if err := validateRule(rule); err != nil {
		return PathScopedRule{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := pathRuleKey{rule.TenantID, rule.Slug}
	if _, exists := r.rules[key]; exists {
		return PathScopedRule{}, fmt.Errorf("%w: %q", ErrPathScopedRuleDuplicate, rule.Slug)
	}
	if rule.ID == uuid.Nil {
		rule.ID = uuid.New()
	}
	if rule.CreatedAt.IsZero() {
		rule.CreatedAt = r.now()
	}
	r.rules[key] = rule
	return rule, nil
}

// Find returns one rule by tenant + slug.
func (r *InMemoryPathScopedRuleRegistry) Find(ctx context.Context, tenantID, slug string) (PathScopedRule, error) {
	if err := ctx.Err(); err != nil {
		return PathScopedRule{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	rule, ok := r.rules[pathRuleKey{tenantID, slug}]
	if !ok {
		return PathScopedRule{}, ErrPathScopedRuleNotFound
	}
	return rule, nil
}

// List returns all rules for tenant, sorted by slug.
func (r *InMemoryPathScopedRuleRegistry) List(ctx context.Context, tenantID string) ([]PathScopedRule, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []PathScopedRule{}
	for k, rule := range r.rules {
		if k.tenant == tenantID {
			out = append(out, rule)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out, nil
}

// ListByScope filters by scope.
func (r *InMemoryPathScopedRuleRegistry) ListByScope(ctx context.Context, tenantID string, scope PathScopedRuleScope) ([]PathScopedRule, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	all, err := r.List(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := []PathScopedRule{}
	for _, rule := range all {
		if rule.Scope == scope {
			out = append(out, rule)
		}
	}
	return out, nil
}

// Match returns rules whose PathGlob matches queryPath, sorted by
// specificity descending (more concrete first), tie-break by Priority
// descending then Slug.
//
// Disabled rules are excluded.
func (r *InMemoryPathScopedRuleRegistry) Match(ctx context.Context, tenantID, queryPath string) ([]PathScopedRule, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(queryPath) == "" {
		return nil, errors.New("path scoped rule: query_path required")
	}
	all, err := r.List(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	matched := []PathScopedRule{}
	for _, rule := range all {
		if !rule.Enabled {
			continue
		}
		if matchPathGlob(rule.PathGlob, queryPath) {
			matched = append(matched, rule)
		}
	}
	sort.SliceStable(matched, func(i, j int) bool {
		si := specificityScore(matched[i].PathGlob)
		sj := specificityScore(matched[j].PathGlob)
		if si != sj {
			return si > sj
		}
		if matched[i].Priority != matched[j].Priority {
			return matched[i].Priority > matched[j].Priority
		}
		return matched[i].Slug < matched[j].Slug
	})
	return matched, nil
}

// Delete removes a rule.
func (r *InMemoryPathScopedRuleRegistry) Delete(ctx context.Context, tenantID, slug string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := pathRuleKey{tenantID, slug}
	if _, ok := r.rules[key]; !ok {
		return ErrPathScopedRuleNotFound
	}
	delete(r.rules, key)
	return nil
}
