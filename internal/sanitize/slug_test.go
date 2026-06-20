package sanitize_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/sanitize"
)

func TestValidSlug_CanonicalPattern(t *testing.T) {
	valid := []string{"a", "ab", "agent-1", strings.Repeat("a", sanitize.SlugMaxLength)}
	for _, slug := range valid {
		assert.True(t, sanitize.ValidSlug(slug), slug)
	}

	invalid := []string{"-agent", "agent-", "Agent", "agent_1", "agent.1", strings.Repeat("a", sanitize.SlugMaxLength+1)}
	for _, slug := range invalid {
		assert.False(t, sanitize.ValidSlug(slug), slug)
	}
}

func TestToSlug_UsesCanonicalFormat(t *testing.T) {
	slug := sanitize.ToSlug("HTTP.User Lookup!", "tool")
	assert.Equal(t, "http-user-lookup", slug)
	assert.True(t, sanitize.ValidSlug(slug))
}

func TestToSlug_DropsNonCanonicalLetters(t *testing.T) {
	slug := sanitize.ToSlug("Café API", "tool")
	assert.Equal(t, "caf-api", slug)
	assert.True(t, sanitize.ValidSlug(slug))
}

func TestToSlug_TruncatesToCanonicalMaxLength(t *testing.T) {
	slug := sanitize.ToSlug(strings.Repeat("a", sanitize.SlugMaxLength+10), "tool")
	assert.Len(t, slug, sanitize.SlugMaxLength)
	assert.True(t, sanitize.ValidSlug(slug))
}
