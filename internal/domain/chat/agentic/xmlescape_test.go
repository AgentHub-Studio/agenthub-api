package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- EscapeXML ---

func TestEscapeXML_NoSpecialChars(t *testing.T) {
	assert.Equal(t, "hello world", agentic.EscapeXML("hello world"))
}

func TestEscapeXML_Ampersand(t *testing.T) {
	assert.Equal(t, "a &amp; b", agentic.EscapeXML("a & b"))
}

func TestEscapeXML_LessThan(t *testing.T) {
	assert.Equal(t, "&lt;tag&gt;", agentic.EscapeXML("<tag>"))
}

func TestEscapeXML_AllSpecialChars(t *testing.T) {
	assert.Equal(t, "&lt;div class=&amp;&gt;", agentic.EscapeXML(`<div class=&>`))
}

func TestEscapeXML_Empty(t *testing.T) {
	assert.Equal(t, "", agentic.EscapeXML(""))
}

func TestEscapeXML_MultipleOccurrences(t *testing.T) {
	assert.Equal(t, "a &amp; b &amp; c", agentic.EscapeXML("a & b & c"))
}

// --- EscapeXMLAttr ---

func TestEscapeXMLAttr_NoSpecialChars(t *testing.T) {
	assert.Equal(t, "hello", agentic.EscapeXMLAttr("hello"))
}

func TestEscapeXMLAttr_Quotes(t *testing.T) {
	assert.Equal(t, "&quot;hello&quot;", agentic.EscapeXMLAttr(`"hello"`))
}

func TestEscapeXMLAttr_SingleQuotes(t *testing.T) {
	assert.Equal(t, "&apos;hello&apos;", agentic.EscapeXMLAttr("'hello'"))
}

func TestEscapeXMLAttr_AllChars(t *testing.T) {
	assert.Equal(t,
		"&lt;a href=&quot;/&amp;x&quot;&gt;",
		agentic.EscapeXMLAttr(`<a href="/&x">`),
	)
}

func TestEscapeXMLAttr_Empty(t *testing.T) {
	assert.Equal(t, "", agentic.EscapeXMLAttr(""))
}

// --- UnescapeXML ---

func TestUnescapeXML_Entities(t *testing.T) {
	assert.Equal(t, "<div class=\"a&b\">", agentic.UnescapeXML("&lt;div class=&quot;a&amp;b&quot;&gt;"))
}

func TestUnescapeXML_NoEntities(t *testing.T) {
	assert.Equal(t, "hello world", agentic.UnescapeXML("hello world"))
}

func TestUnescapeXML_Empty(t *testing.T) {
	assert.Equal(t, "", agentic.UnescapeXML(""))
}

// --- Roundtrip ---

func TestEscapeXML_Roundtrip(t *testing.T) {
	original := `<div class="test" id='x'>&special</div>`
	assert.Equal(t, original, agentic.UnescapeXML(agentic.EscapeXMLAttr(original)))
}
