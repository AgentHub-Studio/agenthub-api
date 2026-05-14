-- Seed capability example prompt rows in ah_core
-- (migration 000108). These 12 rows define example user prompts shown to new
-- users when starting a conversation with capability agents in the AgentHub web
-- UI. Example prompts help users understand what each agent can do and how to
-- phrase effective requests. Adapted from Claude Code's example queries in
-- README/docs patterns:
--
-- Researcher (4 examples):
--   example-researcher-web-research       — web_research
--   example-researcher-competitor         — competitive_analysis
--   example-researcher-technical          — technical_research
--   example-researcher-market             — market_research
--
-- Analyst (4 examples):
--   example-analyst-doc-summary           — document_analysis
--   example-analyst-compare               — comparison
--   example-analyst-extract               — extraction
--   example-analyst-pattern               — pattern_analysis
--
-- Planner (4 examples):
--   example-planner-project               — project_planning
--   example-planner-sprint                — task_breakdown
--   example-planner-release               — release_planning
--   example-planner-debug                 — debugging
--
-- Design notes:
--   - PRIMARY KEY (slug VARCHAR(120)) — human-readable key; no UUID needed.
--   - agent_slug VARCHAR(120): the capability agent this example belongs to.
--   - category VARCHAR(80): functional category for grouping/filtering.
--   - sort_order INTEGER: display order within each agent's prompt list.
--   - ON CONFLICT (slug) DO NOTHING — idempotent seeds.
--   - Table is created here in ah_core (schema created by migration 000001).

CREATE TABLE IF NOT EXISTS ah_core.capability_example_prompt (
    slug         VARCHAR(120) PRIMARY KEY,
    agent_slug   VARCHAR(120) NOT NULL,
    prompt_text  TEXT         NOT NULL,
    category     VARCHAR(80)  NOT NULL,
    sort_order   INTEGER      NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_example_prompt_agent_slug
    ON ah_core.capability_example_prompt (agent_slug);

-- ==============================
-- 12 capability example prompt rows
-- ==============================
INSERT INTO ah_core.capability_example_prompt
    (slug, agent_slug, prompt_text, category, sort_order)
VALUES
    -- Researcher examples
    (
        'example-researcher-web-research',
        'core-researcher',
        'Research the latest advances in vector database performance optimization and summarize the key findings',
        'web_research',
        1
    ),
    (
        'example-researcher-competitor',
        'core-researcher',
        'Compare the top 3 PostgreSQL vector search extensions: pgvector, pgvectorscale, and pg_embedding — features, performance, and limitations',
        'competitive_analysis',
        2
    ),
    (
        'example-researcher-technical',
        'core-researcher',
        'Find recent benchmarks comparing transformer-based embedding models for code similarity search',
        'technical_research',
        3
    ),
    (
        'example-researcher-market',
        'core-researcher',
        'What are the current market leaders in enterprise AI agent platforms and what differentiates them?',
        'market_research',
        4
    ),

    -- Analyst examples
    (
        'example-analyst-doc-summary',
        'core-analyst',
        'Analyze the uploaded architecture document and extract the key design decisions and their rationale',
        'document_analysis',
        5
    ),
    (
        'example-analyst-compare',
        'core-analyst',
        'Compare the two API specification documents and identify breaking changes between versions',
        'comparison',
        6
    ),
    (
        'example-analyst-extract',
        'core-analyst',
        'Extract all performance metrics and SLA requirements from the uploaded contract document',
        'extraction',
        7
    ),
    (
        'example-analyst-pattern',
        'core-analyst',
        'Identify recurring patterns and anti-patterns in the uploaded code review logs',
        'pattern_analysis',
        8
    ),

    -- Planner examples
    (
        'example-planner-project',
        'core-planner',
        'Create a detailed migration plan for moving our monolithic Rails app to microservices, including dependencies and timeline',
        'project_planning',
        9
    ),
    (
        'example-planner-sprint',
        'core-planner',
        'Break down the user story ''As a user I want to export my data as CSV'' into development tasks with estimates',
        'task_breakdown',
        10
    ),
    (
        'example-planner-release',
        'core-planner',
        'Plan the release checklist for deploying our new authentication service to production',
        'release_planning',
        11
    ),
    (
        'example-planner-debug',
        'core-planner',
        'I need to investigate and fix the performance regression in our search API. Help me plan the debugging approach',
        'debugging',
        12
    )

ON CONFLICT (slug) DO NOTHING;
