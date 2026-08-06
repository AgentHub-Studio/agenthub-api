package agentic

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// EXT-004 — Plugin manifest.
//
// PDF arXiv:2604.14228v1 §6 (plugin manifest as the source-of-truth
// declaration of what an extension provides: identity + components +
// dependencies + signature).
//
// Distinct from existing AgentHub plumbing:
//   - EXT-001 ExtensionRegistry = INSTALLED extensions per tenant.
//   - EXT-003 HookSchemaRegistry = lifecycle event schemas.
//   - plugin_manifest.go (this file) = MANIFEST DOCUMENT SPEC:
//     pure-domain shape + parser (JSON) + validator. This is what
//     ships INSIDE the extension bundle (manifest.json/yaml) and what
//     the marketplace + installer parse to decide whether to install.
//
// The manifest is the contract between extension AUTHORS and the
// platform. ExtensionRegistry stores a subset of these fields once
// installed.

// PluginManifestKind bounded enum identifies the top-level packaging.
type PluginManifestKind string

const (
	// PluginManifestKindAgent — bundles one or more agents.
	PluginManifestKindAgent PluginManifestKind = "agent"
	// PluginManifestKindSkillPack — bundles skills + their tools.
	PluginManifestKindSkillPack PluginManifestKind = "skill_pack"
	// PluginManifestKindToolPack — bundles only tools.
	PluginManifestKindToolPack PluginManifestKind = "tool_pack"
	// PluginManifestKindHookPack — bundles hooks + handlers.
	PluginManifestKindHookPack PluginManifestKind = "hook_pack"
	// PluginManifestKindRulePack — bundles rules.
	PluginManifestKindRulePack PluginManifestKind = "rule_pack"
	// PluginManifestKindThemePack — bundles output styles + UI presets.
	PluginManifestKindThemePack PluginManifestKind = "theme_pack"
	// PluginManifestKindBundle — multi-kind extension (combines several).
	PluginManifestKindBundle PluginManifestKind = "bundle"
)

var allPluginManifestKinds = []PluginManifestKind{
	PluginManifestKindAgent, PluginManifestKindSkillPack, PluginManifestKindToolPack,
	PluginManifestKindHookPack, PluginManifestKindRulePack, PluginManifestKindThemePack,
	PluginManifestKindBundle,
}

// IsValidPluginManifestKind returns true for the bounded set.
func IsValidPluginManifestKind(k PluginManifestKind) bool {
	for _, v := range allPluginManifestKinds {
		if k == v {
			return true
		}
	}
	return false
}

// AllPluginManifestKinds returns a copy.
func AllPluginManifestKinds() []PluginManifestKind {
	out := make([]PluginManifestKind, len(allPluginManifestKinds))
	copy(out, allPluginManifestKinds)
	return out
}

// PluginManifestComponent declares one component the plugin provides.
type PluginManifestComponent struct {
	// Kind identifies the component type. MUST be one of the 8 CTX-004 /
	// EXT-001 component kinds (commands/agents/skills/tools/hooks/rules/
	// mcp_servers/output_styles).
	Kind ExtensionComponentKind `json:"kind"`
	// Slug is the component's canonical identifier within the plugin.
	Slug string `json:"slug"`
	// EntryPath is the relative file path within the bundle.
	EntryPath string `json:"entryPath"`
}

// PluginManifestDependency declares a required dependency.
type PluginManifestDependency struct {
	// Slug is the dependency plugin's slug.
	Slug string `json:"slug"`
	// MinVersion is the minimum acceptable semver.
	MinVersion string `json:"minVersion"`
	// IsOptional means the plugin still loads if dep is missing.
	IsOptional bool `json:"isOptional,omitempty"`
}

