package core

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// CORE-SEED-001 — Shared platform catalog contract (ah_core).
//
// PDF arXiv:2604.14228v1 §6.1 (Plugin manifest: 10 component types) +
// §8.x (subagents/policies). AgentHub's ah_core schema is the
// platform-shared catalog of default templates: a fresh tenant needs no
// custom registration to operate, because cross-tenant defaults live in
// ah_core (subagent roles, policy templates, lane configurations, fork
// strategies, etc).
//
// This feature formalizes the *contract* every seed category must
// satisfy so the seed-loop ecosystem (59+ categories today) is
// self-describing:
//   - one bounded PlatformCatalogKind per category family,
//   - one PlatformCatalogManifest per concrete table (slug + migration
//     number + loader package + expected row count),
//   - PlatformCatalogRegistry: thread-safe in-memory registry that
//     admin tooling, /ah-status, and onboarding flows consult.
//
// Distinct from neighbouring abstractions:
//   - Individual table loaders (e.g., CoreBuiltinSubagentDefaultTemplateLoader)
//     load *rows* from one catalog. CORE-SEED-001 catalogs the catalogs.
//   - SUB-002 BuiltinSubagentRegistry stores in-memory subagent
//     descriptors at runtime; this stores manifest metadata about
//     persisted catalogs.

// PlatformCatalogKind bounded enum classifies what kind of catalog a
// table is. Kinds map to families of cross-tenant defaults.
type PlatformCatalogKind string

const (
	PlatformCatalogKindSubagentRoster      PlatformCatalogKind = "subagent_roster"
	PlatformCatalogKindAgentDefinition     PlatformCatalogKind = "agent_definition"
	PlatformCatalogKindToolsetPolicy       PlatformCatalogKind = "toolset_policy"
	PlatformCatalogKindInheritanceMode     PlatformCatalogKind = "inheritance_mode"
	PlatformCatalogKindSummaryShape        PlatformCatalogKind = "summary_shape"
	PlatformCatalogKindForkStrategy        PlatformCatalogKind = "fork_strategy"
	PlatformCatalogKindBackgroundLane      PlatformCatalogKind = "background_lane"
	PlatformCatalogKindContextPolicy       PlatformCatalogKind = "context_policy"
	PlatformCatalogKindPermissionPolicy    PlatformCatalogKind = "permission_policy"
	PlatformCatalogKindExtensionDescriptor PlatformCatalogKind = "extension_descriptor"
	PlatformCatalogKindOperationalTemplate PlatformCatalogKind = "operational_template"
)

var allPlatformCatalogKinds = []PlatformCatalogKind{
	PlatformCatalogKindSubagentRoster,
	PlatformCatalogKindAgentDefinition,
	PlatformCatalogKindToolsetPolicy,
	PlatformCatalogKindInheritanceMode,
	PlatformCatalogKindSummaryShape,
	PlatformCatalogKindForkStrategy,
	PlatformCatalogKindBackgroundLane,
	PlatformCatalogKindContextPolicy,
	PlatformCatalogKindPermissionPolicy,
	PlatformCatalogKindExtensionDescriptor,
	PlatformCatalogKindOperationalTemplate,
}

// IsValidPlatformCatalogKind returns true for the bounded set.
func IsValidPlatformCatalogKind(k PlatformCatalogKind) bool {
	for _, v := range allPlatformCatalogKinds {
		if k == v {
			return true
		}
	}
	return false
}

// AllPlatformCatalogKinds returns a defensive copy.
func AllPlatformCatalogKinds() []PlatformCatalogKind {
	out := make([]PlatformCatalogKind, len(allPlatformCatalogKinds))
	copy(out, allPlatformCatalogKinds)
	return out
}

// PlatformCatalogManifest is the descriptor for one persisted catalog
// table living inside ah_core.
type PlatformCatalogManifest struct {
	Slug                 string // kebab-case; matches the suffix of table name
	Kind                 PlatformCatalogKind
	MigrationNumber      int    // e.g., 62 for 000062_...
	LoaderPackage        string // e.g., "core" for internal/domain/core
	ExpectedRowCount     int    // seeded row count
	RequiresAdminApproval bool  // tenant cannot opt-in without admin
	IsTenantShared       bool   // true for ah_core defaults; false would be tenant-overridable
}

var platformCatalogSlugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9_]*[a-z0-9]$`)
var platformCatalogPkgRE = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// Validate enforces invariants. Cross-feature checks (e.g., migration
// file actually exists) live in admin tooling — Validate is pure.
func (m PlatformCatalogManifest) Validate() error {
	if !platformCatalogSlugRE.MatchString(m.Slug) {
		return fmt.Errorf("%w: %q must be snake-or-kebab (lower) with no edge separators",
			ErrPlatformCatalogBadSlug, m.Slug)
	}
	if !IsValidPlatformCatalogKind(m.Kind) {
		return fmt.Errorf("%w: %q", ErrPlatformCatalogBadKind, m.Kind)
	}
	if m.MigrationNumber <= 0 {
		return fmt.Errorf("%w: got %d", ErrPlatformCatalogBadMigration, m.MigrationNumber)
	}
	if !platformCatalogPkgRE.MatchString(m.LoaderPackage) {
		return fmt.Errorf("%w: %q", ErrPlatformCatalogBadPackage, m.LoaderPackage)
	}
	if strings.TrimSpace(m.LoaderPackage) == "" {
		return ErrPlatformCatalogBadPackage
	}
	if m.ExpectedRowCount <= 0 {
		return fmt.Errorf("%w: must be > 0, got %d",
			ErrPlatformCatalogBadRowCount, m.ExpectedRowCount)
	}
	return nil
}

// PlatformCatalogRegistry is a thread-safe in-memory store of catalog
// manifests. Initialized at boot from a hard-coded list and consulted
// by admin tooling.
type PlatformCatalogRegistry struct {
	mu        sync.RWMutex
	manifests map[string]PlatformCatalogManifest
}

// NewPlatformCatalogRegistry creates an empty registry.
func NewPlatformCatalogRegistry() *PlatformCatalogRegistry {
	return &PlatformCatalogRegistry{
		manifests: map[string]PlatformCatalogManifest{},
	}
}

// Register adds a manifest. Rejects bad manifests and duplicate slugs.
func (r *PlatformCatalogRegistry) Register(m PlatformCatalogManifest) error {
	if err := m.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.manifests[m.Slug]; exists {
		return fmt.Errorf("%w: %q", ErrPlatformCatalogDuplicateSlug, m.Slug)
	}
	for _, existing := range r.manifests {
		if existing.MigrationNumber == m.MigrationNumber {
			return fmt.Errorf("%w: migration %d already used by %q",
				ErrPlatformCatalogDuplicateMigration, m.MigrationNumber, existing.Slug)
		}
	}
	r.manifests[m.Slug] = m
	return nil
}

// Lookup returns one manifest by slug.
func (r *PlatformCatalogRegistry) Lookup(slug string) (PlatformCatalogManifest, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.manifests[slug]
	return m, ok
}

// ListAll returns all manifests sorted by slug.
func (r *PlatformCatalogRegistry) ListAll() []PlatformCatalogManifest {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]PlatformCatalogManifest, 0, len(r.manifests))
	for _, m := range r.manifests {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out
}

// ListByKind returns manifests of a given kind sorted by slug.
func (r *PlatformCatalogRegistry) ListByKind(kind PlatformCatalogKind) []PlatformCatalogManifest {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []PlatformCatalogManifest{}
	for _, m := range r.manifests {
		if m.Kind == kind {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out
}

// TotalExpectedRowCount sums ExpectedRowCount across all manifests —
// useful for /ah-status reports ("ah_core ships N rows across M catalogs").
func (r *PlatformCatalogRegistry) TotalExpectedRowCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	total := 0
	for _, m := range r.manifests {
		total += m.ExpectedRowCount
	}
	return total
}

// Size returns the registry count.
func (r *PlatformCatalogRegistry) Size() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.manifests)
}

// Sentinel errors.
var (
	ErrPlatformCatalogBadSlug            = errors.New("platform catalog: slug must be snake-or-kebab (lower) with no edge separators")
	ErrPlatformCatalogBadKind            = errors.New("platform catalog: invalid kind")
	ErrPlatformCatalogBadMigration       = errors.New("platform catalog: migration number must be > 0")
	ErrPlatformCatalogBadPackage         = errors.New("platform catalog: loader package must be lower-snake identifier")
	ErrPlatformCatalogBadRowCount        = errors.New("platform catalog: expected row count must be > 0")
	ErrPlatformCatalogDuplicateSlug      = errors.New("platform catalog: duplicate slug in registry")
	ErrPlatformCatalogDuplicateMigration = errors.New("platform catalog: duplicate migration number across catalogs")
)
