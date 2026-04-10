DROP INDEX IF EXISTS idx_agent_skill_priority;
ALTER TABLE agent_skill DROP COLUMN IF EXISTS priority;
