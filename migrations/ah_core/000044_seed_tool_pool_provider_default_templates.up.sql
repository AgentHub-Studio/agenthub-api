-- TOOL-003-paired: tool pool provider default templates.
-- 6 provider blueprints covering all 5 ToolSources so fresh tenants
-- assemble a non-empty pool from day one.

CREATE TABLE IF NOT EXISTS ah_core.tool_pool_provider_default_template (
    id                              UUID PRIMARY KEY,
    slug                            TEXT NOT NULL UNIQUE,
    name                            TEXT NOT NULL,
    description                     TEXT NOT NULL,
    target_source                   TEXT NOT NULL,
    target_use_case                 TEXT NOT NULL,
    exposes_tool_names              JSONB NOT NULL DEFAULT '[]'::jsonb,
    default_priority                INTEGER NOT NULL DEFAULT 0,
    recommended_for_tenant_kind     TEXT NOT NULL DEFAULT 'general',
    requires_admin_review           BOOLEAN NOT NULL DEFAULT FALSE,
    is_recommended                  BOOLEAN NOT NULL DEFAULT FALSE,
    is_active                       BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order                      INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_tool_pool_provider_dt_source
    ON ah_core.tool_pool_provider_default_template(target_source);
CREATE INDEX IF NOT EXISTS idx_tool_pool_provider_dt_use_case
    ON ah_core.tool_pool_provider_default_template(target_use_case);
CREATE INDEX IF NOT EXISTS idx_tool_pool_provider_dt_active
    ON ah_core.tool_pool_provider_default_template(is_active);
CREATE INDEX IF NOT EXISTS idx_tool_pool_provider_dt_recommended
    ON ah_core.tool_pool_provider_default_template(is_recommended)
    WHERE is_recommended = TRUE;

INSERT INTO ah_core.tool_pool_provider_default_template
    (id, slug, name, description, target_source, target_use_case,
     exposes_tool_names, default_priority, recommended_for_tenant_kind,
     requires_admin_review, is_recommended, is_active, sort_order)
VALUES
    ('11111111-1111-1111-1111-000000000001',
     'builtin-readonly-core',
     'Builtin Read-Only Core',
     'Ships the read-only harness builtins (Read/Grep/Glob) that every fresh tenant needs to inspect data without mutating it. Always wins source precedence over skill/mcp shadowing.',
     'builtin',
     'inspection',
     '["Read","Grep","Glob"]'::jsonb,
     10, 'general', FALSE, TRUE, TRUE, 10),

    ('11111111-1111-1111-1111-000000000002',
     'builtin-mutating-core',
     'Builtin Mutating Core',
     'Ships the mutating harness builtins (Edit/Write/Bash) gated by PERM rules. Tenants that need agents to change state enable this; the deny-first PERM engine still controls each call.',
     'builtin',
     'mutation',
     '["Edit","Write","Bash"]'::jsonb,
     20, 'general', TRUE, TRUE, TRUE, 20),

    ('11111111-1111-1111-1111-000000000003',
     'skill-document-search',
     'Skill Document Search',
     'Materialises a tool from the document_search skill backed by an active knowledge base. Lets agents answer questions over tenant-uploaded documents on day one without manual tool wiring.',
     'skill',
     'rag_search',
     '["document_search"]'::jsonb,
     50, 'general', FALSE, TRUE, TRUE, 30),

    ('11111111-1111-1111-1111-000000000004',
     'mcp-filesystem-default',
     'MCP Filesystem Default',
     'Wires a default MCP filesystem provider so agents can list/read files in a sandboxed area. Recommended for engineering tenants that bridge local resources to agents via the MCP runtime.',
     'mcp',
     'external_integration',
     '["mcp_fs_list","mcp_fs_read"]'::jsonb,
     60, 'general', FALSE, TRUE, TRUE, 40),

    ('11111111-1111-1111-1111-000000000005',
     'subagent-readonly-allowlist',
     'Subagent Read-Only Allowlist',
     'Restricts subagent pools to read-only builtins via an allowlist. Used when a parent agent delegates investigation tasks but does not trust the subagent with mutation.',
     'subagent',
     'delegation_safety',
     '["Read","Grep","Glob"]'::jsonb,
     40, 'general', FALSE, TRUE, TRUE, 50),

    ('11111111-1111-1111-1111-000000000006',
     'extension-platform-utilities',
     'Extension Platform Utilities',
     'Sample extension provider exposing platform utility tools (token usage, conversation summary). Demonstrates EXT-001 contract and gives tenants a non-trivial example of vendor-shipped tools.',
     'extension',
     'observability',
     '["token_usage","conversation_summary"]'::jsonb,
     70, 'general', FALSE, TRUE, TRUE, 60)
ON CONFLICT (slug) DO NOTHING;
