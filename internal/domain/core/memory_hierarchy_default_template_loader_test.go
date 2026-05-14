package core

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCoreMemoryHierarchyDefaultTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 8, len(SeedExpectedMemoryHierarchyDefaultTemplateSlugs))
	assert.Equal(t, 8, SeedExpectedMemoryHierarchyDefaultTemplateRowCount)
}

func TestCoreMemoryHierarchyDefaultTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedMemoryHierarchyDefaultTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCoreMemoryHierarchyDefaultTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedMemoryHierarchyDefaultTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_")
	}
}

func TestCoreMemoryHierarchyDefaultTemplate_ScopesAreCTX003Subset(t *testing.T) {
	// Only global + tenant make sense for fresh-tenant defaults.
	// session/user/agent are subject-bound (no defaults possible).
	expected := map[string]bool{"global": true, "tenant": true}
	for _, s := range SeedExpectedMemoryHierarchyDefaultTemplateScopes {
		assert.True(t, expected[s])
	}
	assert.Equal(t, 2, len(SeedExpectedMemoryHierarchyDefaultTemplateScopes))
}

func TestCoreMemoryHierarchyDefaultTemplate_TenantKindsAreClosedSet(t *testing.T) {
	expected := map[string]bool{
		"general": true, "regulated": true, "dev_local": true,
	}
	for _, k := range SeedExpectedMemoryHierarchyDefaultTemplateTenantKinds {
		assert.True(t, expected[k])
	}
}

func TestCoreMemoryHierarchyDefaultTemplate_RecommendedExcludesAdminReview(t *testing.T) {
	// Admin-review templates must not be recommended one-click — admin
	// must explicitly opt in.
	recSet := map[string]bool{}
	for _, s := range SeedRecommendedMemoryHierarchyDefaultTemplateSlugs {
		recSet[s] = true
	}
	for _, s := range SeedAdminReviewMemoryHierarchyDefaultTemplateSlugs {
		assert.False(t, recSet[s], "admin-review %q must not be recommended", s)
	}
}

func TestCoreMemoryHierarchyDefaultTemplate_AdminReviewSubsetIsCompliance(t *testing.T) {
	// Only compliance-mode requires admin review (audit implications).
	assert.Equal(t, []string{"tenant-compliance-mode"},
		SeedAdminReviewMemoryHierarchyDefaultTemplateSlugs)
}

func TestCoreMemoryHierarchyDefaultTemplate_MaxAgeReturnsDuration(t *testing.T) {
	tmpl := CoreMemoryHierarchyDefaultTemplate{MaxAgeSeconds: 60}
	assert.Equal(t, time.Minute, tmpl.MaxAge())

	never := CoreMemoryHierarchyDefaultTemplate{MaxAgeSeconds: 0}
	assert.Equal(t, time.Duration(0), never.MaxAge(), "0 seconds = never expire")
}
