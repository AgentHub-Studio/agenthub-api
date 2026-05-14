-- PERM-003b-paired: bubble mode policy default templates.
-- 5 templates spanning the depth ladder so fresh tenants pick proven
-- bubble-mode escalation policies without inventing them.

CREATE TABLE IF NOT EXISTS ah_core.bubble_mode_policy_default_template (
    id                              UUID PRIMARY KEY,
    slug                            TEXT NOT NULL UNIQUE,
    name                            TEXT NOT NULL,
    description                     TEXT NOT NULL,
    target_use_case                 TEXT NOT NULL,
    safety_posture                  TEXT NOT NULL,
    max_bubble_depth                INTEGER NOT NULL DEFAULT 0,
    requires_parent_resolver        BOOLEAN NOT NULL DEFAULT FALSE,
    records_chain_history           BOOLEAN NOT NULL DEFAULT FALSE,
    recommended_for_tenant_kind     TEXT NOT NULL DEFAULT 'general',
    requires_admin_review           BOOLEAN NOT NULL DEFAULT FALSE,
    is_recommended                  BOOLEAN NOT NULL DEFAULT FALSE,
    is_active                       BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order                      INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_bubble_mode_dt_use_case
    ON ah_core.bubble_mode_policy_default_template(target_use_case);
CREATE INDEX IF NOT EXISTS idx_bubble_mode_dt_posture
    ON ah_core.bubble_mode_policy_default_template(safety_posture);
CREATE INDEX IF NOT EXISTS idx_bubble_mode_dt_active
    ON ah_core.bubble_mode_policy_default_template(is_active);
CREATE INDEX IF NOT EXISTS idx_bubble_mode_dt_recommended
    ON ah_core.bubble_mode_policy_default_template(is_recommended)
    WHERE is_recommended = TRUE;

INSERT INTO ah_core.bubble_mode_policy_default_template
    (id, slug, name, description, target_use_case, safety_posture,
     max_bubble_depth, requires_parent_resolver, records_chain_history,
     recommended_for_tenant_kind, requires_admin_review,
     is_recommended, is_active, sort_order)
VALUES
    ('cccccccc-cccc-cccc-cccc-000000000001',
     'leaf-auto-deny',
     'Leaf — Auto Deny',
     'Most restrictive: max_bubble_depth=0 means no parent resolver, every Confirm-tier request is auto-denied. Used for leaf-level subagents with no orchestration above them. Defensive default that never silently approves.',
     'leaf_subagent', 'strict',
     0, FALSE, FALSE,
     'general', TRUE, TRUE, TRUE, 10),

    ('cccccccc-cccc-cccc-cccc-000000000002',
     'single-hop',
     'Single Hop',
     'Allows one bubble level: subagent → parent. Parent decides; no further escalation. Routine choice for two-tier orchestration (parent + worker).',
     'two_tier', 'balanced',
     1, TRUE, FALSE,
     'general', FALSE, TRUE, TRUE, 20),

    ('cccccccc-cccc-cccc-cccc-000000000003',
     'two-hop-routine',
     'Two Hop Routine',
     'Allows two bubble levels: subagent → parent → grandparent. Used for three-tier orchestration where the grandparent (typically a controller) makes final decisions. Records chain history for audit.',
     'three_tier_orchestration', 'balanced',
     2, TRUE, TRUE,
     'general', TRUE, TRUE, TRUE, 30),

    ('cccccccc-cccc-cccc-cccc-000000000004',
     'three-hop-orchestration',
     'Three Hop Orchestration',
     'Allows three bubble levels — deep orchestration for multi-stage agentic pipelines. Records chain history; admin review required because deep escalation chains can mask responsibility.',
     'multi_stage_pipeline', 'permissive',
     3, TRUE, TRUE,
     'general', TRUE, TRUE, TRUE, 40),

    ('cccccccc-cccc-cccc-cccc-000000000005',
     'five-hop-research',
     'Five Hop Research',
     'Deepest allowed bubble depth — for research/exploration agents where confirm requests must travel through a curator network. Records chain history; admin review and the recursion-safety SUB-005 template must be combined to prevent loops.',
     'research_curator_network', 'permissive',
     5, TRUE, TRUE,
     'general', TRUE, TRUE, TRUE, 50)
ON CONFLICT (slug) DO NOTHING;
