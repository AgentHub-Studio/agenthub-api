-- Seed platform-managed slash commands in ah_core.
-- These are WEB-FRIENDLY commands available to every tenant via CoreCommandLoader,
-- mirroring Claude Code's /commands surface adapted for AgentHub's web UX.
--
-- Inspired by:
--   - PDF arXiv:2604.14228v1 Section 6.1 (Plugin manifest component types: commands)
--   - Claude Code .claude/commands/ directory + built-in slash commands
--
-- CLI/IDE-specific commands (/init, /add-dir, /mcp, /ide, /sandbox, /vim, /editor,
-- /shell) are intentionally OMITTED — AgentHub is a web product.

CREATE TABLE IF NOT EXISTS ah_core.command (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    -- name is the human-friendly title shown in command palettes.
    name        VARCHAR(255) NOT NULL,
    -- slug is the slash-token (without leading /), e.g. "help", "clear".
    -- This is what the LLM and the UI dispatch on.
    slug        VARCHAR(64)  NOT NULL UNIQUE,
    description TEXT,
    -- argument_hint: short usage shown next to slug, e.g. "<topic>".
    argument_hint TEXT,
    -- category groups commands in the UI palette: session | analytics | discovery | feedback.
    category    VARCHAR(64)  NOT NULL DEFAULT 'session',
    -- handler_type controls how the runner dispatches the command:
    --   "builtin"  — runner has hardcoded behavior keyed by slug
    --   "prompt"   — command expands to a prompt template (template field)
    --   "tool"     — command invokes a specific tool by slug (tool_slug field)
    handler_type VARCHAR(32) NOT NULL DEFAULT 'builtin',
    -- prompt_template is used when handler_type = 'prompt'.
    -- Supports {{input}} placeholder for arguments.
    prompt_template TEXT,
    -- tool_slug is used when handler_type = 'tool'.
    tool_slug   VARCHAR(255),
    -- requires_admin gates platform-management commands behind admin role.
    requires_admin BOOLEAN  NOT NULL DEFAULT FALSE,
    -- disable_model_invocation: true means user-only (slash typed by human),
    -- false means LLM may also invoke as a meta-tool. Mirrors Claude Code's
    -- BundledSkillDefinition.disableModelInvocation.
    disable_model_invocation BOOLEAN NOT NULL DEFAULT TRUE,
    is_active   BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_order  INTEGER      NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_command_slug      ON ah_core.command (slug);
CREATE INDEX IF NOT EXISTS idx_ah_core_command_is_active ON ah_core.command (is_active);
CREATE INDEX IF NOT EXISTS idx_ah_core_command_category  ON ah_core.command (category);

-- ============================
-- SESSION CONTROL
-- ============================
INSERT INTO ah_core.command (slug, name, description, category, handler_type, sort_order) VALUES
('help', 'Help', 'List available commands and short descriptions', 'session', 'builtin', 10),
('clear', 'Clear', 'Clear the current chat session messages from view (session preserved on disk)', 'session', 'builtin', 20),
('reset', 'Reset', 'Reset the current session to its initial agent + skill bindings', 'session', 'builtin', 30);

-- /summarize — force a compact pass on the current session.
INSERT INTO ah_core.command (slug, name, description, argument_hint, category, handler_type, sort_order) VALUES
('summarize', 'Summarize', 'Compact the current session via LLM-generated summary (frees context tokens)', NULL, 'session', 'builtin', 40);

-- /export — download session as JSON or Markdown.
INSERT INTO ah_core.command (slug, name, description, argument_hint, category, handler_type, sort_order) VALUES
('export', 'Export Session', 'Download the current session transcript as JSON or Markdown', '<format: json|md>', 'session', 'builtin', 50);

-- /resume + /branch — session lifecycle (PERSIST-004 + PERSIST-005).
INSERT INTO ah_core.command (slug, name, description, argument_hint, category, handler_type, sort_order) VALUES
('resume', 'Resume Session', 'Reopen a previous session by id (rebuilds conversation; permissions re-granted)', '<sessionId>', 'session', 'builtin', 60),
('branch', 'Branch Session', 'Fork the current session to explore an alternative path without losing the original', NULL, 'session', 'builtin', 70);

-- ============================
-- DISCOVERY
-- ============================
INSERT INTO ah_core.command (slug, name, description, category, handler_type, sort_order) VALUES
('skills', 'List Skills', 'Show skills available to the current agent', 'discovery', 'builtin', 100),
('agents', 'Switch Agent', 'Open the agent picker to switch the current session agent', 'discovery', 'builtin', 110),
('memory', 'Memory Inspector', 'Show stored memories for the current session', 'discovery', 'builtin', 120);

-- ============================
-- ANALYTICS
-- ============================
INSERT INTO ah_core.command (slug, name, description, category, handler_type, sort_order) VALUES
('cost', 'Cost', 'Show token usage and cost for the current session', 'analytics', 'builtin', 200);

-- ============================
-- COLLABORATION
-- ============================
INSERT INTO ah_core.command (slug, name, description, argument_hint, category, handler_type, sort_order) VALUES
('share', 'Share Session', 'Generate a shareable link for the current session (read-only)', '<email|public>', 'collaboration', 'builtin', 300);

-- ============================
-- FEEDBACK
-- ============================
INSERT INTO ah_core.command (slug, name, description, argument_hint, category, handler_type, sort_order) VALUES
('feedback', 'Send Feedback', 'Submit feedback about the current session, agent, or response', '<message>', 'feedback', 'builtin', 400);
