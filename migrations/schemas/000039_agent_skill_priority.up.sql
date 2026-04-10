-- P-C289-1: Add priority ordering to agent_skill bindings.
-- Skills are injected into the LLM system prompt in priority order (lower = higher priority).
-- Default 0 means existing bindings are ordered by created_at (preserved behavior).
ALTER TABLE agent_skill ADD COLUMN IF NOT EXISTS priority INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_agent_skill_priority ON agent_skill (agent_id, priority, created_at);
