-- Seed capability onboarding checklist rows in ah_core
-- (migration 000106). These 5 rows define a guided onboarding path for new
-- tenants, walking them through the minimum steps required to run their first
-- successful agent chat session:
--
--   onboarding-connect-llm    — step 1 (blocking): configure an LLM API key
--   onboarding-create-agent   — step 2 (blocking): create a capability agent
--   onboarding-assign-skill   — step 3 (blocking): bind a capability skill
--   onboarding-configure-kb   — step 4 (optional): create a knowledge base
--   onboarding-test-run       — step 5 (optional): run first chat session
--
-- Design notes:
--   - PRIMARY KEY (slug VARCHAR(120)) — human-readable key; no UUID needed.
--   - UNIQUE (step_order) — one row per ordered step; prevents misordering.
--   - is_blocking=TRUE for steps 1-3: these are mandatory for a working agent.
--   - is_blocking=FALSE for steps 4-5: KB and test-run are optional extras.
--   - is_automated=FALSE for all: none of these steps self-complete.
--   - ON CONFLICT (slug) DO NOTHING — idempotent seeds.
--   - Table is created here in ah_core (schema created by migration 000001).

CREATE TABLE IF NOT EXISTS ah_core.capability_onboarding_checklist (
    slug          VARCHAR(120) PRIMARY KEY,
    title         VARCHAR(255) NOT NULL,
    description   TEXT         NOT NULL,
    resource_type VARCHAR(120) NOT NULL,
    step_order    INTEGER      NOT NULL,
    is_blocking   BOOLEAN      NOT NULL DEFAULT FALSE,
    is_automated  BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ah_core_onboarding_step_order
    ON ah_core.capability_onboarding_checklist (step_order);

-- ======================================
-- 5 capability onboarding checklist rows
-- ======================================
INSERT INTO ah_core.capability_onboarding_checklist
    (slug, title, description, resource_type, step_order, is_blocking, is_automated)
VALUES
    (
        'onboarding-connect-llm',
        'Connect LLM Provider',
        'Configure an LLM API key (OpenAI, Anthropic, OpenRouter) in Settings so your agents can call language models.',
        'llm_provider_settings',
        1,
        TRUE,
        FALSE
    ),
    (
        'onboarding-create-agent',
        'Create Your First Agent',
        'Create an agent using one of the capability agent templates (Researcher, Analyst, or Planner).',
        'agent',
        2,
        TRUE,
        FALSE
    ),
    (
        'onboarding-assign-skill',
        'Assign a Capability Skill',
        'Bind a capability skill (web-research, doc-analysis, or task-workflow) to the agent so it can take actions.',
        'agent_skill',
        3,
        TRUE,
        FALSE
    ),
    (
        'onboarding-configure-kb',
        'Configure Knowledge Base',
        '(Optional) Create a knowledge base and upload documents for the agent to search using RAG.',
        'knowledge_base',
        4,
        FALSE,
        FALSE
    ),
    (
        'onboarding-test-run',
        'Run Your First Chat',
        'Start a chat session with your agent to verify it responds correctly end-to-end.',
        'chat_session',
        5,
        FALSE,
        FALSE
    )

ON CONFLICT (slug) DO NOTHING;
