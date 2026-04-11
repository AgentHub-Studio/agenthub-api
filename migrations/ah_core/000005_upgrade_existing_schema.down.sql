-- Rollback: remove columns added in migration 5.
-- Note: rolling back on a live system is risky — only use in dev/test.
ALTER TABLE ah_core.tool
    DROP COLUMN IF EXISTS slug,
    DROP COLUMN IF EXISTS is_active;

ALTER TABLE ah_core.agent
    DROP COLUMN IF EXISTS agent_type,
    DROP COLUMN IF EXISTS enable_management,
    DROP COLUMN IF EXISTS is_active;
