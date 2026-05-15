-- Seed capability system prompt templates in ah_core
-- (migration 000100). These 3 system prompts provide role-specific LLM
-- instructions for the three capability agents introduced in migration 000091:
--
--   capability-researcher-system-prompt  →  core-researcher  (sort_order 1)
--   capability-analyst-system-prompt     →  core-analyst     (sort_order 2)
--   capability-planner-system-prompt     →  core-planner     (sort_order 3)
--
-- Each system prompt is tailored to the agent's primary responsibility:
--   Researcher — web research, source verification, knowledge synthesis.
--   Analyst    — document analysis, pattern extraction, evidence-based conclusions.
--   Planner    — task decomposition, dependency mapping, progress tracking.
--
-- Design notes:
--   - PRIMARY KEY (slug) — unique identifier per template.
--   - agent_slug FK-like reference to ah_core capability agents (no hard FK —
--     capability agents may not be present in all deployments).
--   - is_recommended — marks platform-recommended prompts for the UI to surface.
--   - sort_order — controls display ordering in the prompt template catalogue.
--   - ON CONFLICT (slug) DO NOTHING — migration is safe to re-apply (idempotent).
--
-- The system_prompt_template table does not exist before this migration;
-- it is created here in ah_core (schema created by migration 000001).

CREATE TABLE IF NOT EXISTS ah_core.system_prompt_template (
    slug           VARCHAR(120) PRIMARY KEY,
    name           VARCHAR(255) NOT NULL,
    description    TEXT,
    content        TEXT         NOT NULL,
    agent_slug     VARCHAR(120),
    is_recommended BOOLEAN      NOT NULL DEFAULT FALSE,
    sort_order     INTEGER      NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_system_prompt_template_agent
    ON ah_core.system_prompt_template (agent_slug);

CREATE INDEX IF NOT EXISTS idx_ah_core_system_prompt_template_sort
    ON ah_core.system_prompt_template (sort_order);

-- ============================
-- 3 capability system prompt templates (one per capability agent)
-- ============================
INSERT INTO ah_core.system_prompt_template
    (slug, name, description, content, agent_slug, is_recommended, sort_order)
VALUES
    -- 1. Researcher system prompt
    (
        'capability-researcher-system-prompt',
        'Capability Researcher System Prompt',
        'Role-specific system prompt for the core-researcher capability agent. Focuses on web research, source verification, and knowledge synthesis.',
        'You are a research specialist agent in AgentHub. Your role is to gather, verify, and synthesize information from web sources and knowledge bases. Use web search tools to find relevant sources, fetch full page content when needed, and document findings with proper citations. Always verify information across multiple sources before presenting conclusions. Store research results in the designated knowledge base.',
        'core-researcher',
        TRUE,
        1
    ),
    -- 2. Analyst system prompt
    (
        'capability-analyst-system-prompt',
        'Capability Analyst System Prompt',
        'Role-specific system prompt for the core-analyst capability agent. Focuses on document analysis, pattern extraction, and evidence-based conclusions.',
        'You are an analysis specialist agent in AgentHub. Your role is to analyze documents, extract patterns, and deliver evidence-based insights. Use document search tools to query knowledge bases and read documents thoroughly. Structure your analysis with clear sections: findings, supporting evidence, confidence level, and recommendations. Reference specific document sources for every claim.',
        'core-analyst',
        TRUE,
        2
    ),
    -- 3. Planner system prompt
    (
        'capability-planner-system-prompt',
        'Capability Planner System Prompt',
        'Role-specific system prompt for the core-planner capability agent. Focuses on task decomposition, dependency mapping, and progress tracking.',
        'You are a planning specialist agent in AgentHub. Your role is to decompose complex tasks into actionable steps, identify dependencies, and track progress. Create and maintain structured task lists using todo tools. Break down goals into concrete, measurable subtasks. Update task status as work progresses and surface blockers promptly.',
        'core-planner',
        TRUE,
        3
    )

ON CONFLICT (slug) DO NOTHING;
