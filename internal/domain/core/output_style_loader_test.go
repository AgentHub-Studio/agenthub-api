package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreOutputStyle_SeedExpectedSlugs_NoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, slug := range SeedExpectedOutputStyleSlugs {
		assert.False(t, seen[slug], "duplicate slug in seed expectations: %q", slug)
		seen[slug] = true
	}
}

func TestCoreOutputStyle_SeedExpectedSlugs_AllNonEmpty(t *testing.T) {
	for i, slug := range SeedExpectedOutputStyleSlugs {
		assert.NotEmpty(t, slug, "seed slug at index %d must be non-empty", i)
	}
}

func TestCoreOutputStyle_SeedExpectedSlugs_CanonicalCount(t *testing.T) {
	// 8 styles in canonical seed.
	assert.Equal(t, 8, len(SeedExpectedOutputStyleSlugs),
		"8 output styles in canonical seed (refactor must update migration too)")
}

func TestCoreOutputStyle_SeedExpectedFormats_AreDistinct(t *testing.T) {
	seen := map[string]bool{}
	for _, f := range SeedExpectedOutputStyleFormats {
		assert.False(t, seen[f], "duplicate format %q", f)
		seen[f] = true
	}
}

func TestCoreOutputStyle_SeedExpectedFormats_CanonicalCount(t *testing.T) {
	// markdown + json = 2 formats.
	assert.Equal(t, 2, len(SeedExpectedOutputStyleFormats),
		"2 output formats in canonical seed")
}

func TestCoreOutputStyle_SeedExpectedAudiences_AreDistinct(t *testing.T) {
	seen := map[string]bool{}
	for _, a := range SeedExpectedOutputStyleAudiences {
		assert.False(t, seen[a], "duplicate audience %q", a)
		seen[a] = true
	}
}

func TestCoreOutputStyle_SeedExpectedAudiences_CanonicalCount(t *testing.T) {
	// general / technical / executive / beginner = 4 audiences.
	assert.Equal(t, 4, len(SeedExpectedOutputStyleAudiences),
		"4 audiences in canonical seed")
}

func TestCoreOutputStyle_DefaultSlugIsInSeed(t *testing.T) {
	seedSet := map[string]bool{}
	for _, s := range SeedExpectedOutputStyleSlugs {
		seedSet[s] = true
	}
	assert.True(t, seedSet[SeedDefaultOutputStyleSlug],
		"default slug %q must appear in SeedExpectedOutputStyleSlugs",
		SeedDefaultOutputStyleSlug)
}

func TestCoreOutputStyle_DefaultIsConversational(t *testing.T) {
	// Conversational is the safest default for a multi-purpose web product.
	assert.Equal(t, "conversational", SeedDefaultOutputStyleSlug,
		"platform default must be 'conversational' (most user-friendly)")
}

func TestCoreOutputStyle_FormatAllowlist(t *testing.T) {
	// Only markdown / json — plain/html etc would surprise the renderer.
	allowed := map[string]bool{}
	for _, f := range SeedExpectedOutputStyleFormats {
		allowed[f] = true
	}
	assert.True(t, allowed["markdown"], "markdown format required")
	assert.True(t, allowed["json"], "json format required for tool pipelines")
	assert.False(t, allowed["plain"], "plain not in allowlist (use markdown)")
	assert.False(t, allowed["html"], "html not in allowlist (XSS risk)")
}
