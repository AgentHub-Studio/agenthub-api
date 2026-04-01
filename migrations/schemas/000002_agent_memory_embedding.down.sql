-- Revert agent_memory embedding additions.
DROP INDEX IF EXISTS idx_agent_memory_embedding;

ALTER TABLE agent_memory
    DROP COLUMN IF EXISTS embedding,
    DROP COLUMN IF EXISTS last_accessed_at;
