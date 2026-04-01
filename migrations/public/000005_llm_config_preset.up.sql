CREATE TABLE IF NOT EXISTS public.llm_config_preset (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(255) NOT NULL UNIQUE,
    provider    VARCHAR(100) NOT NULL,
    model       VARCHAR(255) NOT NULL,
    base_url    TEXT,
    api_key_env VARCHAR(255) NOT NULL DEFAULT '',
    max_tokens  INTEGER      NOT NULL DEFAULT 4096,
    temperature NUMERIC(4,2) NOT NULL DEFAULT 0.7,
    is_default  BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_llm_config_preset_provider ON public.llm_config_preset (provider);
CREATE INDEX IF NOT EXISTS idx_llm_config_preset_is_default ON public.llm_config_preset (is_default);
