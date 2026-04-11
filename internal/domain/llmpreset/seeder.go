package llmpreset

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// defaultPresets are the platform-provided LLM configuration templates.
var defaultPresets = []struct {
	Name        string
	Provider    string
	Model       string
	MaxTokens   int
	Temperature float64
	IsDefault   bool
}{
	{"Claude Sonnet 4.6", "anthropic", "claude-sonnet-4-6", 8192, 0.7, false},
	{"Claude Opus 4.6", "anthropic", "claude-opus-4-6", 8192, 0.7, false},
	{"GPT-4o", "openai", "gpt-4o", 4096, 0.7, false},
	{"GPT-4o Mini", "openai", "gpt-4o-mini", 4096, 0.7, true},
	{"OpenRouter GPT-OSS-20b", "openrouter", "openai/gpt-oss-20b", 4096, 0.7, false},
	{"Llama 3.3 70B (Ollama)", "ollama", "llama3.3:70b", 4096, 0.7, false},
}

// Seeder seeds default LLM presets for new tenants.
type Seeder struct {
	pool *pgxpool.Pool
}

// NewSeeder creates a Seeder backed by the given connection pool.
func NewSeeder(pool *pgxpool.Pool) *Seeder {
	return &Seeder{pool: pool}
}

// SeedDefaults inserts the platform default presets for tenantID.
// Uses ON CONFLICT DO NOTHING so it is safe to call multiple times.
func (s *Seeder) SeedDefaults(ctx context.Context, tenantID string) error {
	const q = `
INSERT INTO public.llm_config_preset
    (tenant_id, name, provider, model, max_tokens, temperature, is_default)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (tenant_id, name) DO NOTHING`

	for _, p := range defaultPresets {
		if _, err := s.pool.Exec(ctx, q,
			tenantID, p.Name, p.Provider, p.Model, p.MaxTokens, p.Temperature, p.IsDefault,
		); err != nil {
			return fmt.Errorf("llmpreset.SeedDefaults: %w", err)
		}
	}
	return nil
}
