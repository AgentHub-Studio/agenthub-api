package agentic

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// SUB-002 — Built-in subagents.
//
// PDF arXiv:2604.14228v1 §8.2 (Subagents — built-in roster). The
// platform ships a curated set of subagents ready-to-use (researcher,
// coder, reviewer, etc) so tenants do not have to design every
// subagent from scratch. Each builtin declares:
//   - the role (bounded enum) for routing,
//   - default toolset policy (SUB-005 strategy reference),
//   - default permission inheritance (SUB-006 mode reference),
//   - default summary shape (SUB-010 shape reference),
//   - a system prompt template that the runner consumes.
//
// Distinct from existing AgentHub plumbing:
//   - SUB-001 (Agent tool) is the SPAWN primitive.
//   - SUB-003 (Custom agents) — tenants compose their own subagent
//     definitions; SUB-002 is the platform-shipped roster.
//   - SUB-005/006/010 are the per-spawn policies; SUB-002 references
//     them by name as defaults.

// BuiltinSubagentRole bounded enum identifies one of the curated roles
// the platform supports.
type BuiltinSubagentRole string

const (
	BuiltinSubagentResearcher BuiltinSubagentRole = "researcher"
	BuiltinSubagentCoder      BuiltinSubagentRole = "coder"
	BuiltinSubagentReviewer   BuiltinSubagentRole = "reviewer"
	BuiltinSubagentExplorer   BuiltinSubagentRole = "explorer"
	BuiltinSubagentPlanner    BuiltinSubagentRole = "planner"
	BuiltinSubagentCurator    BuiltinSubagentRole = "curator"
	BuiltinSubagentDocumenter BuiltinSubagentRole = "documenter"
)

var allBuiltinSubagentRoles = []BuiltinSubagentRole{
	BuiltinSubagentResearcher, BuiltinSubagentCoder,
	BuiltinSubagentReviewer, BuiltinSubagentExplorer,
	BuiltinSubagentPlanner, BuiltinSubagentCurator,
	BuiltinSubagentDocumenter,
}

// IsValidBuiltinSubagentRole returns true for the bounded set.
func IsValidBuiltinSubagentRole(r BuiltinSubagentRole) bool {
	for _, v := range allBuiltinSubagentRoles {
		if r == v {
			return true
		}
	}
	return false
}

// AllBuiltinSubagentRoles returns a defensive copy.
func AllBuiltinSubagentRoles() []BuiltinSubagentRole {
	out := make([]BuiltinSubagentRole, len(allBuiltinSubagentRoles))
	copy(out, allBuiltinSubagentRoles)
	return out
}

// BuiltinSubagentDescriptor describes one curated subagent. References
// to SUB-005/006/010 policies are by slug so the registry stays
// loosely coupled to those features' loaders.
type BuiltinSubagentDescriptor struct {
	Role                       BuiltinSubagentRole
	Slug                       string
	Name                       string
	Description                string
	SystemPromptTemplate       string
	DefaultToolsetPolicySlug   string // references SUB-005 SubagentToolsetPolicy template
	DefaultInheritanceModeSlug string // references SUB-006 PermissionInheritanceMode template
	DefaultSummaryShapeSlug    string // references SUB-010 SubagentReturnSummary template
}

var builtinSubagentSlugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

// Validate enforces invariants. References are NOT existence-checked
// here — the loader/admin tool validates against the live SUB-005/006/010
// catalog. Validate only ensures the field shape.
func (d BuiltinSubagentDescriptor) Validate() error {
	if !IsValidBuiltinSubagentRole(d.Role) {
		return fmt.Errorf("%w: %q", ErrBuiltinSubagentBadRole, d.Role)
	}
	if !builtinSubagentSlugRE.MatchString(d.Slug) {
		return fmt.Errorf("%w: %q must be kebab-case", ErrBuiltinSubagentBadSlug, d.Slug)
	}
	if strings.TrimSpace(d.Name) == "" {
		return ErrBuiltinSubagentEmptyName
	}
	if strings.TrimSpace(d.Description) == "" {
		return ErrBuiltinSubagentEmptyDescription
	}
	if strings.TrimSpace(d.SystemPromptTemplate) == "" {
		return ErrBuiltinSubagentEmptySystemPrompt
	}
	if strings.TrimSpace(d.DefaultToolsetPolicySlug) == "" {
		return ErrBuiltinSubagentEmptyToolsetSlug
	}
	if strings.TrimSpace(d.DefaultInheritanceModeSlug) == "" {
		return ErrBuiltinSubagentEmptyInheritanceSlug
	}
	if strings.TrimSpace(d.DefaultSummaryShapeSlug) == "" {
		return ErrBuiltinSubagentEmptySummarySlug
	}
	return nil
}

// BuiltinSubagentRegistry is an in-memory store of curated builtins.
// Thread-safe.
type BuiltinSubagentRegistry struct {
	mu    sync.RWMutex
	items map[string]BuiltinSubagentDescriptor
}

// NewBuiltinSubagentRegistry builds an empty registry.
func NewBuiltinSubagentRegistry() *BuiltinSubagentRegistry {
	return &BuiltinSubagentRegistry{items: map[string]BuiltinSubagentDescriptor{}}
}

// Register adds a descriptor. Rejects bad descriptors and duplicates.
func (r *BuiltinSubagentRegistry) Register(d BuiltinSubagentDescriptor) error {
	if err := d.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.items[d.Slug]; exists {
		return fmt.Errorf("%w: %q", ErrBuiltinSubagentDuplicateSlug, d.Slug)
	}
	r.items[d.Slug] = d
	return nil
}

// Lookup returns the descriptor by slug.
func (r *BuiltinSubagentRegistry) Lookup(slug string) (BuiltinSubagentDescriptor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.items[slug]
	return d, ok
}

// ListAll returns all descriptors sorted by slug.
func (r *BuiltinSubagentRegistry) ListAll() []BuiltinSubagentDescriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]BuiltinSubagentDescriptor, 0, len(r.items))
	for _, d := range r.items {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out
}

// ListByRole returns descriptors with the given role, sorted by slug.
func (r *BuiltinSubagentRegistry) ListByRole(role BuiltinSubagentRole) []BuiltinSubagentDescriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []BuiltinSubagentDescriptor{}
	for _, d := range r.items {
		if d.Role == role {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out
}

// Size returns the registry count.
func (r *BuiltinSubagentRegistry) Size() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.items)
}

// Sentinel errors.
var (
	ErrBuiltinSubagentBadRole              = errors.New("builtin subagent: invalid role")
	ErrBuiltinSubagentBadSlug              = errors.New("builtin subagent: slug must be kebab-case")
	ErrBuiltinSubagentEmptyName            = errors.New("builtin subagent: name required")
	ErrBuiltinSubagentEmptyDescription     = errors.New("builtin subagent: description required")
	ErrBuiltinSubagentEmptySystemPrompt    = errors.New("builtin subagent: system prompt template required")
	ErrBuiltinSubagentEmptyToolsetSlug     = errors.New("builtin subagent: default toolset policy slug required")
	ErrBuiltinSubagentEmptyInheritanceSlug = errors.New("builtin subagent: default inheritance mode slug required")
	ErrBuiltinSubagentEmptySummarySlug     = errors.New("builtin subagent: default summary shape slug required")
	ErrBuiltinSubagentDuplicateSlug        = errors.New("builtin subagent: duplicate slug in registry")
)
