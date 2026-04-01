-- Fix column mismatches between Java-created tables and Go schema expectations.
-- Java used package_name/package_slug/package_type; Go queries name/slug/type.
-- Java used model_id; Go queries model.
-- All operations are idempotent via ADD COLUMN IF NOT EXISTS / generated columns.

-- marketplace_listing: add Go-expected aliases backed by Java columns.
ALTER TABLE public.marketplace_listing
    ADD COLUMN IF NOT EXISTS name VARCHAR(255)  GENERATED ALWAYS AS (package_name) STORED,
    ADD COLUMN IF NOT EXISTS slug VARCHAR(255)  GENERATED ALWAYS AS (package_slug) STORED,
    ADD COLUMN IF NOT EXISTS type VARCHAR(50)   GENERATED ALWAYS AS (package_type) STORED;

-- llm_config_preset: add Go-expected columns missing from Java schema.
ALTER TABLE public.llm_config_preset
    ADD COLUMN IF NOT EXISTS model       VARCHAR(255) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS base_url    TEXT,
    ADD COLUMN IF NOT EXISTS api_key_env VARCHAR(255) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS max_tokens  INTEGER      NOT NULL DEFAULT 4096,
    ADD COLUMN IF NOT EXISTS temperature NUMERIC(4,2) NOT NULL DEFAULT 0.7,
    ADD COLUMN IF NOT EXISTS is_default  BOOLEAN      NOT NULL DEFAULT FALSE;

-- Backfill model from model_id for existing rows.
UPDATE public.llm_config_preset SET model = COALESCE(model_id, '') WHERE model = '';

CREATE INDEX IF NOT EXISTS idx_llm_config_preset_is_default ON public.llm_config_preset (is_default);
