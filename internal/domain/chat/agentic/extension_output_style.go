package agentic

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// EXT-009 — Output styles.
//
// PDF arXiv:2604.14228v1 §6.5 (extensions can contribute output styles
// that re-render LLM output for specific consumer audiences; runtime
// resolves the active style via a cascade — explicit request beats
// agent default beats tenant default beats platform default).
//
// Distinct from existing AgentHub plumbing:
//   - ah_core.output_style table (CoreOutputStyleLoader) = PLATFORM
//     CATALOG of seed styles (8 baseline styles like conversational/
//     technical/json_only).
//   - extension_output_style.go (this file) = EXTENSION-CONTRIBUTED
//     style bindings + cascade resolver. Lets vendors ship custom
//     output formats and lets admin override per-tenant or per-agent.

// OutputStyleFormat bounded enum identifies the output format family.
type OutputStyleFormat string

const (
	OutputStyleFormatMarkdown OutputStyleFormat = "markdown"
	OutputStyleFormatJSON     OutputStyleFormat = "json"
	OutputStyleFormatPlain    OutputStyleFormat = "plain"
	// OutputStyleFormatHTMLSanitized — HTML output that the renderer
	// must sanitize (XSS guard at boundary).
	OutputStyleFormatHTMLSanitized OutputStyleFormat = "html_sanitized"
)

var allOutputStyleFormats = []OutputStyleFormat{
	OutputStyleFormatMarkdown, OutputStyleFormatJSON,
	OutputStyleFormatPlain, OutputStyleFormatHTMLSanitized,
}

// IsValidOutputStyleFormat returns true for the bounded set.
func IsValidOutputStyleFormat(f OutputStyleFormat) bool {
	for _, v := range allOutputStyleFormats {
		if f == v {
			return true
		}
	}
	return false
}

// AllOutputStyleFormats returns a copy.
func AllOutputStyleFormats() []OutputStyleFormat {
	out := make([]OutputStyleFormat, len(allOutputStyleFormats))
	copy(out, allOutputStyleFormats)
	return out
}

// OutputStyleScope bounded enum identifies the binding's authority tier.
// Resolver cascade: explicit (highest) → agent → tenant → platform (lowest).
type OutputStyleScope string

const (
	OutputStyleScopeExplicit OutputStyleScope = "explicit" // request-level override
	OutputStyleScopeAgent    OutputStyleScope = "agent"    // per-agent default
	OutputStyleScopeTenant   OutputStyleScope = "tenant"   // tenant-wide default
	OutputStyleScopePlatform OutputStyleScope = "platform" // ah_core seed default
)

var allOutputStyleScopes = []OutputStyleScope{
	OutputStyleScopeExplicit, OutputStyleScopeAgent,
	OutputStyleScopeTenant, OutputStyleScopePlatform,
}

// scopeRank returns the cascade priority (lower = higher precedence).
func outputStyleScopeRank(s OutputStyleScope) int {
	for i, v := range allOutputStyleScopes {
		if v == s {
			return i
		}
	}
	return len(allOutputStyleScopes)
}

// IsValidOutputStyleScope returns true for the bounded set.
func IsValidOutputStyleScope(s OutputStyleScope) bool {
	for _, v := range allOutputStyleScopes {
		if s == v {
			return true
		}
	}
	return false
}

// AllOutputStyleScopes returns a copy (cascade order: explicit first).
func AllOutputStyleScopes() []OutputStyleScope {
	out := make([]OutputStyleScope, len(allOutputStyleScopes))
	copy(out, allOutputStyleScopes)
	return out
}

// ExtensionOutputStyleBinding records that an extension or admin
// associated a style with a (tenant, scope, scope_id) tuple.
type ExtensionOutputStyleBinding struct {
	ID            uuid.UUID         `json:"id"`
	TenantID      string            `json:"tenantId"`
	StyleSlug     string            `json:"styleSlug"`
	Format        OutputStyleFormat `json:"format"`
	Scope         OutputStyleScope  `json:"scope"`
	// ScopeID is the bound subject (agent_id for agent scope, "" for
	// tenant/platform scope, request_id for explicit scope).
	ScopeID         string    `json:"scopeId,omitempty"`
	SourceExtension string    `json:"sourceExtension,omitempty"`
	Priority        int       `json:"priority"`
	Enabled         bool      `json:"enabled"`
	CreatedAt       time.Time `json:"createdAt"`
}

// ResolutionContext bundles the (tenant + agent + explicit override)
// triple the resolver uses to pick the active style.
type ResolutionContext struct {
	TenantID         string
	AgentID          string
	ExplicitRequestID string // empty unless caller forced a specific request-bound binding
}

