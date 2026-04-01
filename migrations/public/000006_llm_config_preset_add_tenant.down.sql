ALTER TABLE public.llm_config_preset
    DROP CONSTRAINT IF EXISTS llm_config_preset_tenant_name_key;

ALTER TABLE public.llm_config_preset
    ADD CONSTRAINT llm_config_preset_name_key UNIQUE (name);

DROP INDEX IF EXISTS public.idx_llm_config_preset_tenant;
DROP INDEX IF EXISTS public.idx_llm_config_preset_visibility;

ALTER TABLE public.llm_config_preset
    DROP COLUMN IF EXISTS tenant_id,
    DROP COLUMN IF EXISTS description,
    DROP COLUMN IF EXISTS config_json,
    DROP COLUMN IF EXISTS is_public,
    DROP COLUMN IF EXISTS visibility,
    DROP COLUMN IF EXISTS updated_at;
