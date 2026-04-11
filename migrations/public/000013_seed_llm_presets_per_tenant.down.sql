-- Remove the default presets seeded by migration 000013.
-- Deletes only the 6 preset names that were seeded; tenant-created presets are untouched.
DELETE FROM public.llm_config_preset
WHERE name IN (
    'Claude Sonnet 4.6',
    'Claude Opus 4.6',
    'GPT-4o',
    'GPT-4o Mini',
    'OpenRouter GPT-OSS-20b',
    'Llama 3.3 70B (Ollama)'
)
AND tenant_id IN (SELECT id FROM public.tenants);
