package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

const sampleGitConfig = `[core]
	repositoryformatversion = 0
	filemode = true
	bare = false
	logallrefupdates = true
[remote "origin"]
	url = https://github.com/org/repo.git
	fetch = +refs/heads/*:refs/remotes/origin/*
[remote "upstream"]
	url = https://github.com/upstream/repo.git
	fetch = +refs/heads/*:refs/remotes/upstream/*
[branch "main"]
	remote = origin
	merge = refs/heads/main
[branch "develop"]
	remote = origin
	merge = refs/heads/develop
[user]
	name = Test User
	email = test@example.com
`

func TestParseGitConfigString_SimpleSection(t *testing.T) {
	val := agentic.ParseGitConfigString(sampleGitConfig, "core", "", "filemode")
	assert.Equal(t, "true", val)
}

func TestParseGitConfigString_WithSubsection(t *testing.T) {
	val := agentic.ParseGitConfigString(sampleGitConfig, "remote", "origin", "url")
	assert.Equal(t, "https://github.com/org/repo.git", val)
}

func TestParseGitConfigString_DifferentSubsection(t *testing.T) {
	val := agentic.ParseGitConfigString(sampleGitConfig, "remote", "upstream", "url")
	assert.Equal(t, "https://github.com/upstream/repo.git", val)
}

func TestParseGitConfigString_BranchRemote(t *testing.T) {
	val := agentic.ParseGitConfigString(sampleGitConfig, "branch", "main", "remote")
	assert.Equal(t, "origin", val)
}

func TestParseGitConfigString_CaseInsensitiveSection(t *testing.T) {
	val := agentic.ParseGitConfigString(sampleGitConfig, "CORE", "", "filemode")
	assert.Equal(t, "true", val)
}

func TestParseGitConfigString_CaseInsensitiveKey(t *testing.T) {
	val := agentic.ParseGitConfigString(sampleGitConfig, "core", "", "FileMode")
	assert.Equal(t, "true", val)
}

func TestParseGitConfigString_NotFound(t *testing.T) {
	val := agentic.ParseGitConfigString(sampleGitConfig, "core", "", "nonexistent")
	assert.Equal(t, "", val)
}

func TestParseGitConfigString_SubsectionNotFound(t *testing.T) {
	val := agentic.ParseGitConfigString(sampleGitConfig, "remote", "nonexistent", "url")
	assert.Equal(t, "", val)
}

func TestParseGitConfigString_UserEmail(t *testing.T) {
	val := agentic.ParseGitConfigString(sampleGitConfig, "user", "", "email")
	assert.Equal(t, "test@example.com", val)
}

// --- Escapes ---

func TestParseGitConfigString_QuotedValue(t *testing.T) {
	config := `[section]
	key = "hello world"
`
	val := agentic.ParseGitConfigString(config, "section", "", "key")
	assert.Equal(t, "hello world", val)
}

func TestParseGitConfigString_EscapeInQuotes(t *testing.T) {
	config := `[section]
	key = "line1\nline2"
`
	val := agentic.ParseGitConfigString(config, "section", "", "key")
	assert.Equal(t, "line1\nline2", val)
}

func TestParseGitConfigString_InlineComment(t *testing.T) {
	config := `[section]
	key = value # comment
`
	val := agentic.ParseGitConfigString(config, "section", "", "key")
	assert.Equal(t, "value", val)
}

func TestParseGitConfigString_InlineCommentSemicolon(t *testing.T) {
	config := `[section]
	key = value ; comment
`
	val := agentic.ParseGitConfigString(config, "section", "", "key")
	assert.Equal(t, "value", val)
}

func TestParseGitConfigString_SubsectionWithEscape(t *testing.T) {
	config := `[remote "path/with\\slash"]
	url = test
`
	val := agentic.ParseGitConfigString(config, "remote", `path/with\slash`, "url")
	assert.Equal(t, "test", val)
}

// --- ParseAllGitConfigValues ---

func TestParseAllGitConfigValues_Multiple(t *testing.T) {
	vals := agentic.ParseAllGitConfigValues(sampleGitConfig, "remote", "origin", "fetch")
	assert.Equal(t, []string{"+refs/heads/*:refs/remotes/origin/*"}, vals)
}

func TestParseAllGitConfigValues_NotFound(t *testing.T) {
	vals := agentic.ParseAllGitConfigValues(sampleGitConfig, "section", "", "key")
	assert.Nil(t, vals)
}

// --- ListGitConfigSections ---

func TestListGitConfigSections_All(t *testing.T) {
	sections := agentic.ListGitConfigSections(sampleGitConfig)
	assert.Contains(t, sections, "core")
	assert.Contains(t, sections, `remote "origin"`)
	assert.Contains(t, sections, `branch "main"`)
	assert.Contains(t, sections, "user")
}

func TestListGitConfigSections_Empty(t *testing.T) {
	sections := agentic.ListGitConfigSections("")
	assert.Nil(t, sections)
}

// --- Edge cases ---

func TestParseGitConfigString_Empty(t *testing.T) {
	val := agentic.ParseGitConfigString("", "section", "", "key")
	assert.Equal(t, "", val)
}

func TestParseGitConfigString_CommentsOnly(t *testing.T) {
	config := "# just comments\n; and more\n"
	val := agentic.ParseGitConfigString(config, "section", "", "key")
	assert.Equal(t, "", val)
}

func TestParseGitConfigString_TrailingWhitespace(t *testing.T) {
	config := "[section]\n\tkey = value   \n"
	val := agentic.ParseGitConfigString(config, "section", "", "key")
	assert.Equal(t, "value", val)
}
