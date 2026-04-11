-- agent_mcp_server: binds agents to mcp_server_config entries.
-- Allows per-agent control over which MCP servers are active in the agentic loop.
CREATE TABLE IF NOT EXISTS agent_mcp_server (
    agent_id          UUID        NOT NULL REFERENCES agent(id)            ON DELETE CASCADE,
    mcp_server_id     UUID        NOT NULL REFERENCES mcp_server_config(id) ON DELETE CASCADE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (agent_id, mcp_server_id)
);

CREATE INDEX IF NOT EXISTS idx_agent_mcp_server_agent_id ON agent_mcp_server(agent_id);
