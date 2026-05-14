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

// EXT-005 — Plugin component routing.
//
// PDF arXiv:2604.14228v1 §6.2 (when the agentic runtime needs a
// component — skill, tool, hook, etc — it asks the router for "which
// installed extension provides (kind, slug)?"; router returns a route
// pointing at the extension + entry path).
//
// Distinct from existing AgentHub plumbing:
//   - EXT-001 ExtensionRegistry = INSTALLED extensions per tenant.
//   - EXT-004 PluginManifest = MANIFEST DOCUMENT spec.
//   - component_router.go (this file) = LOOKUP/ROUTING layer that joins
//     (kind, slug) request → installed extension route. Includes
//     conflict policy (when 2 extensions claim the same component slug),
//     explicit pins (admin overrides), and an audit trace for every
//     resolution.

// RoutingConflictPolicy bounded enum identifies how to resolve when 2+
// extensions provide the same (kind, slug).
type RoutingConflictPolicy string

const (
	// RoutingConflictFirstInstallWins — earliest InstalledAt wins.
	RoutingConflictFirstInstallWins RoutingConflictPolicy = "first_install_wins"
	// RoutingConflictLatestInstallWins — most recent InstalledAt wins.
	RoutingConflictLatestInstallWins RoutingConflictPolicy = "latest_install_wins"
	// RoutingConflictRequireExplicitPin — must have an explicit Pin
	// for the (kind, slug); else error.
	RoutingConflictRequireExplicitPin RoutingConflictPolicy = "require_explicit_pin"
	// RoutingConflictErrorOnConflict — abort with error when conflict
	// detected (forces admin intervention).
	RoutingConflictErrorOnConflict RoutingConflictPolicy = "error_on_conflict"
)

var allRoutingConflictPolicies = []RoutingConflictPolicy{
	RoutingConflictFirstInstallWins, RoutingConflictLatestInstallWins,
	RoutingConflictRequireExplicitPin, RoutingConflictErrorOnConflict,
}

// IsValidRoutingConflictPolicy returns true for the bounded set.
func IsValidRoutingConflictPolicy(p RoutingConflictPolicy) bool {
	for _, v := range allRoutingConflictPolicies {
		if p == v {
			return true
		}
	}
	return false
}

// AllRoutingConflictPolicies returns a copy.
func AllRoutingConflictPolicies() []RoutingConflictPolicy {
	out := make([]RoutingConflictPolicy, len(allRoutingConflictPolicies))
	copy(out, allRoutingConflictPolicies)
	return out
}

// ComponentRoute is one (kind, slug) → extension binding.
type ComponentRoute struct {
	Kind            ExtensionComponentKind `json:"kind"`
	Slug            string                 `json:"slug"`
	ExtensionSlug   string                 `json:"extensionSlug"`
	ExtensionVersion string                `json:"extensionVersion"`
	EntryPath       string                 `json:"entryPath"`
	InstalledAt     time.Time              `json:"installedAt"`
}

// ComponentPin is an explicit admin override that forces a specific
// extension to win for a (kind, slug) regardless of conflict policy.
type ComponentPin struct {
	TenantID      string                 `json:"tenantId"`
	Kind          ExtensionComponentKind `json:"kind"`
	Slug          string                 `json:"slug"`
	ExtensionSlug string                 `json:"extensionSlug"`
	PinnedAt      time.Time              `json:"pinnedAt"`
	Reason        string                 `json:"reason"`
}

// RoutingResolution is the result of a Resolve call (audit-friendly).
type RoutingResolution struct {
	Route             ComponentRoute        `json:"route"`
	ConflictDetected  bool                  `json:"conflictDetected,omitempty"`
	ConflictCandidates []ComponentRoute     `json:"conflictCandidates,omitempty"`
	ResolvedBy        string                `json:"resolvedBy"` // "pin" / "policy:<value>" / "single_match"
}

// Sentinels.
var (
	ErrRoutingInvalidPolicy        = errors.New("component router: invalid conflict policy")
	ErrRoutingTenantRequired       = errors.New("component router: tenant_id required")
	ErrRoutingNoMatch              = errors.New("component router: no extension provides this (kind, slug)")
	ErrRoutingConflictUnresolved   = errors.New("component router: conflict and policy=error_on_conflict")
	ErrRoutingPinRequired          = errors.New("component router: conflict and policy=require_explicit_pin (no pin found)")
	ErrRoutingInvalidComponentKind = errors.New("component router: invalid component kind")
	ErrRoutingPinInvalidExtension  = errors.New("component router: pin references extension not in candidates")
)

