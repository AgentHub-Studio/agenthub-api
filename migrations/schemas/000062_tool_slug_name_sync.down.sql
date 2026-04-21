-- Restore the emergency default so older binaries that do not set slug on INSERT
-- keep working. This does NOT reverse the backfill (slugs remain human-readable).
ALTER TABLE tool ALTER COLUMN slug SET DEFAULT (
    'tool-' || substr(md5(random()::text || clock_timestamp()::text), 1, 10)
);
