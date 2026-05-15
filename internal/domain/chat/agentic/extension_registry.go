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

// EXT-001 — Extension registry.
//
// PDF arXiv:2604.14228v1 §6 (Plugin manifest 10 component types) +
// §6.1 (extensions are versioned bundles installed per-tenant or global,
// each declaring which component slugs it provides).
//
// The registry is the single source of truth for "what is installed",
// distinct from the platform-managed catalog in ah_core (which describes
// what could be installed). Tenants install extensions; the registry
// records them and tracks lifecycle: pending_review → enabled → disabled
// → quarantined (the last is sticky and requires admin reset).
//
// Component slugs claimed by an extension are namespaced under that
// extension's slug ({extension-slug}/{component-slug}) to avoid
// cross-extension collisions.

// ExtensionStatus bounded enum tracks lifecycle.
type ExtensionStatus string

const (
	// ExtensionStatusPendingReview — installed but admin has not enabled yet.
	ExtensionStatusPendingReview ExtensionStatus = "pending_review"
	// ExtensionStatusEnabled — active and discoverable.
	ExtensionStatusEnabled ExtensionStatus = "enabled"
	// ExtensionStatusDisabled — admin paused without removing.
	ExtensionStatusDisabled ExtensionStatus = "disabled"
	// ExtensionStatusQuarantined — sticky failure state (signature mismatch,
	// security flag, repeated runtime errors). Cannot be re-enabled without
	// explicit admin reset (mirrors FUTURE-002 mistrusted contract).
	ExtensionStatusQuarantined ExtensionStatus = "quarantined"
)

var allExtensionStatuses = []ExtensionStatus{
	ExtensionStatusPendingReview, ExtensionStatusEnabled,
	ExtensionStatusDisabled, ExtensionStatusQuarantined,
}

// IsValidExtensionStatus returns true for the bounded set.
func IsValidExtensionStatus(s ExtensionStatus) bool {
	for _, v := range allExtensionStatuses {
		if s == v {
			return true
		}
	}
	return false
}

// AllExtensionStatuses returns a copy.
func AllExtensionStatuses() []ExtensionStatus {
	out := make([]ExtensionStatus, len(allExtensionStatuses))
	copy(out, allExtensionStatuses)
	return out
}

// ExtensionSource bounded enum identifies provenance.
type ExtensionSource string

const (
	// ExtensionSourceBuiltin — shipped with platform (ah_core seed).
	ExtensionSourceBuiltin ExtensionSource = "builtin"
	// ExtensionSourceMarketplace — installed from public marketplace.
	ExtensionSourceMarketplace ExtensionSource = "marketplace"
	// ExtensionSourceGit — cloned from git repository URL.
	ExtensionSourceGit ExtensionSource = "git"
	// ExtensionSourceLocalPath — sideloaded from local filesystem (dev).
	ExtensionSourceLocalPath ExtensionSource = "local_path"
	// ExtensionSourceURL — fetched from arbitrary HTTP URL.
	ExtensionSourceURL ExtensionSource = "url"
)

var allExtensionSources = []ExtensionSource{
	ExtensionSourceBuiltin, ExtensionSourceMarketplace,
	ExtensionSourceGit, ExtensionSourceLocalPath, ExtensionSourceURL,
}

// IsValidExtensionSource returns true for the bounded set.
func IsValidExtensionSource(s ExtensionSource) bool {
	for _, v := range allExtensionSources {
		if s == v {
			return true
		}
	}
	return false
}

// AllExtensionSources returns a copy.
func AllExtensionSources() []ExtensionSource {
	out := make([]ExtensionSource, len(allExtensionSources))
	copy(out, allExtensionSources)
	return out
}

// ExtensionComponentKind bounded enum mirrors the plugin manifest's
// 10-component-type taxonomy from PDF §6.1 (PluginManifestSchema,
// utils/plugins/schemas.ts). Web-applicable subset: 10 total − LSP servers
// (IDE-specific) − channels (CLI-terminal-specific) = 8 original + 2 added
// in FEAT-010: settings and user_configuration.
type ExtensionComponentKind string

