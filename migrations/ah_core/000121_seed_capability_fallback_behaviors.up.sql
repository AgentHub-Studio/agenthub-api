-- Seed capability_fallback_behavior rows in ah_core (migration 000121).
-- These 9 rows encode the per-agent fallback behaviors for the three core
-- capability agents: what each agent does when a primary tool or capability
-- fails. Behaviors cover graceful degradation, retry strategies, and user
-- clarification prompts.
--
-- core-researcher (3 rows — search + fetch failure handling):
--   search_failure → graceful_degrade  (web search unavailable — fall back to training knowledge)
--   fetch_failure  → retry_with_cache  (page fetch error — try alternative sources, retry up to 3x)
--   no_results     → rephrase_and_retry (empty search results — rephrase query, retry up to 2x)
--
-- core-analyst (3 rows — document + LLM failure handling):
--   doc_unavailable  → graceful_degrade          (empty KB — prompt user to upload files)
--   analysis_timeout → retry_with_simpler_prompt  (LLM timeout — break into smaller parts, retry 2x)
--   ambiguous_data   → ask_clarification          (insufficient context — request clarification, 0 retries)
--
-- core-planner (3 rows — delegation + scope + goal failure handling):
--   subagent_failure → retry_direct         (agent delegation error — handle directly, retry 2x)
--   scope_too_large  → decompose_and_retry  (context limit exceeded — decompose into phases, retry 1x)
--   no_goal          → ask_clarification    (goal unclear — request success criteria, 0 retries)
--
-- Design notes:
--   - PRIMARY KEY (id BIGSERIAL) — auto-generated numeric PK; fallback identity
--     is (agent_slug, behavior_key) enforced by UNIQUE constraint.
--   - agent_slug TEXT NOT NULL: the capability agent that owns this behavior row
--     (e.g. "core-researcher", "core-analyst", "core-planner").
--   - behavior_key TEXT NOT NULL: machine-readable key for the failure mode
--     (e.g. "search_failure", "doc_unavailable", "subagent_failure").
--   - trigger TEXT NOT NULL: the condition that activates this fallback
--     (e.g. "web_search_tool_unavailable", "knowledge_base_empty").
--   - action TEXT NOT NULL: the recovery action, constrained to the 7 known values.
--   - fallback_message TEXT NOT NULL: human-readable message delivered to the user.
--   - retry_count INTEGER NOT NULL DEFAULT 0: number of automatic retries before
--     the action is taken (0 = immediate fallback, no retries).
--   - ON CONFLICT (agent_slug, behavior_key) DO NOTHING — idempotent seeds.
--   - Table is created in ah_core (schema created by migration 000001).

CREATE TABLE IF NOT EXISTS ah_core.capability_fallback_behavior (
    id               BIGSERIAL   NOT NULL,
    agent_slug       TEXT        NOT NULL,
    behavior_key     TEXT        NOT NULL,
    trigger          TEXT        NOT NULL,
    action           TEXT        NOT NULL,
    fallback_message TEXT        NOT NULL,
    retry_count      INTEGER     NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT pk_capability_fallback_behavior PRIMARY KEY (id),
    CONSTRAINT uq_capability_fallback_behavior_agent_key UNIQUE (agent_slug, behavior_key),
    CONSTRAINT chk_capability_fallback_behavior_action CHECK (action IN (
        'graceful_degrade',
        'retry_with_cache',
        'rephrase_and_retry',
        'retry_with_simpler_prompt',
        'ask_clarification',
        'retry_direct',
        'decompose_and_retry'
    ))
);

CREATE INDEX IF NOT EXISTS idx_ah_core_capability_fallback_behavior_agent_slug
    ON ah_core.capability_fallback_behavior (agent_slug);

-- =====================================
-- 9 capability fallback behavior rows
-- =====================================
INSERT INTO ah_core.capability_fallback_behavior
    (agent_slug, behavior_key, trigger, action, fallback_message, retry_count)
VALUES
    -- core-researcher (search and fetch failure handling)
    ('core-researcher',
     'search_failure',
     'web_search_tool_unavailable',
     'graceful_degrade',
     'Web search is temporarily unavailable. I''ll answer based on my training knowledge, which may not reflect recent events.',
     2),
    ('core-researcher',
     'fetch_failure',
     'web_fetch_tool_error',
     'retry_with_cache',
     'Unable to fetch that page. Trying alternative sources.',
     3),
    ('core-researcher',
     'no_results',
     'search_returns_empty',
     'rephrase_and_retry',
     'No results found. Let me try a different search approach.',
     2),

    -- core-analyst (document and LLM failure handling)
    ('core-analyst',
     'doc_unavailable',
     'knowledge_base_empty',
     'graceful_degrade',
     'No documents found in your knowledge base. Please upload relevant files to enable document analysis.',
     0),
    ('core-analyst',
     'analysis_timeout',
     'llm_timeout',
     'retry_with_simpler_prompt',
     'Analysis is taking too long. Breaking this into smaller parts.',
     2),
    ('core-analyst',
     'ambiguous_data',
     'insufficient_context',
     'ask_clarification',
     'The data provided is insufficient for analysis. Could you clarify: {clarification_needed}?',
     0),

    -- core-planner (delegation, scope, and goal failure handling)
    ('core-planner',
     'subagent_failure',
     'subagent_tool_error',
     'retry_direct',
     'Agent delegation failed. I''ll handle this step directly instead.',
     2),
    ('core-planner',
     'scope_too_large',
     'context_limit_exceeded',
     'decompose_and_retry',
     'This task is too large for a single plan. Breaking it into phases.',
     1),
    ('core-planner',
     'no_goal',
     'goal_unclear',
     'ask_clarification',
     'I need a clearer goal to create a plan. Could you describe what success looks like?',
     0)

ON CONFLICT (agent_slug, behavior_key) DO NOTHING;
