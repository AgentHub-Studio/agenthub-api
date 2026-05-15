package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCorePDADTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 6, len(SeedExpectedPDADTemplateSlugs))
	assert.Equal(t, 6, SeedExpectedPDADTemplateRowCount)
}

func TestCorePDADTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedPDADTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCorePDADTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedPDADTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_")
	}
}

func TestCorePDADTemplate_ReasonsMatchPERM006Enum(t *testing.T) {
	expected := map[string]bool{
		"rule_match": true, "hook_override": true,
		"prefilter_drop": true, "mode_block": true,
		"sandbox_violation": true, "rate_limit": true,
	}
	for _, r := range SeedExpectedPDADTemplateReasons {
		assert.True(t, expected[r], "reason %q outside PERM-006 enum", r)
	}
}

func TestCorePDADTemplate_RetryHintsMatchPERM006Enum(t *testing.T) {
	expected := map[string]bool{
		"not_retryable": true, "retry_with_different_input": true,
		"request_user_confirmation": true,
		"suggest_alternative_tool":  true, "wait_and_retry": true,
	}
	for _, h := range SeedExpectedPDADTemplateRetryHints {
		assert.True(t, expected[h], "hint %q outside PERM-006 enum", h)
	}
}

func TestCorePDADTemplate_TenantKindsAreClosedSet(t *testing.T) {
	for _, k := range SeedExpectedPDADTemplateTenantKinds {
		assert.Equal(t, "general", k)
	}
}

func TestCorePDADTemplate_AllPresetsRecommended(t *testing.T) {
	assert.ElementsMatch(t,
		SeedExpectedPDADTemplateSlugs,
		SeedRecommendedPDADTemplateSlugs)
}

func TestCorePDADTemplate_AdminReviewOnlyForModeChange(t *testing.T) {
	// Only dont_ask mode template requires review — session-wide impact.
	assert.Equal(t,
		[]string{"dontask-mode-confirm-required"},
		SeedAdminReviewPDADTemplateSlugs)
}
