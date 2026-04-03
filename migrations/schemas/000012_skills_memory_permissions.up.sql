-- 000012_skills_memory_permissions.up.sql
-- Issue #125: prompt templates (skills system), structured memory types, permission rules DSL.

-- ─── prompt_template: reusable system prompt templates per agent ─────────────

CREATE TABLE IF NOT EXISTS prompt_template (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id        UUID         REFERENCES agent (id) ON DELETE CASCADE,
    name            VARCHAR(255) NOT NULL,
    slug            VARCHAR(255) NOT NULL,
    description     TEXT         NOT NULL DEFAULT '',
    content         TEXT         NOT NULL,
    category        VARCHAR(100) NOT NULL DEFAULT 'custom',
    model_override  VARCHAR(255),
    allowed_tools   JSONB,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (agent_id, slug)
);

COMMENT ON TABLE  prompt_template                  IS 'Reusable system prompt templates; agent_id NULL = global built-in';
COMMENT ON COLUMN prompt_template.slug             IS 'URL-safe identifier (e.g. rag_assistant)';
COMMENT ON COLUMN prompt_template.category         IS 'general | rag | data | api | custom';
COMMENT ON COLUMN prompt_template.model_override   IS 'Optional model to use when this template is active';
COMMENT ON COLUMN prompt_template.allowed_tools    IS 'JSON array of tool slugs allowed when this template is active';

CREATE INDEX idx_prompt_template_agent_id ON prompt_template (agent_id);
CREATE INDEX idx_prompt_template_category ON prompt_template (category);

-- ─── agent_memory: structured memory type ───────────────────────────────────

ALTER TABLE agent_memory
    ADD COLUMN memory_type VARCHAR(50) NOT NULL DEFAULT 'general';

COMMENT ON COLUMN agent_memory.memory_type IS 'user | feedback | project | reference | general';

CREATE INDEX idx_agent_memory_type ON agent_memory (agent_id, memory_type);

-- ─── agent: permission rules DSL ────────────────────────────────────────────

ALTER TABLE agent
    ADD COLUMN permission_rules JSONB;

COMMENT ON COLUMN agent.permission_rules IS '{"allow":["tool(pattern)"],"deny":["tool(pattern)"],"confirm":["tool(pattern)"]}';