// ComponentRouter is the runtime resolver.
type ComponentRouter interface {
	IndexRoute(ctx context.Context, tenantID string, route ComponentRoute) error
	IndexRoutes(ctx context.Context, tenantID string, routes []ComponentRoute) error
	SetPin(ctx context.Context, pin ComponentPin) error
	RemovePin(ctx context.Context, tenantID string, kind ExtensionComponentKind, slug string) error
	Resolve(ctx context.Context, tenantID string, kind ExtensionComponentKind, slug string, policy RoutingConflictPolicy) (RoutingResolution, error)
	ListRoutes(ctx context.Context, tenantID string, kind ExtensionComponentKind) ([]ComponentRoute, error)
	ListPins(ctx context.Context, tenantID string) ([]ComponentPin, error)
	ClearTenant(ctx context.Context, tenantID string) error
}

// --- InMemoryComponentRouter ---

type routeKey struct {
	tenant string
	kind   ExtensionComponentKind
	slug   string
}

type pinKey routeKey

type InMemoryComponentRouter struct {
	mu     sync.Mutex
	// routes: per-(tenant, kind, slug) → list of candidate routes
	// (multiple = conflict).
	routes map[routeKey][]ComponentRoute
	// pins: per-(tenant, kind, slug) → admin pin.
	pins map[pinKey]ComponentPin
}

// NewInMemoryComponentRouter returns a concurrent-safe router.
func NewInMemoryComponentRouter() *InMemoryComponentRouter {
	return &InMemoryComponentRouter{
		routes: map[routeKey][]ComponentRoute{},
		pins:   map[pinKey]ComponentPin{},
	}
}

