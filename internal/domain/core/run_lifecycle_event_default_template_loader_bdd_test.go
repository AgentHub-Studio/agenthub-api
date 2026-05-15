package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for RunLifecycleEventDefaultTemplate seed.
// Lifecycle event templates define hooks that fire during agent run execution
// (RunStarted, RunComplete, ContextWindowAlert, KnowledgeBaseQueried) and route
// to handler kinds (webhook, notification, audit).

func TestBDD_AhCoreRunLifecycleEventSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantReceivesSixLifecycleEventTemplates", func(t *testing.T) {
		// Given a new tenant needs default event wiring for agent runs
		// When the lifecycle event template catalog is loaded
		// Then exactly 6 templates are seeded
		assert.Equal(t, 6, SeedExpectedRLETRowCount)
		assert.Equal(t, 6, len(SeedExpectedRLETSlugs))
	})

	t.Run("Scenario_ThreeDefaultEnabledTemplatesForFreshTenants", func(t *testing.T) {
		// Given a fresh tenant should have basic audit coverage without
		// requiring configuration of external webhooks or notifications
		// When enabled-by-default templates are listed
		// Then exactly 3 templates are enabled: run-started-audit-log,
		// context-window-alert-compact, knowledge-base-queried-audit
		assert.Equal(t, 3, len(SeedRLETEnabledByDefaultSlugs))
		assert.Contains(t, SeedRLETEnabledByDefaultSlugs, "run-started-audit-log")
		assert.Contains(t, SeedRLETEnabledByDefaultSlugs, "context-window-alert-compact")
		assert.Contains(t, SeedRLETEnabledByDefaultSlugs, "knowledge-base-queried-audit")
	})

	t.Run("Scenario_FourHookEventsAreCoveredByTemplates", func(t *testing.T) {
		// Given the run execution loop emits four distinct event types
		// When the closed set of hook events is inspected
		// Then all four — RunStarted, RunComplete, ContextWindowAlert,
		// KnowledgeBaseQueried — are represented
		assert.Equal(t, 4, len(SeedRLETHookEvents))
		eventSet := map[string]bool{}
		for _, e := range SeedRLETHookEvents {
			eventSet[e] = true
		}
		assert.True(t, eventSet["RunStarted"])
		assert.True(t, eventSet["RunComplete"])
		assert.True(t, eventSet["ContextWindowAlert"])
		assert.True(t, eventSet["KnowledgeBaseQueried"])
	})

	t.Run("Scenario_ThreeHandlerKindsCoverAllDeliveryMechanisms", func(t *testing.T) {
		// Given events are delivered via webhook, notification, and audit channels
		// When the closed set of handler kinds is inspected
		// Then exactly webhook, notification, and audit are supported
		assert.Equal(t, 3, len(SeedRLETHandlerKinds))
		kindSet := map[string]bool{}
		for _, k := range SeedRLETHandlerKinds {
			kindSet[k] = true
		}
		assert.True(t, kindSet["webhook"])
		assert.True(t, kindSet["notification"])
		assert.True(t, kindSet["audit"])
	})

	t.Run("Scenario_AllSlugsMustBeKebabCase", func(t *testing.T) {
		// Given naming conventions require kebab-case slugs
		// When each lifecycle event template slug is validated
		// Then every slug matches ^[a-z0-9][a-z0-9-]*[a-z0-9]$
		for _, s := range SeedExpectedRLETSlugs {
			assert.True(t, RLETSlugRE.MatchString(s),
				"slug %q violates kebab-case pattern", s)
		}
	})

	t.Run("Scenario_DefaultEnabledSubsetIsStrictlySmaller", func(t *testing.T) {
		// Given not all templates should be active for new tenants
		// When the enabled-by-default count is compared to the full catalog
		// Then fewer templates are enabled by default than the total
		assert.Less(t, len(SeedRLETEnabledByDefaultSlugs), SeedExpectedRLETRowCount)
	})
}
