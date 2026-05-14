package agentic

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// SUB-003 — Custom agents (tenant-side).
//
// PDF arXiv:2604.14228v1 §8.3. Tenants compose their own subagent
// definitions instead of being limited to the SUB-002 platform-curated
// roster. A custom agent is a recipe:
//   - slug + name + description for routing,
//   - system prompt template (the agent's persona/instructions),
//   - 3 policy refs (toolset = SUB-005, inheritance = SUB-006,
//     summary shape = SUB-010),
//   - optional derived_from_builtin_slug (clone-and-modify lineage to
//     a SUB-002 builtin),
//   - visibility (private | shared_tenant | marketplace_listing),
//   - owner slug (tenant) + version (semver).
//
// Distinct from neighbouring features:
//   - SUB-001 (Agent tool) is the spawn primitive — both builtin and
//     custom agents are invoked via it.
//   - SUB-002 (Built-in) is the platform-shipped curated roster.
//   - SUB-003 (this) is the tenant-composed analog. A custom agent may
//     reference a SUB-002 slug to denote "derived from" — the runner
//     does NOT auto-merge fields, the value is purely lineage metadata.

// CustomAgentVisibility bounded enum controls who may invoke the
// definition.
type CustomAgentVisibility string

const (
	CustomAgentVisibilityPrivate     CustomAgentVisibility = "private"
	CustomAgentVisibilityShared      CustomAgentVisibility = "shared_tenant"
	CustomAgentVisibilityMarketplace CustomAgentVisibility = "marketplace_listing"
)

var allCustomAgentVisibilities = []CustomAgentVisibility{
	CustomAgentVisibilityPrivate,
	CustomAgentVisibilityShared,
	CustomAgentVisibilityMarketplace,
}

// IsValidCustomAgentVisibility returns true for the bounded set.
func IsValidCustomAgentVisibility(v CustomAgentVisibility) bool {
	for _, c := range allCustomAgentVisibilities {
		if v == c {
			return true
		}
	}
	return false
}

// AllCustomAgentVisibilities returns a defensive copy.
func AllCustomAgentVisibilities() []CustomAgentVisibility {
	out := make([]CustomAgentVisibility, len(allCustomAgentVisibilities))
	copy(out, allCustomAgentVisibilities)
	return out
}

// CustomAgentDefinition is one tenant-composed subagent recipe. The
// Version field is optional but, when set, must be semver-shaped
// (X.Y.Z). DerivedFromBuiltinSlug is metadata only — runner does not
// auto-merge.
type CustomAgentDefinition struct {
	Slug                    string
	OwnerTenantSlug         string
	Name                    string
	Description             string
	SystemPromptTemplate    string
	ToolsetPolicySlug       string // SUB-005
	InheritanceModeSlug     string // SUB-006
	SummaryShapeSlug        string // SUB-010
	DerivedFromBuiltinSlug  string // optional SUB-002 lineage
	Visibility              CustomAgentVisibility
	Version                 string // optional semver "X.Y.Z"
	IsActive                bool
}

var customAgentSlugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)
var customAgentSemverRE = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// Validate enforces invariants. Cross-catalog refs are NOT
// existence-checked here — the admin/persistence layer validates
// against live SUB-002/005/006/010 catalogs.
func (d CustomAgentDefinition) Validate() error {
	if !customAgentSlugRE.MatchString(d.Slug) {
		return fmt.Errorf("%w: %q must be kebab-case", ErrCustomAgentBadSlug, d.Slug)
	}
	if !customAgentSlugRE.MatchString(d.OwnerTenantSlug) {
		return fmt.Errorf("%w: %q must be kebab-case", ErrCustomAgentBadOwner, d.OwnerTenantSlug)
	}
	if strings.TrimSpace(d.Name) == "" {
		return ErrCustomAgentEmptyName
	}
	if strings.TrimSpace(d.Description) == "" {
		return ErrCustomAgentEmptyDescription
	}
	if strings.TrimSpace(d.SystemPromptTemplate) == "" {
		return ErrCustomAgentEmptySystemPrompt
	}
	if strings.TrimSpace(d.ToolsetPolicySlug) == "" {
		return ErrCustomAgentEmptyToolsetSlug
	}
	if strings.TrimSpace(d.InheritanceModeSlug) == "" {
		return ErrCustomAgentEmptyInheritanceSlug
	}
	if strings.TrimSpace(d.SummaryShapeSlug) == "" {
		return ErrCustomAgentEmptySummarySlug
	}
	if !IsValidCustomAgentVisibility(d.Visibility) {
		return fmt.Errorf("%w: %q", ErrCustomAgentBadVisibility, d.Visibility)
	}
	if d.DerivedFromBuiltinSlug != "" && !customAgentSlugRE.MatchString(d.DerivedFromBuiltinSlug) {
		return fmt.Errorf("%w: %q must be kebab-case",
			ErrCustomAgentBadDerivedSlug, d.DerivedFromBuiltinSlug)
	}
	if d.Version != "" && !customAgentSemverRE.MatchString(d.Version) {
		return fmt.Errorf("%w: %q must be semver X.Y.Z",
			ErrCustomAgentBadVersion, d.Version)
	}
	return nil
}

