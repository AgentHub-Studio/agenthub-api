package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCorePermissionMode_SlugsCount(t *testing.T) {
	assert.Equal(t, 5, len(SeedExpectedPermissionModeSlugs))
	assert.Equal(t, 5, SeedExpectedPermissionModeRowCount)
}

func TestCorePermissionMode_SlugsUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedPermissionModeSlugs {
		assert.False(t, seen[s], "duplicate slug %q", s)
		seen[s] = true
	}
}

func TestCorePermissionMode_AllSlugsMatchRegex(t *testing.T) {
	for _, s := range SeedExpectedPermissionModeSlugs {
		assert.True(t, SeedPermissionModeSlugRE.MatchString(s), "slug %q must match regex", s)
	}
}

func TestCorePermissionMode_RowCountEqualsSlugCount(t *testing.T) {
	assert.Equal(t, SeedExpectedPermissionModeRowCount, len(SeedExpectedPermissionModeSlugs))
}

func TestCorePermissionMode_DefaultSlugInSlugs(t *testing.T) {
	found := false
	for _, s := range SeedExpectedPermissionModeSlugs {
		if s == SeedPermissionModeDefaultSlug {
			found = true
		}
	}
	assert.True(t, found, "default must be in the slug list")
}

func TestCorePermissionMode_SafestSlugInSlugs(t *testing.T) {
	found := false
	for _, s := range SeedExpectedPermissionModeSlugs {
		if s == SeedPermissionModeSafestSlug {
			found = true
		}
	}
	assert.True(t, found, "plan must be in the slug list")
}

func TestCorePermissionMode_BypassSlugInSlugs(t *testing.T) {
	found := false
	for _, s := range SeedExpectedPermissionModeSlugs {
		if s == SeedPermissionModeBypassSlug {
			found = true
		}
	}
	assert.True(t, found, "bypassPermissions must be in the slug list")
}

func TestCorePermissionMode_GradientOrderImpliedBySlugs(t *testing.T) {
	// Canonical gradient order must be maintained in the slice
	expected := []string{"plan", "default", "acceptEdits", "auto", "bypassPermissions"}
	assert.Equal(t, expected, SeedExpectedPermissionModeSlugs)
}

func TestCorePermissionMode_AcceptEditsSlugInSlugs(t *testing.T) {
	found := false
	for _, s := range SeedExpectedPermissionModeSlugs {
		if s == "acceptEdits" {
			found = true
		}
	}
	assert.True(t, found, "acceptEdits must be in the slug list")
}

func TestCorePermissionMode_AutoSlugInSlugs(t *testing.T) {
	found := false
	for _, s := range SeedExpectedPermissionModeSlugs {
		if s == "auto" {
			found = true
		}
	}
	assert.True(t, found, "auto must be in the slug list")
}
