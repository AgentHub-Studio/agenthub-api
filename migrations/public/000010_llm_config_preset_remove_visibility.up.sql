-- Remove redundant columns is_public and visibility from llm_config_preset
ALTER TABLE public.llm_config_preset
    DROP COLUMN IF EXISTS is_public,
    DROP COLUMN IF EXISTS visibility;

DROP INDEX IF EXISTS idx_llm_config_preset_visibility;
