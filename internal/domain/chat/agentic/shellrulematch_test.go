package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- PermissionRuleExtractPrefix ---

func TestPermissionRuleExtractPrefix_Legacy(t *testing.T) {
	p, ok := agentic.PermissionRuleExtractPrefix("npm:*")
	assert.True(t, ok)
	assert.Equal(t, "npm", p)
}

func TestPermissionRuleExtractPrefix_NotLegacy(t *testing.T) {
	_, ok := agentic.PermissionRuleExtractPrefix("npm install")
	assert.False(t, ok)
}

func TestPermissionRuleExtractPrefix_Wildcard(t *testing.T) {
	_, ok := agentic.PermissionRuleExtractPrefix("npm *")
	assert.False(t, ok)
}

func TestPermissionRuleExtractPrefix_EmptyPrefix(t *testing.T) {
	_, ok := agentic.PermissionRuleExtractPrefix(":*")
	assert.False(t, ok)
}

// --- HasWildcards ---

func TestHasWildcards_True(t *testing.T) {
	assert.True(t, agentic.HasWildcards("git *"))
}

func TestHasWildcards_LegacySuffix(t *testing.T) {
	assert.False(t, agentic.HasWildcards("npm:*"))
}

func TestHasWildcards_EscapedStar(t *testing.T) {
	assert.False(t, agentic.HasWildcards(`echo \*`))
}

func TestHasWildcards_DoubleBackslashStar(t *testing.T) {
	// \\\* → two backslashes (even) + star = unescaped star
	assert.True(t, agentic.HasWildcards(`echo \\*`))
}

func TestHasWildcards_NoWildcard(t *testing.T) {
	assert.False(t, agentic.HasWildcards("ls -la"))
}

func TestHasWildcards_Empty(t *testing.T) {
	assert.False(t, agentic.HasWildcards(""))
}

// --- MatchWildcardPattern ---

func TestMatchWildcardPattern_SimpleWildcard(t *testing.T) {
	assert.True(t, agentic.MatchWildcardPattern("git *", "git add .", false))
}

func TestMatchWildcardPattern_TrailingWildcardOptional(t *testing.T) {
	// "git *" should match bare "git" (trailing space+args optional)
	assert.True(t, agentic.MatchWildcardPattern("git *", "git", false))
}

func TestMatchWildcardPattern_MiddleWildcard(t *testing.T) {
	assert.True(t, agentic.MatchWildcardPattern("npm * build", "npm run build", false))
}

func TestMatchWildcardPattern_MultipleWildcards(t *testing.T) {
	assert.True(t, agentic.MatchWildcardPattern("* run *", "npm run build", false))
}

func TestMatchWildcardPattern_MultipleWildcardsNoTrailingOptional(t *testing.T) {
	// With multiple wildcards, trailing " *" is NOT optional
	assert.False(t, agentic.MatchWildcardPattern("* run *", "npm run", false))
}

func TestMatchWildcardPattern_EscapedStar(t *testing.T) {
	// \* matches literal *
	assert.True(t, agentic.MatchWildcardPattern(`echo \*`, "echo *", false))
	assert.False(t, agentic.MatchWildcardPattern(`echo \*`, "echo hello", false))
}

func TestMatchWildcardPattern_EscapedBackslash(t *testing.T) {
	assert.True(t, agentic.MatchWildcardPattern(`echo \\`, `echo \`, false))
}

func TestMatchWildcardPattern_NoMatch(t *testing.T) {
	assert.False(t, agentic.MatchWildcardPattern("git *", "npm install", false))
}

func TestMatchWildcardPattern_ExactMatch(t *testing.T) {
	assert.True(t, agentic.MatchWildcardPattern("ls", "ls", false))
	assert.False(t, agentic.MatchWildcardPattern("ls", "ls -la", false))
}

func TestMatchWildcardPattern_CaseInsensitive(t *testing.T) {
	assert.True(t, agentic.MatchWildcardPattern("GIT *", "git add", true))
	assert.False(t, agentic.MatchWildcardPattern("GIT *", "git add", false))
}

func TestMatchWildcardPattern_DotInPattern(t *testing.T) {
	// . should be literal, not regex wildcard
	assert.True(t, agentic.MatchWildcardPattern("file.txt", "file.txt", false))
	assert.False(t, agentic.MatchWildcardPattern("file.txt", "fileXtxt", false))
}

func TestMatchWildcardPattern_Newlines(t *testing.T) {
	// Wildcards match across newlines (dotAll)
	assert.True(t, agentic.MatchWildcardPattern("echo *", "echo hello\nworld", false))
}

func TestMatchWildcardPattern_Trimming(t *testing.T) {
	assert.True(t, agentic.MatchWildcardPattern("  git *  ", "git add", false))
}

// --- ParseShellPermissionRule ---

func TestParseShellPermissionRule_Exact(t *testing.T) {
	r := agentic.ParseShellPermissionRule("ls -la")
	assert.Equal(t, agentic.ShellRuleExact, r.Type)
	assert.Equal(t, "ls -la", r.Command)
}

func TestParseShellPermissionRule_Prefix(t *testing.T) {
	r := agentic.ParseShellPermissionRule("npm:*")
	assert.Equal(t, agentic.ShellRulePrefix, r.Type)
	assert.Equal(t, "npm", r.Prefix)
}

func TestParseShellPermissionRule_Wildcard(t *testing.T) {
	r := agentic.ParseShellPermissionRule("git *")
	assert.Equal(t, agentic.ShellRuleWildcard, r.Type)
	assert.Equal(t, "git *", r.Pattern)
}

// --- MatchShellPermissionRule ---

func TestMatchShellPermissionRule_ExactMatch(t *testing.T) {
	r := agentic.ParseShellPermissionRule("ls -la")
	assert.True(t, agentic.MatchShellPermissionRule(r, "ls -la"))
	assert.False(t, agentic.MatchShellPermissionRule(r, "ls"))
}

func TestMatchShellPermissionRule_PrefixMatch(t *testing.T) {
	r := agentic.ParseShellPermissionRule("npm:*")
	assert.True(t, agentic.MatchShellPermissionRule(r, "npm install"))
	assert.True(t, agentic.MatchShellPermissionRule(r, "npm"))
	assert.False(t, agentic.MatchShellPermissionRule(r, "npx run"))
}

func TestMatchShellPermissionRule_WildcardMatch(t *testing.T) {
	r := agentic.ParseShellPermissionRule("docker * build")
	assert.True(t, agentic.MatchShellPermissionRule(r, "docker image build"))
	assert.False(t, agentic.MatchShellPermissionRule(r, "docker pull"))
}

func TestMatchShellPermissionRule_PrefixBare(t *testing.T) {
	r := agentic.ParseShellPermissionRule("git:*")
	assert.True(t, agentic.MatchShellPermissionRule(r, "git"))
}

func TestMatchShellPermissionRule_PrefixNoPartial(t *testing.T) {
	r := agentic.ParseShellPermissionRule("git:*")
	// "gitable" does not start with "git " and is not "git"
	assert.False(t, agentic.MatchShellPermissionRule(r, "gitable"))
}