const (
	ExtensionComponentCommands          ExtensionComponentKind = "commands"
	ExtensionComponentAgents            ExtensionComponentKind = "agents"
	ExtensionComponentSkills            ExtensionComponentKind = "skills"
	ExtensionComponentTools             ExtensionComponentKind = "tools"
	ExtensionComponentHooks             ExtensionComponentKind = "hooks"
	ExtensionComponentRules             ExtensionComponentKind = "rules"
	ExtensionComponentMCPServers        ExtensionComponentKind = "mcp_servers"
	ExtensionComponentOutputStyles      ExtensionComponentKind = "output_styles"
	// ExtensionComponentSettings — extension-declared key-value configuration
	// stored in settings.json; surfaced to users as structured forms in the UI.
	ExtensionComponentSettings          ExtensionComponentKind = "settings"
	// ExtensionComponentUserConfiguration — per-user preference overrides scoped
	// to this extension; does not affect other tenants or users.
	ExtensionComponentUserConfiguration ExtensionComponentKind = "user_configuration"
)

var allExtensionComponentKinds = []ExtensionComponentKind{
	ExtensionComponentCommands, ExtensionComponentAgents, ExtensionComponentSkills,
	ExtensionComponentTools, ExtensionComponentHooks, ExtensionComponentRules,
	ExtensionComponentMCPServers, ExtensionComponentOutputStyles,
	ExtensionComponentSettings, ExtensionComponentUserConfiguration,
}

// IsValidExtensionComponentKind returns true for the bounded set.
func IsValidExtensionComponentKind(k ExtensionComponentKind) bool {
	for _, v := range allExtensionComponentKinds {
		if k == v {
			return true
		}
	}
	return false
}

// AllExtensionComponentKinds returns a copy.
func AllExtensionComponentKinds() []ExtensionComponentKind {
	out := make([]ExtensionComponentKind, len(allExtensionComponentKinds))
	copy(out, allExtensionComponentKinds)
	return out
}

// ExtensionDescriptor is one installed extension's record.
type ExtensionDescriptor struct {
	ID         uuid.UUID                                `json:"id"`
	TenantID   string                                   `json:"tenantId"`
	Slug       string                                   `json:"slug"`
	Name       string                                   `json:"name"`
	Version    string                                   `json:"version"` // semver MAJOR.MINOR.PATCH
	Source     ExtensionSource                          `json:"source"`
	SourceRef  string                                   `json:"sourceRef"` // URL, path, marketplace ID, etc.
	Status     ExtensionStatus                          `json:"status"`
	Components map[ExtensionComponentKind][]string      `json:"components"` // kind → component slugs
	Checksum   string                                   `json:"checksum"`   // sha256 hex of signed bundle
	InstalledAt time.Time                               `json:"installedAt"`
	EnabledAt   time.Time                               `json:"enabledAt,omitempty"`
	UpdatedAt   time.Time                               `json:"updatedAt"`
}

// Sentinels.
var (
	ErrExtensionNotFound          = errors.New("extension registry: not found")
	ErrExtensionAlreadyExists     = errors.New("extension registry: slug already installed")
	ErrInvalidExtensionStatus     = errors.New("extension registry: invalid status")
	ErrInvalidExtensionSource     = errors.New("extension registry: invalid source")
	ErrInvalidComponentKind       = errors.New("extension registry: invalid component kind")
	ErrExtensionQuarantined       = errors.New("extension registry: quarantined extensions cannot be re-enabled without admin reset")
	ErrInvalidExtensionVersion    = errors.New("extension registry: version must be semver MAJOR.MINOR.PATCH")
	ErrExtensionChecksumRequired  = errors.New("extension registry: checksum required for marketplace/git/url sources")
)

var semverPattern = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

// validateDescriptor checks structural invariants (slug format, semver,
// bounded enums, checksum required for untrusted sources).
func validateDescriptor(d ExtensionDescriptor) error {
	if d.TenantID == "" {
		return errors.New("extension registry: tenantId required")
	}
	if d.Slug == "" {
		return errors.New("extension registry: slug required")
	}
	if !slugPattern.MatchString(d.Slug) {
		return fmt.Errorf("extension registry: slug %q must be kebab-case", d.Slug)
	}
	if d.Name == "" {
		return errors.New("extension registry: name required")
	}
	if !semverPattern.MatchString(d.Version) {
		return fmt.Errorf("%w: %q", ErrInvalidExtensionVersion, d.Version)
	}
	if !IsValidExtensionSource(d.Source) {
		return fmt.Errorf("%w: %q", ErrInvalidExtensionSource, d.Source)
	}
	// Builtin and local_path may omit checksum (trusted / dev). Other
	// sources MUST carry a checksum so the bundle can be re-verified.
	if d.Source != ExtensionSourceBuiltin && d.Source != ExtensionSourceLocalPath {
		if d.Checksum == "" {
			return ErrExtensionChecksumRequired
		}
	}
	for kind, slugs := range d.Components {
		if !IsValidExtensionComponentKind(kind) {
			return fmt.Errorf("%w: %q", ErrInvalidComponentKind, kind)
		}
		// Detect intra-extension duplicate component slugs.
		seen := map[string]bool{}
		for _, s := range slugs {
			if seen[s] {
				return fmt.Errorf("extension registry: component %q duplicated under kind %q", s, kind)
			}
			seen[s] = true
		}
	}
	return nil
}

