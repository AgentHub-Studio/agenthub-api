-- Seed capability feature flag rows in ah_core
-- (migration 000103). These 6 rows provide per-capability feature toggles
-- adapted from Claude Code feature flags in Appendix A.2 Table 8. Each flag
-- controls whether a capability-layer feature is active for the tenant:
--
--   capability-citations               (research,     default_enabled=TRUE,  sort 1)
--   capability-task-tracking           (planning,     default_enabled=TRUE,  sort 2)
--   capability-kb-indexing             (research,     default_enabled=TRUE,  sort 3)
--   capability-subagent-delegation     (orchestration, default_enabled=FALSE, sort 4)
--   capability-doc-citations           (analysis,     default_enabled=TRUE,  sort 5)
--   capability-progressive-summarization (memory,     default_enabled=TRUE,  sort 6)
--
-- Design notes:
--   - PRIMARY KEY (slug VARCHAR(120)) — human-readable key; no UUID needed.
--   - ON CONFLICT (slug) DO NOTHING — idempotent seeds.
--   - sort_order — controls display order in management UIs.
--   - default_enabled=FALSE for subagent-delegation: spawning subagents requires
--     explicit opt-in to avoid unintended cost and recursion in new tenants.
--
-- The capability_feature_flag table is created here in ah_core
-- (schema created by migration 000001).

CREATE TABLE IF NOT EXISTS ah_core.capability_feature_flag (
    slug             VARCHAR(120) PRIMARY KEY,
    name             VARCHAR(255) NOT NULL,
    description      TEXT,
    default_enabled  BOOLEAN      NOT NULL DEFAULT TRUE,
    category         VARCHAR(80)  NOT NULL,
    sort_order       INTEGER      NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_cff_category
    ON ah_core.capability_feature_flag (category);

CREATE INDEX IF NOT EXISTS idx_ah_core_cff_sort
    ON ah_core.capability_feature_flag (sort_order);

CREATE INDEX IF NOT EXISTS idx_ah_core_cff_enabled
    ON ah_core.capability_feature_flag (default_enabled);

-- ============================
-- 6 capability feature flag rows
-- ============================
INSERT INTO ah_core.capability_feature_flag
    (slug, name, description, default_enabled, category, sort_order)
VALUES
    (
        'capability-citations',
        'Web Citation Tracking',
        'Automatically cite web sources in research outputs',
        TRUE,
        'research',
        1
    ),
    (
        'capability-task-tracking',
        'Task Progress Tracking',
        'Enable structured task list management for planner agent',
        TRUE,
        'planning',
        2
    ),
    (
        'capability-kb-indexing',
        'Knowledge Base Auto-Indexing',
        'Automatically index research findings into knowledge base',
        TRUE,
        'research',
        3
    ),
    (
        'capability-subagent-delegation',
        'Subagent Delegation',
        'Allow capability agents to spawn and delegate to subagents',
        FALSE,
        'orchestration',
        4
    ),
    (
        'capability-doc-citations',
        'Document Citation Tracking',
        'Track and cite document sources in analysis outputs',
        TRUE,
        'analysis',
        5
    ),
    (
        'capability-progressive-summarization',
        'Progressive Summarization',
        'Summarize context progressively as it grows',
        TRUE,
        'memory',
        6
    )

ON CONFLICT (slug) DO NOTHING;
