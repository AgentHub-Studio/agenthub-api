package ai

// ProviderFactory is a function that creates a ChatModel from a base URL and API key.
// The factory pattern allows new providers to be registered without modifying the registry.
type ProviderFactory func(baseURL, apiKey string) ChatModel

// registeredFactories maps provider type names (e.g., "OPENAI") to their factories.
// Populated by calls to RegisterFactory.
var registeredFactories = map[string]ProviderFactory{}

// RegisterFactory registers a factory function for the given provider type.
// Provider types are case-insensitive and stored in uppercase.
// Call RegisterFactory in the provider's init() or in main before building the registry.
func RegisterFactory(providerType string, factory ProviderFactory) {
	registeredFactories[normaliseProviderType(providerType)] = factory
}

// FactoryFor returns the ProviderFactory registered for the given provider type,
// or ErrProviderNotFound if none is registered.
func FactoryFor(providerType string) (ProviderFactory, error) {
	f, ok := registeredFactories[normaliseProviderType(providerType)]
	if !ok {
		return nil, ErrProviderNotFound
	}
	return f, nil
}

func normaliseProviderType(t string) string {
	b := make([]byte, len(t))
	for i := range t {
		c := t[i]
		if c >= 'a' && c <= 'z' {
			c -= 32
		}
		b[i] = c
	}
	return string(b)
}
