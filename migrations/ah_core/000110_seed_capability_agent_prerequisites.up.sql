-- Seed capability agent prerequisite rows in ah_core
-- (migration 000110). These 6 rows encode the minimum viable configuration
-- each capability agent requires to function — referencing feature flags and
-- skills that must exist for the agent to work properly. Two prerequisites
-- per agent (one feature_flag + one skill):
--
-- Researcher (2 prerequisites):
--   prereq-researcher-citations-flag        — feature_flag: capability-citations
--   prereq-researcher-web-research-skill    — skill:        core-web-research
--
-- Analyst (2 prerequisites):
--   prereq-analyst-doc-citations-flag       — feature_flag: capability-doc-citations
--   prereq-analyst-doc-analysis-skill       — skill:        core-doc-analysis
--
-- Planner (2 prerequisites):
--   prereq-planner-task-tracking-flag       — feature_flag: capability-task-tracking
--   prereq-planner-task-workflow-skill      — skill:        core-task-workflow
--
-- Design notes:
--   - PRIMARY KEY (slug VARCHAR(120)) — human-readable key; no UUID needed.
--   - agent_slug VARCHAR(120): the capability agent this prerequisite belongs to.
--   - prerequisite_type VARCHAR(80): "feature_flag" or "skill".
--   - prerequisite_slug VARCHAR(120): slug of the referenced flag or skill.
--   - is_required BOOLEAN: all 6 rows are required (true).
--   - reason TEXT: human-readable explanation for the dependency.
--   - sort_order INTEGER: display order within each agent's prerequisite list.
--   - UNIQUE (agent_slug, prerequisite_type, prerequisite_slug): prevents duplicates.
--   - ON CONFLICT (slug) DO NOTHING — idempotent seeds.
--   - Table is created here in ah_core (schema created by migration 000001).

CREATE TABLE IF NOT EXISTS ah_core.capability_agent_prerequisite (
    slug               VARCHAR(120) PRIMARY KEY,
    agent_slug         VARCHAR(120) NOT NULL,
    prerequisite_type  VARCHAR(80)  NOT NULL,
    prerequisite_slug  VARCHAR(120) NOT NULL,
    is_required        BOOLEAN      NOT NULL DEFAULT TRUE,
    reason             TEXT,
    sort_order         INTEGER      NOT NULL DEFAULT 0,
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (agent_slug, prerequisite_type, prerequisite_slug)
);

CREATE INDEX IF NOT EXISTS idx_ah_core_agent_prereq_agent_slug
    ON ah_core.capability_agent_prerequisite (agent_slug);

-- ==============================
-- 6 capability agent prerequisite rows
-- ==============================
INSERT INTO ah_core.capability_agent_prerequisite
    (slug, agent_slug, prerequisite_type, prerequisite_slug, is_required, reason, sort_order)
VALUES
    -- Researcher prerequisites
    ('prereq-researcher-citations-flag',
     'core-researcher', 'feature_flag', 'capability-citations', TRUE,
     'Researcher cites sources automatically — citations feature flag must be enabled', 1),
    ('prereq-researcher-web-research-skill',
     'core-researcher', 'skill', 'core-web-research', TRUE,
     'Researcher needs web search + fetch tools provided by core-web-research skill', 2),

    -- Analyst prerequisites
    ('prereq-analyst-doc-citations-flag',
     'core-analyst', 'feature_flag', 'capability-doc-citations', TRUE,
     'Analyst cites document sources — doc-citations feature flag must be enabled', 1),
    ('prereq-analyst-doc-analysis-skill',
     'core-analyst', 'skill', 'core-doc-analysis', TRUE,
     'Analyst needs doc search + read tools provided by core-doc-analysis skill', 2),

    -- Planner prerequisites
    ('prereq-planner-task-tracking-flag',
     'core-planner', 'feature_flag', 'capability-task-tracking', TRUE,
     'Planner tracks tasks automatically — task-tracking feature flag must be enabled', 1),
    ('prereq-planner-task-workflow-skill',
     'core-planner', 'skill', 'core-task-workflow', TRUE,
     'Planner needs todo create + list tools provided by core-task-workflow skill', 2)

ON CONFLICT (slug) DO NOTHING;