// PluginManifest is the complete spec document for an extension bundle.
// PDF §6.1 lists 10 component types; this manifest references them by
// the EXT-001 ExtensionComponentKind enum (web-applicable subset).
type PluginManifest struct {
	// SchemaVersion is the manifest spec version itself (not the plugin).
	// Allows the platform to evolve the manifest format.
	SchemaVersion string `json:"schemaVersion"`
	// ID is the plugin's globally unique slug (namespace optional).
	ID string `json:"id"`
	// Name is the human-readable plugin name.
	Name string `json:"name"`
	// Version is the plugin version (semver MAJOR.MINOR.PATCH).
	Version string `json:"version"`
	// Description is a short summary (≤500 chars).
	Description string `json:"description"`
	// Author identifies who built the plugin.
	Author string `json:"author"`
	// Kind is the manifest packaging kind.
	Kind PluginManifestKind `json:"kind"`
	// MinPlatformVersion is the lowest AgentHub version this plugin supports.
	MinPlatformVersion string `json:"minPlatformVersion"`
	// Components is the list of components provided.
	Components []PluginManifestComponent `json:"components"`
	// Dependencies is the list of required plugins.
	Dependencies []PluginManifestDependency `json:"dependencies,omitempty"`
	// Permissions is the set of permissions the plugin requests.
	Permissions []string `json:"permissions,omitempty"`
	// Tags is free-form discoverability metadata.
	Tags []string `json:"tags,omitempty"`
	// License is the SPDX license identifier (e.g. "MIT", "Apache-2.0").
	License string `json:"license,omitempty"`
	// Homepage is the project URL.
	Homepage string `json:"homepage,omitempty"`
	// PublishedAt is the manifest's publish timestamp.
	PublishedAt time.Time `json:"publishedAt,omitempty"`
}

// CurrentManifestSchemaVersion is the version this codebase emits/parses by default.
const CurrentManifestSchemaVersion = "1.0.0"

