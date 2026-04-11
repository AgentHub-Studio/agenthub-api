DROP INDEX IF EXISTS idx_agent_memory_execution_id;
DROP INDEX IF EXISTS idx_agent_memory_scope;

ALTER TABLE agent_memory
    DROP COLUMN IF EXISTS execution_id,
    DROP COLUMN IF EXISTS scope;
