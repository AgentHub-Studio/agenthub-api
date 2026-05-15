-- Seed platform-managed MCP SERVER CATALOG in ah_core.
-- Catalog entries describe AVAILABLE servers — they are NOT active per-
-- tenant configurations. Tenants enable a catalog entry by inserting a
-- tenant-scoped row in their schema's `mcp_server_config` table (the
-- per-tenant runtime config table — see CLAUDE.md MCP section).
--
-- Inspired by:
--   - PDF arXiv:2604.14228v1 Section 6.1 (MCP servers as one of 4
--     extension mechanisms)
--   - Anthropic MCP reference catalog
--
-- AgentHub is predominantly WEB → only HTTP-transport servers are
-- catalogued. STDIO servers (filesystem, sqlite-cli, etc.) require local
-- subprocess and are NOT seeded — tenants register those manually if
-- their deployment supports it.

CREATE TABLE IF NOT EXISTS ah_core.mcp_server (
    id                UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name              VARCHAR(255) NOT NULL,
    -- slug is the stable identifier for this catalog entry.
    slug              VARCHAR(64)  NOT NULL UNIQUE,
    description       TEXT,
    -- transport_type is the wire protocol — seed catalog is HTTP-only
    -- for web compatibility; tenants may register stdio servers.
    transport_type    VARCHAR(32)  NOT NULL DEFAULT 'http',
    -- http_base_url is the public endpoint template for the server.
    -- Tenants override or set authentication on top.
    http_base_url     TEXT,
    -- category groups servers in the UI picker.
    -- One of: search / development / monitoring / collaboration /
    --         data / utility / ai.
    category          VARCHAR(32)  NOT NULL,
    -- icon_slug is the UI asset identifier (CDN-resolved).
    icon_slug         VARCHAR(64),
    -- requires_auth indicates whether the server needs credentials
    -- before any tool/resource invocation.
    requires_auth     BOOLEAN      NOT NULL DEFAULT FALSE,
    -- auth_type tells the UI which credential picker to show.
    -- One of: none / api_key / bearer_token / oauth2 / basic.
    auth_type         VARCHAR(32)  NOT NULL DEFAULT 'none',
    -- vendor identifies the upstream maintainer (anthropic / github /
    -- gitlab / brave / sentry / slack / postgres / generic / etc.).
    vendor            VARCHAR(64)  NOT NULL,
    -- documentation_url points users to vendor docs.
    documentation_url TEXT,
    -- is_official flags entries published or audited by Anthropic
    -- (or the named vendor's official org). Drives a UI badge.
    is_official       BOOLEAN      NOT NULL DEFAULT FALSE,
    -- is_active controls catalog visibility (allows soft-disable
    -- without dropping rows).
    is_active         BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_order        INTEGER      NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_mcp_server_slug      ON ah_core.mcp_server (slug);
CREATE INDEX IF NOT EXISTS idx_ah_core_mcp_server_is_active ON ah_core.mcp_server (is_active);
CREATE INDEX IF NOT EXISTS idx_ah_core_mcp_server_category  ON ah_core.mcp_server (category);
CREATE INDEX IF NOT EXISTS idx_ah_core_mcp_server_vendor    ON ah_core.mcp_server (vendor);

-- ============================
-- SEARCH (2)
-- ============================
INSERT INTO ah_core.mcp_server (slug, name, description, transport_type, http_base_url, category, icon_slug, requires_auth, auth_type, vendor, documentation_url, is_official, sort_order) VALUES
('brave-search',
 'Brave Search',
 'Web search via Brave Search API. Privacy-respecting general-purpose search with web/news/image results.',
 'http', 'https://api.search.brave.com/res/v1', 'search',
 'brave', TRUE, 'api_key', 'brave',
 'https://brave.com/search/api/', TRUE, 10),

('web-fetch',
 'Web Fetch',
 'Fetch arbitrary HTTP(S) URLs and return content. Generic fetcher for unauth public resources.',
 'http', '', 'search',
 'web', FALSE, 'none', 'anthropic',
 'https://modelcontextprotocol.io/servers/fetch', TRUE, 20);

-- ============================
-- DEVELOPMENT (3)
-- ============================
INSERT INTO ah_core.mcp_server (slug, name, description, transport_type, http_base_url, category, icon_slug, requires_auth, auth_type, vendor, documentation_url, is_official, sort_order) VALUES
('github',
 'GitHub',
 'GitHub API: repos, issues, pull requests, code search, commits, workflow runs.',
 'http', 'https://api.github.com', 'development',
 'github', TRUE, 'oauth2', 'github',
 'https://docs.github.com/en/rest', TRUE, 30),

('gitlab',
 'GitLab',
 'GitLab API: projects, issues, merge requests, pipelines.',
 'http', 'https://gitlab.com/api/v4', 'development',
 'gitlab', TRUE, 'oauth2', 'gitlab',
 'https://docs.gitlab.com/ee/api/', TRUE, 40),

('postgres-readonly',
 'PostgreSQL (read-only)',
 'Run SELECT queries against a PostgreSQL database. WRITE statements are blocked at the protocol layer.',
 'http', '', 'development',
 'postgres', TRUE, 'basic', 'postgres',
 'https://modelcontextprotocol.io/servers/postgres', TRUE, 50);

-- ============================
-- MONITORING (1)
-- ============================
INSERT INTO ah_core.mcp_server (slug, name, description, transport_type, http_base_url, category, icon_slug, requires_auth, auth_type, vendor, documentation_url, is_official, sort_order) VALUES
('sentry',
 'Sentry',
 'Sentry API: issues, events, releases, performance traces.',
 'http', 'https://sentry.io/api/0', 'monitoring',
 'sentry', TRUE, 'bearer_token', 'sentry',
 'https://docs.sentry.io/api/', TRUE, 60);

-- ============================
-- COLLABORATION (1)
-- ============================
INSERT INTO ah_core.mcp_server (slug, name, description, transport_type, http_base_url, category, icon_slug, requires_auth, auth_type, vendor, documentation_url, is_official, sort_order) VALUES
('slack',
 'Slack',
 'Slack API: send messages, list channels, search history.',
 'http', 'https://slack.com/api', 'collaboration',
 'slack', TRUE, 'oauth2', 'slack',
 'https://api.slack.com/methods', TRUE, 70);

-- ============================
-- UTILITY (2)
-- ============================
INSERT INTO ah_core.mcp_server (slug, name, description, transport_type, http_base_url, category, icon_slug, requires_auth, auth_type, vendor, documentation_url, is_official, sort_order) VALUES
('memory',
 'Ephemeral Memory',
 'Per-session in-memory key/value store. Useful for short-lived scratchpad state.',
 'http', '', 'utility',
 'memory', FALSE, 'none', 'anthropic',
 'https://modelcontextprotocol.io/servers/memory', TRUE, 80),

('time',
 'Time & Calendar',
 'Get current time in any timezone, format dates, compute durations.',
 'http', '', 'utility',
 'time', FALSE, 'none', 'anthropic',
 'https://modelcontextprotocol.io/servers/time', TRUE, 90);
