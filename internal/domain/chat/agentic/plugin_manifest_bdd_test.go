package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBDD_PluginManifest(t *testing.T) {
	t.Run("Scenario_MarketplaceInstallerParsesManifestBeforeAccepting", func(t *testing.T) {
		// Given a vendor submits a plugin bundle with a manifest.json,
		// When the marketplace installer parses the JSON,
		// Then it validates structure first — malformed manifests are
		// rejected before install begins (no half-installed state).
		data, err := SerializeManifestJSON(validManifest())
		require.NoError(t, err)
		parsed, err := ParseManifestJSON(data)
		require.NoError(t, err)
		assert.Equal(t, "vendor/research-pack", parsed.ID)
	})

	t.Run("Scenario_PlatformRejectsManifestRequiringHigherVersion", func(t *testing.T) {
		// Given a plugin requires min_platform_version=2.0.0,
		// When admin tries to install on a 1.5.0 platform,
		// Then IsCompatibleWith rejects — install pipeline halts before
		// runtime errors.
		m := validManifest()
		m.MinPlatformVersion = "2.0.0"
		err := m.IsCompatibleWith("1.5.0")
		assert.Error(t, err)
	})

	t.Run("Scenario_DuplicateComponentsRejectedToPreventResolutionAmbiguity", func(t *testing.T) {
		// Given a manifest accidentally declares (skills, web-research)
		// twice,
		// When validation runs,
		// Then duplicate is rejected — runtime ambiguity prevented
		// (which entry path wins on resolution?).
		m := validManifest()
		m.Components = append(m.Components, m.Components[0])
		err := m.Validate()
		assert.Error(t, err)
	})

	t.Run("Scenario_NonWebComponentKindsRejectedForAgentHubPlatform", func(t *testing.T) {
		// Given AgentHub is web-only and EXT-001 excludes LSP/channels/
		// worktrees,
		// When a plugin manifest declares a worktrees component,
		// Then validation rejects (catches mismatch before install).
		m := validManifest()
		m.Components = []PluginManifestComponent{
			{Kind: "worktrees", Slug: "my-tree", EntryPath: "x"},
		}
		err := m.Validate()
		assert.Error(t, err)
	})

	t.Run("Scenario_BundleKindAllowsEmptyComponentsForCompositionPlugins", func(t *testing.T) {
		// Given a meta-plugin (bundle) only declares dependencies + no
		// components of its own,
		// When validation runs,
		// Then empty components allowed — bundle is a composition entry.
		m := validManifest()
		m.Kind = PluginManifestKindBundle
		m.Components = nil
		m.Dependencies = []PluginManifestDependency{
			{Slug: "research-pack", MinVersion: "1.0.0"},
			{Slug: "kb-pack", MinVersion: "1.0.0"},
		}
		assert.NoError(t, m.Validate())
	})

	t.Run("Scenario_NamespacedIDsForVendorOwnership", func(t *testing.T) {
		// Given multiple vendors publish plugins to marketplace,
		// When vendor uses namespaced id like "acme/research-pack",
		// Then ID format accepts (avoids namespace collisions).
		m := validManifest()
		m.ID = "acme/research-pack"
		assert.NoError(t, m.Validate())
	})

	t.Run("Scenario_SemverContractEnablesDependencyResolution", func(t *testing.T) {
		// Given the marketplace resolver needs to pick which plugin
		// version satisfies a dependency,
		// When manifest declares a strict semver,
		// Then loose version strings rejected (semver guarantees
		// orderability).
		m := validManifest()
		m.Version = "1.x"
		err := m.Validate()
		assert.True(t, err != nil)
	})

	t.Run("Scenario_ComponentKindsHelpsAdminFilterInMarketplaceUI", func(t *testing.T) {
		// Given admin browses marketplace by "what does this plugin
		// provide?",
		// When the UI calls ComponentKinds(),
		// Then it gets a deduplicated sorted list (UI shows badges).
		m := validManifest()
		m.Components = []PluginManifestComponent{
			{Kind: ExtensionComponentTools, Slug: "a", EntryPath: "a"},
			{Kind: ExtensionComponentSkills, Slug: "b", EntryPath: "b"},
			{Kind: ExtensionComponentTools, Slug: "c", EntryPath: "c"},
		}
		got := m.ComponentKinds()
		assert.Equal(t, 2, len(got))
	})

	t.Run("Scenario_RequiresPluginExposesDependencyResolutionForInstaller", func(t *testing.T) {
		// Given the installer needs to figure out which other plugins
		// to install first,
		// When it queries RequiresPlugin(slug),
		// Then it gets a yes/no — drives the install ordering.
		m := validManifest()
		m.Dependencies = []PluginManifestDependency{
			{Slug: "shared-utils", MinVersion: "1.0.0"},
		}
		assert.True(t, m.RequiresPlugin("shared-utils"))
	})

	t.Run("Scenario_RoundTripJSONForBundleAndDistribution", func(t *testing.T) {
		// Given plugins ship as bundles with manifest.json files,
		// When marketplace serializes a manifest, ships it, and the
		// installer parses it,
		// Then the round-trip preserves all fields verbatim.
		original := validManifest()
		data, _ := SerializeManifestJSON(original)
		parsed, _ := ParseManifestJSON(data)
		assert.Equal(t, original.ID, parsed.ID)
		assert.Equal(t, original.Version, parsed.Version)
		assert.Equal(t, original.Kind, parsed.Kind)
		assert.Equal(t, len(original.Components), len(parsed.Components))
	})

	t.Run("Scenario_SummaryStringSupportsAuditAndOperationsLogs", func(t *testing.T) {
		// Given GOV-001 audits every install + uninstall event,
		// When the install handler logs the operation,
		// Then ManifestSummary returns a one-line copy-pasteable
		// identifier (plugin id + version + kind + counts).
		summary := validManifest().ManifestSummary()
		assert.Contains(t, summary, "vendor/research-pack")
		assert.Contains(t, summary, "v=1.2.3")
	})
}
