-- CORE-SEED-001-paired: platform catalog manifest default templates.
-- 11 manifest rows, one per PlatformCatalogKind, each pointing at a
-- concrete catalog already seeded. Lets admin UI and /ah-status query
-- the "catalog of catalogs" without reconstructing the registry from
-- code at runtime.

CREATE TABLE IF NOT EXISTS ah_core.platform_catalog_manifest_default_template (
    id                          UUID PRIMARY KEY,
    slug                        TEXT NOT NULL UNIQUE,
    kind                        TEXT NOT NULL,
    target_table_slug           TEXT NOT NULL,
    migration_number            INTEGER NOT NULL UNIQUE,
    loader_package              TEXT NOT NULL,
    expected_row_count          INTEGER NOT NULL,
    requires_admin_approval     BOOLEAN NOT NULL DEFAULT FALSE,
    is_tenant_shared            BOOLEAN NOT NULL DEFAULT TRUE,
    description                 TEXT NOT NULL,
    is_recommended              BOOLEAN NOT NULL DEFAULT TRUE,
    is_active                   BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order                  INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_platform_catalog_manifest_dt_kind
    ON ah_core.platform_catalog_manifest_default_template(kind);
CREATE INDEX IF NOT EXISTS idx_platform_catalog_manifest_dt_active
    ON ah_core.platform_catalog_manifest_default_template(is_active);

INSERT INTO ah_core.platform_catalog_manifest_default_template
    (id, slug, kind, target_table_slug, migration_number, loader_package,
     expected_row_count, requires_admin_approval, is_tenant_shared,
     description, is_recommended, is_active, sort_order)
VALUES
    ('00000006-0000-0000-0000-000000000001',
     'builtin_subagent_default_template',
     'subagent_roster',
     'builtin_subagent_default_template',
     61, 'core', 7, FALSE, TRUE,
     'SUB-002 builtin subagent roster: 7 curated roles (researcher/coder/reviewer/explorer/planner/curator/documenter).',
     TRUE, TRUE, 10),

    ('00000006-0000-0000-0000-000000000002',
     'custom_agent_definition_default_template',
     'agent_definition',
     'custom_agent_definition_default_template',
     62, 'core', 4, FALSE, TRUE,
     'SUB-003 custom agent starter shapes: 4 templates (bare/researcher-derived/coder-derived/dual-loop).',
     TRUE, TRUE, 20),

    ('00000006-0000-0000-0000-000000000003',
     'subagent_toolset_policy_default_template',
     'toolset_policy',
     'subagent_toolset_policy_default_template',
     47, 'core', 4, FALSE, TRUE,
     'SUB-005 toolset isolation modes per spawned subagent.',
     TRUE, TRUE, 30),

    ('00000006-0000-0000-0000-000000000004',
     'subagent_inheritance_mode_default_template',
     'inheritance_mode',
     'subagent_inheritance_mode_default_template',
     46, 'core', 4, FALSE, TRUE,
     'SUB-006 permission inheritance modes (extend-parent / restrict-readonly / etc).',
     TRUE, TRUE, 40),

    ('00000006-0000-0000-0000-000000000005',
     'subagent_return_summary_default_template',
     'summary_shape',
     'subagent_return_summary_default_template',
     53, 'core', 4, FALSE, TRUE,
     'SUB-010 subagent return summary shapes (success-with-artifacts / structured-findings / plan-only).',
     TRUE, TRUE, 50),

    ('00000006-0000-0000-0000-000000000006',
     'session_fork_strategy_default_template',
     'fork_strategy',
     'session_fork_strategy_default_template',
     60, 'core', 3, FALSE, TRUE,
     'PERSIST-005a session fork strategies (full_copy / branch_pointer / snapshot_isolated).',
     TRUE, TRUE, 60),

    ('00000006-0000-0000-0000-000000000007',
     'background_subagent_lane_default_template',
     'background_lane',
     'background_subagent_lane_default_template',
     63, 'core', 4, FALSE, TRUE,
     'SUB-008 background subagent operational lanes (quick-glance / planner / research / coder).',
     TRUE, TRUE, 70),

    ('00000006-0000-0000-0000-000000000008',
     'context_section_budget_template',
     'context_policy',
     'context_section_budget_template',
     27, 'core', 8, FALSE, TRUE,
     'CTX-* context section budget templates: per-section token allocation policies.',
     TRUE, TRUE, 80),

    ('00000006-0000-0000-0000-000000000009',
     'permission_audit_retention_default_template',
     'permission_policy',
     'permission_audit_retention_default_template',
     48, 'core', 4, FALSE, TRUE,
     'PERM-010 audit retention windows (30d / 90d / 365d / 7y).',
     TRUE, TRUE, 90),

    ('00000006-0000-0000-0000-000000000010',
     'extension_descriptor_default_template',
     'extension_descriptor',
     'extension_descriptor_default_template',
     52, 'core', 5, FALSE, TRUE,
     'EXT-* extension descriptor templates: plugin manifest scaffolds.',
     TRUE, TRUE, 100),

    ('00000006-0000-0000-0000-000000000011',
     'multi_agent_coordination_plan_default_template',
     'operational_template',
     'multi_agent_coordination_plan_default_template',
     58, 'core', 4, FALSE, TRUE,
     'SUB-011 multi-agent coordination plan templates (sequential / parallel / pipeline / dag).',
     TRUE, TRUE, 110)
ON CONFLICT (slug) DO NOTHING;
