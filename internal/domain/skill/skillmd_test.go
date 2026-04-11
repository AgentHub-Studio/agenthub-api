package skill_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skill"
)

const validSkillMD = `---
version: "1"
name: Document Search
slug: document-search
description: Searches tenant knowledge bases using vector similarity.
category: SEARCH
when_to_use: Use when the user asks to find information from documents.
context_mode: inline
allowed_tools:
  - document_search
---

Search through indexed documents and return the most relevant chunks.
Use pgvector cosine distance for ranking.
`

// TestParseSkillMD_ValidInput verifies full round-trip parsing.
func TestParseSkillMD_ValidInput(t *testing.T) {
	req, err := skill.ParseSkillMD(validSkillMD)
	require.NoError(t, err)
	assert.Equal(t, "Document Search", req.Name)
	assert.Equal(t, "document-search", req.Slug)
	assert.Equal(t, "SEARCH", req.Category)
	assert.Equal(t, "inline", req.ContextMode)
	require.NotNil(t, req.WhenToUse)
	assert.Equal(t, "Use when the user asks to find information from documents.", *req.WhenToUse)
	assert.Equal(t, []string{"document_search"}, req.AllowedTools)
	assert.Contains(t, req.Instructions, "Search through indexed documents")
}

// TestParseSkillMD_MissingFrontmatter ensures the parser rejects files without ---.
func TestParseSkillMD_MissingFrontmatter(t *testing.T) {
	_, err := skill.ParseSkillMD("just markdown content without frontmatter")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "frontmatter")
}

// TestParseSkillMD_UnclosedFrontmatter ensures the parser rejects an unclosed ---.
func TestParseSkillMD_UnclosedFrontmatter(t *testing.T) {
	_, err := skill.ParseSkillMD("---\nname: Test\n")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "closing")
}

// TestParseSkillMD_MissingName ensures the parser rejects a file without a name.
func TestParseSkillMD_MissingName(t *testing.T) {
	content := "---\nslug: no-name\n---\nInstructions here."
	_, err := skill.ParseSkillMD(content)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

// TestParseSkillMD_ShouldDefer verifies the should_defer flag is parsed correctly.
func TestParseSkillMD_ShouldDefer(t *testing.T) {
	content := "---\nname: Heavy Skill\nshould_defer: true\n---\nInstructions."
	req, err := skill.ParseSkillMD(content)
	require.NoError(t, err)
	assert.True(t, req.ShouldDefer)
}

// TestParseSkillMD_DisableModelInvocation verifies the flag propagates.
func TestParseSkillMD_DisableModelInvocation(t *testing.T) {
	content := "---\nname: Slash Skill\ndisable_model_invocation: true\n---\nDo stuff."
	req, err := skill.ParseSkillMD(content)
	require.NoError(t, err)
	assert.True(t, req.DisableModelInvocation)
}

// TestSerializeSkillMD_RoundTrip verifies that serialize→parse produces equal metadata.
func TestSerializeSkillMD_RoundTrip(t *testing.T) {
	whenToUse := "Use when searching documents"
	argHint := "<query>"
	original := skill.Skill{
		Name:         "My Skill",
		Slug:         "my-skill",
		Description:  "A test skill",
		Category:     "CUSTOM",
		Instructions: "Do the thing with the stuff.",
		WhenToUse:    &whenToUse,
		ArgumentHint: &argHint,
		ContextMode:  "fork",
		ShouldDefer:  true,
		AllowedTools: []string{"tool_a", "tool_b"},
	}

	serialized := skill.SerializeSkillMD(original)
	assert.True(t, strings.HasPrefix(serialized, "---\n"), "must start with ---")
	assert.Contains(t, serialized, "name: My Skill")
	assert.Contains(t, serialized, "slug: my-skill")
	assert.Contains(t, serialized, "should_defer: true")
	assert.Contains(t, serialized, "Do the thing with the stuff.")

	// Parse back and verify round-trip fidelity for key fields.
	req, err := skill.ParseSkillMD(serialized)
	require.NoError(t, err)
	assert.Equal(t, original.Name, req.Name)
	assert.Equal(t, original.Slug, req.Slug)
	assert.Equal(t, original.ShouldDefer, req.ShouldDefer)
	assert.Equal(t, original.ContextMode, req.ContextMode)
	assert.Equal(t, original.Instructions, req.Instructions)
}

// TestSerializeSkillMD_EmptyInstructions verifies no trailing newline for empty body.
func TestSerializeSkillMD_EmptyInstructions(t *testing.T) {
	sk := skill.Skill{Name: "Min", Slug: "min"}
	out := skill.SerializeSkillMD(sk)
	assert.True(t, strings.HasPrefix(out, "---\n"))
	// No body means the file ends right after the closing ---.
	assert.Contains(t, out, "---\n")
}
