-- Seed capability response template rows in ah_core
-- (migration 000116). These 9 rows encode per-agent structured response
-- templates that agents use to ensure consistent, predictable responses for
-- common interaction patterns — greetings, error states, and handoffs.
--
-- Researcher (3 rows):
--   greeting          → opening message shown to user at session start
--   not_found         → response when search yields no useful results
--   source_disclaimer → footer appended when citing external sources
--
-- Analyst (3 rows):
--   greeting          → opening message shown to user at session start
--   ambiguity_prompt  → used when input is ambiguous before starting analysis
--   confidence_footer → appended to analysis results with confidence metadata
--
-- Planner (3 rows):
--   greeting              → opening message shown to user at session start
--   clarification_request → used to ask for clarification before generating a plan
--   handoff               → closing message after presenting a completed plan
--
-- Design notes:
--   - PRIMARY KEY (id BIGSERIAL) — auto-generated numeric PK; template identity
--     is (agent_slug, template_key) enforced by the UNIQUE constraint.
--   - agent_slug TEXT NOT NULL: the capability agent that owns this template
--     (e.g. "core-researcher", "core-analyst", "core-planner").
--   - template_key TEXT NOT NULL: the logical name of this template
--     (e.g. "greeting", "not_found", "source_disclaimer").
--   - template_body TEXT NOT NULL: the template text; may contain {variable}
--     placeholder tokens that callers must fill before rendering.
--   - description TEXT NOT NULL DEFAULT '': human-readable explanation of when
--     this template is used and what it communicates to the user.
--   - has_placeholders BOOLEAN NOT NULL DEFAULT FALSE: TRUE when template_body
--     contains {variable} placeholder tokens that require interpolation before
--     being shown to the user.
--   - ON CONFLICT (agent_slug, template_key) DO NOTHING — idempotent seeds.
--   - Table is created in ah_core (schema created by migration 000001).
--   - Four templates have placeholders: ambiguity_prompt, confidence_footer,
--     clarification_request, handoff — callers must interpolate before rendering.

CREATE TABLE IF NOT EXISTS ah_core.capability_response_template (
    id               BIGSERIAL    NOT NULL,
    agent_slug       TEXT         NOT NULL,
    template_key     TEXT         NOT NULL,
    template_body    TEXT         NOT NULL,
    description      TEXT         NOT NULL DEFAULT '',
    has_placeholders BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT pk_capability_response_template PRIMARY KEY (id),
    CONSTRAINT uq_capability_response_template_agent_key UNIQUE (agent_slug, template_key)
);

CREATE INDEX IF NOT EXISTS idx_ah_core_response_template_agent_slug
    ON ah_core.capability_response_template (agent_slug);

CREATE INDEX IF NOT EXISTS idx_ah_core_response_template_agent_key
    ON ah_core.capability_response_template (agent_slug, template_key);

-- ==============================
-- 9 capability response template rows
-- ==============================
INSERT INTO ah_core.capability_response_template
    (agent_slug, template_key, template_body, description, has_placeholders)
VALUES
    -- Researcher response templates
    ('core-researcher',
     'greeting',
     'I''m your research assistant. I can search the web, fetch documents, and synthesize findings. What would you like me to research?',
     'Opening message shown to user at session start',
     FALSE),
    ('core-researcher',
     'not_found',
     'I searched but couldn''t find reliable information on that topic. Try rephrasing or providing more context.',
     'Response when search yields no useful results',
     FALSE),
    ('core-researcher',
     'source_disclaimer',
     '**Sources:** The above findings are based on the sources cited. Verify critical information independently.',
     'Footer appended when citing external sources',
     FALSE),

    -- Analyst response templates
    ('core-analyst',
     'greeting',
     'I''m your data analyst. Share data, documents, or describe what you need analyzed. I''ll break it down systematically.',
     'Opening message shown to user at session start',
     FALSE),
    ('core-analyst',
     'ambiguity_prompt',
     'Before I analyze this, I need to clarify: {clarification_question}. Please provide more context.',
     'Used when input is ambiguous before starting analysis',
     TRUE),
    ('core-analyst',
     'confidence_footer',
     '**Confidence:** {confidence_level} — based on {data_points} data points. {caveats}',
     'Appended to analysis results with confidence metadata',
     TRUE),

    -- Planner response templates
    ('core-planner',
     'greeting',
     'I''m your planning assistant. Describe your goal or project and I''ll help break it into actionable steps.',
     'Opening message shown to user at session start',
     FALSE),
    ('core-planner',
     'clarification_request',
     'Before I create a plan, I need to understand: {clarification_question}',
     'Used to ask for clarification before generating a plan',
     TRUE),
    ('core-planner',
     'handoff',
     '**Next Steps:** The plan above is ready. Assign tasks to your team or proceed with Step 1: {first_step}.',
     'Closing message after presenting a completed plan',
     TRUE)

ON CONFLICT (agent_slug, template_key) DO NOTHING;
