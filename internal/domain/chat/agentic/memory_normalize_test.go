package agentic

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Tests for normalizeKey (ACT-F3-10, P-C333-1).

func TestNormalizeKey_Lowercase(t *testing.T) {
	// CamelCase is lowercased as-is (no camelCase splitting)
	assert.Equal(t, "userprefs", normalizeKey("UserPrefs"))
}

func TestNormalizeKey_SpacesToHyphens(t *testing.T) {
	assert.Equal(t, "my-memory-key", normalizeKey("My Memory Key"))
}

func TestNormalizeKey_SpecialCharsToHyphens(t *testing.T) {
	assert.Equal(t, "user-email", normalizeKey("user.email"))
}

func TestNormalizeKey_CollapseMultipleHyphens(t *testing.T) {
	assert.Equal(t, "a-b", normalizeKey("a___b"))
}

func TestNormalizeKey_Empty_ReturnsGeneral(t *testing.T) {
	assert.Equal(t, "general", normalizeKey(""))
}

func TestNormalizeKey_OnlySpecialChars_ReturnsGeneral(t *testing.T) {
	assert.Equal(t, "general", normalizeKey("!!!"))
}

func TestNormalizeKey_TruncatesAt100Chars(t *testing.T) {
	long := strings.Repeat("a", 200)
	result := normalizeKey(long)
	assert.Equal(t, 100, len(result))
}

func TestNormalizeKey_AlreadyNormalized(t *testing.T) {
	assert.Equal(t, "my-key", normalizeKey("my-key"))
}

func TestNormalizeKey_NumericAllowed(t *testing.T) {
	assert.Equal(t, "key-123", normalizeKey("key 123"))
}

func TestNormalizeKey_TrimTrailingHyphen(t *testing.T) {
	assert.Equal(t, "my-key", normalizeKey("my-key---"))
}
