-- 000062_tool_slug_name_sync
--
-- Cleans up the `tool.slug` column so the application owns slug generation:
--   1. Backfills slugs that match the emergency pattern "tool-<md5-prefix>"
--      (introduced during the iter-3 E2E loop as a NOT NULL workaround) with
--      a sanitized version of the tool name. Uniqueness is preserved by
--      appending the short UUID prefix when a conflict occurs.
--   2. Drops the random-hash DEFAULT so future INSERTs that forget to pass a
--      slug fail loudly instead of being silently decorated with a meaningless
--      identifier. The application (internal/domain/tool/service.go) now
--      derives slug from name via ToSlug() whenever the request omits it.
--
-- Safe to re-run. Affects one schema at a time via MigrateAllTenants.

-- 1. backfill
WITH generated AS (
    SELECT
        id,
        regexp_replace(
            lower(btrim(name)),
            '[^a-z0-9_.-]+',
            '_',
            'g'
        ) AS raw_slug
    FROM tool
    WHERE slug ~ '^tool-[0-9a-f]{10}$'
)
UPDATE tool t
   SET slug = CASE
       WHEN g.raw_slug = '' OR g.raw_slug ~ '^[_.-]+$'
           THEN 'tool-' || substr(t.id::text, 1, 8)
       WHEN EXISTS (
           SELECT 1 FROM tool o
            WHERE o.id <> t.id AND o.slug = btrim(g.raw_slug, '_.-')
       )
           THEN btrim(g.raw_slug, '_.-') || '-' || substr(t.id::text, 1, 8)
       ELSE btrim(g.raw_slug, '_.-')
   END
  FROM generated g
 WHERE t.id = g.id;

-- 2. drop the emergency default
ALTER TABLE tool ALTER COLUMN slug DROP DEFAULT;
