package sanitize_test

import (
	"testing"

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
