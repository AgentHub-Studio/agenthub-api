-- Add redundant columns is_public and visibility back to llm_config_preset
ALTER TABLE public.llm_config_preset
    ADD COLUMN IF NOT EXISTS is_public    BOOLEAN       NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS visibility   VARCHAR(30)   NOT NULL DEFAULT 'PRIVATE';

CREATE INDEX IF NOT EXISTS idx_llm_config_preset_visibility ON public.llm_config_preset (visibility);
