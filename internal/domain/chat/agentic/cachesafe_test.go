package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

func makeTool(name string) ai.Tool {
	return ai.Tool{Type: "function", Function: ai.ToolSchema{Name: name, Description: name + " desc"}}
}

func TestCacheSafeParams_Hash_Deterministic(t *testing.T) {
	tools := []ai.Tool{makeTool("search")}

	p1 := agentic.NewCacheSafeParams("system prompt", tools, "anthropic", "claude-sonnet-4", true)
	p2 := agentic.NewCacheSafeParams("system prompt", tools, "anthropic", "claude-sonnet-4", true)

	assert.Equal(t, p1.Hash(), p2.Hash(), "identical params should produce same hash")
	assert.Len(t, p1.Hash(), 16) // truncated sha256
}

func TestCacheSafeParams_Hash_DifferentPrompt(t *testing.T) {
	tools := []ai.Tool{makeTool("search")}

	p1 := agentic.NewCacheSafeParams("prompt A", tools, "anthropic", "claude-sonnet-4", true)
	p2 := agentic.NewCacheSafeParams("prompt B", tools, "anthropic", "claude-sonnet-4", true)

	assert.NotEqual(t, p1.Hash(), p2.Hash())
}

func TestCacheSafeParams_Hash_DifferentTools(t *testing.T) {
	p1 := agentic.NewCacheSafeParams("prompt", []ai.Tool{makeTool("a")}, "anthropic", "claude-sonnet-4", true)
	p2 := agentic.NewCacheSafeParams("prompt", []ai.Tool{makeTool("b")}, "anthropic", "claude-sonnet-4", true)

	assert.NotEqual(t, p1.Hash(), p2.Hash())
}

func TestCacheSafeParams_Hash_DifferentModel(t *testing.T) {
	tools := []ai.Tool{makeTool("search")}

	p1 := agentic.NewCacheSafeParams("prompt", tools, "anthropic", "claude-sonnet-4", true)
	p2 := agentic.NewCacheSafeParams("prompt", tools, "anthropic", "claude-opus-4-6", true)

	assert.NotEqual(t, p1.Hash(), p2.Hash())
}

func TestCacheSafeParams_Matches(t *testing.T) {
	tools := []ai.Tool{makeTool("search")}

	p1 := agentic.NewCacheSafeParams("prompt", tools, "anthropic", "model", true)
	p2 := agentic.NewCacheSafeParams("prompt", tools, "anthropic", "model", true)

	assert.True(t, p1.Matches(p2))
}

func TestCacheSafeParams_Matches_Different(t *testing.T) {
	p1 := agentic.NewCacheSafeParams("A", nil, "anthropic", "model", true)
	p2 := agentic.NewCacheSafeParams("B", nil, "anthropic", "model", true)

	assert.False(t, p1.Matches(p2))
}

func TestCacheSafeParams_Matches_Nil(t *testing.T) {
	p := agentic.NewCacheSafeParams("A", nil, "anthropic", "model", true)

	assert.False(t, p.Matches(nil))
	assert.True(t, (*agentic.CacheSafeParams)(nil).Matches(nil))
}

func TestCacheSafeParams_Hash_Nil(t *testing.T) {
	var p *agentic.CacheSafeParams
	assert.Empty(t, p.Hash())
}
