-- Add scope and execution_id to agent_memory for workflow-scoped memory support.
-- scope:        'agent' (default), 'workflow', or 'execution'
-- execution_id: nullable; set when scope = 'execution' to link to a specific run

ALTER TABLE agent_memory
    ADD COLUMN IF NOT EXISTS scope        TEXT    NOT NULL DEFAULT 'agent',
    ADD COLUMN IF NOT EXISTS execution_id UUID;

CREATE INDEX IF NOT EXISTS idx_agent_memory_scope        ON agent_memory (agent_id, scope);
CREATE INDEX IF NOT EXISTS idx_agent_memory_execution_id ON agent_memory (execution_id) WHERE execution_id IS NOT NULL;
