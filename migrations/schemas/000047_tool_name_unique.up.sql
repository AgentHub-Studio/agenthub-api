-- Add unique constraint on tool.name within a schema/tenant.
-- P-C338-1 (ACT-F3-15): prevents duplicate tool names that confuse skill binding.

-- Deduplicate: keep the most recently updated row for each name, removing earlier duplicates.
-- This covers seed migrations that inserted the same tool under different IDs.
DELETE FROM tool
WHERE id IN (
    SELECT id FROM (
        SELECT id,
               ROW_NUMBER() OVER (PARTITION BY name ORDER BY updated_at DESC, id DESC) AS rn
        FROM tool
    ) ranked
    WHERE rn > 1
);

-- Use CREATE UNIQUE INDEX with IF NOT EXISTS instead of ALTER TABLE ADD CONSTRAINT
-- to be idempotent in case the constraint was partially applied (e.g. pgx multi-statement
-- execution splitting the batch in an unexpected order).
CREATE UNIQUE INDEX IF NOT EXISTS uq_tool_name ON tool (name);