// manifestIDPattern allows alphanumeric, dash, underscore, dot, slash
// (for namespaced IDs like "vendor/plugin-name").
var manifestIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._/-]*[a-z0-9]$`)

// Sentinels.
var (
	ErrPluginManifestInvalidSchemaVersion = errors.New("plugin manifest: invalid schema_version")
	ErrPluginManifestIDEmpty              = errors.New("plugin manifest: id required")
	ErrPluginManifestIDInvalid            = errors.New("plugin manifest: id format invalid")
	ErrPluginManifestNameEmpty            = errors.New("plugin manifest: name required")
	ErrPluginManifestInvalidVersion       = errors.New("plugin manifest: version must be semver MAJOR.MINOR.PATCH")
	ErrPluginManifestDescriptionTooLong   = errors.New("plugin manifest: description must be ≤500 chars")
	ErrPluginManifestInvalidKind          = errors.New("plugin manifest: invalid kind")
	ErrPluginManifestComponentRequired    = errors.New("plugin manifest: at least one component required (unless bundle is empty)")
	ErrPluginManifestComponentInvalid     = errors.New("plugin manifest: invalid component kind")
	ErrPluginManifestComponentSlugEmpty   = errors.New("plugin manifest: component slug required")
	ErrPluginManifestComponentDuplicate   = errors.New("plugin manifest: duplicate (kind, slug) component")
	ErrPluginManifestInvalidDependency    = errors.New("plugin manifest: invalid dependency spec")
	ErrPluginManifestParseFailed          = errors.New("plugin manifest: parse failed")
)

// Validate checks structural invariants on the manifest.
func (m PluginManifest) Validate() error {
	if !semverPattern.MatchString(m.SchemaVersion) {
		return fmt.Errorf("%w: %q", ErrPluginManifestInvalidSchemaVersion, m.SchemaVersion)
	}
	if strings.TrimSpace(m.ID) == "" {
		return ErrPluginManifestIDEmpty
	}
	if !manifestIDPattern.MatchString(m.ID) {
		return fmt.Errorf("%w: %q", ErrPluginManifestIDInvalid, m.ID)
	}
	if strings.TrimSpace(m.Name) == "" {
		return ErrPluginManifestNameEmpty
	}
	if !semverPattern.MatchString(m.Version) {
		return fmt.Errorf("%w: %q", ErrPluginManifestInvalidVersion, m.Version)
	}
	if len(m.Description) > 500 {
		return ErrPluginManifestDescriptionTooLong
	}
	if !IsValidPluginManifestKind(m.Kind) {
		return fmt.Errorf("%w: %q", ErrPluginManifestInvalidKind, m.Kind)
	}
	if !semverPattern.MatchString(m.MinPlatformVersion) {
		return fmt.Errorf("%w: min_platform_version %q must be semver",
			ErrPluginManifestInvalidVersion, m.MinPlatformVersion)
	}

	// Components: at least one required UNLESS Kind=bundle (which may be empty).
	if len(m.Components) == 0 && m.Kind != PluginManifestKindBundle {
		return ErrPluginManifestComponentRequired
	}

	seen := map[string]bool{}
	for _, c := range m.Components {
		if !IsValidExtensionComponentKind(c.Kind) {
			return fmt.Errorf("%w: %q", ErrPluginManifestComponentInvalid, c.Kind)
		}
		if strings.TrimSpace(c.Slug) == "" {
			return ErrPluginManifestComponentSlugEmpty
		}
		key := string(c.Kind) + "/" + c.Slug
		if seen[key] {
			return fmt.Errorf("%w: %q", ErrPluginManifestComponentDuplicate, key)
		}
		seen[key] = true
	}

	for _, d := range m.Dependencies {
		if strings.TrimSpace(d.Slug) == "" {
			return fmt.Errorf("%w: dependency slug required", ErrPluginManifestInvalidDependency)
		}
		if !semverPattern.MatchString(d.MinVersion) {
			return fmt.Errorf("%w: dep %q min_version %q must be semver",
				ErrPluginManifestInvalidDependency, d.Slug, d.MinVersion)
		}
	}

	return nil
}

// ParseManifestJSON parses a JSON manifest document and validates it.
func ParseManifestJSON(data []byte) (PluginManifest, error) {
	var m PluginManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return PluginManifest{}, fmt.Errorf("%w: %v", ErrPluginManifestParseFailed, err)
	}
	if err := m.Validate(); err != nil {
		return PluginManifest{}, err
	}
	return m, nil
}

// SerializeManifestJSON serializes a manifest to canonical JSON.
// Validates before emitting.
func SerializeManifestJSON(m PluginManifest) ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return json.MarshalIndent(m, "", "  ")
}

// ComponentKinds returns the unique component kinds in the manifest,
// sorted alphabetically (deterministic for replay/audit).
func (m PluginManifest) ComponentKinds() []ExtensionComponentKind {
	seen := map[ExtensionComponentKind]bool{}
	for _, c := range m.Components {
		seen[c.Kind] = true
	}
	out := []ExtensionComponentKind{}
	for k := range seen {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// HasComponent returns true if the manifest declares (kind, slug).
func (m PluginManifest) HasComponent(kind ExtensionComponentKind, slug string) bool {
	for _, c := range m.Components {
		if c.Kind == kind && c.Slug == slug {
			return true
		}
	}
	return false
}

// RequiresPlugin returns true if this manifest declares slug as a dependency.
func (m PluginManifest) RequiresPlugin(slug string) bool {
	for _, d := range m.Dependencies {
		if d.Slug == slug {
			return true
		}
	}
	return false
}

// IsCompatibleWith returns nil if the platform version meets the
// manifest's minimum. Otherwise returns an error explaining the gap.
func (m PluginManifest) IsCompatibleWith(platformVersion string) error {
	if !semverPattern.MatchString(platformVersion) {
		return fmt.Errorf("plugin manifest: platform version %q is not semver", platformVersion)
	}
	plat := splitSemver(platformVersion)
	min := splitSemver(m.MinPlatformVersion)
	for i := 0; i < 3; i++ {
		if plat[i] < min[i] {
			return fmt.Errorf("plugin manifest: platform %q < min %q",
				platformVersion, m.MinPlatformVersion)
		}
		if plat[i] > min[i] {
			return nil
		}
	}
	return nil
}

// splitSemver returns [major, minor, patch] from a known-valid semver.
func splitSemver(v string) [3]int {
	parts := strings.Split(v, ".")
	out := [3]int{}
	for i := 0; i < 3 && i < len(parts); i++ {
		_, _ = fmt.Sscanf(parts[i], "%d", &out[i])
	}
	return out
}

// ManifestSummary returns a short audit-friendly summary string.
func (m PluginManifest) ManifestSummary() string {
	return fmt.Sprintf("plugin=%q v=%s kind=%s components=%d deps=%d",
		m.ID, m.Version, m.Kind, len(m.Components), len(m.Dependencies))
}
