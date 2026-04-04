package agentic_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- HashContent ---

func TestHashContent_Deterministic(t *testing.T) {
	h1 := agentic.HashContent("hello world")
	h2 := agentic.HashContent("hello world")
	assert.Equal(t, h1, h2)
	assert.Len(t, h1, 64) // SHA-256 hex
}

func TestHashContent_Different(t *testing.T) {
	h1 := agentic.HashContent("hello")
	h2 := agentic.HashContent("world")
	assert.NotEqual(t, h1, h2)
}

// --- FormatPastedTextRef ---

func TestFormatPastedTextRef_NoLabel(t *testing.T) {
	hash := agentic.HashContent("test")
	ref := agentic.FormatPastedTextRef(hash, "")
	assert.Contains(t, ref, "{{paste:text:")
	assert.Contains(t, ref, hash)
	assert.True(t, strings.HasSuffix(ref, "}}"))
}

func TestFormatPastedTextRef_WithLabel(t *testing.T) {
	hash := agentic.HashContent("test")
	ref := agentic.FormatPastedTextRef(hash, "config.yaml")
	assert.Contains(t, ref, ":config.yaml}}")
}

// --- FormatImageRef ---

func TestFormatImageRef_NoLabel(t *testing.T) {
	hash := agentic.HashContent("imgdata")
	ref := agentic.FormatImageRef(hash, "")
	assert.Contains(t, ref, "{{paste:image:")
}

func TestFormatImageRef_WithLabel(t *testing.T) {
	hash := agentic.HashContent("imgdata")
	ref := agentic.FormatImageRef(hash, "screenshot.png")
	assert.Contains(t, ref, ":screenshot.png}}")
}

// --- ParseReferences ---

func TestParseReferences_None(t *testing.T) {
	refs := agentic.ParseReferences("no references here")
	assert.Nil(t, refs)
}

func TestParseReferences_SingleText(t *testing.T) {
	hash := agentic.HashContent("content")
	text := "Look at this: " + agentic.FormatPastedTextRef(hash, "")
	refs := agentic.ParseReferences(text)
	require.Len(t, refs, 1)
	assert.Equal(t, agentic.PasteTypeText, refs[0].Type)
	assert.Equal(t, hash, refs[0].Hash)
	assert.Empty(t, refs[0].Label)
}

func TestParseReferences_WithLabel(t *testing.T) {
	hash := agentic.HashContent("data")
	text := agentic.FormatPastedTextRef(hash, "main.go")
	refs := agentic.ParseReferences(text)
	require.Len(t, refs, 1)
	assert.Equal(t, "main.go", refs[0].Label)
}

func TestParseReferences_Multiple(t *testing.T) {
	h1 := agentic.HashContent("a")
	h2 := agentic.HashContent("b")
	text := agentic.FormatPastedTextRef(h1, "file1") + " and " + agentic.FormatImageRef(h2, "img")
	refs := agentic.ParseReferences(text)
	require.Len(t, refs, 2)
	assert.Equal(t, agentic.PasteTypeText, refs[0].Type)
	assert.Equal(t, agentic.PasteTypeImage, refs[1].Type)
}

func TestParseReferences_Image(t *testing.T) {
	hash := agentic.HashContent("image-data")
	text := agentic.FormatImageRef(hash, "photo")
	refs := agentic.ParseReferences(text)
	require.Len(t, refs, 1)
	assert.Equal(t, agentic.PasteTypeImage, refs[0].Type)
}

// --- PasteStore ---

func TestPasteStore_StoreAndGet(t *testing.T) {
	ps := agentic.NewPasteStore()
	hash, ref := ps.Store("hello world", agentic.PasteTypeText, "greeting")

	assert.NotEmpty(t, hash)
	assert.Contains(t, ref, hash)

	entry, ok := ps.Get(hash)
	assert.True(t, ok)
	assert.Equal(t, "hello world", entry.Content)
	assert.Equal(t, agentic.PasteTypeText, entry.Type)
	assert.Equal(t, "greeting", entry.Label)
}

func TestPasteStore_StoreDeduplicates(t *testing.T) {
	ps := agentic.NewPasteStore()
	h1, _ := ps.Store("same content", agentic.PasteTypeText, "first")
	h2, _ := ps.Store("same content", agentic.PasteTypeText, "second")

	assert.Equal(t, h1, h2)
	assert.Equal(t, 1, ps.Size())

	// First label wins.
	entry, _ := ps.Get(h1)
	assert.Equal(t, "first", entry.Label)
}

