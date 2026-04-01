ALTER TABLE public.marketplace_listing
    DROP COLUMN IF EXISTS name,
    DROP COLUMN IF EXISTS slug,
    DROP COLUMN IF EXISTS type;

ALTER TABLE public.llm_config_preset
    DROP COLUMN IF EXISTS model,
    DROP COLUMN IF EXISTS base_url,
    DROP COLUMN IF EXISTS api_key_env,
    DROP COLUMN IF EXISTS max_tokens,
    DROP COLUMN IF EXISTS temperature,
    DROP COLUMN IF EXISTS is_default;
