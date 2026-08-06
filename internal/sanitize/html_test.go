package sanitize_test

import (
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/sanitize"
)

func TestContainsHTML_DetectsNamedTags(t *testing.T) {
	assert.True(t, sanitize.ContainsHTML(`<img src=x onerror="alert(1)">Agent`))
	assert.True(t, sanitize.ContainsHTML(`<script>alert(1)</script>`))
}

func TestContainsHTML_AllowsComparisonText(t *testing.T) {
	assert.False(t, sanitize.ContainsHTML("Use 2 < 3 and 4 > 1 as constraints"))
}

func TestStripHTML_RemovesInvalidUTF8(t *testing.T) {
	got := sanitize.StripHTML(string([]byte{0xff, 0xfe, '<', 'b', '>', 'x', '<', '/', 'b', '>'}))

	assert.True(t, utf8.ValidString(got))
	assert.Equal(t, "x", got)
}

func TestStripHTML_RemovesTagsCreatedByOverlappingMarkup(t *testing.T) {
	got := sanitize.StripHTML("<A<A>>")

	assert.Empty(t, got)
	assert.False(t, sanitize.ContainsHTML(got))
}
