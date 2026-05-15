-- 000075: seed ah_core.skill_effort_level_template
-- Maps Claude Code's effortLevel frontmatter field to Anthropic thinking-budget
-- tiers. The runner uses these to set extended_thinking budgets before LLM calls.
-- Adapted for AgentHub web: no CLI-specific tiers; focused on Anthropic API limits.
CREATE TABLE IF NOT EXISTS ah_core.skill_effort_level_template (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug            TEXT NOT NULL UNIQUE,
    label           TEXT NOT NULL,
    description     TEXT NOT NULL,
    thinking_tokens INTEGER NOT NULL DEFAULT 0,  -- 0 = disabled, >0 = budget cap
    recommended_for JSONB NOT NULL DEFAULT '[]',
    sort_order      INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT skill_effort_level_template_slug_format CHECK (slug ~ '^[a-z0-9][a-z0-9-]*[a-z0-9]$')
);

INSERT INTO ah_core.skill_effort_level_template (slug, label, description, thinking_tokens, recommended_for, sort_order)
VALUES
    ('lowest',  'Lowest',  'No extended thinking. Fastest and cheapest. Use for simple, unambiguous tasks.',                                     0,     '["classification","routing","summarisation","slot-filling"]',              10),
    ('low',     'Low',     '1 024-token thinking budget. Minimal reasoning overhead. Suitable for light analysis.',                              1024,  '["light-analysis","data-extraction","qa-over-context","formatting"]',      20),
    ('medium',  'Medium',  '8 192-token thinking budget. Balanced reasoning. Default for most interactive skills.',                              8192,  '["research","multi-step-reasoning","code-review","planning"]',             30),
    ('high',    'High',    '32 768-token thinking budget. Deep reasoning. Use for complex multi-source synthesis.',                              32768, '["deep-research","compliance-audit","architecture-review","debugging"]',   40),
    ('highest', 'Highest', '65 536-token thinking budget. Maximum reasoning depth. Reserve for the hardest problems.', 65536, '["formal-verification","multi-document-synthesis","critical-decisions"]',   50)
ON CONFLICT (slug) DO NOTHING;
