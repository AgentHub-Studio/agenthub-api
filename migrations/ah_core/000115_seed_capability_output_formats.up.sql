-- Seed capability output format rows in ah_core
-- (migration 000115). These 9 rows encode per-agent preferred output format
-- settings that define how each capability agent structures and styles its
-- responses — adapted from agent output style templates already seeded in
-- migration 000095.
--
-- Researcher (3 rows):
--   default_format     → markdown    — markdown headers and lists for readability
--   citation_style     → inline      — [Source: URL] notation inline with claims
--   summary_position   → top         — executive summary at top before details
--
-- Analyst (3 rows):
--   default_format       → structured — labeled headings for each response section
--   number_format        → grouped    — thousands separator (1,234,567) for numbers
--   uncertainty_notation → range      — ranges (80-90%) not point estimates
--
-- Planner (3 rows):
--   default_format  → checklist  — markdown [ ] checkboxes for task lists
--   step_numbering  → sequential — 1. 2. 3. step numbering for clarity
--   code_blocks     → always     — fenced code blocks with language tag for all code
--
-- Design notes:
--   - PRIMARY KEY (id BIGSERIAL) — auto-generated numeric PK; format identity is
--     (agent_slug, format_key) which is enforced by the UNIQUE constraint.
--   - agent_slug TEXT NOT NULL: the capability agent this format setting applies to.
--   - format_key TEXT NOT NULL: the name of the format dimension
--     (e.g. "default_format", "citation_style", "number_format").
--   - format_value TEXT NOT NULL: the chosen value for that dimension
--     (e.g. "markdown", "inline", "grouped").
--   - description TEXT NOT NULL DEFAULT '': human-readable explanation of what
--     this format setting means and why it was chosen for this agent.
--   - display_order INTEGER NOT NULL DEFAULT 0: controls presentation order within
--     an agent's format settings. Values 1/2/3 per agent (sequential).
--   - ON CONFLICT (agent_slug, format_key) DO NOTHING — idempotent seeds.
--   - Table is created in ah_core (schema created by migration 000001).
--   - Researcher default_format=markdown is display_order 1 — the primary format
--     choice that governs all output unless overridden by a more specific key.
--   - Analyst default_format=structured is display_order 1 — drives the labeled
--     heading structure that distinguishes analyst output from prose responses.
--   - Planner default_format=checklist is display_order 1 — the primary format
--     that ensures plans are immediately actionable as task lists.

CREATE TABLE IF NOT EXISTS ah_core.capability_output_format (
    id            BIGSERIAL    NOT NULL,
    agent_slug    TEXT         NOT NULL,
    format_key    TEXT         NOT NULL,
    format_value  TEXT         NOT NULL,
    description   TEXT         NOT NULL DEFAULT '',
    display_order INTEGER      NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT pk_capability_output_format PRIMARY KEY (id),
    CONSTRAINT uq_capability_output_format_agent_key UNIQUE (agent_slug, format_key)
);

CREATE INDEX IF NOT EXISTS idx_ah_core_output_format_agent_slug
    ON ah_core.capability_output_format (agent_slug);

CREATE INDEX IF NOT EXISTS idx_ah_core_output_format_agent_order
    ON ah_core.capability_output_format (agent_slug, display_order);

-- ==============================
-- 9 capability output format rows
-- ==============================
INSERT INTO ah_core.capability_output_format
    (agent_slug, format_key, format_value, description, display_order)
VALUES
    -- Researcher output format settings
    ('core-researcher',
     'default_format', 'markdown',
     'Default response format uses markdown for headers and lists',
     1),
    ('core-researcher',
     'citation_style', 'inline',
     'Cite sources inline with [Source: URL] notation',
     2),
    ('core-researcher',
     'summary_position', 'top',
     'Executive summary appears at top before details',
     3),

    -- Analyst output format settings
    ('core-analyst',
     'default_format', 'structured',
     'Responses use structured sections with labeled headings',
     1),
    ('core-analyst',
     'number_format', 'grouped',
     'Large numbers use thousands separator (1,234,567)',
     2),
    ('core-analyst',
     'uncertainty_notation', 'range',
     'Uncertainty shown as ranges (e.g. 80-90%) not point estimates',
     3),

    -- Planner output format settings
    ('core-planner',
     'default_format', 'checklist',
     'Tasks presented as markdown checklists with [ ] boxes',
     1),
    ('core-planner',
     'step_numbering', 'sequential',
     'Steps numbered sequentially (1. 2. 3.) for clarity',
     2),
    ('core-planner',
     'code_blocks', 'always',
     'All code always wrapped in fenced code blocks with language tag',
     3)

ON CONFLICT (agent_slug, format_key) DO NOTHING;
