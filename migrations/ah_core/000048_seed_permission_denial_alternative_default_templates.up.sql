-- PERM-006-paired: permission denial alternative default templates.
-- 6 blueprints mapping commonly-denied tools to safer alternatives so
-- the LLM sees actionable `alt=` hints in PermissionDeniedFeedback
-- (PERM-006). Fresh tenants do not have to learn what tools exist —
-- the platform ships the pivots.

CREATE TABLE IF NOT EXISTS ah_core.permission_denial_alternative_default_template (
    id                              UUID PRIMARY KEY,
    slug                            TEXT NOT NULL UNIQUE,
    name                            TEXT NOT NULL,
    description                     TEXT NOT NULL,
    target_reason                   TEXT NOT NULL,
    target_retry_hint               TEXT NOT NULL,
    denied_tool_name                TEXT NOT NULL,
    alternative_tool_names          JSONB NOT NULL DEFAULT '[]'::jsonb,
    recommended_for_tenant_kind     TEXT NOT NULL DEFAULT 'general',
    requires_admin_review           BOOLEAN NOT NULL DEFAULT FALSE,
    is_recommended                  BOOLEAN NOT NULL DEFAULT FALSE,
    is_active                       BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order                      INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_perm_denial_alt_dt_reason
    ON ah_core.permission_denial_alternative_default_template(target_reason);
CREATE INDEX IF NOT EXISTS idx_perm_denial_alt_dt_retry
    ON ah_core.permission_denial_alternative_default_template(target_retry_hint);
CREATE INDEX IF NOT EXISTS idx_perm_denial_alt_dt_denied
    ON ah_core.permission_denial_alternative_default_template(denied_tool_name);
CREATE INDEX IF NOT EXISTS idx_perm_denial_alt_dt_active
    ON ah_core.permission_denial_alternative_default_template(is_active);
CREATE INDEX IF NOT EXISTS idx_perm_denial_alt_dt_recommended
    ON ah_core.permission_denial_alternative_default_template(is_recommended)
    WHERE is_recommended = TRUE;

INSERT INTO ah_core.permission_denial_alternative_default_template
    (id, slug, name, description, target_reason, target_retry_hint,
     denied_tool_name, alternative_tool_names,
     recommended_for_tenant_kind, requires_admin_review,
     is_recommended, is_active, sort_order)
VALUES
    ('55555555-5555-5555-5555-000000000001',
     'bash-to-sandboxed-shell',
     'Bash → Sandboxed Shell',
     'When Bash is denied (typical for non-engineering tenants), suggest the sandboxed shell that limits filesystem and network access. The LLM pivots without giving up.',
     'rule_match', 'suggest_alternative_tool',
     'Bash', '["shell_sandboxed","Read"]'::jsonb,
     'general', FALSE, TRUE, TRUE, 10),

    ('55555555-5555-5555-5555-000000000002',
     'execute-sql-to-document-search',
     'execute-sql → document_search',
     'When direct SQL is denied (most knowledge-base tenants), suggest document_search which uses pre-indexed embeddings. Same goal (retrieve info), safer surface.',
     'rule_match', 'suggest_alternative_tool',
     'execute-sql', '["document_search","Read"]'::jsonb,
     'general', FALSE, TRUE, TRUE, 20),

    ('55555555-5555-5555-5555-000000000003',
     'write-to-edit-fallback',
     'Write → Edit',
     'When Write is denied by a hook (typically because creating new files is restricted), Edit allows modifying existing files only — narrower surface that may still satisfy intent.',
     'hook_override', 'suggest_alternative_tool',
     'Write', '["Edit"]'::jsonb,
     'general', FALSE, TRUE, TRUE, 30),

    ('55555555-5555-5555-5555-000000000004',
     'http-fetch-rate-limited-wait',
     'http_fetch Rate-Limited Wait',
     'When http_fetch is rate-limited, the LLM should wait+retry not pivot. This template encodes the wait stance and no alternative — pivot would defeat the rate limit.',
     'rate_limit', 'wait_and_retry',
     'http_fetch', '[]'::jsonb,
     'general', FALSE, TRUE, TRUE, 40),

    ('55555555-5555-5555-5555-000000000005',
     'mcp-tool-prefilter-drop-pivot',
     'MCP Tool Pre-filter Drop → Builtin',
     'When an MCP tool is pre-filter-dropped (server offline or tenant policy), suggest the closest builtin counterpart so the LLM does not retry the unavailable tool.',
     'prefilter_drop', 'suggest_alternative_tool',
     'mcp_fs_list', '["Glob","Read"]'::jsonb,
     'general', FALSE, TRUE, TRUE, 50),

    ('55555555-5555-5555-5555-000000000006',
     'dontask-mode-confirm-required',
     'dont_ask Mode → User Confirmation',
     'When the session is in dont_ask mode and a confirm-tier tool was attempted, suggest no alternative; the path forward is asking the user explicitly to switch session mode.',
     'mode_block', 'request_user_confirmation',
     'Bash', '[]'::jsonb,
     'general', TRUE, TRUE, TRUE, 60)
ON CONFLICT (slug) DO NOTHING;
