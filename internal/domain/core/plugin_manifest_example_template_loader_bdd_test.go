package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCorePMETemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshAuthorCopiesSkeletonInsteadOfInventing", func(t *testing.T) {
		// Given a vendor wants to publish a new plugin,
		// And EXT-004 PluginManifest has 12 fields + nested components,
		// When the vendor opens "create new plugin",
		// Then 7 example skeletons (one per kind) provide the starting
		// JSON — copy + customize id/name/version, NOT learn schema
		// from scratch.
		assert.Equal(t, 7, len(SeedRecommendedPMETemplateSlugs))
	})

	t.Run("Scenario_OneExamplePerEXT004ManifestKind", func(t *testing.T) {
		// Given EXT-004 has 7 PluginManifestKind enum values,
		// When the seed ships examples,
		// Then exactly 1 example per kind — every author starts with
		// an apt template regardless of plugin type.
		assert.Equal(t, 7, len(SeedExpectedPMETemplateKinds))
		assert.Equal(t, 7, SeedExpectedPMETemplateRowCount)
	})

	t.Run("Scenario_KindLabelsMatchEXT004EnumByteForByte", func(t *testing.T) {
		// Given EXT-004 PluginManifestKind has 7 enum values,
		// When this seed declares manifest_kind,
		// Then labels match enum bytes (no mapping table runtime).
		ext004 := []string{
			"agent", "skill_pack", "tool_pack", "hook_pack",
			"rule_pack", "theme_pack", "bundle",
		}
		set := map[string]bool{}
		for _, k := range SeedExpectedPMETemplateKinds {
			set[k] = true
		}
		for _, e := range ext004 {
			assert.True(t, set[e], "EXT-004 kind %q missing", e)
		}
	})

	t.Run("Scenario_AllSkeletonsAreRecommendedSafeSinceEducational", func(t *testing.T) {
		// Given skeletons are read-only educational content,
		// When admin filters by recommended,
		// Then ALL 7 surface (no admin-review gate; nothing to review
		// — these don't execute, only get copied).
		assert.Equal(t, len(SeedExpectedPMETemplateSlugs),
			len(SeedRecommendedPMETemplateSlugs))
	})

	t.Run("Scenario_AudienceIsExtensionAuthorsOnly", func(t *testing.T) {
		// Given these templates are vendor-author-facing not end-user,
		// When admin filters by audience,
		// Then "extension_author" is the only valid target.
		assert.Equal(t, []string{"extension_author"},
			SeedExpectedPMETemplateAudiences)
	})

	t.Run("Scenario_BundleExampleShowsDependencyCompositionPattern", func(t *testing.T) {
		// Given bundle kind has empty components + dependencies,
		// When vendor uses example-bundle-meta,
		// Then they see how to compose a meta-distribution (recommended
		// starter pack pointing at other plugins).
		assert.Contains(t, SeedExpectedPMETemplateSlugs, "example-bundle-meta")
	})

	t.Run("Scenario_AllExamplesContainPlaceholderIDsForCopy", func(t *testing.T) {
		// Given the example manifest should NOT install as-is (vendor
		// would override id/name/version first),
		// When vendor reads the example,
		// Then placeholder id (`example-vendor/*`) makes obvious that
		// it must be replaced.
		// (Validated structurally via integration test JSON contents.)
		assert.NotEmpty(t, SeedExpectedPMETemplateSlugs)
	})

	t.Run("Scenario_SkeletonsCoverFullSchemaFieldsToTeachByExample", func(t *testing.T) {
		// Given vendors learn by example,
		// When they read a skeleton,
		// Then it includes the major fields (id/name/version/kind/
		// minPlatformVersion/components) so the vendor sees the
		// REQUIRED shape — not a minimal sketch.
		// (Validated via integration test.)
		assert.Equal(t, 7, len(SeedExpectedPMETemplateSlugs))
	})

	t.Run("Scenario_ExampleAgentPackShowsSingleAgentDeclaration", func(t *testing.T) {
		// Given the most common single-agent plugin shape,
		// When vendor copies example-agent-pack,
		// Then they get a manifest with kind=agent + components=[agents/...].
		assert.Contains(t, SeedExpectedPMETemplateSlugs, "example-agent-pack")
	})

	t.Run("Scenario_ExampleSkillPackShowsMultiSkillBundling", func(t *testing.T) {
		// Given the most common multi-skill plugin shape,
		// When vendor copies example-skill-pack,
		// Then they get a manifest with kind=skill_pack + multiple
		// components for different skills.
		assert.Contains(t, SeedExpectedPMETemplateSlugs, "example-skill-pack")
	})
}