// CustomAgentRegistry is an in-memory store of tenant-composed agent
// definitions. Thread-safe. Uniqueness key is (owner_slug, slug) — two
// tenants may share a slug without collision.
type CustomAgentRegistry struct {
	mu    sync.RWMutex
	items map[string]CustomAgentDefinition // key = owner + "/" + slug
}

// NewCustomAgentRegistry creates an empty registry.
func NewCustomAgentRegistry() *CustomAgentRegistry {
	return &CustomAgentRegistry{items: map[string]CustomAgentDefinition{}}
}

func registryKey(owner, slug string) string {
	return owner + "/" + slug
}

// Register adds a definition. Rejects bad descriptors and (owner,
// slug) duplicates.
func (r *CustomAgentRegistry) Register(d CustomAgentDefinition) error {
	if err := d.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	k := registryKey(d.OwnerTenantSlug, d.Slug)
	if _, exists := r.items[k]; exists {
		return fmt.Errorf("%w: %q owned by %q",
			ErrCustomAgentDuplicate, d.Slug, d.OwnerTenantSlug)
	}
	r.items[k] = d
	return nil
}

// Lookup returns the definition by (owner, slug).
func (r *CustomAgentRegistry) Lookup(owner, slug string) (CustomAgentDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.items[registryKey(owner, slug)]
	return d, ok
}

// ListByOwner returns all definitions for a tenant, sorted by slug.
func (r *CustomAgentRegistry) ListByOwner(owner string) []CustomAgentDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []CustomAgentDefinition{}
	for _, d := range r.items {
		if d.OwnerTenantSlug == owner {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out
}

// ListByDerivedBuiltin returns all custom agents whose lineage points
// to a given SUB-002 builtin slug, sorted by owner then slug.
func (r *CustomAgentRegistry) ListByDerivedBuiltin(builtinSlug string) []CustomAgentDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []CustomAgentDefinition{}
	for _, d := range r.items {
		if d.DerivedFromBuiltinSlug == builtinSlug {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].OwnerTenantSlug != out[j].OwnerTenantSlug {
			return out[i].OwnerTenantSlug < out[j].OwnerTenantSlug
		}
		return out[i].Slug < out[j].Slug
	})
	return out
}

// ListByVisibility returns definitions matching a visibility,
// regardless of owner. Useful for marketplace listing queries.
func (r *CustomAgentRegistry) ListByVisibility(v CustomAgentVisibility) []CustomAgentDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []CustomAgentDefinition{}
	for _, d := range r.items {
		if d.Visibility == v {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].OwnerTenantSlug != out[j].OwnerTenantSlug {
			return out[i].OwnerTenantSlug < out[j].OwnerTenantSlug
		}
		return out[i].Slug < out[j].Slug
	})
	return out
}

// Size returns the registry count.
func (r *CustomAgentRegistry) Size() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.items)
}

// Sentinel errors.
var (
	ErrCustomAgentBadSlug              = errors.New("custom agent: slug must be kebab-case")
	ErrCustomAgentBadOwner             = errors.New("custom agent: owner tenant slug must be kebab-case")
	ErrCustomAgentEmptyName            = errors.New("custom agent: name required")
	ErrCustomAgentEmptyDescription     = errors.New("custom agent: description required")
	ErrCustomAgentEmptySystemPrompt    = errors.New("custom agent: system prompt template required")
	ErrCustomAgentEmptyToolsetSlug     = errors.New("custom agent: toolset policy slug required")
	ErrCustomAgentEmptyInheritanceSlug = errors.New("custom agent: inheritance mode slug required")
	ErrCustomAgentEmptySummarySlug     = errors.New("custom agent: summary shape slug required")
	ErrCustomAgentBadVisibility        = errors.New("custom agent: invalid visibility")
	ErrCustomAgentBadDerivedSlug       = errors.New("custom agent: derived-from builtin slug must be kebab-case")
	ErrCustomAgentBadVersion           = errors.New("custom agent: version must be semver X.Y.Z")
	ErrCustomAgentDuplicate            = errors.New("custom agent: duplicate (owner, slug) in registry")
)
