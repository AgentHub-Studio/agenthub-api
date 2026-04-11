-- Remove seeded presets (only those whose names match exactly).
DELETE FROM public.llm_config_preset
WHERE name IN (
    'Claude Sonnet 4.6',
    'Claude Opus 4.6',
    'GPT-4o',
    'GPT-4o Mini',
    'OpenRouter GPT-OSS-20b',
    'Llama 3.3 70B (Ollama)'
);
