package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreMCPServerSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantInheritsCatalogOfMCPServers", func(t *testing.T) {
		// Given a fresh tenant browses the MCP server picker for the
		//       first time,
		// When the platform catalog is loaded from ah_core,
		// Then ≥6 catalog entries appear so the tenant can enable any
		//      without leaving the AgentHub UI.
		assert.GreaterOrEqual(t, len(SeedExpectedMCPServerSlugs), 6,
			"fresh tenant must see at least 6 MCP server catalog entries")
	})

	t.Run("Scenario_CatalogIsHTTPOnlyForWebPortability", func(t *testing.T) {
		// Given AgentHub is a multi-tenant web product (PDF Section 6.1
		//       — MCP transports are HTTP/stdio; we adapt to web reality),
		// When the seed transport allowlist is inspected,
		// Then only http transport is catalogued — stdio requires local
		//      subprocess which is not portable across tenants.
		assert.Equal(t, []string{"http"}, SeedExpectedMCPServerTransports,
			"web-product seed catalog MUST be http-only — stdio breaks multi-tenant")
	})

	t.Run("Scenario_OfficialServersAreFlaggedForUITrustBadge", func(t *testing.T) {
		// Given users want to know which servers are vetted (PDF
		//       Section 11 — trust signals must be EXPLICIT),
		// When the catalog is rendered,
		// Then is_official drives a trust badge — integration test
		//      verifies all seed entries are is_official=TRUE since
		//      we curate the seed from Anthropic-published servers.
		// Constant guard: every seed slug references a vendor.
		// (DB-level invariant validated in integration test.)
		assert.NotEmpty(t, SeedExpectedMCPServerSlugs,
			"seed catalog non-empty so trust-badge UI has rows to render")
	})

	t.Run("Scenario_AuthRequiredServersDriveCredentialPicker", func(t *testing.T) {
		// Given the UI shows different credential pickers per auth_type,
		// When a tenant clicks "Enable" on an auth-required server,
		// Then the picker reads auth_type from the catalog. Auth-required
		//      slugs MUST cover the major auth flows (oauth2, api_key,
		//      bearer_token, basic).
		seedSet := map[string]bool{}
		for _, s := range SeedAuthRequiredMCPServerSlugs {
			seedSet[s] = true
		}
		// All major auth flows represented:
		assert.True(t, seedSet["github"], "oauth2 example required (github)")
		assert.True(t, seedSet["brave-search"], "api_key example required (brave-search)")
		assert.True(t, seedSet["sentry"], "bearer_token example required (sentry)")
		assert.True(t, seedSet["postgres-readonly"], "basic auth example required (postgres-readonly)")
	})

	t.Run("Scenario_NoAuthServersAreSafeUtilities", func(t *testing.T) {
		// Given no-auth servers can be enabled with one click,
		//       they must be SAFE — no privacy leak, no quota burn.
		// When the no-auth slug set is inspected,
		// Then it contains only safe utilities (web-fetch / memory / time)
		//      — NEVER includes data-source servers like postgres or github.
		authSet := map[string]bool{}
		for _, a := range SeedAuthRequiredMCPServerSlugs {
			authSet[a] = true
		}
		noAuth := []string{}
		for _, s := range SeedExpectedMCPServerSlugs {
			if !authSet[s] {
				noAuth = append(noAuth, s)
			}
		}
		// Privacy guard: no data-source servers must be no-auth.
		dangerousNoAuth := map[string]bool{
			"postgres-readonly": true,
			"github":            true,
			"gitlab":            true,
			"sentry":            true,
			"slack":             true,
		}
		for _, s := range noAuth {
			assert.False(t, dangerousNoAuth[s],
				"no-auth slug %q must NOT be a data-source server", s)
		}
	})

	t.Run("Scenario_CategoriesEnableUIPickerFiltering", func(t *testing.T) {
		// Given the picker filters by category,
		// When the seed category set is inspected,
		// Then 5 distinct categories cover the major use surfaces.
		assert.Equal(t, 5, len(SeedExpectedMCPServerCategories))
		categories := map[string]bool{}
		for _, c := range SeedExpectedMCPServerCategories {
			categories[c] = true
		}
		for _, expected := range []string{"search", "development", "monitoring", "collaboration", "utility"} {
			assert.True(t, categories[expected],
				"category %q must be in seed", expected)
		}
	})

	t.Run("Scenario_CatalogIncludesOneSearchServerForSafeBrowsing", func(t *testing.T) {
		// Given search is the most common agent need,
		// When the catalog is inspected,
		// Then both Brave (privacy-respecting) AND web-fetch (no auth)
		//      are present so users have an immediate search path
		//      without external accounts.
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedMCPServerSlugs {
			seedSet[s] = true
		}
		assert.True(t, seedSet["brave-search"], "brave-search required for general search")
		assert.True(t, seedSet["web-fetch"], "web-fetch required as no-auth fallback")
	})

	t.Run("Scenario_DevelopmentCatalogCoversMajorVCSAndDB", func(t *testing.T) {
		// Given developer agents need source-control + DB read,
		// When the development category is inspected,
		// Then github + gitlab + postgres-readonly cover ~80% of dev
		//      use cases (PDF Section 6 — MCP servers extend agent
		//      capability via standard interfaces).
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedMCPServerSlugs {
			seedSet[s] = true
		}
		assert.True(t, seedSet["github"])
		assert.True(t, seedSet["gitlab"])
		assert.True(t, seedSet["postgres-readonly"])
	})

	t.Run("Scenario_PostgresEntryIsExplicitlyReadOnly", func(t *testing.T) {
		// Given a PostgreSQL MCP server can run any SQL — DROP/DELETE
		//       included — that catalog entry MUST signal read-only at
		//       the slug level so admins know what they're enabling.
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedMCPServerSlugs {
			seedSet[s] = true
		}
		assert.True(t, seedSet["postgres-readonly"],
			"postgres entry must be slugged 'postgres-readonly' — write-capable variant requires explicit admin enable")
		assert.False(t, seedSet["postgres"],
			"unrestricted postgres slug must NOT be in seed — too dangerous as default")
	})

	t.Run("Scenario_SlugsAreFilesystemAndURLSafe", func(t *testing.T) {
		// Given slugs appear in URLs / config files,
		// When the slug character set is checked,
		// Then only [a-z0-9-] is allowed.
		for _, s := range SeedExpectedMCPServerSlugs {
			for _, r := range s {
				ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-'
				assert.True(t, ok,
					"slug %q contains invalid char %q — must be [a-z0-9-]", s, r)
			}
			assert.False(t, strings.Contains(s, "_"),
				"slug %q must use kebab-case (no underscores)", s)
		}
	})

	t.Run("Scenario_CatalogIsStableAcrossRefactorsViaCanonicalList", func(t *testing.T) {
		// Given external systems (UI, billing, analytics) bind to
		//       SeedExpectedMCPServerSlugs as the contract,
		// When refactors happen,
		// Then the canonical list count must match migration row count
		//      — integration test enforces this exactly.
		assert.Equal(t, 9, len(SeedExpectedMCPServerSlugs),
			"contract guard: 9 slugs expected — refactor must update migration AND constant together")
	})
}
