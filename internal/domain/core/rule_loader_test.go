package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Unit tests for the CoreRule contract that don't require a database.

func TestCoreRule_SeedExpectedSlugs_NoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, slug := range SeedExpectedRuleSlugs {
		assert.False(t, seen[slug], "duplicate slug in seed expectations: %q", slug)
		seen[slug] = true
	}
}

func TestCoreRule_SeedExpectedSlugs_AllNonEmpty(t *testing.T) {
	for i, slug := range SeedExpectedRuleSlugs {
		assert.NotEmpty(t, slug, "seed slug at index %d must be non-empty", i)
	}
}

func TestCoreRule_SeedExpectedSlugs_AreLowercaseKebab(t *testing.T) {
	for _, slug := range SeedExpectedRuleSlugs {
		for _, r := range slug {
			isValid := (r >= 'a' && r <= 'z') ||
				(r >= '0' && r <= '9') ||
				r == '-'
			assert.True(t, isValid,
				"slug %q contains invalid char %q (must be lowercase letters, digits, or '-')",
				slug, r)
		}
	}
}

func TestCoreRule_SeedExpectedCategories_FiniteAndStable(t *testing.T) {
	// 4 categories matches the admin UI grouping.
	assert.Len(t, SeedExpectedRuleCategories, 4,
		"4 stable rule categories — adding new requires admin UI update")
}

func TestCoreRule_SeedExpectedSlugs_CanonicalCount(t *testing.T) {
	// 13 rules across 4 categories: safety(4) + quality(3) + behavior(3) + privacy(3).
	assert.Equal(t, 13, len(SeedExpectedRuleSlugs),
		"13 rules in canonical web seed (refactor must update seed migration too)")
}

func TestCoreRule_AdminOnlyDisableSlugs_AreAllInSeed(t *testing.T) {
	// Every admin-only-disable slug must appear in the canonical seed list —
	// otherwise the runtime authorization gate would reference a non-existent rule.
	seedSet := map[string]bool{}
	for _, s := range SeedExpectedRuleSlugs {
		seedSet[s] = true
	}
	for _, adminSlug := range SeedAdminOnlyDisableSlugs {
		assert.True(t, seedSet[adminSlug],
			"admin-only slug %q must also appear in SeedExpectedRuleSlugs", adminSlug)
	}
}

func TestCoreRule_AdminOnlyDisableSlugs_CoverAllSafetyAndPrivacy(t *testing.T) {
	// Safety + privacy categories MUST be entirely admin-only-disable —
	// tenants cannot opt out of these without admin role.
	expected := []string{
		// safety
		"safety-no-secret-disclosure",
		"safety-confirm-irreversible",
		"safety-decline-illegal",
		"safety-escalate-uncertain",
		// privacy
		"privacy-redact-pii",
		"privacy-minimize-collection",
		"privacy-no-cross-tenant-leak",
	}
	assert.Equal(t, len(expected), len(SeedAdminOnlyDisableSlugs),
		"admin-only set must equal the union of safety + privacy slugs")
	adminSet := map[string]bool{}
	for _, s := range SeedAdminOnlyDisableSlugs {
		adminSet[s] = true
	}
	for _, slug := range expected {
		assert.True(t, adminSet[slug], "expected admin-only slug %q is missing", slug)
	}
}

func TestCoreRule_CategoriesAreReferencedBySlugPrefixes(t *testing.T) {
	// Convention: every slug starts with its category prefix
	// (safety-*, quality-*, behavior-*, privacy-*) — tests this so a
	// typo'd category in the migration is caught.
	prefixes := map[string]bool{}
	for _, c := range SeedExpectedRuleCategories {
		prefixes[c+"-"] = true
	}
	for _, slug := range SeedExpectedRuleSlugs {
		matched := false
		for prefix := range prefixes {
			if len(slug) > len(prefix) && slug[:len(prefix)] == prefix {
				matched = true
				break
			}
		}
		assert.True(t, matched,
			"slug %q must start with one of the category prefixes %v", slug, SeedExpectedRuleCategories)
	}
}