// IndexRoute registers one route. Multiple routes for same (tenant,
// kind, slug) become conflict candidates.
func (r *InMemoryComponentRouter) IndexRoute(ctx context.Context, tenantID string, route ComponentRoute) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(tenantID) == "" {
		return ErrRoutingTenantRequired
	}
	if !IsValidExtensionComponentKind(route.Kind) {
		return fmt.Errorf("%w: %q", ErrRoutingInvalidComponentKind, route.Kind)
	}
	if strings.TrimSpace(route.Slug) == "" {
		return errors.New("component router: slug required")
	}
	if strings.TrimSpace(route.ExtensionSlug) == "" {
		return errors.New("component router: extension_slug required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := routeKey{tenantID, route.Kind, route.Slug}
	// De-dup: if same extension already indexed, replace (newer entry).
	existing := r.routes[key]
	for i, e := range existing {
		if e.ExtensionSlug == route.ExtensionSlug {
			existing[i] = route
			r.routes[key] = existing
			return nil
		}
	}
	r.routes[key] = append(existing, route)
	return nil
}

// IndexRoutes is a batch convenience.
func (r *InMemoryComponentRouter) IndexRoutes(ctx context.Context, tenantID string, routes []ComponentRoute) error {
	for _, route := range routes {
		if err := r.IndexRoute(ctx, tenantID, route); err != nil {
			return fmt.Errorf("route %s/%s: %w", route.Kind, route.Slug, err)
		}
	}
	return nil
}

// SetPin records an admin pin.
func (r *InMemoryComponentRouter) SetPin(ctx context.Context, pin ComponentPin) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(pin.TenantID) == "" {
		return ErrRoutingTenantRequired
	}
	if !IsValidExtensionComponentKind(pin.Kind) {
		return fmt.Errorf("%w: %q", ErrRoutingInvalidComponentKind, pin.Kind)
	}
	if strings.TrimSpace(pin.Slug) == "" || strings.TrimSpace(pin.ExtensionSlug) == "" {
		return errors.New("component router: pin slug + extension_slug required")
	}
	if strings.TrimSpace(pin.Reason) == "" {
		return errors.New("component router: pin reason required for audit trail")
	}
	if pin.PinnedAt.IsZero() {
		pin.PinnedAt = time.Now()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pins[pinKey{pin.TenantID, pin.Kind, pin.Slug}] = pin
	return nil
}

// RemovePin clears an admin pin.
func (r *InMemoryComponentRouter) RemovePin(ctx context.Context, tenantID string, kind ExtensionComponentKind, slug string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := pinKey{tenantID, kind, slug}
	if _, ok := r.pins[key]; !ok {
		return errors.New("component router: pin not found")
	}
	delete(r.pins, key)
	return nil
}

// Resolve looks up the winning route for (kind, slug) under policy.
// Returns RoutingResolution with conflict trace for audit.
func (r *InMemoryComponentRouter) Resolve(ctx context.Context, tenantID string, kind ExtensionComponentKind, slug string, policy RoutingConflictPolicy) (RoutingResolution, error) {
	if err := ctx.Err(); err != nil {
		return RoutingResolution{}, err
	}
	if strings.TrimSpace(tenantID) == "" {
		return RoutingResolution{}, ErrRoutingTenantRequired
	}
	if !IsValidRoutingConflictPolicy(policy) {
		return RoutingResolution{}, fmt.Errorf("%w: %q", ErrRoutingInvalidPolicy, policy)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := routeKey{tenantID, kind, slug}
	candidates := r.routes[key]
	if len(candidates) == 0 {
		return RoutingResolution{}, fmt.Errorf("%w: %s/%s", ErrRoutingNoMatch, kind, slug)
	}

	// Single candidate — no conflict.
	if len(candidates) == 1 {
		return RoutingResolution{
			Route:      candidates[0],
			ResolvedBy: "single_match",
		}, nil
	}

	// Multi-candidate. First check for explicit pin.
	if pin, hasPin := r.pins[pinKey(key)]; hasPin {
		for _, c := range candidates {
			if c.ExtensionSlug == pin.ExtensionSlug {
				return RoutingResolution{
					Route:              c,
					ConflictDetected:   true,
					ConflictCandidates: append([]ComponentRoute{}, candidates...),
					ResolvedBy:         "pin",
				}, nil
			}
		}
		// Pin references an extension that's not among current candidates.
		return RoutingResolution{}, fmt.Errorf("%w: pin=%s candidates=%d",
			ErrRoutingPinInvalidExtension, pin.ExtensionSlug, len(candidates))
	}

	// No pin — apply policy.
	switch policy {
	case RoutingConflictErrorOnConflict:
		return RoutingResolution{}, fmt.Errorf("%w: %s/%s candidates=%d",
			ErrRoutingConflictUnresolved, kind, slug, len(candidates))

	case RoutingConflictRequireExplicitPin:
		return RoutingResolution{}, fmt.Errorf("%w: %s/%s",
			ErrRoutingPinRequired, kind, slug)

	case RoutingConflictFirstInstallWins:
		winner := candidates[0]
		for _, c := range candidates[1:] {
			if c.InstalledAt.Before(winner.InstalledAt) {
				winner = c
			}
		}
		return RoutingResolution{
			Route:              winner,
			ConflictDetected:   true,
			ConflictCandidates: append([]ComponentRoute{}, candidates...),
			ResolvedBy:         "policy:first_install_wins",
		}, nil

	case RoutingConflictLatestInstallWins:
		winner := candidates[0]
		for _, c := range candidates[1:] {
			if c.InstalledAt.After(winner.InstalledAt) {
				winner = c
			}
		}
		return RoutingResolution{
			Route:              winner,
			ConflictDetected:   true,
			ConflictCandidates: append([]ComponentRoute{}, candidates...),
			ResolvedBy:         "policy:latest_install_wins",
		}, nil
	}
	return RoutingResolution{}, fmt.Errorf("component router: unhandled policy %q", policy)
}

// ListRoutes returns all routes for a tenant + kind, sorted by slug.
func (r *InMemoryComponentRouter) ListRoutes(ctx context.Context, tenantID string, kind ExtensionComponentKind) ([]ComponentRoute, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []ComponentRoute{}
	for k, candidates := range r.routes {
		if k.tenant != tenantID || k.kind != kind {
			continue
		}
		out = append(out, candidates...)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Slug != out[j].Slug {
			return out[i].Slug < out[j].Slug
		}
		return out[i].ExtensionSlug < out[j].ExtensionSlug
	})
	return out, nil
}

// ListPins returns all pins for a tenant, sorted by (kind, slug).
func (r *InMemoryComponentRouter) ListPins(ctx context.Context, tenantID string) ([]ComponentPin, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []ComponentPin{}
	for k, p := range r.pins {
		if k.tenant == tenantID {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Slug < out[j].Slug
	})
	return out, nil
}

// ClearTenant removes all routes + pins for a tenant (uninstall flow).
func (r *InMemoryComponentRouter) ClearTenant(ctx context.Context, tenantID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for k := range r.routes {
		if k.tenant == tenantID {
			delete(r.routes, k)
		}
	}
	for k := range r.pins {
		if k.tenant == tenantID {
			delete(r.pins, k)
		}
	}
	return nil
}
