-- Capability tools and skills adapted from Claude Code for AgentHub web.
-- These give fresh tenants functional AI capabilities without any
-- custom configuration. Adapted from Claude Code's built-in tools:
--   WebSearch  → core-web-search
--   WebFetch   → core-web-fetch
--   Grep/search→ core-doc-search  (DOCUMENT_SEARCH)
--   Read       → core-doc-read
--   TodoWrite  → core-todo-create
--   TodoRead   → core-todo-list
--   Agent      → core-subagent-run

INSERT INTO ah_core.tool (slug, name, description, type, config)
VALUES
  ('core-web-search',
   'Web Search',
   'Search the web for current information. Adapted from Claude Code WebSearch.',
   'HTTP',
   '{"url":"/api/tools/web-search","method":"POST","useCallerToken":true}'::jsonb),

  ('core-web-fetch',
   'Web Fetch',
   'Fetch and extract content from a URL. Adapted from Claude Code WebFetch.',
   'HTTP',
   '{"url":"/api/tools/web-fetch","method":"POST","useCallerToken":true}'::jsonb),

  ('core-doc-search',
   'Document Search',
   'Semantic search across knowledge base documents. Adapted from Claude Code Grep/search.',
   'DOCUMENT_SEARCH',
   '{"matchType":"semantic","topK":10}'::jsonb),

  ('core-doc-read',
   'Document Read',
   'Read the full content of a knowledge base document by ID. Adapted from Claude Code Read.',
   'HTTP',
   '{"url":"/api/knowledge-bases/documents/{documentId}/content","method":"GET","useCallerToken":true}'::jsonb),

  ('core-todo-create',
   'Task Create',
   'Create a new task or to-do item to track work progress. Adapted from Claude Code TodoWrite.',
   'HTTP',
   '{"url":"/api/tasks","method":"POST","useCallerToken":true}'::jsonb),

  ('core-todo-list',
   'Task List',
   'List current tasks and to-do items for the active session. Adapted from Claude Code TodoRead.',
   'HTTP',
   '{"url":"/api/tasks","method":"GET","useCallerToken":true}'::jsonb),

  ('core-subagent-run',
   'Run Agent',
   'Trigger a specialist agent run as a subagent. Adapted from Claude Code Agent spawning.',
   'HTTP',
   '{"url":"/api/chat/sessions","method":"POST","useCallerToken":true}'::jsonb)
ON CONFLICT (slug) DO NOTHING;

-- Capability skills: grouped capabilities for actual task execution
-- (category 'capability', distinct from 'platform' management skills).

INSERT INTO ah_core.skill (slug, name, description, instructions, category, context_mode, when_to_use)
VALUES
  ('core-web-research',
   'Web Research',
   'Search the web and fetch URL content to gather current information on any topic.',
   'Use web search to find current information, then fetch specific URLs for detailed content. Always cite sources with URLs. Summarize findings concisely and flag information older than 6 months.',
   'capability',
   'inline',
   'When the user needs current information, facts, news, or content not available in the knowledge base.'),

  ('core-doc-analysis',
   'Document Analysis',
   'Search and read knowledge base documents to analyse, summarise, and answer questions.',
   'Search the knowledge base semantically for relevant documents. Read specific documents for full details. Synthesise information from multiple sources and cite document titles and IDs.',
   'capability',
   'inline',
   'When the user needs to find or analyse information from uploaded documents or knowledge bases.'),

  ('core-task-workflow',
   'Task Workflow',
   'Create and manage task lists to track progress on multi-step work.',
   'Break complex requests into discrete tasks. Create tasks with clear action-oriented descriptions. List tasks to show progress. Update the user as each task completes.',
   'capability',
   'inline',
   'When the user has a multi-step goal that benefits from explicit task tracking and visibility.')
ON CONFLICT (slug) DO NOTHING;

-- Skill-tool bindings for capability skills
-- core-web-research → core-web-search (priority 0), core-web-fetch (priority 1)
INSERT INTO ah_core.skill_tool (skill_id, tool_id, priority)
SELECT s.id, t.id, 0
  FROM ah_core.skill s, ah_core.tool t
 WHERE s.slug = 'core-web-research' AND t.slug = 'core-web-search'
ON CONFLICT (skill_id, tool_id) DO NOTHING;

INSERT INTO ah_core.skill_tool (skill_id, tool_id, priority)
SELECT s.id, t.id, 1
  FROM ah_core.skill s, ah_core.tool t
 WHERE s.slug = 'core-web-research' AND t.slug = 'core-web-fetch'
ON CONFLICT (skill_id, tool_id) DO NOTHING;

-- core-doc-analysis → core-doc-search (priority 0), core-doc-read (priority 1)
INSERT INTO ah_core.skill_tool (skill_id, tool_id, priority)
SELECT s.id, t.id, 0
  FROM ah_core.skill s, ah_core.tool t
 WHERE s.slug = 'core-doc-analysis' AND t.slug = 'core-doc-search'
ON CONFLICT (skill_id, tool_id) DO NOTHING;

INSERT INTO ah_core.skill_tool (skill_id, tool_id, priority)
SELECT s.id, t.id, 1
  FROM ah_core.skill s, ah_core.tool t
 WHERE s.slug = 'core-doc-analysis' AND t.slug = 'core-doc-read'
ON CONFLICT (skill_id, tool_id) DO NOTHING;

-- core-task-workflow → core-todo-create (priority 0), core-todo-list (priority 1)
INSERT INTO ah_core.skill_tool (skill_id, tool_id, priority)
SELECT s.id, t.id, 0
  FROM ah_core.skill s, ah_core.tool t
 WHERE s.slug = 'core-task-workflow' AND t.slug = 'core-todo-create'
ON CONFLICT (skill_id, tool_id) DO NOTHING;

INSERT INTO ah_core.skill_tool (skill_id, tool_id, priority)
SELECT s.id, t.id, 1
  FROM ah_core.skill s, ah_core.tool t
 WHERE s.slug = 'core-task-workflow' AND t.slug = 'core-todo-list'
ON CONFLICT (skill_id, tool_id) DO NOTHING;
