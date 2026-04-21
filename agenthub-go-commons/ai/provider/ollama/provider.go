// Package ollama provides a ChatModel implementation backed by the Ollama OpenAI-compatible API.
package ollama

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai/provider/openai"
)

const defaultBaseURL = "http://localhost:11434/v1"

// defaultOllamaTimeout gives the model time to load into memory *and* emit
// its first response headers. Large models (gpt-oss:20b and up) running on
// CPU were observed taking >2 minutes for a short reply on the cluster,
// so the generic openai 120s default caused Client.Timeout to fire before
// the run ever produced a token. 600s matches the practical upper bound
// for cold CPU inference of 20B-parameter models; tune via
// OLLAMA_TIMEOUT_SECONDS at deploy time.
const defaultOllamaTimeout = 600 * time.Second

// Provider implements ai.ChatModel using Ollama's OpenAI-compatible endpoint.
// It delegates all HTTP work to the openai.Provider with an empty API key.
type Provider struct {
	inner *openai.Provider
}

// New creates a new Ollama Provider.
// If baseURL is empty, http://localhost:11434/v1 is used.
// apiKey is optional: local Ollama ignores it; Ollama Cloud
// (https://ollama.com/v1) and other authenticated deployments require it
// and the openai-compatible inner provider will inject
//   Authorization: Bearer <apiKey>
// whenever apiKey is non-empty.
// The per-request timeout defaults to 10 minutes, overridable via the
// OLLAMA_TIMEOUT_SECONDS environment variable.
func New(baseURL, apiKey string) *Provider {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Provider{inner: openai.NewWithTimeout(apiKey, baseURL, resolveTimeout())}
}

// resolveTimeout reads OLLAMA_TIMEOUT_SECONDS from the environment and
// falls back to defaultOllamaTimeout when unset or malformed.
func resolveTimeout() time.Duration {
	raw := os.Getenv("OLLAMA_TIMEOUT_SECONDS")
	if raw == "" {
		return defaultOllamaTimeout
	}
	secs, err := strconv.Atoi(raw)
	if err != nil || secs <= 0 {
		return defaultOllamaTimeout
	}
	return time.Duration(secs) * time.Second
}

// GetProviderName returns "ollama".
func (p *Provider) GetProviderName() string { return "ollama" }

// Chat delegates to the underlying OpenAI-compatible provider after injecting
// Ollama-specific options (num_ctx, num_predict, …) into ProviderOptions.
func (p *Provider) Chat(ctx context.Context, messages []ai.Message, opts ai.ChatOptions) (*ai.ChatResponse, error) {
	opts = withOllamaOptions(opts)
	return p.inner.Chat(ctx, messages, opts)
}

// ChatStream delegates to the underlying OpenAI-compatible provider after
// injecting Ollama-specific options.
func (p *Provider) ChatStream(ctx context.Context, messages []ai.Message, opts ai.ChatOptions) (<-chan ai.StreamChunk, error) {
	opts = withOllamaOptions(opts)
	return p.inner.ChatStream(ctx, messages, opts)
}

// withOllamaOptions merges the caller's ProviderOptions with values derived
// from environment variables. Caller values win so an agent can override the
// cluster default for a single run. Keys recognised by Ollama (any unknown
// key is ignored by the server, not an error):
//
//   - num_ctx         (OLLAMA_NUM_CTX, int)         context window in tokens
//   - num_predict     (OLLAMA_NUM_PREDICT, int)     max output tokens
//   - seed            (OLLAMA_SEED, int)            deterministic sampling seed
//   - top_k           (OLLAMA_TOP_K, int)
//   - repeat_penalty  (OLLAMA_REPEAT_PENALTY, float)
//
// Env parsing is tolerant: invalid values are dropped rather than erroring.
func withOllamaOptions(opts ai.ChatOptions) ai.ChatOptions {
	envOpts := collectEnvOptions()
	if len(envOpts) == 0 && opts.ProviderOptions == nil {
		return opts
	}
	merged := make(map[string]any, len(envOpts)+len(opts.ProviderOptions))
	for k, v := range envOpts {
		merged[k] = v
	}
	for k, v := range opts.ProviderOptions {
		merged[k] = v
	}
	opts.ProviderOptions = merged
	return opts
}

// collectEnvOptions reads OLLAMA_* env vars and returns the matching options
// map. Called on every request so operators can flip values at runtime by
// restarting the pod — but cached results are a valid future optimisation.
func collectEnvOptions() map[string]any {
	intKeys := map[string]string{
		"OLLAMA_NUM_CTX":        "num_ctx",
		"OLLAMA_NUM_PREDICT":    "num_predict",
		"OLLAMA_SEED":           "seed",
		"OLLAMA_TOP_K":          "top_k",
	}
	floatKeys := map[string]string{
		"OLLAMA_REPEAT_PENALTY": "repeat_penalty",
	}
	out := map[string]any{}
	for env, key := range intKeys {
		if raw := os.Getenv(env); raw != "" {
			if v, err := strconv.Atoi(raw); err == nil {
				out[key] = v
			}
		}
	}
	for env, key := range floatKeys {
		if raw := os.Getenv(env); raw != "" {
			if v, err := strconv.ParseFloat(raw, 64); err == nil {
				out[key] = v
			}
		}
	}
	return out
}
