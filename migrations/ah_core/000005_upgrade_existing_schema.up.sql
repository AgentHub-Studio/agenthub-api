-- Migration 5: Upgrade existing ah_core schema created by Java-era migrations.
-- Adds columns expected by Go CoreToolLoader / CoreAgentLoader that were missing
-- from the old schema. All statements use IF NOT EXISTS / DO NOTHING so they are
-- safe on both fresh installs (where migration 1 already created columns) and
-- upgrades from legacy Java-created schemas.

-- ah_core.tool: add slug and is_active if missing (migration 1 creates them fresh)
ALTER TABLE ah_core.tool
    ADD COLUMN IF NOT EXISTS slug      VARCHAR(255),
    ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT TRUE;

-- Backfill slug from name (snake_case) for existing rows
UPDATE ah_core.tool
   SET slug = lower(regexp_replace(name, '[^a-zA-Z0-9]+', '_', 'g'))
 WHERE slug IS NULL;

-- Now enforce NOT NULL and unique (safe if already set)
ALTER TABLE ah_core.tool
    ALTER COLUMN slug SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_ah_core_tool_slug    ON ah_core.tool (slug);
CREATE INDEX IF NOT EXISTS idx_ah_core_tool_slug          ON ah_core.tool (slug);
CREATE INDEX IF NOT EXISTS idx_ah_core_tool_is_active     ON ah_core.tool (is_active);

-- ah_core.agent: add agent_type, enable_management, is_active if missing
ALTER TABLE ah_core.agent
    ADD COLUMN IF NOT EXISTS agent_type        VARCHAR(50)  NOT NULL DEFAULT 'ASSISTANT',
    ADD COLUMN IF NOT EXISTS enable_management BOOLEAN      NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS is_active         BOOLEAN      NOT NULL DEFAULT TRUE;

CREATE INDEX IF NOT EXISTS idx_ah_core_agent_type      ON ah_core.agent (agent_type);
CREATE INDEX IF NOT EXISTS idx_ah_core_agent_is_active ON ah_core.agent (is_active);
