package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreToolSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantInheritsBaselinePlatformTools", func(t *testing.T) {
		// Given a fresh tenant (no tools of its own),
		// When the runtime asks ah_core for tools,
		// Then ≥20 baseline platform-management tools are seeded so
		//      the agent can manage agents/skills/tools/KBs/MCP/etc.
		//      from day one without configuration.
		assert.GreaterOrEqual(t, len(SeedExpectedToolSlugs), 20,
			"fresh tenant must inherit at least 20 baseline tools")
	})

	t.Run("Scenario_AllSeedToolsUseCorePrefixForNamespaceIsolation", func(t *testing.T) {
		// Given tenants can register CUSTOM tools alongside platform
		//       defaults — namespace collision is a real risk,
		// When the seed catalogue is inspected,
		// Then every slug starts with "core-" so a tenant tool with
		//      slug "list-agents" does NOT collide with "core-list-agents".
		for _, s := range SeedExpectedToolSlugs {
			assert.True(t, strings.HasPrefix(s, "core-"),
				"slug %q must use core- prefix for namespace isolation", s)
		}
	})

	t.Run("Scenario_SeedToolsAreAllHTTPTypeForBackendProxy", func(t *testing.T) {
		// Given seed tools proxy REST calls back to AgentHub backend
		//       (PDF Section 6.1 — tools as standardised interfaces),
		// When the type contract is inspected,
		// Then HTTP is the only allowed type for seed rows. Tenants
		//      register SQL/DOCUMENT_SEARCH/CUSTOM tools themselves.
		assert.Equal(t, "HTTP", SeedExpectedToolType,
			"backend-proxy contract requires HTTP type")
	})

	t.Run("Scenario_ManagementCRUDIsCompleteForCoreEntities", func(t *testing.T) {
		// Given the agent must be able to manage core entities (agents,
		//       skills, tools, KBs, MCP servers, settings),
		// When the seed slug set is inspected,
		// Then for each entity, list/create/update/delete (or appropriate
		//      subset) is present.
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedToolSlugs {
			seedSet[s] = true
		}
		// agents — full CRUD + publish
		for _, s := range []string{
			"core-list-agents", "core-get-agent", "core-create-agent",
			"core-update-agent", "core-delete-agent", "core-publish-agent",
		} {
			assert.True(t, seedSet[s], "agents CRUD: %q must be seeded", s)
		}
		// skills CRUD
		for _, s := range []string{
			"core-list-skills", "core-create-skill", "core-update-skill", "core-delete-skill",
		} {
			assert.True(t, seedSet[s], "skills CRUD: %q must be seeded", s)
		}
		// tools CRUD
		for _, s := range []string{
			"core-list-tools", "core-create-tool", "core-update-tool", "core-delete-tool",
		} {
			assert.True(t, seedSet[s], "tools CRUD: %q must be seeded", s)
		}
		// knowledge bases CRUD
		for _, s := range []string{
			"core-list-kbs", "core-create-kb", "core-update-kb", "core-delete-kb",
		} {
			assert.True(t, seedSet[s], "kbs CRUD: %q must be seeded", s)
		}
		// MCP servers CRUD
		for _, s := range []string{
			"core-list-mcp-servers", "core-create-mcp-server",
			"core-update-mcp-server", "core-delete-mcp-server",
		} {
			assert.True(t, seedSet[s], "mcp-servers CRUD: %q must be seeded", s)
		}
	})

	t.Run("Scenario_SkillToolBindingIsExposed", func(t *testing.T) {
		// Given agents need to bind skills explicitly (PDF Section 6.1),
		// When the seed is inspected,
		// Then bind-agent-skills + list-agent-skills are seeded.
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedToolSlugs {
			seedSet[s] = true
		}
		assert.True(t, seedSet["core-bind-agent-skills"])
		assert.True(t, seedSet["core-list-agent-skills"])
	})

	t.Run("Scenario_ImportExportEnablesAgentPortability", func(t *testing.T) {
		// Given users want to share/migrate agents across tenants,
		// When the seed is inspected,
		// Then import/export endpoints are exposed.
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedToolSlugs {
			seedSet[s] = true
		}
		assert.True(t, seedSet["core-export-agent"], "export required")
		assert.True(t, seedSet["core-import-agent"], "import required")
	})

	t.Run("Scenario_ConfigKeyContractIsClosed", func(t *testing.T) {
		// Given the HTTP tool config schema MUST be predictable so the
		//       executor can dispatch — unknown keys break the executor,
		// When the allowed-keys allowlist is inspected,
		// Then it covers exactly url+method+useCallerToken+bodyTemplate.
		assert.ElementsMatch(t,
			[]string{"url", "method", "useCallerToken", "bodyTemplate"},
			SeedExpectedToolConfigKeys,
			"config key allowlist must be closed (refactor must update both migration and constants)")
	})

	t.Run("Scenario_RequiredKeysSubsetOfAllowedKeys", func(t *testing.T) {
		// Given Required ⊆ Allowed by definition,
		// When the relationship is checked,
		// Then every required key appears in allowed (refactor guard).
		allowed := map[string]bool{}
		for _, k := range SeedExpectedToolConfigKeys {
			allowed[k] = true
		}
		for _, k := range SeedRequiredToolConfigKeys {
			assert.True(t, allowed[k],
				"required key %q must be in allowed set", k)
		}
	})

	t.Run("Scenario_CanonicalSlugCountStableAcrossRefactors", func(t *testing.T) {
		// Given external systems (UI, billing, analytics) bind to the
		//       count of seed tools as a contract,
		// When the count is inspected,
		// Then 31 slugs exist (changed iff migration changed too).
		assert.Equal(t, 31, len(SeedExpectedToolSlugs),
			"refactor guard: 31 slugs expected")
	})
}
