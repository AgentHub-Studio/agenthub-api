SET search_path TO ah_core, public;

-- Seed capability_suggested_followup rows in ah_core (migration 000125).
-- These 9 rows define suggested follow-up prompts for the three core capability
-- agents. The frontend can show these prompts after an agent response to help
-- users discover useful next actions without custom tenant configuration.
--
-- Design notes:
--   - PRIMARY KEY (id BIGSERIAL) keeps row identity simple.
--   - agent_slug TEXT NOT NULL names the capability agent that owns the prompt.
--   - followup_slug TEXT NOT NULL is the stable per-agent prompt key.
--   - prompt_text TEXT NOT NULL is the user-facing suggested prompt.
--   - display_order SMALLINT NOT NULL is 1..3 for each agent.
--   - UNIQUE (agent_slug, followup_slug) enforces per-agent seed identity.
--   - UNIQUE (agent_slug, display_order) prevents duplicate card positions.
--   - ON CONFLICT (agent_slug, followup_slug) DO NOTHING keeps the seed idempotent.

CREATE TABLE IF NOT EXISTS capability_suggested_followup (
    id            BIGSERIAL   NOT NULL,
    agent_slug    TEXT        NOT NULL,
    followup_slug TEXT        NOT NULL,
    prompt_text   TEXT        NOT NULL,
    display_order SMALLINT    NOT NULL,
    description   TEXT        NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT pk_capability_suggested_followup PRIMARY KEY (id),
    CONSTRAINT uq_capability_suggested_followup_agent_slug UNIQUE (agent_slug, followup_slug),
    CONSTRAINT uq_capability_suggested_followup_agent_order UNIQUE (agent_slug, display_order),
    CONSTRAINT chk_capability_suggested_followup_display_order CHECK (display_order BETWEEN 1 AND 3)
);

CREATE INDEX IF NOT EXISTS idx_capability_suggested_followup_agent_slug
    ON capability_suggested_followup (agent_slug);

-- =====================================
-- 9 capability suggested follow-up rows
-- =====================================
INSERT INTO capability_suggested_followup
    (agent_slug, followup_slug, prompt_text, display_order, description)
VALUES
    -- core-researcher
    ('core-researcher',
     'research-current-sources',
     'Find the most reliable current sources about this topic and summarize the key findings.',
     1,
     'Research prompt for source discovery and synthesis'),
    ('core-researcher',
     'compare-official-and-recent',
     'Compare the official documentation with recent analysis and highlight what changed.',
     2,
     'Research prompt for comparing canonical docs with newer material'),
    ('core-researcher',
     'fact-check-claim',
     'Fact-check this claim, list supporting and conflicting evidence, and include citations.',
     3,
     'Research prompt for evidence-backed claim verification'),

    -- core-analyst
    ('core-analyst',
     'extract-key-patterns',
     'Extract the key patterns from this data or document and explain what they imply.',
     1,
     'Analysis prompt for pattern extraction'),
    ('core-analyst',
     'compare-options',
     'Compare these options with trade-offs, confidence levels, and a recommended choice.',
     2,
     'Analysis prompt for structured option comparison'),
    ('core-analyst',
     'turn-findings-into-actions',
     'Turn these findings into prioritized actions with risks, assumptions, and next steps.',
     3,
     'Analysis prompt for converting findings into action'),

    -- core-planner
    ('core-planner',
     'break-into-milestones',
     'Break this goal into sequenced milestones with dependencies and acceptance criteria.',
     1,
     'Planning prompt for milestone decomposition'),
    ('core-planner',
     'identify-risks-and-owners',
     'Identify the main risks, dependencies, owners, and decisions needed before execution.',
     2,
     'Planning prompt for risk and ownership mapping'),
    ('core-planner',
     'weekly-checklist',
     'Convert this plan into a checklist for this week with clear order and expected outcomes.',
     3,
     'Planning prompt for short-horizon execution')

ON CONFLICT (agent_slug, followup_slug) DO NOTHING;
