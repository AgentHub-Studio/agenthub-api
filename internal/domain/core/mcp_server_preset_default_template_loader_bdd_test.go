package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for MCPServerPresetDefaultTemplate seed.
// MCP server presets define pre-configured server connections covering
// stdio-subprocess and streamable HTTP transports per MCP spec 2025-03-26.

func TestBDD_AhCoreMCPServerPresetSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantReceivesSixMCPPresets", func(t *testing.T) {
		// Given a new tenant activates MCP integration
		// When the preset catalog is loaded
		// Then exactly 6 MCP server presets are available
		assert.Equal(t, 6, SeedExpectedMCPPresetRowCount)
		assert.Equal(t, 6, len(SeedExpectedMCPPresetSlugs))
	})

	t.Run("Scenario_StdioAndHTTPTransportsBothRepresented", func(t *testing.T) {
		// Given MCP spec 2025-03-26 supports stdio and streamable HTTP transports
		// When the transport breakdown of presets is inspected
		// Then stdio presets and HTTP presets together equal the full catalog
		total := len(SeedMCPStdioPresetSlugs) + len(SeedMCPHTTPPresetSlugs)
		assert.Equal(t, SeedExpectedMCPPresetRowCount, total)
	})

	t.Run("Scenario_RemoteSSEIsOnlyHTTPPreset", func(t *testing.T) {
		// Given streamable HTTP (Streamable HTTP 2025-03-26) replaces SSE transport
		// When HTTP presets are listed
		// Then only remote-sse uses the streamable_http transport
		assert.Equal(t, 1, len(SeedMCPHTTPPresetSlugs))
		assert.Contains(t, SeedMCPHTTPPresetSlugs, "remote-sse")
	})

	t.Run("Scenario_AutoStartPresetsMustBeStdioOnly", func(t *testing.T) {
		// Given HTTP presets require user-supplied base URLs and cannot auto-start
		// When auto-start presets are compared against the stdio list
		// Then every auto-start preset uses the stdio transport
		stdioSet := map[string]bool{}
		for _, s := range SeedMCPStdioPresetSlugs {
			stdioSet[s] = true
		}
		for _, as := range SeedMCPAutoStartSlugs {
			assert.True(t, stdioSet[as],
				"auto-start preset %q should be stdio-only", as)
		}
	})

	t.Run("Scenario_AllSlugsMustBeKebabCase", func(t *testing.T) {
		// Given naming conventions require kebab-case slugs
		// When each preset slug is validated
		// Then every slug matches the ^[a-z0-9][a-z0-9-]*[a-z0-9]$ pattern
		for _, s := range SeedExpectedMCPPresetSlugs {
			assert.True(t, SeedMCPPresetSlugRE.MatchString(s),
				"slug %q violates kebab-case pattern", s)
		}
	})

	t.Run("Scenario_CommonToolingPresetsAreInCatalog", func(t *testing.T) {
		// Given filesystem, github, and sqlite are the most commonly used MCP servers
		// When the preset catalog is inspected
		// Then all three are present as distinct presets
		slugSet := map[string]bool{}
		for _, s := range SeedExpectedMCPPresetSlugs {
			slugSet[s] = true
		}
		assert.True(t, slugSet["filesystem"])
		assert.True(t, slugSet["github"])
		assert.True(t, slugSet["sqlite"])
	})
}
