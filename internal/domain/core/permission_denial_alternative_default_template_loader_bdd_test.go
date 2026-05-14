package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCorePDADTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantHasReadyPivotsForCommonDenials", func(t *testing.T) {
		// Given fresh tenants face common denials (Bash, execute-sql, ...)
		// and the LLM cannot guess what alternatives exist,
		// When admin enables denial-alternative templates,
		// Then 6 recommended pivots surface for the LLM to use.
		assert.Equal(t, 6, len(SeedRecommendedPDADTemplateSlugs))
	})

	t.Run("Scenario_BashFallsBackToSandboxedShell", func(t *testing.T) {
		// Given non-engineering tenants deny Bash but want the LLM to
		// still operate on tenant-controlled paths,
		// When admin uses bash-to-sandboxed-shell,
		// Then shell_sandboxed is the suggested pivot.
		assert.Contains(t, SeedExpectedPDADTemplateSlugs, "bash-to-sandboxed-shell")
	})

	t.Run("Scenario_SQLDirectFallsBackToDocumentSearch", func(t *testing.T) {
		// Given direct SQL is denied (knowledge-base tenant safety),
		// When admin uses execute-sql-to-document-search,
		// Then document_search backed by KB embeddings is the pivot.
		assert.Contains(t, SeedExpectedPDADTemplateSlugs, "execute-sql-to-document-search")
	})

	t.Run("Scenario_WriteFallsBackToEditOnNoNewFiles", func(t *testing.T) {
		// Given a hook denies Write (no new files policy),
		// When admin uses write-to-edit-fallback,
		// Then Edit is the narrower-surface pivot.
		assert.Contains(t, SeedExpectedPDADTemplateSlugs, "write-to-edit-fallback")
	})

	t.Run("Scenario_RateLimitHasNoAlternateButWaitHint", func(t *testing.T) {
		// Given http_fetch is rate-limited, suggesting a pivot would
		// defeat the limit,
		// When admin uses http-fetch-rate-limited-wait,
		// Then alternatives are empty and retry=wait_and_retry.
		assert.Contains(t, SeedExpectedPDADTemplateSlugs, "http-fetch-rate-limited-wait")
	})

	t.Run("Scenario_MCPDroppedPivotsToBuiltin", func(t *testing.T) {
		// Given an MCP tool is pre-filter-dropped (server offline),
		// When admin uses mcp-tool-prefilter-drop-pivot,
		// Then a builtin counterpart is suggested.
		assert.Contains(t, SeedExpectedPDADTemplateSlugs, "mcp-tool-prefilter-drop-pivot")
	})

	t.Run("Scenario_DontAskModeRequiresUserConfirmation", func(t *testing.T) {
		// Given dont_ask mode blocks a confirm-tier call,
		// When admin uses dontask-mode-confirm-required,
		// Then retry=request_user_confirmation tells LLM to ask user,
		// and admin review required (session-wide impact).
		assert.Contains(t, SeedExpectedPDADTemplateSlugs, "dontask-mode-confirm-required")
		set := map[string]bool{}
		for _, s := range SeedAdminReviewPDADTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["dontask-mode-confirm-required"])
	})

	t.Run("Scenario_ReasonsMatchPERM006EnumByteForByte", func(t *testing.T) {
		// Given PERM-006 PermissionDenialReason has 6 values,
		// When seed declares target_reason,
		// Then every declared label is valid PERM-006 (no mapping table
		// at runtime).
		perm006 := map[string]bool{
			"rule_match": true, "hook_override": true,
			"prefilter_drop": true, "mode_block": true,
			"sandbox_violation": true, "rate_limit": true,
		}
		for _, r := range SeedExpectedPDADTemplateReasons {
			assert.True(t, perm006[r], "label %q not in PERM-006", r)
		}
	})

	t.Run("Scenario_RetryHintsMatchPERM006EnumByteForByte", func(t *testing.T) {
		// Given PERM-006 PermissionRetryHint has 5 values,
		// When seed declares target_retry_hint,
		// Then every declared label is valid PERM-006.
		perm006 := map[string]bool{
			"not_retryable": true, "retry_with_different_input": true,
			"request_user_confirmation": true,
			"suggest_alternative_tool":  true, "wait_and_retry": true,
		}
		for _, h := range SeedExpectedPDADTemplateRetryHints {
			assert.True(t, perm006[h], "label %q not in PERM-006", h)
		}
	})

	t.Run("Scenario_AdminReviewExcludesSafeAlternates", func(t *testing.T) {
		// Given alternates that just swap a tool for a safer one have
		// no session-wide impact,
		// When admin checks admin-review subset,
		// Then only the mode-change template requires review.
		assert.Equal(t, 1, len(SeedAdminReviewPDADTemplateSlugs))
	})
}
