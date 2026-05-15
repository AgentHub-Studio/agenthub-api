package agentic

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// fakeModelFactory is a ChatModelFactory test double for resolveRunConfig tests.
// resolveRunConfig only consults ResolveDefaultProvider / ResolveModel; Build is
// present solely to satisfy the interface.
type fakeModelFactory struct {
	defaultProvider string
	modelByProvider map[string]string
}

func (f fakeModelFactory) Build(context.Context, string, string) (ai.ChatModel, error) {
	return nil, nil
}

func (f fakeModelFactory) ResolveModel(_ context.Context, provider string) string {
	return f.modelByProvider[provider]
}

func (f fakeModelFactory) ResolveDefaultProvider(context.Context) string {
	return f.defaultProvider
}

// TestResolveRunConfig_EmptyProviderAndModel_UsesTenantSettings covers the
// out-of-the-box case: the auto-seeded default agent ships with an empty
// provider/model, so the resolution chain must fill both from tenant settings.
func TestResolveRunConfig_EmptyProviderAndModel_UsesTenantSettings(t *testing.T) {
	mc := json.RawMessage(`{"provider":"","model":"","temperature":0.3}`)
	factory := fakeModelFactory{
		defaultProvider: "openrouter",
		modelByProvider: map[string]string{"openrouter": "mistralai/mistral-nemo"},
	}
	cfg := resolveRunConfig(context.Background(), factory, mc, 0)
	assert.Equal(t, "openrouter", cfg.Provider)
	assert.Equal(t, "mistralai/mistral-nemo", cfg.Model)
}

// TestResolveRunConfig_ExplicitProvider_EmptyModel_ResolvesModel verifies that
// an explicit provider is kept while an empty model is still resolved from the
// matching per-provider setting.
func TestResolveRunConfig_ExplicitProvider_EmptyModel_ResolvesModel(t *testing.T) {
	mc := json.RawMessage(`{"provider":"openai","model":""}`)
	factory := fakeModelFactory{
		modelByProvider: map[string]string{"openai": "gpt-4o-mini"},
	}
	cfg := resolveRunConfig(context.Background(), factory, mc, 0)
	assert.Equal(t, "openai", cfg.Provider)
	assert.Equal(t, "gpt-4o-mini", cfg.Model)
}

// TestResolveRunConfig_ExplicitProviderAndModel_NotOverridden verifies that an
// agent with an explicit provider AND model is left untouched — tenant defaults
// must not clobber a deliberate per-agent choice.
func TestResolveRunConfig_ExplicitProviderAndModel_NotOverridden(t *testing.T) {
	mc := json.RawMessage(`{"provider":"ollama","model":"llama3.1"}`)
	factory := fakeModelFactory{
		defaultProvider: "openrouter",
		modelByProvider: map[string]string{"ollama": "should-not-be-used"},
	}
	cfg := resolveRunConfig(context.Background(), factory, mc, 0)
	assert.Equal(t, "ollama", cfg.Provider)
	assert.Equal(t, "llama3.1", cfg.Model)
}

// TestResolveRunConfig_NoTenantSettings_KeepsCompiledDefaults verifies the
// fallback when neither the agent nor the tenant configures anything.
func TestResolveRunConfig_NoTenantSettings_KeepsCompiledDefaults(t *testing.T) {
	cfg := resolveRunConfig(context.Background(), fakeModelFactory{}, nil, 0)
	def := DefaultRunConfig()
	assert.Equal(t, def.Provider, cfg.Provider)
	assert.Equal(t, def.Model, cfg.Model)
}

// TestResolveRunConfig_LLMCallTimeoutOverride verifies the server-level timeout
// is applied when the agent did not override it.
func TestResolveRunConfig_LLMCallTimeoutOverride(t *testing.T) {
	cfg := resolveRunConfig(context.Background(), fakeModelFactory{}, nil, 90*time.Second)
	assert.Equal(t, 90*time.Second, cfg.LLMCallTimeout)
}
