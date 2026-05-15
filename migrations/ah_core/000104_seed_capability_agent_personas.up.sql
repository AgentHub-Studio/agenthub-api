-- Seed capability agent persona rows in ah_core
-- (migration 000104). These 3 rows provide personality/identity profiles for
-- each capability agent, adapted from the Claude Code Identity Model §3
-- (identity, tone, and communication characteristics per capability role):
--
--   capability-researcher-persona  (agent: core-researcher, tone: curious,   sort 100)
--   capability-analyst-persona     (agent: core-analyst,    tone: precise,   sort 101)
--   capability-planner-persona     (agent: core-planner,    tone: pragmatic, sort 102)
--
-- Design notes:
--   - PRIMARY KEY (slug VARCHAR(120)) — human-readable key; no UUID needed.
--   - agent_slug links persona to the owning capability agent (core-researcher,
--     core-analyst, core-planner).
--   - traits TEXT[] — array of 4 personality trait labels per persona; pgx v5
--     scans this natively into []string.
--   - communication_style — prose description of how the agent delivers output.
--   - expertise_domain — primary functional domain of the agent.
--   - is_recommended=TRUE for all 3 rows — all are recommended defaults.
--   - ON CONFLICT (slug) DO NOTHING — idempotent seeds.
--   - sort_order starts at 100 to leave space for future inserts before persona 1.
--
-- The capability_agent_persona table is created here in ah_core
-- (schema created by migration 000001).

CREATE TABLE IF NOT EXISTS ah_core.capability_agent_persona (
    slug                VARCHAR(120) PRIMARY KEY,
    agent_slug          VARCHAR(120) NOT NULL,
    tone                VARCHAR(80)  NOT NULL,
    traits              TEXT[]       NOT NULL DEFAULT '{}',
    communication_style TEXT         NOT NULL,
    expertise_domain    TEXT         NOT NULL,
    is_recommended      BOOLEAN      NOT NULL DEFAULT FALSE,
    sort_order          INTEGER      NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_cap_agent_slug
    ON ah_core.capability_agent_persona (agent_slug);

CREATE INDEX IF NOT EXISTS idx_ah_core_cap_tone
    ON ah_core.capability_agent_persona (tone);

CREATE INDEX IF NOT EXISTS idx_ah_core_cap_sort
    ON ah_core.capability_agent_persona (sort_order);

-- ============================
-- 3 capability agent persona rows
-- ============================
INSERT INTO ah_core.capability_agent_persona
    (slug, agent_slug, tone, traits, communication_style, expertise_domain, is_recommended, sort_order)
VALUES
    (
        'capability-researcher-persona',
        'core-researcher',
        'curious',
        ARRAY['methodical', 'thorough', 'source-aware', 'skeptical'],
        'concise with citations',
        'web research and information synthesis',
        TRUE,
        100
    ),
    (
        'capability-analyst-persona',
        'core-analyst',
        'precise',
        ARRAY['analytical', 'evidence-driven', 'systematic', 'objective'],
        'structured with supporting data',
        'document analysis and pattern extraction',
        TRUE,
        101
    ),
    (
        'capability-planner-persona',
        'core-planner',
        'pragmatic',
        ARRAY['structured', 'action-oriented', 'dependency-aware', 'iterative'],
        'bullet-point with clear actions',
        'task decomposition and project planning',
        TRUE,
        102
    )

ON CONFLICT (slug) DO NOTHING;
