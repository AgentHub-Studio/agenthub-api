-- Separate context window capacity from max output tokens in LLM presets.

ALTER TABLE public.llm_config_preset
    ADD COLUMN IF NOT EXISTS context_window INTEGER NOT NULL DEFAULT 0;

UPDATE public.llm_config_preset
SET max_tokens = 64000,
    context_window = 1000000
WHERE name = 'Claude Sonnet 4.6'
  AND provider = 'anthropic'
  AND model = 'claude-sonnet-4-6';

UPDATE public.llm_config_preset
SET max_tokens = 128000,
    context_window = 1000000
WHERE name = 'Claude Opus 4.6'
  AND provider = 'anthropic'
  AND model = 'claude-opus-4-6';

UPDATE public.llm_config_preset
SET max_tokens = 16384,
    context_window = 128000
WHERE name = 'GPT-4o'
  AND provider = 'openai'
  AND model = 'gpt-4o';

UPDATE public.llm_config_preset
SET max_tokens = 16384,
    context_window = 128000
WHERE name = 'GPT-4o Mini'
  AND provider = 'openai'
  AND model = 'gpt-4o-mini';

UPDATE public.llm_config_preset
SET max_tokens = 131072,
    context_window = 131072
WHERE name = 'OpenRouter GPT-OSS-20b'
  AND provider = 'openrouter'
  AND model = 'openai/gpt-oss-20b';

UPDATE public.llm_config_preset
SET max_tokens = 128000,
    context_window = 128000
WHERE name = 'Llama 3.3 70B (Ollama)'
  AND provider = 'ollama'
  AND model = 'llama3.3:70b';
