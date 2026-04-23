-- 000062_tool_slug_name_sync
--
-- Cleans up the `tool.slug` column so the application owns slug generation:
--   0. Ensures the slug column exists (was added ad-hoc on some tenants during
--      the iter-3 E2E loop; never had a proper migration — this guarantees
--      every tenant ends up with a slug column regardless of prior state).
--   1. Backfills NULL / empty / emergency-pattern slugs with "<sanitized-name>-<id8>"
--      where the 8-char UUID suffix guarantees uniqueness even when multiple
--      tools share the same name.
--   2. Adds NOT NULL + UNIQUE constraints.
--   3. Drops the random-hash DEFAULT so future INSERTs that forget to pass a
--      slug fail loudly. The application (internal/domain/tool/service.go)
--      derives slug from name via ToSlug() whenever the request omits it.
--
-- Safe to re-run. Affects one schema at a time via MigrateAllTenants.

-- 0. Ensure slug column exists.
ALTER TABLE tool ADD COLUMN IF NOT EXISTS slug VARCHAR(255);

-- 1. Backfill NULL / empty / emergency-pattern slugs. The id-suffix guarantees
-- uniqueness without needing a subquery over the current table state.
UPDATE tool
   SET slug = COALESCE(
       NULLIF(
           regexp_replace(
               regexp_replace(lower(btrim(name)), '[^a-z0-9_.-]+', '_', 'g'),
               '^[_.-]+|[_.-]+$',
               '',
               'g'
           ),
           ''
       ),
       'tool'
   ) || '-' || substr(id::text, 1, 8)
 WHERE slug IS NULL OR slug = '' OR slug ~ '^tool-[0-9a-f]{10}$';

-- 2. Enforce NOT NULL + UNIQUE. ADD CONSTRAINT IF NOT EXISTS does not exist
-- in PostgreSQL, so wrap in a DO block that checks pg_constraint first.
ALTER TABLE tool ALTER COLUMN slug SET NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conrelid = 'tool'::regclass
           AND contype = 'u'
           AND conname = 'tool_slug_key'
    ) THEN
        ALTER TABLE tool ADD CONSTRAINT tool_slug_key UNIQUE (slug);
    END IF;
END $$;

-- 3. Drop the emergency default (may not exist on fresh tenants — safe either way).
ALTER TABLE tool ALTER COLUMN slug DROP DEFAULT;
