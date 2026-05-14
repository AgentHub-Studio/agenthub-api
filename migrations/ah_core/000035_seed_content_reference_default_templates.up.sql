-- Seed platform-managed CONTENT REFERENCE DEFAULT TEMPLATES in ah_core.
-- Templates are blueprints paired with CTX-009 ContentReferenceRegistry.
-- Each row encodes which content kinds get registered as @ref tokens
-- (vs inlined verbatim) + per-kind size threshold + idle GC window,
-- so fresh tenants pick a policy without inventing thresholds.

CREATE TABLE IF NOT EXISTS ah_core.content_reference_default_template (
    id                          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    slug                        VARCHAR(96)  NOT NULL UNIQUE,
    name                        VARCHAR(160) NOT NULL,
    description                 TEXT         NOT NULL,
    -- enabled_kinds is a comma-separated list of CTX-009 ContentReferenceKind
    -- values (kb_chunk/tool_result/file_blob/web_fetch/memory_snapshot)
    -- whose content gets registered as @ref tokens. Empty string = none
    -- (everything inlined; reference registry not used).
    enabled_kinds               TEXT         NOT NULL DEFAULT '',
    -- min_bytes_to_reference is the threshold above which content is
    -- registered as @ref vs inlined verbatim. Below = inline; above = ref.
    min_bytes_to_reference      INTEGER      NOT NULL,
    -- idle_gc_seconds is how long unused references live before PurgeIdle
    -- sweeps them. 0 = never sweep.
    idle_gc_seconds             INTEGER      NOT NULL,
    -- target_use_case classifies the policy.
    target_use_case             VARCHAR(48)  NOT NULL,
    -- recommended_for_tenant_kind hints the audience.
    recommended_for_tenant_kind VARCHAR(48)  NOT NULL,
    -- requires_admin_review marks templates with audit implications.
    requires_admin_review       BOOLEAN      NOT NULL DEFAULT FALSE,
    is_recommended              BOOLEAN      NOT NULL DEFAULT FALSE,
    is_active                   BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_order                  INTEGER      NOT NULL DEFAULT 0,
    created_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_crdt_use_case ON ah_core.content_reference_default_template (target_use_case);
CREATE INDEX IF NOT EXISTS idx_ah_core_crdt_active   ON ah_core.content_reference_default_template (is_active);
CREATE INDEX IF NOT EXISTS idx_ah_core_crdt_rec      ON ah_core.content_reference_default_template (is_recommended) WHERE is_recommended = TRUE;

-- Seed catalog: 6 profiles. Use cases: general / research / kb_heavy /
-- code / cost_strict / dev_debug. Threshold ladder: dev (always ref) →
-- balanced → cost_strict (almost never ref).

INSERT INTO ah_core.content_reference_default_template
    (slug, name, description, enabled_kinds, min_bytes_to_reference,
     idle_gc_seconds, target_use_case, recommended_for_tenant_kind,
     requires_admin_review, is_recommended, sort_order)
VALUES
    ('balanced-default',
     'Balanced Default',
     'Reference KB chunks + tool results + file blobs above 4KB. Idle 24h GC. Safe one-click for general tenants — most hot content stays inline; only large stuff gets opaque @ref tokens.',
     'kb_chunk,tool_result,file_blob',
     4096, 86400,
     'general', 'general', FALSE, TRUE, 10),

    ('research-heavy',
     'Research Heavy (KB-aggressive)',
     'Aggressively reference KB chunks (1KB threshold) for research workflows reading many sources. Tool results + web fetches also referenced. Long idle window (7 days) for replay debugging.',
     'kb_chunk,tool_result,web_fetch',
     1024, 604800,
     'research', 'general', FALSE, TRUE, 20),

    ('kb-heavy-only',
     'KB Heavy (KB chunks only)',
     'Only KB chunks get referenced. Tool results + file blobs always inline. Use for tenants whose KBs are huge but tool outputs stay small. Idle 12h.',
     'kb_chunk',
     2048, 43200,
     'general', 'general', FALSE, TRUE, 30),

    ('code-heavy',
     'Code Heavy (tool results + file blobs)',
     'Reference tool results (test runs, build logs) and file blobs (code files) but inline KB chunks (small docstrings stay verbatim). Idle 24h.',
     'tool_result,file_blob',
     4096, 86400,
     'code', 'general', FALSE, TRUE, 40),

    ('cost-strict',
     'Cost Strict (max inlining)',
     'Reference ALL kinds but with very high threshold (16KB) — almost everything inlined. Use when @ref token resolution is too costly (e.g. cross-region storage). REQUIRES ADMIN REVIEW (defeats CTX-009 cache benefits).',
     'kb_chunk,tool_result,file_blob,web_fetch,memory_snapshot',
     16384, 21600,
     'general', 'general', TRUE, TRUE, 50),

    ('dev-debug',
     'Dev Debug (no references)',
     'Disable content references entirely — everything inlined for verbose debugging. Massive prompts but full visibility. Use ONLY in dev environments. NOT recommended for production.',
     '',
     0, 0,
     'general', 'dev_local', FALSE, FALSE, 60)
ON CONFLICT (slug) DO NOTHING;
