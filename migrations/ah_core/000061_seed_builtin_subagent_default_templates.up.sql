-- SUB-002-paired: built-in subagent default templates.
-- 7 curated subagent role templates so fresh tenants spawn ready-to-use
-- subagents (researcher/coder/reviewer/etc) without designing each from
-- scratch. Each template references SUB-005 (toolset policy), SUB-006
-- (inheritance mode) and SUB-010 (summary shape) by slug — the runner
-- resolves those at spawn time. Aligned byte-for-byte with the
-- BuiltinSubagentRole enum in internal/domain/chat/agentic/builtin_subagents.go.

CREATE TABLE IF NOT EXISTS ah_core.builtin_subagent_default_template (
    id                              UUID PRIMARY KEY,
    slug                            TEXT NOT NULL UNIQUE,
    role                            TEXT NOT NULL,
    name                            TEXT NOT NULL,
    description                     TEXT NOT NULL,
    system_prompt_template          TEXT NOT NULL,
    default_toolset_policy_slug     TEXT NOT NULL,
    default_inheritance_mode_slug   TEXT NOT NULL,
    default_summary_shape_slug      TEXT NOT NULL,
    typical_task_class              TEXT NOT NULL DEFAULT 'general',
    recommended_for_tenant_kind     TEXT NOT NULL DEFAULT 'general',
    requires_admin_review           BOOLEAN NOT NULL DEFAULT FALSE,
    is_recommended                  BOOLEAN NOT NULL DEFAULT TRUE,
    is_active                       BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order                      INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_builtin_subagent_dt_role
    ON ah_core.builtin_subagent_default_template(role);
CREATE INDEX IF NOT EXISTS idx_builtin_subagent_dt_active
    ON ah_core.builtin_subagent_default_template(is_active);
CREATE INDEX IF NOT EXISTS idx_builtin_subagent_dt_recommended
    ON ah_core.builtin_subagent_default_template(is_recommended)
    WHERE is_recommended = TRUE;

INSERT INTO ah_core.builtin_subagent_default_template
    (id, slug, role, name, description, system_prompt_template,
     default_toolset_policy_slug, default_inheritance_mode_slug,
     default_summary_shape_slug,
     typical_task_class, recommended_for_tenant_kind,
     requires_admin_review, is_recommended, is_active, sort_order)
VALUES
    ('00000003-0000-0000-0000-000000000001',
     'researcher-baseline', 'researcher',
     'Researcher — Baseline',
     'Investigates topics by reading documentation, searching code, and synthesizing findings. Read-only by default. Returns a concise summary plus citations.',
     'You are a research subagent. Investigate the question thoroughly using read-only tools. Cite every claim by file path or URL. Return a brief summary plus a list of citations. Do not modify any state.',
     'documentation-readonly-allowlist',
     'extend-parent-rights',
     'success-with-artifacts',
     'investigation', 'general',
     FALSE, TRUE, TRUE, 10),

    ('00000003-0000-0000-0000-000000000002',
     'coder-baseline', 'coder',
     'Coder — Baseline',
     'Implements a focused code change scoped to the parent task. Has write access via inherited parent rights. Returns a brief summary of files changed.',
     'You are a coding subagent. Implement the requested change with minimal scope. Keep edits surgical. Do not add features beyond what was asked. Return a one-paragraph summary citing each file path you touched.',
     'code-write-scoped',
     'extend-parent-rights',
     'success-with-artifacts',
     'implementation', 'general',
     FALSE, TRUE, TRUE, 20),

    ('00000003-0000-0000-0000-000000000003',
     'reviewer-baseline', 'reviewer',
     'Reviewer — Baseline',
     'Reviews code for correctness, security, and style. Read-only. Returns findings as a structured list of issues with severity.',
     'You are a review subagent. Read the listed files and report concrete issues — correctness, security, style — with severity (blocker/major/minor) and file:line citations. Do not modify any code.',
     'documentation-readonly-allowlist',
     'restrict-to-readonly',
     'structured-findings',
     'review', 'general',
     FALSE, TRUE, TRUE, 30),

    ('00000003-0000-0000-0000-000000000004',
     'explorer-baseline', 'explorer',
     'Explorer — Baseline',
     'Maps the codebase structure: directories, key entry points, naming conventions. Read-only. Returns a structured overview.',
     'You are a codebase exploration subagent. Map the structure: directories, entry points, conventions, key abstractions. Return a hierarchical overview with file paths as anchors.',
     'documentation-readonly-allowlist',
     'restrict-to-readonly',
     'structured-findings',
     'exploration', 'general',
     FALSE, TRUE, TRUE, 40),

    ('00000003-0000-0000-0000-000000000005',
     'planner-baseline', 'planner',
     'Planner — Baseline',
     'Decomposes a goal into an ordered plan of steps. Read-only investigation. Returns a numbered plan with explicit dependencies.',
     'You are a planning subagent. Decompose the goal into a numbered, ordered plan. State dependencies between steps. Do not implement — only plan. Return the plan as a structured list.',
     'documentation-readonly-allowlist',
     'restrict-to-readonly',
     'plan-only',
     'planning', 'general',
     FALSE, TRUE, TRUE, 50),

    ('00000003-0000-0000-0000-000000000006',
     'curator-baseline', 'curator',
     'Curator — Baseline',
     'Curates and organizes information: grouping, tagging, deduplication. Read-only. Returns a structured catalog.',
     'You are a curator subagent. Organize the provided information: group, tag, and deduplicate. Return a structured catalog grouped by category, with brief descriptions per item.',
     'documentation-readonly-allowlist',
     'restrict-to-readonly',
     'structured-findings',
     'curation', 'general',
     FALSE, TRUE, TRUE, 60),

    ('00000003-0000-0000-0000-000000000007',
     'documenter-baseline', 'documenter',
     'Documenter — Baseline',
     'Writes documentation scoped to a feature or module. Has write access to documentation paths only. Returns a brief summary of doc files created.',
     'You are a documentation subagent. Write or update documentation pages for the specified scope. Be concise and structural — prefer headings and short paragraphs. Return a summary citing each doc file path you wrote.',
     'docs-write-scoped',
     'extend-parent-rights',
     'success-with-artifacts',
     'documentation', 'general',
     FALSE, TRUE, TRUE, 70)
ON CONFLICT (slug) DO NOTHING;
