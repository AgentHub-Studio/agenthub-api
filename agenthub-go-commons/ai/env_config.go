package ai

import "os"

// EnvConfig holds API keys and base URLs for AI providers, loaded from environment variables.
// API keys in environment variables take precedence over values in LlmPresetConfig.
type EnvConfig struct {
	OpenAIAPIKey      string // OPENAI_API_KEY
	OpenAIBaseURL     string // OPENAI_BASE_URL (default: https://api.openai.com/v1)
	AnthropicAPIKey   string // ANTHROPIC_API_KEY
	AnthropicBaseURL  string // ANTHROPIC_BASE_URL (default: https://api.anthropic.com)
	OllamaBaseURL     string // OLLAMA_BASE_URL (default: http://localhost:11434)
	OpenRouterAPIKey  string // OPENROUTER_API_KEY
	OpenRouterBaseURL string // OPENROUTER_BASE_URL (default: https://openrouter.ai/api/v1)
}

// EnvConfigFromEnvironment reads provider configuration from environment variables.
func EnvConfigFromEnvironment() EnvConfig {
	return EnvConfig{
		OpenAIAPIKey:      os.Getenv("OPENAI_API_KEY"),
		OpenAIBaseURL:     envOrDefault("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		AnthropicAPIKey:   os.Getenv("ANTHROPIC_API_KEY"),
		AnthropicBaseURL:  envOrDefault("ANTHROPIC_BASE_URL", "https://api.anthropic.com"),
		OllamaBaseURL:     envOrDefault("OLLAMA_BASE_URL", "http://localhost:11434"),
		OpenRouterAPIKey:  os.Getenv("OPENROUTER_API_KEY"),
		OpenRouterBaseURL: envOrDefault("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1"),
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
