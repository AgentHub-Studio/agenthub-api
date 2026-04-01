-- marketplace_listing: Java schema uses published_at; Go queries expect created_at.
-- Add created_at as a generated alias so existing rows are covered without data migration.
ALTER TABLE public.marketplace_listing
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ GENERATED ALWAYS AS (published_at) STORED;
