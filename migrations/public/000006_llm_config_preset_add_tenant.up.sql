-- Add tenant isolation + missing columns to llm_config_preset (issue #34)
ALTER TABLE public.llm_config_preset
    ADD COLUMN IF NOT EXISTS tenant_id    VARCHAR(255)  NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS description  TEXT,
    ADD COLUMN IF NOT EXISTS config_json  JSONB         NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS is_public    BOOLEAN       NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS visibility   VARCHAR(30)   NOT NULL DEFAULT 'PRIVATE',
    ADD COLUMN IF NOT EXISTS updated_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW();

-- Drop the global unique constraint on name (replaced by per-tenant unique).
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE table_schema = 'public'
          AND table_name   = 'llm_config_preset'
          AND constraint_type = 'UNIQUE'
          AND constraint_name = 'llm_config_preset_name_key'
    ) THEN
        ALTER TABLE public.llm_config_preset DROP CONSTRAINT llm_config_preset_name_key;
    END IF;
END $$;

-- Per-tenant unique name.
ALTER TABLE public.llm_config_preset
    ADD CONSTRAINT llm_config_preset_tenant_name_key UNIQUE (tenant_id, name);

CREATE INDEX IF NOT EXISTS idx_llm_config_preset_tenant ON public.llm_config_preset (tenant_id);
CREATE INDEX IF NOT EXISTS idx_llm_config_preset_visibility ON public.llm_config_preset (visibility);
