CREATE TABLE IF NOT EXISTS agent_hook (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id    UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    event       VARCHAR(30)  NOT NULL,  -- pre_tool_use, post_tool_use, session_start, session_end
    matcher     VARCHAR(255),           -- glob pattern for tool name (e.g. "execute-sql", "document_*")
    hook_type   VARCHAR(20)  NOT NULL,  -- http, prompt
    config      JSONB        NOT NULL DEFAULT '{}',
    enabled     BOOLEAN      NOT NULL DEFAULT TRUE,
    priority    INTEGER      NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_agent_hook_agent_id ON agent_hook(agent_id);
CREATE INDEX idx_agent_hook_event ON agent_hook(agent_id, event) WHERE enabled = TRUE;
