package ai

import (
	"fmt"
	"sort"
	"sync"
)

// ProviderRegistry holds configured LLM providers keyed by name.
// It also caches per-preset provider instances (keyed by provider+apiKey+baseURL)
// so that multiple presets sharing the same credentials reuse the same HTTP client.
type ProviderRegistry struct {
	mu            sync.RWMutex
	providers     map[string]ChatModel
	defaults      map[string]ChatModel // keyed by provider type ("OPENAI", "ANTHROPIC", etc.)
	instanceCache sync.Map             // key: "PROVIDER|apiKey|baseURL" -> ChatModel
}

// NewProviderRegistry creates an empty ProviderRegistry.
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]ChatModel),
		defaults:  make(map[string]ChatModel),
	}
}

// Register adds a provider under the given name key.
// The first provider registered for a given provider type becomes the default for that type.
func (r *ProviderRegistry) Register(name string, model ChatModel) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.providers[name] = model

	// Register as default for its provider type if not already set.
	providerType := model.GetProviderName()
	if _, exists := r.defaults[providerType]; !exists {
		r.defaults[providerType] = model
	}
}

// Get returns the provider registered under name.
func (r *ProviderRegistry) Get(name string) (ChatModel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	m, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("ai: provider %q not registered", name)
	}
	return m, nil
}

// GetDefault returns the default provider for the given provider type (e.g., "openai").
func (r *ProviderRegistry) GetDefault(providerType string) (ChatModel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	m, ok := r.defaults[providerType]
	if !ok {
		return nil, fmt.Errorf("ai: no default provider registered for type %q", providerType)
	}
	return m, nil
}

// Available returns all registered provider name keys in sorted order.
func (r *ProviderRegistry) Available() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// GetChatModel resolves a ChatModel from an LlmPresetConfig.
//
// Resolution order:
//  1. If config_json contains api_key, create (or return cached) a provider
//     instance with those credentials — preset credentials override env vars.
//  2. Try the preset name as a registry key (allows per-preset named overrides).
//  3. Fall back to the default registered provider for the provider type.
//
// Returns ErrProviderNotFound if no suitable provider can be resolved.
func (r *ProviderRegistry) GetChatModel(preset LlmPresetConfig) (ChatModel, ChatOptions, error) {
	opts, err := preset.ToChatOptions()
	if err != nil {
		return nil, ChatOptions{}, err
	}

	// Preset-level credential override — takes highest priority.
	if apiKey, baseURL := preset.OverrideCredentials(); apiKey != "" {
		model, err := r.getOrCreateCachedModel(normaliseProviderType(preset.Provider), apiKey, baseURL)
		if err != nil {
			return nil, ChatOptions{}, err
		}
		return model, opts, nil
	}

	// Try by preset name (allows per-preset named overrides).
	if model, lookupErr := r.Get(preset.Name); lookupErr == nil {
		return model, opts, nil
	}

	// Fall back to provider type default.
	model, err := r.GetDefault(normaliseProviderType(preset.Provider))
	if err != nil {
		return nil, ChatOptions{}, fmt.Errorf("%w: %s", ErrProviderNotFound, preset.Provider)
	}
	return model, opts, nil
}

// getOrCreateCachedModel returns a ChatModel for the given provider type and credentials,
// creating it via the registered factory on first access and caching it for reuse.
// This avoids allocating multiple HTTP clients when multiple presets share the same config.
func (r *ProviderRegistry) getOrCreateCachedModel(providerType, apiKey, baseURL string) (ChatModel, error) {
	cacheKey := providerType + "|" + apiKey + "|" + baseURL
	if cached, ok := r.instanceCache.Load(cacheKey); ok {
		return cached.(ChatModel), nil
	}

	factory, err := FactoryFor(providerType)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrProviderNotFound, providerType)
	}

	model := factory(baseURL, apiKey)
	// Use LoadOrStore so a concurrent caller wins the race gracefully.
	actual, _ := r.instanceCache.LoadOrStore(cacheKey, model)
	return actual.(ChatModel), nil
}
