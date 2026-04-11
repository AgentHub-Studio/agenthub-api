-- Re-seed default LLM config presets for every existing tenant.
-- Migration 000011 seeded with tenant_id='' (empty string default), but the
-- repository always queries by the actual tenant slug. This migration backfills
-- the 6 default presets for every tenant row in public.tenants.
-- Uses ON CONFLICT (tenant_id, name) DO NOTHING so it is safe to re-run.

INSERT INTO public.llm_config_preset (tenant_id, name, provider, model, max_tokens, temperature, is_default)
SELECT
    t.id,
    p.name,
    p.provider,
    p.model,
    p.max_tokens,
    p.temperature,
    p.is_default
FROM public.tenants t
CROSS JOIN (VALUES
    ('Claude Sonnet 4.6',      'anthropic',  'claude-sonnet-4-6',     8192, 0.7::numeric, FALSE),
    ('Claude Opus 4.6',        'anthropic',  'claude-opus-4-6',       8192, 0.7::numeric, FALSE),
    ('GPT-4o',                 'openai',     'gpt-4o',                4096, 0.7::numeric, FALSE),
    ('GPT-4o Mini',            'openai',     'gpt-4o-mini',           4096, 0.7::numeric, TRUE),
    ('OpenRouter GPT-OSS-20b', 'openrouter', 'openai/gpt-oss-20b',   4096, 0.7::numeric, FALSE),
    ('Llama 3.3 70B (Ollama)', 'ollama',     'llama3.3:70b',          4096, 0.7::numeric, FALSE)
) AS p(name, provider, model, max_tokens, temperature, is_default)
ON CONFLICT (tenant_id, name) DO NOTHING;
