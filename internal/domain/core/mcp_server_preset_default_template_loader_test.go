package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreMCPPreset_SlugsCount(t *testing.T) {
	assert.Equal(t, 6, len(SeedExpectedMCPPresetSlugs))
	assert.Equal(t, 6, SeedExpectedMCPPresetRowCount)
}

func TestCoreMCPPreset_SlugsUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedMCPPresetSlugs {
		assert.False(t, seen[s], "duplicate slug %q", s)
		seen[s] = true
	}
}

func TestCoreMCPPreset_AllSlugsMatchKebabRegex(t *testing.T) {
	for _, s := range SeedExpectedMCPPresetSlugs {
		assert.True(t, SeedMCPPresetSlugRE.MatchString(s), "slug %q must match kebab regex", s)
	}
}

func TestCoreMCPPreset_RowCountEqualsSlugCount(t *testing.T) {
	assert.Equal(t, SeedExpectedMCPPresetRowCount, len(SeedExpectedMCPPresetSlugs))
}

func TestCoreMCPPreset_StdioAndHTTPSlugsDisjoint(t *testing.T) {
	stdioSet := map[string]bool{}
	for _, s := range SeedMCPStdioPresetSlugs {
		stdioSet[s] = true
	}
	for _, s := range SeedMCPHTTPPresetSlugs {
		assert.False(t, stdioSet[s], "slug %q must not appear in both stdio and http lists", s)
	}
}

func TestCoreMCPPreset_StdioAndHTTPCoversAll(t *testing.T) {
	total := len(SeedMCPStdioPresetSlugs) + len(SeedMCPHTTPPresetSlugs)
	assert.Equal(t, SeedExpectedMCPPresetRowCount, total)
}

func TestCoreMCPPreset_AutoStartSubsetOfAll(t *testing.T) {
	all := map[string]bool{}
	for _, s := range SeedExpectedMCPPresetSlugs {
		all[s] = true
	}
	for _, s := range SeedMCPAutoStartSlugs {
		assert.True(t, all[s], "auto-start slug %q not in all slugs", s)
	}
}

func TestCoreMCPPreset_RemoteSSENotAutoStart(t *testing.T) {
	for _, s := range SeedMCPAutoStartSlugs {
		assert.NotEqual(t, "remote-sse", s, "remote-sse must not be auto-start (no subprocess)")
	}
}

func TestCoreMCPPreset_FilesystemInAutoStart(t *testing.T) {
	found := false
	for _, s := range SeedMCPAutoStartSlugs {
		if s == "filesystem" {
			found = true
		}
	}
	assert.True(t, found)
}
