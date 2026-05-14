-- SUB-003-paired: starter custom agent definition shapes.
-- 4 templates that fresh tenants can clone-and-modify as starting
-- points for their own subagents. Each template references SUB-005
-- (toolset) + SUB-006 (inheritance) + SUB-010 (summary) by slug, and
-- optionally a SUB-002 builtin slug as lineage metadata.

CREATE TABLE IF NOT EXISTS ah_core.custom_agent_definition_default_template (
    id                              UUID PRIMARY KEY,
    slug                            TEXT NOT NULL UNIQUE,
    shape_kind                      TEXT NOT NULL, -- bare | derived | dual_loop | researcher_derived
    name                            TEXT NOT NULL,
    description                     TEXT NOT NULL,
    system_prompt_template          TEXT NOT NULL,
    toolset_policy_slug             TEXT NOT NULL,
    inheritance_mode_slug           TEXT NOT NULL,
    summary_shape_slug              TEXT NOT NULL,
    derived_from_builtin_slug       TEXT NOT NULL DEFAULT '', -- '' means none (greenfield)
    suggested_visibility            TEXT NOT NULL DEFAULT 'private',
    recommended_starting_version    TEXT NOT NULL DEFAULT '0.1.0',
    is_recommended                  BOOLEAN NOT NULL DEFAULT TRUE,
    is_active                       BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order                      INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_custom_agent_def_dt_shape_kind
    ON ah_core.custom_agent_definition_default_template(shape_kind);
CREATE INDEX IF NOT EXISTS idx_custom_agent_def_dt_active
    ON ah_core.custom_agent_definition_default_template(is_active);
CREATE INDEX IF NOT EXISTS idx_custom_agent_def_dt_derived
    ON ah_core.custom_agent_definition_default_template(derived_from_builtin_slug)
    WHERE derived_from_builtin_slug <> '';

INSERT INTO ah_core.custom_agent_definition_default_template
    (id, slug, shape_kind, name, description, system_prompt_template,
     toolset_policy_slug, inheritance_mode_slug, summary_shape_slug,
     derived_from_builtin_slug, suggested_visibility,
     recommended_starting_version, is_recommended, is_active, sort_order)
VALUES
    ('00000004-0000-0000-0000-000000000001',
     'bare-greenfield',
     'bare',
     'Bare Greenfield Agent',
     'Minimum viable shape with no lineage to any SUB-002 builtin. Read-only toolset, restrict-to-readonly inheritance, structured findings summary. Used when the tenant wants to design an agent from scratch.',
     'You are a domain-specialized agent. Follow the tenant''s instructions exactly. Cite sources for every claim. Do not modify any state unless explicitly instructed.',
     'documentation-readonly-allowlist',
     'restrict-to-readonly',
     'structured-findings',
     '',
     'private',
     '0.1.0', TRUE, TRUE, 10),

    ('00000004-0000-0000-0000-000000000002',
     'researcher-derived',
     'researcher_derived',
     'Researcher Derived Fork',
     'Cloned from SUB-002 researcher-baseline as a starting point. Tenants then specialize the system prompt for their domain (e.g., legal research, medical literature, financial filings).',
     'You are a research agent specialized for {{tenant_domain}}. Investigate questions in this domain thoroughly using read-only tools. Cite every claim by file path or URL. Return a brief summary plus a list of citations. Do not modify any state.',
     'documentation-readonly-allowlist',
     'extend-parent-rights',
     'success-with-artifacts',
     'researcher-baseline',
     'private',
     '0.1.0', TRUE, TRUE, 20),

    ('00000004-0000-0000-0000-000000000003',
     'coder-derived',
     'derived',
     'Coder Derived Fork',
     'Cloned from SUB-002 coder-baseline. Tenants narrow the toolset or refine prompt to enforce internal coding standards (style guides, security rules, framework conventions).',
     'You are a coding agent that follows {{tenant_org}} internal coding standards. Implement scoped changes with minimal surface area. Apply our style guide. Return a one-paragraph summary citing each file path.',
     'code-write-scoped',
     'extend-parent-rights',
     'success-with-artifacts',
     'coder-baseline',
     'private',
     '0.1.0', TRUE, TRUE, 30),

    ('00000004-0000-0000-0000-000000000004',
     'dual-loop-planner',
     'dual_loop',
     'Dual-Loop Planner Agent',
     'Dual-loop pattern: first plans (using planner shape) then implements (using coder shape) in two separate runs. This template seeds the planner half. Tenant must also register a paired coder-derived agent.',
     'You are the planning half of a dual-loop agent. Decompose the goal into a numbered plan with explicit dependencies. Do not implement — only plan. Hand off the plan to the paired coder-derived agent.',
     'documentation-readonly-allowlist',
     'restrict-to-readonly',
     'plan-only',
     'planner-baseline',
     'private',
     '0.1.0', TRUE, TRUE, 40)
ON CONFLICT (slug) DO NOTHING;