// Sentinels.
var (
	ErrExtOutputStyleInvalidFormat   = errors.New("extension output style: invalid format")
	ErrExtOutputStyleInvalidScope    = errors.New("extension output style: invalid scope")
	ErrExtOutputStyleTenantRequired  = errors.New("extension output style: tenant_id required")
	ErrExtOutputStyleSlugRequired    = errors.New("extension output style: style_slug required")
	ErrExtOutputStyleScopeIDRequired = errors.New("extension output style: scope_id required for agent/explicit scopes")
	ErrExtOutputStyleNotFound        = errors.New("extension output style: not found")
	ErrExtOutputStyleNoMatch         = errors.New("extension output style: no binding matches resolution context")
	ErrExtOutputStyleDuplicate       = errors.New("extension output style: binding already exists for (tenant, scope, scope_id, style)")
)

// validateBinding checks structural invariants.
func validateBinding(b ExtensionOutputStyleBinding) error {
	if strings.TrimSpace(b.TenantID) == "" {
		return ErrExtOutputStyleTenantRequired
	}
	if strings.TrimSpace(b.StyleSlug) == "" {
		return ErrExtOutputStyleSlugRequired
	}
	if !IsValidOutputStyleFormat(b.Format) {
		return fmt.Errorf("%w: %q", ErrExtOutputStyleInvalidFormat, b.Format)
	}
	if !IsValidOutputStyleScope(b.Scope) {
		return fmt.Errorf("%w: %q", ErrExtOutputStyleInvalidScope, b.Scope)
	}
	// Agent + explicit scopes require a scope_id; tenant + platform may omit.
	if (b.Scope == OutputStyleScopeAgent || b.Scope == OutputStyleScopeExplicit) &&
		strings.TrimSpace(b.ScopeID) == "" {
		return ErrExtOutputStyleScopeIDRequired
	}
	return nil
}

