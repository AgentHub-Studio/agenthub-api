-- Seed capability rate limit rows in ah_core
-- (migration 000105). These 9 rows define per-agent rate limits for each
-- capability agent (researcher, analyst, planner) to prevent runaway LLM
-- costs for new tenants. Three limit types per agent:
--
--   requests_per_minute — max LLM calls in a 60 s window
--   tokens_per_minute   — max tokens consumed in a 60 s window
--   max_concurrent_runs — max simultaneous agent runs (no time window)
--
-- Rows:
--   capability-researcher-req-per-min     (core-researcher, requests_per_minute,  10, window 60s)
--   capability-researcher-tokens-per-min  (core-researcher, tokens_per_minute,  50000, window 60s)
--   capability-researcher-max-concurrent  (core-researcher, max_concurrent_runs,     2, window 0s)
--   capability-analyst-req-per-min        (core-analyst,    requests_per_minute,   8, window 60s)
--   capability-analyst-tokens-per-min     (core-analyst,    tokens_per_minute,  80000, window 60s)
--   capability-analyst-max-concurrent     (core-analyst,    max_concurrent_runs,    2, window 0s)
--   capability-planner-req-per-min        (core-planner,    requests_per_minute,   5, window 60s)
--   capability-planner-tokens-per-min     (core-planner,    tokens_per_minute,  20000, window 60s)
--   capability-planner-max-concurrent     (core-planner,    max_concurrent_runs,    3, window 0s)
--
-- Design notes:
--   - PRIMARY KEY (slug VARCHAR(120)) — human-readable key; no UUID needed.
--   - UNIQUE (agent_slug, limit_key) — one row per agent+limit combination.
--   - window_seconds=0 for max_concurrent_runs: concurrency is not time-windowed.
--   - Analyst has highest tokens_per_minute (80 000): it processes large documents.
--   - Planner has highest max_concurrent_runs (3): lightweight planning tasks.
--   - Researcher has highest requests_per_minute (10): active web research loops.
--   - ON CONFLICT (slug) DO NOTHING — idempotent seeds.
--   - Table is created here in ah_core (schema created by migration 000001).

CREATE TABLE IF NOT EXISTS ah_core.capability_rate_limit (
    slug            VARCHAR(120) PRIMARY KEY,
    agent_slug      VARCHAR(120) NOT NULL,
    limit_key       VARCHAR(120) NOT NULL,
    limit_value     INTEGER      NOT NULL,
    window_seconds  INTEGER      NOT NULL DEFAULT 60,
    description     TEXT,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (agent_slug, limit_key)
);

CREATE INDEX IF NOT EXISTS idx_ah_core_rate_limit_agent_slug
    ON ah_core.capability_rate_limit (agent_slug);

CREATE INDEX IF NOT EXISTS idx_ah_core_rate_limit_key
    ON ah_core.capability_rate_limit (limit_key);

-- ============================
-- 9 capability rate limit rows
-- ============================
INSERT INTO ah_core.capability_rate_limit
    (slug, agent_slug, limit_key, limit_value, window_seconds, description)
VALUES
    (
        'capability-researcher-req-per-min',
        'core-researcher',
        'requests_per_minute',
        10,
        60,
        'Max LLM requests per minute for the researcher agent — active web research loops require higher call rates'
    ),
    (
        'capability-researcher-tokens-per-min',
        'core-researcher',
        'tokens_per_minute',
        50000,
        60,
        'Max tokens consumed per minute for the researcher agent — web search results are typically concise'
    ),
    (
        'capability-researcher-max-concurrent',
        'core-researcher',
        'max_concurrent_runs',
        2,
        0,
        'Max simultaneous researcher runs — two parallel research tasks is sufficient without overloading search quota'
    ),
    (
        'capability-analyst-req-per-min',
        'core-analyst',
        'requests_per_minute',
        8,
        60,
        'Max LLM requests per minute for the analyst agent — document analysis is deliberate and iterative'
    ),
    (
        'capability-analyst-tokens-per-min',
        'core-analyst',
        'tokens_per_minute',
        80000,
        60,
        'Max tokens consumed per minute for the analyst agent — processes large documents, highest token budget'
    ),
    (
        'capability-analyst-max-concurrent',
        'core-analyst',
        'max_concurrent_runs',
        2,
        0,
        'Max simultaneous analyst runs — heavy document processing limits safe concurrency to two'
    ),
    (
        'capability-planner-req-per-min',
        'core-planner',
        'requests_per_minute',
        5,
        60,
        'Max LLM requests per minute for the planner agent — planning is structured and uses fewer LLM calls'
    ),
    (
        'capability-planner-tokens-per-min',
        'core-planner',
        'tokens_per_minute',
        20000,
        60,
        'Max tokens consumed per minute for the planner agent — task decomposition uses smaller context windows'
    ),
    (
        'capability-planner-max-concurrent',
        'core-planner',
        'max_concurrent_runs',
        3,
        0,
        'Max simultaneous planner runs — lightweight tasks allow the highest concurrency among the three agents'
    )

ON CONFLICT (slug) DO NOTHING;
