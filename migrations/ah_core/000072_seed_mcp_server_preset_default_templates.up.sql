-- EXT-008a paired seed: MCP server preset connection templates.
-- Six ready-to-use MCP server presets covering common integrations so fresh
-- tenants can bootstrap MCP connections without writing config from scratch.
-- Two transport types: stdio (subprocess) and streamable_http (remote server).
-- Adapted for AgentHub web context: filesystem paths use /tmp/agenthub mounts.
--
-- Idempotent: ON CONFLICT (slug) DO NOTHING.

CREATE TABLE IF NOT EXISTS ah_core.mcp_server_preset_template (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    slug             TEXT        UNIQUE NOT NULL,
    label            TEXT        NOT NULL,
    description      TEXT        NOT NULL,
    transport_type   TEXT        NOT NULL CHECK (transport_type IN ('stdio', 'streamable_http')),
    command          TEXT,
    args             JSONB       NOT NULL DEFAULT '[]',
    env_keys         JSONB       NOT NULL DEFAULT '[]',
    base_url         TEXT,
    auto_start       BOOLEAN     NOT NULL DEFAULT false,
    sort_order       INTEGER     NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO ah_core.mcp_server_preset_template
    (id, slug, label, description, transport_type, command, args, env_keys, base_url, auto_start, sort_order)
VALUES
    (
        'ffff0001-0000-0000-0000-000000000001',
        'filesystem',
        'Filesystem',
        'Read and write files inside the /tmp/agenthub workspace mount. Useful for agents that process uploaded documents or produce artifacts.',
        'stdio',
        'npx',
        '["-y", "@modelcontextprotocol/server-filesystem", "/tmp/agenthub"]',
        '[]',
        NULL,
        true,
        10
    ),
    (
        'ffff0002-0000-0000-0000-000000000002',
        'github',
        'GitHub',
        'Search repositories, read files, open issues and create pull requests via the GitHub API. Requires GITHUB_PERSONAL_ACCESS_TOKEN in the environment.',
        'stdio',
        'npx',
        '["-y", "@modelcontextprotocol/server-github"]',
        '["GITHUB_PERSONAL_ACCESS_TOKEN"]',
        NULL,
        true,
        20
    ),
    (
        'ffff0003-0000-0000-0000-000000000003',
        'sqlite',
        'SQLite',
        'Query and update a SQLite database stored in the agent workspace. Useful for structured data tasks without a full SQL server.',
        'stdio',
        'uvx',
        '["mcp-server-sqlite", "--db-path", "/tmp/agenthub/agent.db"]',
        '[]',
        NULL,
        true,
        30
    ),
    (
        'ffff0004-0000-0000-0000-000000000004',
        'brave-search',
        'Brave Search',
        'Web and news search powered by the Brave Search API. Requires BRAVE_API_KEY in the environment.',
        'stdio',
        'npx',
        '["-y", "@modelcontextprotocol/server-brave-search"]',
        '["BRAVE_API_KEY"]',
        NULL,
        true,
        40
    ),
    (
        'ffff0005-0000-0000-0000-000000000005',
        'sequential-thinking',
        'Sequential Thinking',
        'Structured multi-step reasoning tool that helps agents decompose complex problems before acting.',
        'stdio',
        'npx',
        '["-y", "@modelcontextprotocol/server-sequential-thinking"]',
        '[]',
        NULL,
        true,
        50
    ),
    (
        'ffff0006-0000-0000-0000-000000000006',
        'remote-sse',
        'Remote SSE (Streamable HTTP)',
        'Template for connecting to a remote MCP server via the Streamable HTTP transport (POST /mcp with SSE). Replace base_url with the actual server URL.',
        'streamable_http',
        NULL,
        '[]',
        '[]',
        'https://mcp.example.com',
        false,
        60
    )
ON CONFLICT (slug) DO NOTHING;
