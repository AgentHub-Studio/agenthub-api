DROP INDEX IF EXISTS public.uq_tenant_chat_default_id;

ALTER TABLE public.tenant_chat_default
    DROP COLUMN IF EXISTS enable_management,
    DROP COLUMN IF EXISTS retrieval_config,
    DROP COLUMN IF EXISTS model_config,
    DROP COLUMN IF EXISTS system_prompt,
    DROP COLUMN IF EXISTS name,
    DROP COLUMN IF EXISTS id;
