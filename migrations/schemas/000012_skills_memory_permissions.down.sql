-- 000012_skills_memory_permissions.down.sql

ALTER TABLE agent DROP COLUMN IF EXISTS permission_rules;

DROP INDEX IF EXISTS idx_agent_memory_type;
ALTER TABLE agent_memory DROP COLUMN IF EXISTS memory_type;

DROP TABLE IF EXISTS prompt_template;
