package ai_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

func TestProviderRegistry_RegisterAndGet(t *testing.T) {
	reg := ai.NewProviderRegistry()
	m := &stubModel{name: "openai"}

	reg.Register("gpt-4o", m)

	got, err := reg.Get("gpt-4o")
	require.NoError(t, err)
	assert.Equal(t, m, got)
}

func TestProviderRegistry_Get_NotFound(t *testing.T) {
	reg := ai.NewProviderRegistry()
	_, err := reg.Get("unknown")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown")
}

func TestProviderRegistry_GetDefault(t *testing.T) {
	reg := ai.NewProviderRegistry()
	m1 := &stubModel{name: "openai"}
	m2 := &stubModel{name: "openai"}

	// First registered becomes the default.
	reg.Register("gpt-4o", m1)
	reg.Register("gpt-3.5", m2)

	got, err := reg.GetDefault("openai")
	require.NoError(t, err)
	assert.Equal(t, m1, got) // m1 was registered first
}

func TestProviderRegistry_GetDefault_NotFound(t *testing.T) {
	reg := ai.NewProviderRegistry()
	_, err := reg.GetDefault("anthropic")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "anthropic")
}

func TestProviderRegistry_Available(t *testing.T) {
	reg := ai.NewProviderRegistry()
	reg.Register("gpt-4o", &stubModel{name: "openai"})
	reg.Register("claude-3", &stubModel{name: "anthropic"})
	reg.Register("llama3", &stubModel{name: "ollama"})

	names := reg.Available()
	assert.Equal(t, []string{"claude-3", "gpt-4o", "llama3"}, names) // sorted
}

func TestProviderRegistry_Available_Empty(t *testing.T) {
	reg := ai.NewProviderRegistry()
	assert.Empty(t, reg.Available())
}

func TestProviderRegistry_MultipleProviderTypes(t *testing.T) {
	reg := ai.NewProviderRegistry()
	openaiModel := &stubModel{name: "openai"}
	anthropicModel := &stubModel{name: "anthropic"}

	reg.Register("gpt-4o", openaiModel)
	reg.Register("claude-3", anthropicModel)

	gotOpenAI, err := reg.GetDefault("openai")
	require.NoError(t, err)
	assert.Equal(t, openaiModel, gotOpenAI)

	gotAnthropic, err := reg.GetDefault("anthropic")
	require.NoError(t, err)
	assert.Equal(t, anthropicModel, gotAnthropic)
}

func TestProviderRegistry_Register_ReplacesExisting(t *testing.T) {
	reg := ai.NewProviderRegistry()
	m1 := &stubModel{name: "openai"}
	m2 := &stubModel{name: "openai"}

	reg.Register("gpt-4o", m1)
	reg.Register("gpt-4o", m2) // overwrite

	got, err := reg.Get("gpt-4o")
	require.NoError(t, err)
	assert.Equal(t, m2, got)
}
