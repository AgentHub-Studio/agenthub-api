-- Seed capability contextual rule rows in ah_core
-- (migration 000114). These 9 rows encode per-agent contextual behavior rules
-- that define instructions the capability agents follow depending on context.
-- Three rules per agent (researcher, analyst, planner), adapted from Claude
-- Code's CLAUDE.md rule system:
--
-- Researcher (3 rules):
--   ctx-researcher-cite-sources    — always     → always cite sources when making factual claims
--   ctx-researcher-prefer-search   — on_uncertainty → prefer web search over assumptions when data is uncertain
--   ctx-researcher-summarize       — always     → summarize findings before presenting raw results
--
-- Analyst (3 rules):
--   ctx-analyst-break-steps        — on_task_start → break complex analysis into labeled steps before presenting conclusions
--   ctx-analyst-show-uncertainty   — on_ambiguity  → show uncertainty ranges when data is ambiguous
--   ctx-analyst-validate-assumptions — always    → always validate assumptions before analyzing
--
-- Planner (3 rules):
--   ctx-planner-ask-clarification  — on_plan_request → ask for clarification before generating implementation plans
--   ctx-planner-break-tasks        — on_task_start   → break tasks into discrete steps before starting work
--   ctx-planner-present-tradeoffs  — on_multiple_approaches → present trade-offs when multiple approaches exist
--
-- Design notes:
--   - PRIMARY KEY (id BIGSERIAL) — auto-generated numeric PK; rule identity is
--     (agent_slug, rule_text) which is enforced by the UNIQUE constraint.
--   - agent_slug TEXT NOT NULL: the capability agent this rule applies to.
--   - rule_text TEXT NOT NULL: the full instruction text in natural language.
--   - trigger_context TEXT NOT NULL DEFAULT 'always': when the rule is active.
--     Known values: 'always', 'on_factual_claim', 'on_uncertainty',
--     'on_task_start', 'on_ambiguity', 'on_plan_request', 'on_code_generation',
--     'on_multiple_approaches'.
--   - priority INTEGER NOT NULL DEFAULT 0: higher values = evaluated first.
--     Priority 10 = high (core safety rule), 5 = medium, 0 = low (default).
--   - is_active BOOLEAN NOT NULL DEFAULT TRUE: allows disabling without deleting.
--   - ON CONFLICT (agent_slug, rule_text) DO NOTHING — idempotent seeds.
--   - Table is created in ah_core (schema created by migration 000001).
--   - Researcher's cite-sources rule has priority 10 — the most critical rule for
--     research integrity (false citations are the main risk for the researcher agent).
--   - Analyst's validate-assumptions rule has priority 10 — invalid assumptions
--     cascade into downstream analysis errors that are hard to detect post-hoc.
--   - Planner's ask-clarification rule has priority 10 — plans built on ambiguous
--     goals must be redone entirely, making it the highest-cost planning mistake.

CREATE TABLE IF NOT EXISTS ah_core.capability_contextual_rule (
    id              BIGSERIAL    NOT NULL,
    agent_slug      TEXT         NOT NULL,
    rule_text       TEXT         NOT NULL,
    trigger_context TEXT         NOT NULL DEFAULT 'always',
    priority        INTEGER      NOT NULL DEFAULT 0,
    is_active       BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT pk_capability_contextual_rule PRIMARY KEY (id),
    CONSTRAINT uq_capability_contextual_rule_agent_rule UNIQUE (agent_slug, rule_text)
);

CREATE INDEX IF NOT EXISTS idx_ah_core_contextual_rule_agent_slug
    ON ah_core.capability_contextual_rule (agent_slug);

CREATE INDEX IF NOT EXISTS idx_ah_core_contextual_rule_active
    ON ah_core.capability_contextual_rule (agent_slug, is_active, priority DESC);

-- ==============================
-- 9 capability contextual rule rows
-- ==============================
INSERT INTO ah_core.capability_contextual_rule
    (agent_slug, rule_text, trigger_context, priority)
VALUES
    -- Researcher contextual rules
    ('core-researcher',
     'always cite sources when making factual claims',
     'always', 10),
    ('core-researcher',
     'prefer web search over assumptions when data is uncertain',
     'on_uncertainty', 5),
    ('core-researcher',
     'summarize findings before presenting raw results',
     'always', 0),

    -- Analyst contextual rules
    ('core-analyst',
     'break complex analysis into labeled steps before presenting conclusions',
     'on_task_start', 5),
    ('core-analyst',
     'show uncertainty ranges when data is ambiguous',
     'on_ambiguity', 5),
    ('core-analyst',
     'always validate assumptions before analyzing',
     'always', 10),

    -- Planner contextual rules
    ('core-planner',
     'ask for clarification before generating implementation plans',
     'on_plan_request', 10),
    ('core-planner',
     'break tasks into discrete steps before starting work',
     'on_task_start', 5),
    ('core-planner',
     'present trade-offs when multiple approaches exist',
     'on_multiple_approaches', 5)

ON CONFLICT (agent_slug, rule_text) DO NOTHING;
