package agentic_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- ParseDeepLink ---

func TestParseDeepLink_BasicOpen(t *testing.T) {
	action, err := agentic.ParseDeepLink("agenthub://open")
	require.NoError(t, err)
	assert.Equal(t, "", action.Query)
	assert.Equal(t, "", action.CWD)
	assert.Equal(t, "", action.Repo)
}

func TestParseDeepLink_WithQuery(t *testing.T) {
	action, err := agentic.ParseDeepLink("agenthub://open?q=hello+world")
	require.NoError(t, err)
	assert.Equal(t, "hello world", action.Query)
}

func TestParseDeepLink_WithCWD(t *testing.T) {
	action, err := agentic.ParseDeepLink("agenthub://open?cwd=/path/to/project")
	require.NoError(t, err)
	assert.Equal(t, "/path/to/project", action.CWD)
}

func TestParseDeepLink_WithRepo(t *testing.T) {
	action, err := agentic.ParseDeepLink("agenthub://open?repo=owner/repo")
	require.NoError(t, err)
	assert.Equal(t, "owner/repo", action.Repo)
}

func TestParseDeepLink_AllParams(t *testing.T) {
	action, err := agentic.ParseDeepLink("agenthub://open?q=fix+tests&cwd=/home/user&repo=acme/app")
	require.NoError(t, err)
	assert.Equal(t, "fix tests", action.Query)
	assert.Equal(t, "/home/user", action.CWD)
	assert.Equal(t, "acme/app", action.Repo)
}

func TestParseDeepLink_InvalidScheme(t *testing.T) {
	_, err := agentic.ParseDeepLink("http://open?q=test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected agenthub://")
}

func TestParseDeepLink_UnknownAction(t *testing.T) {
	_, err := agentic.ParseDeepLink("agenthub://close")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown deep link action")
}

func TestParseDeepLink_RelativeCWD(t *testing.T) {
	_, err := agentic.ParseDeepLink("agenthub://open?cwd=relative/path")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "absolute path")
}

func TestParseDeepLink_CWDWithControlChars(t *testing.T) {
	_, err := agentic.ParseDeepLink("agenthub://open?cwd=/path%0Ato")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "control characters")
}

func TestParseDeepLink_CWDTooLong(t *testing.T) {
	longPath := "/" + strings.Repeat("a", 5000)
	_, err := agentic.ParseDeepLink("agenthub://open?cwd=" + url_encode(longPath))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds")
}

func TestParseDeepLink_InvalidRepoSlug(t *testing.T) {
	_, err := agentic.ParseDeepLink("agenthub://open?repo=invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "owner/repo")
}

func TestParseDeepLink_QueryWithControlChars(t *testing.T) {
	_, err := agentic.ParseDeepLink("agenthub://open?q=hello%0Aworld")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "control characters")
}

func TestParseDeepLink_QueryTooLong(t *testing.T) {
	longQuery := strings.Repeat("a", 6000)
	_, err := agentic.ParseDeepLink("agenthub://open?q=" + longQuery)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds")
}

func TestParseDeepLink_WindowsCWD(t *testing.T) {
	action, err := agentic.ParseDeepLink("agenthub://open?cwd=C%3A%5Cusers%5Ctest")
	require.NoError(t, err)
	assert.Equal(t, `C:\users\test`, action.CWD)
}

func TestParseDeepLink_ValidRepoSlugs(t *testing.T) {
	for _, slug := range []string{"owner/repo", "my-org/my.repo", "user_1/proj-2"} {
		action, err := agentic.ParseDeepLink("agenthub://open?repo=" + slug)
		require.NoError(t, err, "slug: %s", slug)
		assert.Equal(t, slug, action.Repo)
	}
}

// --- BuildDeepLink ---

func TestBuildDeepLink_Empty(t *testing.T) {
	result := agentic.BuildDeepLink(agentic.DeepLinkAction{})
	assert.Equal(t, "agenthub://open", result)
}

func TestBuildDeepLink_WithQuery(t *testing.T) {
	result := agentic.BuildDeepLink(agentic.DeepLinkAction{Query: "hello world"})
	assert.Contains(t, result, "q=hello+world")
}

func TestBuildDeepLink_Roundtrip(t *testing.T) {
	original := agentic.DeepLinkAction{
		Query: "fix the bug",
		CWD:   "/home/user/project",
		Repo:  "acme/app",
	}
	uri := agentic.BuildDeepLink(original)
	parsed, err := agentic.ParseDeepLink(uri)
	require.NoError(t, err)
	assert.Equal(t, original.Query, parsed.Query)
	assert.Equal(t, original.CWD, parsed.CWD)
	assert.Equal(t, original.Repo, parsed.Repo)
}

// --- ContainsControlChars ---

func TestContainsControlChars_Clean(t *testing.T) {
	assert.False(t, agentic.ContainsControlChars("hello world"))
}

func TestContainsControlChars_Newline(t *testing.T) {
	assert.True(t, agentic.ContainsControlChars("hello\nworld"))
}

func TestContainsControlChars_Tab(t *testing.T) {
	assert.True(t, agentic.ContainsControlChars("hello\tworld"))
}

func TestContainsControlChars_NullByte(t *testing.T) {
	assert.True(t, agentic.ContainsControlChars("hello\x00world"))
}

func TestContainsControlChars_DEL(t *testing.T) {
	assert.True(t, agentic.ContainsControlChars("hello\x7Fworld"))
}

func TestContainsControlChars_Unicode(t *testing.T) {
	assert.False(t, agentic.ContainsControlChars("日本語"))
}

// url_encode is a test helper for URL encoding a string.
func url_encode(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "/", "%2F"), " ", "+")
}
