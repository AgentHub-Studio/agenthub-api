-- Seed default LLM config presets.
-- Uses INSERT ... ON CONFLICT DO NOTHING so re-running is idempotent.
-- P-C327-4 (ACT-F3-09): ensures a fresh installation has usable presets.

INSERT INTO public.llm_config_preset (name, provider, model, max_tokens, temperature, is_default)
VALUES
    ('Claude Sonnet 4.6',     'anthropic',  'claude-sonnet-4-6',          8192, 0.7, FALSE),
    ('Claude Opus 4.6',       'anthropic',  'claude-opus-4-6',            8192, 0.7, FALSE),
    ('GPT-4o',                'openai',     'gpt-4o',                     4096, 0.7, FALSE),
    ('GPT-4o Mini',           'openai',     'gpt-4o-mini',                4096, 0.7, TRUE),
    ('OpenRouter GPT-OSS-20b','openrouter', 'openai/gpt-oss-20b',         4096, 0.7, FALSE),
    ('Llama 3.3 70B (Ollama)','ollama',     'llama3.3:70b',               4096, 0.7, FALSE)
ON CONFLICT (tenant_id, name) DO NOTHING;
