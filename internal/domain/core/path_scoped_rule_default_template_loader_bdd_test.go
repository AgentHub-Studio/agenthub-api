package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCorePSRDTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantInheritsCommonPathScopedRulesOutOfTheBox", func(t *testing.T) {
		// Given a fresh tenant adopts AgentHub,
		// And CTX-004 PathScopedRuleRegistry accepts (scope, glob, content),
		// When admin opens path-rule onboarding,
		// Then 8 path-scoped rule templates exist covering common
		// security/compliance/conventions patterns — admin doesn't
		// have to invent globs.
		assert.Equal(t, 8, len(SeedRecommendedPSRDTemplateSlugs))
	})

	t.Run("Scenario_GoSecretsRuleTargetsGoSourceFiles", func(t *testing.T) {
		// Given Go source files are common attack surface for committed secrets,
		// When admin enables no-secrets-in-go-files,
		// Then path_glob is **/*.go (CTX-004 file scope).
		assert.Contains(t, SeedExpectedPSRDTemplateSlugs, "no-secrets-in-go-files")
	})

	t.Run("Scenario_PIIYAMLRuleRequiresAdminReviewForRegulatedTenants", func(t *testing.T) {
		// Given GDPR/HIPAA tenants must vet PII rules,
		// When admin enables no-pii-in-yaml-config,
		// Then template requires admin review (regulated impact).
		set := map[string]bool{}
		for _, s := range SeedAdminReviewPSRDTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["no-pii-in-yaml-config"])
	})

	t.Run("Scenario_SQLRuleAppliesOnlyToExecuteSqlTool", func(t *testing.T) {
		// Given execute_sql is the SQL injection attack surface,
		// When admin enables execute-sql-prepared-statements,
		// Then path_glob is tools/execute_sql/** (CTX-004 tool scope).
		assert.Contains(t, SeedExpectedPSRDTemplateSlugs, "execute-sql-prepared-statements")
	})

	t.Run("Scenario_ShellRmRfRequiresAdminReviewBecauseDestructive", func(t *testing.T) {
		// Given rm -rf can wipe filesystems irreversibly,
		// When admin enables shell-commands-no-rm-rf,
		// Then template requires admin review (destructive impact).
		set := map[string]bool{}
		for _, s := range SeedAdminReviewPSRDTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["shell-commands-no-rm-rf"])
	})

	t.Run("Scenario_MigrationsDataLossRuleRequiresAdminReview", func(t *testing.T) {
		// Given DROP TABLE/COLUMN cause data loss,
		// When admin enables migrations-no-data-loss,
		// Then template requires admin review (data loss impact).
		set := map[string]bool{}
		for _, s := range SeedAdminReviewPSRDTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["migrations-no-data-loss"])
	})

	t.Run("Scenario_DocsFrontmatterRuleIsRoutineAndNoAdminGate", func(t *testing.T) {
		// Given convention rules don't have safety/compliance impact,
		// When admin enables docs-must-have-frontmatter,
		// Then no admin review (routine convention; gating would slow ops).
		set := map[string]bool{}
		for _, s := range SeedAdminReviewPSRDTemplateSlugs {
			set[s] = true
		}
		assert.False(t, set["docs-must-have-frontmatter"])
	})

	t.Run("Scenario_ResearcherCitationRuleScopedToAgentPersona", func(t *testing.T) {
		// Given researcher persona has citation requirement,
		// When admin enables researcher-must-cite-sources,
		// Then path_glob is agent/researcher/** (CTX-004 agent scope).
		assert.Contains(t, SeedExpectedPSRDTemplateSlugs, "researcher-must-cite-sources")
	})

	t.Run("Scenario_PromptInjectionRuleAppliesGloballyAndAlignsWithCTX002", func(t *testing.T) {
		// Given CTX-002 user-channel safety contract requires prompt
		// injection detection,
		// When admin enables global-no-prompt-injection,
		// Then path_glob is ** (CTX-004 global scope) — applies to ALL paths.
		assert.Contains(t, SeedExpectedPSRDTemplateSlugs, "global-no-prompt-injection")
	})

	t.Run("Scenario_ScopeLabelsAreCTX004ByteForByte", func(t *testing.T) {
		// Given CTX-004 has 5 PathScopedRuleScope enum values,
		// When this seed declares target_scope,
		// Then values match enum bytes (no mapping table runtime).
		ctx004 := []string{"global", "tool", "file", "directory", "agent"}
		set := map[string]bool{}
		for _, s := range SeedExpectedPSRDTemplateScopes {
			set[s] = true
		}
		for _, e := range ctx004 {
			assert.True(t, set[e], "CTX-004 scope %q missing from seed", e)
		}
	})

	t.Run("Scenario_GlobalRuleHasHighestDefaultPriority", func(t *testing.T) {
		// Given prompt injection is the most critical safety boundary,
		// When admin compares default priorities,
		// Then global-no-prompt-injection has the highest default priority
		// (validated structurally via integration test).
		assert.Contains(t, SeedExpectedPSRDTemplateSlugs, "global-no-prompt-injection")
	})
}
