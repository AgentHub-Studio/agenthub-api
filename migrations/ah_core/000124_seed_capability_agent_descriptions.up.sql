-- Seed capability_agent_description rows in ah_core (migration 000124).
-- These 9 rows encode the per-agent user-facing descriptions for the three core
-- capability agents: the short tagline shown in the agent picker card, the full
-- description shown on the agent detail page, and the pipe-separated list of
-- example use cases shown as badges in the frontend UI.
--
-- core-researcher (3 rows — web search and synthesis agent):
--   tagline          → "Search the web and synthesize findings instantly"
--   long_description → full description of web search and fact-checking capability
--   use_cases        → pipe-separated list: fact-checking|research|news|events|docs
--
-- core-analyst (3 rows — data and document analysis agent):
--   tagline          → "Analyze data and documents with structured reasoning"
--   long_description → full description of analysis and insight extraction capability
--   use_cases        → pipe-separated list: reports|options|surveys|patterns|trade-offs
--
-- core-planner (3 rows — goal decomposition and planning agent):
--   tagline          → "Break down complex goals into actionable step-by-step plans"
--   long_description → full description of planning and task decomposition capability
--   use_cases        → pipe-separated list: projects|checklists|tasks|sprint|roadmaps
--
-- Design notes:
--   - PRIMARY KEY (id BIGSERIAL) — auto-generated numeric PK; description identity
--     is (agent_slug, desc_key) enforced by UNIQUE constraint.
--   - agent_slug TEXT NOT NULL: the capability agent that owns this description row
--     (e.g. "core-researcher", "core-analyst", "core-planner").
--   - desc_key TEXT NOT NULL: machine-readable key for the description type
--     (e.g. "tagline", "long_description", "use_cases").
--   - desc_value TEXT NOT NULL: the actual description content for this key.
--     For "use_cases" rows, values are pipe-separated (e.g. "A|B|C").
--   - description TEXT NOT NULL DEFAULT '': human-readable note explaining the
--     purpose of this desc_key for frontend / documentation consumers.
--   - ON CONFLICT (agent_slug, desc_key) DO NOTHING — idempotent seeds.
--   - Table is created in ah_core (schema created by migration 000001).

CREATE TABLE IF NOT EXISTS ah_core.capability_agent_description (
    id          BIGSERIAL   NOT NULL,
    agent_slug  TEXT        NOT NULL,
    desc_key    TEXT        NOT NULL,
    desc_value  TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT pk_capability_agent_description PRIMARY KEY (id),
    CONSTRAINT uq_capability_agent_description_agent_key UNIQUE (agent_slug, desc_key)
);

CREATE INDEX IF NOT EXISTS idx_ah_core_capability_agent_description_agent_slug
    ON ah_core.capability_agent_description (agent_slug);

-- =====================================
-- 9 capability agent description rows
-- =====================================
INSERT INTO ah_core.capability_agent_description
    (agent_slug, desc_key, desc_value, description)
VALUES
    -- core-researcher (tagline, long_description, use_cases)
    ('core-researcher',
     'tagline',
     'Search the web and synthesize findings instantly',
     'Short one-line description for the agent picker card'),

    ('core-researcher',
     'long_description',
     'The Researcher agent searches the web, fetches pages, and scans your knowledge base to gather accurate, up-to-date information. Ideal for fact-checking, competitive research, news summaries, and answering questions that require current data.',
     'Full description shown on agent detail page'),

    ('core-researcher',
     'use_cases',
     'Fact-checking claims|Researching competitors|Summarizing recent news|Answering questions about current events|Finding technical documentation',
     'Pipe-separated list of example use cases shown as badges'),

    -- core-analyst (tagline, long_description, use_cases)
    ('core-analyst',
     'tagline',
     'Analyze data and documents with structured reasoning',
     'Short one-line description for the agent picker card'),

    ('core-analyst',
     'long_description',
     'The Analyst agent examines data, documents, and information to extract patterns, insights, and conclusions. Uses step-by-step reasoning to present findings with appropriate confidence levels. Ideal for interpreting reports, comparing options, and turning raw data into actionable insights.',
     'Full description shown on agent detail page'),

    ('core-analyst',
     'use_cases',
     'Analyzing reports and documents|Comparing product options|Interpreting survey results|Identifying data patterns|Evaluating trade-offs',
     'Pipe-separated list of example use cases shown as badges'),

    -- core-planner (tagline, long_description, use_cases)
    ('core-planner',
     'tagline',
     'Break down complex goals into actionable step-by-step plans',
     'Short one-line description for the agent picker card'),

    ('core-planner',
     'long_description',
     'The Planner agent takes your goal and creates a structured, actionable plan. It breaks work into clear steps, identifies dependencies, and can delegate research or analysis subtasks to specialized agents. Ideal for project planning, onboarding workflows, and tackling multi-step objectives.',
     'Full description shown on agent detail page'),

    ('core-planner',
     'use_cases',
     'Project planning|Onboarding checklists|Multi-step task breakdown|Sprint planning|Creating implementation roadmaps',
     'Pipe-separated list of example use cases shown as badges')

ON CONFLICT (agent_slug, desc_key) DO NOTHING;