func TestPasteStore_StoreTextLines(t *testing.T) {
	ps := agentic.NewPasteStore()
	content := "line1\nline2\nline3"
	hash, _ := ps.Store(content, agentic.PasteTypeText, "")

	entry, _ := ps.Get(hash)
	assert.Equal(t, 3, entry.Lines)
}

func TestPasteStore_StoreImage(t *testing.T) {
	ps := agentic.NewPasteStore()
	hash, ref := ps.StoreImage("base64data", "image/png", "screenshot")

	assert.NotEmpty(t, hash)
	assert.Contains(t, ref, "image")

	entry, ok := ps.Get(hash)
	assert.True(t, ok)
	assert.Equal(t, "image/png", entry.MimeType)
	assert.Equal(t, agentic.PasteTypeImage, entry.Type)
}

func TestPasteStore_GetNotFound(t *testing.T) {
	ps := agentic.NewPasteStore()
	_, ok := ps.Get("nonexistent")
	assert.False(t, ok)
}

func TestPasteStore_Size(t *testing.T) {
	ps := agentic.NewPasteStore()
	assert.Equal(t, 0, ps.Size())

	ps.Store("a", agentic.PasteTypeText, "")
	ps.Store("b", agentic.PasteTypeText, "")
	assert.Equal(t, 2, ps.Size())
}

func TestPasteStore_Hashes(t *testing.T) {
	ps := agentic.NewPasteStore()
	h1, _ := ps.Store("a", agentic.PasteTypeText, "")
	h2, _ := ps.Store("b", agentic.PasteTypeText, "")

	hashes := ps.Hashes()
	assert.Len(t, hashes, 2)
	assert.Contains(t, hashes, h1)
	assert.Contains(t, hashes, h2)
}

// --- ExpandReferences ---

func TestPasteStore_ExpandReferences_Text(t *testing.T) {
	ps := agentic.NewPasteStore()
	hash, ref := ps.Store("package main\nfunc main(){}", agentic.PasteTypeText, "main.go")

	content := "Check this code: " + ref
	expanded := ps.ExpandReferences(content)

	assert.Contains(t, expanded, "package main")
	assert.Contains(t, expanded, "[Pasted: main.go]")
	assert.NotContains(t, expanded, hash)
}

func TestPasteStore_ExpandReferences_TextNoLabel(t *testing.T) {
	ps := agentic.NewPasteStore()
	_, ref := ps.Store("raw content", agentic.PasteTypeText, "")

	expanded := ps.ExpandReferences("Here: " + ref)
	assert.Contains(t, expanded, "raw content")
	assert.NotContains(t, expanded, "[Pasted:")
}

func TestPasteStore_ExpandReferences_Image(t *testing.T) {
	ps := agentic.NewPasteStore()
	_, ref := ps.StoreImage("imgdata", "image/png", "screenshot")

	expanded := ps.ExpandReferences("See " + ref)
	assert.Contains(t, expanded, "[Image: screenshot]")
}

func TestPasteStore_ExpandReferences_ImageNoLabel(t *testing.T) {
	ps := agentic.NewPasteStore()
	_, ref := ps.StoreImage("imgdata", "image/png", "")

	expanded := ps.ExpandReferences(ref)
	assert.Contains(t, expanded, "[Image]")
}

func TestPasteStore_ExpandReferences_Unknown(t *testing.T) {
	ps := agentic.NewPasteStore()
	// Reference to unknown hash.
	unknownRef := "{{paste:text:0000000000000000000000000000000000000000000000000000000000000000}}"
	expanded := ps.ExpandReferences("test " + unknownRef)
	assert.Contains(t, expanded, unknownRef, "unknown ref should be left as-is")
}

func TestPasteStore_ExpandReferences_NoRefs(t *testing.T) {
	ps := agentic.NewPasteStore()
	content := "just plain text"
	assert.Equal(t, content, ps.ExpandReferences(content))
}

func TestPasteStore_ExpandReferences_Multiple(t *testing.T) {
	ps := agentic.NewPasteStore()
	_, ref1 := ps.Store("content A", agentic.PasteTypeText, "fileA")
	_, ref2 := ps.Store("content B", agentic.PasteTypeText, "fileB")

	text := ref1 + " and " + ref2
	expanded := ps.ExpandReferences(text)

	assert.Contains(t, expanded, "content A")
	assert.Contains(t, expanded, "content B")
}