// ExtensionOutputStyleRegistry is the persistence + resolver interface.
type ExtensionOutputStyleRegistry interface {
	Bind(ctx context.Context, b ExtensionOutputStyleBinding) (ExtensionOutputStyleBinding, error)
	Find(ctx context.Context, id uuid.UUID) (ExtensionOutputStyleBinding, error)
	Resolve(ctx context.Context, rctx ResolutionContext) (ExtensionOutputStyleBinding, error)
	ListByTenant(ctx context.Context, tenantID string) ([]ExtensionOutputStyleBinding, error)
	ListByScope(ctx context.Context, tenantID string, scope OutputStyleScope) ([]ExtensionOutputStyleBinding, error)
	Disable(ctx context.Context, id uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
	ClearExtension(ctx context.Context, tenantID, extensionSlug string) (int, error)
}

// --- InMemoryExtensionOutputStyleRegistry ---

type InMemoryExtensionOutputStyleRegistry struct {
	mu       sync.Mutex
	bindings map[uuid.UUID]ExtensionOutputStyleBinding
	now      func() time.Time
}

// NewInMemoryExtensionOutputStyleRegistry returns a concurrent-safe registry.
func NewInMemoryExtensionOutputStyleRegistry() *InMemoryExtensionOutputStyleRegistry {
	return &InMemoryExtensionOutputStyleRegistry{
		bindings: map[uuid.UUID]ExtensionOutputStyleBinding{},
		now:      time.Now,
	}
}

// SetClock allows tests to inject a deterministic clock.
func (r *InMemoryExtensionOutputStyleRegistry) SetClock(clock func() time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.now = clock
}

// Bind records a new binding. Rejects duplicate (tenant, scope, scope_id, style_slug).
func (r *InMemoryExtensionOutputStyleRegistry) Bind(ctx context.Context, b ExtensionOutputStyleBinding) (ExtensionOutputStyleBinding, error) {
	if err := ctx.Err(); err != nil {
		return ExtensionOutputStyleBinding{}, err
	}
	if err := validateBinding(b); err != nil {
		return ExtensionOutputStyleBinding{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.bindings {
		if existing.TenantID == b.TenantID &&
			existing.Scope == b.Scope &&
			existing.ScopeID == b.ScopeID &&
			existing.StyleSlug == b.StyleSlug {
			return ExtensionOutputStyleBinding{}, fmt.Errorf("%w: %s/%s/%s/%s",
				ErrExtOutputStyleDuplicate, b.TenantID, b.Scope, b.ScopeID, b.StyleSlug)
		}
	}
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	if b.CreatedAt.IsZero() {
		b.CreatedAt = r.now()
	}
	r.bindings[b.ID] = b
	return b, nil
}

// Find returns one binding by ID.
func (r *InMemoryExtensionOutputStyleRegistry) Find(ctx context.Context, id uuid.UUID) (ExtensionOutputStyleBinding, error) {
	if err := ctx.Err(); err != nil {
		return ExtensionOutputStyleBinding{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.bindings[id]
	if !ok {
		return ExtensionOutputStyleBinding{}, ErrExtOutputStyleNotFound
	}
	return b, nil
}

// Resolve picks the active binding for a context using the cascade:
// explicit > agent > tenant > platform. Within a scope, higher Priority
// wins; tie-break by newest CreatedAt then by ID.
//
// Disabled bindings are excluded.
func (r *InMemoryExtensionOutputStyleRegistry) Resolve(ctx context.Context, rctx ResolutionContext) (ExtensionOutputStyleBinding, error) {
	if err := ctx.Err(); err != nil {
		return ExtensionOutputStyleBinding{}, err
	}
	if strings.TrimSpace(rctx.TenantID) == "" {
		return ExtensionOutputStyleBinding{}, ErrExtOutputStyleTenantRequired
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	candidates := []ExtensionOutputStyleBinding{}
	for _, b := range r.bindings {
		if !b.Enabled || b.TenantID != rctx.TenantID {
			continue
		}
		match := false
		switch b.Scope {
		case OutputStyleScopeExplicit:
			match = b.ScopeID != "" && b.ScopeID == rctx.ExplicitRequestID
		case OutputStyleScopeAgent:
			match = b.ScopeID != "" && b.ScopeID == rctx.AgentID
		case OutputStyleScopeTenant:
			match = true // any tenant binding applies
		case OutputStyleScopePlatform:
			match = true // platform fallback always applicable
		}
		if match {
			candidates = append(candidates, b)
		}
	}
	if len(candidates) == 0 {
		return ExtensionOutputStyleBinding{}, ErrExtOutputStyleNoMatch
	}

	// Sort: scope rank asc (explicit=0 first), then priority desc, then
	// created_at desc, then ID asc (stable ordering).
	sort.SliceStable(candidates, func(i, j int) bool {
		ri, rj := outputStyleScopeRank(candidates[i].Scope), outputStyleScopeRank(candidates[j].Scope)
		if ri != rj {
			return ri < rj
		}
		if candidates[i].Priority != candidates[j].Priority {
			return candidates[i].Priority > candidates[j].Priority
		}
		if !candidates[i].CreatedAt.Equal(candidates[j].CreatedAt) {
			return candidates[i].CreatedAt.After(candidates[j].CreatedAt)
		}
		return candidates[i].ID.String() < candidates[j].ID.String()
	})
	return candidates[0], nil
}

// ListByTenant returns all bindings for a tenant, sorted by scope cascade
// then style_slug.
func (r *InMemoryExtensionOutputStyleRegistry) ListByTenant(ctx context.Context, tenantID string) ([]ExtensionOutputStyleBinding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []ExtensionOutputStyleBinding{}
	for _, b := range r.bindings {
		if b.TenantID == tenantID {
			out = append(out, b)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		ri, rj := outputStyleScopeRank(out[i].Scope), outputStyleScopeRank(out[j].Scope)
		if ri != rj {
			return ri < rj
		}
		return out[i].StyleSlug < out[j].StyleSlug
	})
	return out, nil
}

// ListByScope filters by scope.
func (r *InMemoryExtensionOutputStyleRegistry) ListByScope(ctx context.Context, tenantID string, scope OutputStyleScope) ([]ExtensionOutputStyleBinding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !IsValidOutputStyleScope(scope) {
		return nil, fmt.Errorf("%w: %q", ErrExtOutputStyleInvalidScope, scope)
	}
	all, err := r.ListByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := []ExtensionOutputStyleBinding{}
	for _, b := range all {
		if b.Scope == scope {
			out = append(out, b)
		}
	}
	return out, nil
}

// Disable marks a binding disabled (idempotent).
func (r *InMemoryExtensionOutputStyleRegistry) Disable(ctx context.Context, id uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.bindings[id]
	if !ok {
		return ErrExtOutputStyleNotFound
	}
	b.Enabled = false
	r.bindings[id] = b
	return nil
}

// Delete removes a binding.
func (r *InMemoryExtensionOutputStyleRegistry) Delete(ctx context.Context, id uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.bindings[id]; !ok {
		return ErrExtOutputStyleNotFound
	}
	delete(r.bindings, id)
	return nil
}

// ClearExtension removes all bindings sourced from extensionSlug
// (uninstall flow). Returns count removed.
func (r *InMemoryExtensionOutputStyleRegistry) ClearExtension(ctx context.Context, tenantID, extensionSlug string) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	removed := 0
	for id, b := range r.bindings {
		if b.TenantID == tenantID && b.SourceExtension == extensionSlug {
			delete(r.bindings, id)
			removed++
		}
	}
	return removed, nil
}