// ExtensionRegistry is the persistence interface.
type ExtensionRegistry interface {
	Install(ctx context.Context, d ExtensionDescriptor) (ExtensionDescriptor, error)
	Uninstall(ctx context.Context, tenantID, slug string) error
	Enable(ctx context.Context, tenantID, slug string) (ExtensionDescriptor, error)
	Disable(ctx context.Context, tenantID, slug string) (ExtensionDescriptor, error)
	Quarantine(ctx context.Context, tenantID, slug, reason string) (ExtensionDescriptor, error)
	AdminResetQuarantine(ctx context.Context, tenantID, slug string) (ExtensionDescriptor, error)
	Find(ctx context.Context, tenantID, slug string) (ExtensionDescriptor, error)
	List(ctx context.Context, tenantID string) ([]ExtensionDescriptor, error)
	ListByStatus(ctx context.Context, tenantID string, status ExtensionStatus) ([]ExtensionDescriptor, error)
	ListByComponent(ctx context.Context, tenantID string, kind ExtensionComponentKind) ([]ExtensionDescriptor, error)
	ListBySource(ctx context.Context, tenantID string, source ExtensionSource) ([]ExtensionDescriptor, error)
}

// --- InMemoryExtensionRegistry ---

type extensionKey struct{ tenant, slug string }

type InMemoryExtensionRegistry struct {
	mu    sync.Mutex
	items map[extensionKey]ExtensionDescriptor
	now   func() time.Time
}

// NewInMemoryExtensionRegistry returns a concurrent-safe in-memory registry.
func NewInMemoryExtensionRegistry() *InMemoryExtensionRegistry {
	return &InMemoryExtensionRegistry{
		items: map[extensionKey]ExtensionDescriptor{},
		now:   time.Now,
	}
}

// SetClock injects a clock for deterministic test timestamps. Calling
// this with nil is a no-op; the default clock remains time.Now.
func (r *InMemoryExtensionRegistry) SetClock(fn func() time.Time) {
	if fn == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.now = fn
}

