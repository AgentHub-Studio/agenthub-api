-- Seed capability-category slash commands in ah_core.command.
-- These give tenants instant access to the capability layer introduced
-- in migrations 000090 (skills/tools) and 000091 (agents) without any
-- custom configuration. Commands are adapted from Claude Code's /help,
-- /research, and /plan built-in commands for AgentHub's web UX.
--
-- Prerequisites: 000008_seed_commands.up.sql (creates ah_core.command table).
--
-- Commands seeded:
--   /research     — web research via core-researcher agent
--   /analyze      — document analysis via core-analyst agent
--   /plan         — task planning via core-planner agent
--   /summarize-doc — document summarisation (distinct from /summarize session command)
--   /tasks        — list current tasks (no argument required)
--
-- handler_type = 'prompt': runner expands prompt_template with user args.
-- disable_model_invocation = false: LLM may also invoke these as meta-tools.

INSERT INTO ah_core.command
    (slug, name, description, argument_hint, category, handler_type,
     prompt_template, requires_admin, disable_model_invocation, is_active, sort_order)
VALUES
  ('research', 'Research', 'Search the web and fetch pages to gather current information on any topic',
   '<topic or question>', 'capability', 'prompt',
   'You are acting as the Web Researcher. Research the following topic thoroughly using web search and URL fetching. Cite all sources. Topic: {{.Args}}',
   false, false, true, 0),

  ('analyze', 'Analyze', 'Search knowledge base documents and provide a comprehensive analysis with citations',
   '<topic or question>', 'capability', 'prompt',
   'You are acting as the Document Analyst. Search the knowledge base for documents related to the following topic and provide a comprehensive analysis with citations. Topic: {{.Args}}',
   false, false, true, 1),

  ('plan', 'Plan', 'Break down a goal into a clear, actionable task list and track progress',
   '<goal or project>', 'capability', 'prompt',
   'You are acting as the Task Planner. Break down the following goal into a clear, actionable task list. Create tasks for each step and track progress. Goal: {{.Args}}',
   false, false, true, 2),

  ('summarize-doc', 'Summarize Document', 'Find a document in the knowledge base and summarise its key points and insights',
   '<document name or ID>', 'capability', 'prompt',
   'Search the knowledge base for the specified document and provide a comprehensive summary including key points, conclusions, and actionable insights. Document: {{.Args}}',
   false, false, true, 3),

  ('tasks', 'List Tasks', 'List all current tasks and their status with a progress summary',
   '', 'capability', 'prompt',
   'List all current tasks and their status. Show completed tasks separately from pending tasks. Provide a progress summary.',
   false, false, true, 4)

ON CONFLICT (slug) DO NOTHING;
