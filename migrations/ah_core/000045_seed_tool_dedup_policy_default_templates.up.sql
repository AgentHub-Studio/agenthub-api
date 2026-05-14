-- TOOL-004-paired: tool dedup policy default templates.
-- 5 policy blueprints spanning every ToolDedupPolicy so fresh tenants
-- pick a version-resolution stance without inventing semantics.

CREATE TABLE IF NOT EXISTS ah_core.tool_dedup_policy_default_template (
    id                              UUID PRIMARY KEY,
    slug                            TEXT NOT NULL UNIQUE,
    name                            TEXT NOT NULL,
    description                     TEXT NOT NULL,
    target_policy                   TEXT NOT NULL,
    target_use_case                 TEXT NOT NULL,
    safety_posture                  TEXT NOT NULL,
    recommended_for_tenant_kind     TEXT NOT NULL DEFAULT 'general',
    requires_admin_review           BOOLEAN NOT NULL DEFAULT FALSE,
    is_recommended                  BOOLEAN NOT NULL DEFAULT FALSE,
    is_active                       BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order                      INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_tool_dedup_policy_dt_policy
    ON ah_core.tool_dedup_policy_default_template(target_policy);
CREATE INDEX IF NOT EXISTS idx_tool_dedup_policy_dt_use_case
    ON ah_core.tool_dedup_policy_default_template(target_use_case);
CREATE INDEX IF NOT EXISTS idx_tool_dedup_policy_dt_active
    ON ah_core.tool_dedup_policy_default_template(is_active);
CREATE INDEX IF NOT EXISTS idx_tool_dedup_policy_dt_recommended
    ON ah_core.tool_dedup_policy_default_template(is_recommended)
    WHERE is_recommended = TRUE;

INSERT INTO ah_core.tool_dedup_policy_default_template
    (id, slug, name, description, target_policy, target_use_case,
     safety_posture, recommended_for_tenant_kind, requires_admin_review,
     is_recommended, is_active, sort_order)
VALUES
    ('22222222-2222-2222-2222-000000000001',
     'default-source-rank',
     'Default Source-Rank Precedence',
     'Default for fresh tenants. When two ToolSources expose the same name, source-rank decides (builtin > subagent > extension > skill > mcp). Predictable for tenants who do not pin versions and accept latest-installed wins.',
     'prefer_source_rank',
     'general_default',
     'balanced',
     'general', FALSE, TRUE, TRUE, 10),

    ('22222222-2222-2222-2222-000000000002',
     'rolling-latest-version',
     'Rolling Latest Version',
     'Always picks the highest semver of a versioned tool. Suitable for tenants who follow upstream releases aggressively and treat tools as commodity utilities.',
     'prefer_latest_version',
     'continuous_upgrade',
     'progressive',
     'general', FALSE, TRUE, TRUE, 20),

    ('22222222-2222-2222-2222-000000000003',
     'admin-pinned-conservative',
     'Admin-Pinned Conservative',
     'Honours admin-declared pins per tool. Used by compliance-sensitive tenants who pin tool versions until QA signs off on an upgrade.',
     'prefer_pinned',
     'compliance_pin',
     'conservative',
     'general', TRUE, TRUE, TRUE, 30),

    ('22222222-2222-2222-2222-000000000004',
     'dual-version-migration',
     'Dual-Version Migration Window',
     'Keeps all versions visible by their fully-qualified name (foo@1, foo@2). Used during platform migrations when old agents still call legacy versions while new agents use the new contract.',
     'keep_all_versions',
     'migration_window',
     'permissive',
     'general', FALSE, TRUE, TRUE, 40),

    ('22222222-2222-2222-2222-000000000005',
     'fail-fast-no-drift',
     'Fail-Fast No Drift',
     'Refuses to assemble the pool when two versions of the same tool are present. Used by tenants who require admin sign-off on every version-add — surprises become errors instead of silent drift.',
     'deny_collision',
     'audit_strict',
     'strict',
     'general', TRUE, TRUE, TRUE, 50)
ON CONFLICT (slug) DO NOTHING;