// Install adds a new extension. Defaults Status to pending_review unless
// caller specifies enabled (e.g. for builtin auto-enabled extensions).
func (r *InMemoryExtensionRegistry) Install(ctx context.Context, d ExtensionDescriptor) (ExtensionDescriptor, error) {
	if err := ctx.Err(); err != nil {
		return ExtensionDescriptor{}, err
	}
	if d.Status == "" {
		d.Status = ExtensionStatusPendingReview
	}
	if !IsValidExtensionStatus(d.Status) {
		return ExtensionDescriptor{}, fmt.Errorf("%w: %q", ErrInvalidExtensionStatus, d.Status)
	}
	if err := validateDescriptor(d); err != nil {
		return ExtensionDescriptor{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	key := extensionKey{d.TenantID, d.Slug}
	if _, exists := r.items[key]; exists {
		return ExtensionDescriptor{}, fmt.Errorf("%w: %q", ErrExtensionAlreadyExists, d.Slug)
	}
	d.ID = uuid.New()
	now := r.now()
	d.InstalledAt = now
	d.UpdatedAt = now
	if d.Status == ExtensionStatusEnabled {
		d.EnabledAt = now
	}
	r.items[key] = d
	return d, nil
}

// Uninstall removes an extension.
func (r *InMemoryExtensionRegistry) Uninstall(ctx context.Context, tenantID, slug string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := extensionKey{tenantID, slug}
	if _, ok := r.items[key]; !ok {
		return ErrExtensionNotFound
	}
	delete(r.items, key)
	return nil
}

// Enable transitions to enabled. Quarantined extensions are blocked.
func (r *InMemoryExtensionRegistry) Enable(ctx context.Context, tenantID, slug string) (ExtensionDescriptor, error) {
	if err := ctx.Err(); err != nil {
		return ExtensionDescriptor{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := extensionKey{tenantID, slug}
	d, ok := r.items[key]
	if !ok {
		return ExtensionDescriptor{}, ErrExtensionNotFound
	}
	if d.Status == ExtensionStatusQuarantined {
		return ExtensionDescriptor{}, ErrExtensionQuarantined
	}
	now := r.now()
	if d.Status != ExtensionStatusEnabled {
		d.Status = ExtensionStatusEnabled
		d.EnabledAt = now
	}
	d.UpdatedAt = now
	r.items[key] = d
	return d, nil
}

// Disable transitions to disabled (idempotent). Cannot disable quarantined.
func (r *InMemoryExtensionRegistry) Disable(ctx context.Context, tenantID, slug string) (ExtensionDescriptor, error) {
	if err := ctx.Err(); err != nil {
		return ExtensionDescriptor{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := extensionKey{tenantID, slug}
	d, ok := r.items[key]
	if !ok {
		return ExtensionDescriptor{}, ErrExtensionNotFound
	}
	if d.Status == ExtensionStatusQuarantined {
		return ExtensionDescriptor{}, ErrExtensionQuarantined
	}
	d.Status = ExtensionStatusDisabled
	d.UpdatedAt = r.now()
	r.items[key] = d
	return d, nil
}

// Quarantine marks an extension as failed (sticky); requires admin
// reset to recover.
func (r *InMemoryExtensionRegistry) Quarantine(ctx context.Context, tenantID, slug, reason string) (ExtensionDescriptor, error) {
	if err := ctx.Err(); err != nil {
		return ExtensionDescriptor{}, err
	}
	if strings.TrimSpace(reason) == "" {
		return ExtensionDescriptor{}, errors.New("extension registry: quarantine reason required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := extensionKey{tenantID, slug}
	d, ok := r.items[key]
	if !ok {
		return ExtensionDescriptor{}, ErrExtensionNotFound
	}
	d.Status = ExtensionStatusQuarantined
	d.UpdatedAt = r.now()
	r.items[key] = d
	return d, nil
}

// AdminResetQuarantine clears quarantine, returning extension to disabled.
func (r *InMemoryExtensionRegistry) AdminResetQuarantine(ctx context.Context, tenantID, slug string) (ExtensionDescriptor, error) {
	if err := ctx.Err(); err != nil {
		return ExtensionDescriptor{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := extensionKey{tenantID, slug}
	d, ok := r.items[key]
	if !ok {
		return ExtensionDescriptor{}, ErrExtensionNotFound
	}
	if d.Status != ExtensionStatusQuarantined {
		return ExtensionDescriptor{}, fmt.Errorf("extension registry: not quarantined (status=%q)", d.Status)
	}
	d.Status = ExtensionStatusDisabled
	d.UpdatedAt = r.now()
	r.items[key] = d
	return d, nil
}

// Find returns one extension by tenant + slug.
func (r *InMemoryExtensionRegistry) Find(ctx context.Context, tenantID, slug string) (ExtensionDescriptor, error) {
	if err := ctx.Err(); err != nil {
		return ExtensionDescriptor{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.items[extensionKey{tenantID, slug}]
	if !ok {
		return ExtensionDescriptor{}, ErrExtensionNotFound
	}
	return d, nil
}

// List returns all extensions for tenant, ordered by slug.
func (r *InMemoryExtensionRegistry) List(ctx context.Context, tenantID string) ([]ExtensionDescriptor, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []ExtensionDescriptor{}
	for k, d := range r.items {
		if k.tenant == tenantID {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out, nil
}

// ListByStatus returns extensions matching status.
func (r *InMemoryExtensionRegistry) ListByStatus(ctx context.Context, tenantID string, status ExtensionStatus) ([]ExtensionDescriptor, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	all, err := r.List(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := []ExtensionDescriptor{}
	for _, d := range all {
		if d.Status == status {
			out = append(out, d)
		}
	}
	return out, nil
}

// ListByComponent returns extensions providing the requested component kind.
func (r *InMemoryExtensionRegistry) ListByComponent(ctx context.Context, tenantID string, kind ExtensionComponentKind) ([]ExtensionDescriptor, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	all, err := r.List(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := []ExtensionDescriptor{}
	for _, d := range all {
		if slugs, ok := d.Components[kind]; ok && len(slugs) > 0 {
			out = append(out, d)
		}
	}
	return out, nil
}

// ListBySource returns extensions provenanced from source.
func (r *InMemoryExtensionRegistry) ListBySource(ctx context.Context, tenantID string, source ExtensionSource) ([]ExtensionDescriptor, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	all, err := r.List(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := []ExtensionDescriptor{}
	for _, d := range all {
		if d.Source == source {
			out = append(out, d)
		}
	}
	return out, nil
}
