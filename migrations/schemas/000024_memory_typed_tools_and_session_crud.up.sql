-- Migration 000023: typed memory extraction, session CRUD tools, prompt-template CRUD tools
--
-- Rationale:
--   1. Memory eval prompt: adopt Claude Code's four-type taxonomy (user|feedback|project|reference)
--      with structured JSON output, replacing the flat key/value approach.
--      Inspired by CC's memdir/memoryTypes.ts + extractMemories/prompts.ts.
--
--   2. New tools: fill CRUD gaps in session management and prompt-template management.
--      Also adds agenthub_list_memories (with type filter) for curate-memory skill.
--
--   3. Update curate-memory skill description to leverage typed memory management.
--
--   4. Add agenthub_list_memories and agenthub_get_session to curate-memory allowed tools.
--
-- IDs: tools use c1000000-0000-0000-0000-0000000000{65..70} range.
-- Skills: b1000000-0000-0000-0002-000000000004 (curate-memory).

-- =====================================================================
-- 1. UPDATE memory_eval_prompt TEMPLATE WITH TYPED EXTRACTION
--    Inspired by CC's four-type taxonomy (user|feedback|project|reference)
--    CC reference: memdir/memoryTypes.ts, extractMemories/prompts.ts
-- =====================================================================

UPDATE prompt_template
SET
    content = $CONTENT$
Analyze the conversation turn below and identify information worth storing in FUTURE conversations.

Use the four-type taxonomy to classify each memory:

**user** — Information about the person: their role, goals, expertise, preferences, communication style.
  When to save: any detail about who they are or how they want to collaborate.
  Example: {"type":"user","key":"preferred_language","value":"user prefers responses in Portuguese BR"}

**feedback** — Guidance about how to approach work: corrections, confirmed approaches, stated preferences.
  When to save: whenever user corrects ("don't do X") OR confirms ("yes, exactly").
  Include WHY so it can be applied in edge cases.
  Example: {"type":"feedback","key":"no_mock_db","value":"never mock the database in tests — prior incident where mocked tests passed but prod migration failed"}

**project** — Ongoing work context: goals, deadlines, bugs, team decisions. Convert relative dates to absolute.
  When to save: who is doing what, why, or by when.
  Example: {"type":"project","key":"merge_freeze","value":"merge freeze from 2026-05-01 for release cut — no non-critical PRs"}

**reference** — Pointers to external systems: Linear projects, Grafana dashboards, Slack channels, APIs.
  When to save: location of information in external systems.
  Example: {"type":"reference","key":"pipeline_bugs","value":"pipeline bugs tracked in Linear project INGEST"}

DO NOT memorize:
- Code patterns, architecture, file paths — derivable from the codebase
- Git history or recent changes — use git log / git blame
- Debugging fixes — the fix is in the code
- Ephemeral data: search results, timestamps, session-specific values
- Greetings, filler, or obvious context

For each memory item, output a JSON object with fields: type, key, value.
- key: short snake_case identifier (e.g. "preferred_language", "project_stack")
- value: concise description, one sentence max
- type: one of user | feedback | project | reference

Respond with a JSON array. If nothing is worth memorizing, respond with [].

---
{{conversation}}
---

Memories (JSON array):
$CONTENT$,
    updated_at = NOW()
WHERE slug = 'memory-eval-prompt'
  AND (agent_id IS NULL);

-- Also update the templatestore.go embedded content by upserting if not present
-- (handles the case where the agent has not yet been seeded)
INSERT INTO prompt_template (id, agent_id, name, slug, description, content, category, is_builtin, created_at, updated_at)
VALUES (
    'f1000000-0000-0000-0001-000000000002',
    NULL,
    'Memory Evaluation Prompt',
    'memory-eval-prompt',
    'Internal prompt used by the memory bridge to extract typed memories from conversation turns. Uses the four-type taxonomy: user, feedback, project, reference.',
    $CONTENT2$
Analyze the conversation turn below and identify information worth storing in FUTURE conversations.

Use the four-type taxonomy to classify each memory:

**user** — Information about the person: their role, goals, expertise, preferences, communication style.
  When to save: any detail about who they are or how they want to collaborate.
  Example: {"type":"user","key":"preferred_language","value":"user prefers responses in Portuguese BR"}

**feedback** — Guidance about how to approach work: corrections, confirmed approaches, stated preferences.
  When to save: whenever user corrects ("don't do X") OR confirms ("yes, exactly").
  Include WHY so it can be applied in edge cases.
  Example: {"type":"feedback","key":"no_mock_db","value":"never mock the database in tests — prior incident where mocked tests passed but prod migration failed"}

**project** — Ongoing work context: goals, deadlines, bugs, team decisions. Convert relative dates to absolute.
  When to save: who is doing what, why, or by when.
  Example: {"type":"project","key":"merge_freeze","value":"merge freeze from 2026-05-01 for release cut — no non-critical PRs"}

**reference** — Pointers to external systems: Linear projects, Grafana dashboards, Slack channels, APIs.
  When to save: location of information in external systems.
  Example: {"type":"reference","key":"pipeline_bugs","value":"pipeline bugs tracked in Linear project INGEST"}

DO NOT memorize:
- Code patterns, architecture, file paths — derivable from the codebase
- Git history or recent changes — use git log / git blame
- Debugging fixes — the fix is in the code
- Ephemeral data: search results, timestamps, session-specific values
- Greetings, filler, or obvious context

For each memory item, output a JSON object with fields: type, key, value.
- key: short snake_case identifier (e.g. "preferred_language", "project_stack")
- value: concise description, one sentence max
- type: one of user | feedback | project | reference

Respond with a JSON array. If nothing is worth memorizing, respond with [].

---
{{conversation}}
---

Memories (JSON array):
$CONTENT2$,
    'system',
    TRUE,
    NOW(), NOW()
)
ON CONFLICT DO NOTHING;

-- =====================================================================
-- 2. ADD agenthub_list_memories TOOL
--    Lists agent memories with optional type filter and semantic search.
--    Inspired by CC's MemoryRecaller interface with type-aware filtering.
-- =====================================================================

INSERT INTO tool (id, name, type, description, config, labels, read_only, should_defer, is_destructive, search_hint, always_load, created_at, updated_at)
VALUES (
    'c1000000-0000-0000-0000-000000000065',
    'agenthub_list_memories',
    'HTTP',
    'Lists stored memories for an agent with optional type filter (user|feedback|project|reference) '
    'and optional semantic search query. Returns up to 50 entries sorted by relevance or recency. '
    'Use this to review, audit, or curate an agent''s long-term memory. '
    'Parameters: ''agent_id'' (required), ''type'' (optional: user|feedback|project|reference), '
    '''query'' (optional semantic search phrase), ''limit'' (optional, default 20, max 50).',
    '{"method":"GET","url":"/api/agents/{agent_id}/memories","inputSchema":{"type":"object","properties":{"agent_id":{"type":"string","description":"Agent UUID"},"type":{"type":"string","description":"Memory type filter: user, feedback, project, reference"},"query":{"type":"string","description":"Optional semantic search query"},"limit":{"type":"integer","description":"Max results (default 20, max 50)"}},"required":["agent_id"]}}',
    ARRAY['memory', 'list', 'search'],
    TRUE,   -- read_only
    FALSE,  -- should_defer
    FALSE,  -- is_destructive
    'list memories search recall',
    FALSE,
    NOW(), NOW()
)
ON CONFLICT (id) DO NOTHING;

-- =====================================================================
-- 3. ADD agenthub_get_session TOOL
--    Gets session metadata (messages count, token usage, agent, created_at).
--    Completes the session CRUD (create + list + get).
-- =====================================================================

INSERT INTO tool (id, name, type, description, config, labels, read_only, should_defer, is_destructive, search_hint, always_load, created_at, updated_at)
VALUES (
    'c1000000-0000-0000-0000-000000000066',
    'agenthub_get_session',
    'HTTP',
    'Gets metadata for a specific chat session including message count, token usage, '
    'linked agent name, creation date, and last activity. '
    'Parameters: ''session_id'' (required UUID).',
    '{"method":"GET","url":"/api/chat/sessions/{session_id}","inputSchema":{"type":"object","properties":{"session_id":{"type":"string","description":"Session UUID"}},"required":["session_id"]}}',
    ARRAY['session', 'chat', 'get'],
    TRUE,   -- read_only
    FALSE,  -- should_defer
    FALSE,  -- is_destructive
    'get session info metadata',
    FALSE,
    NOW(), NOW()
)
ON CONFLICT (id) DO NOTHING;

-- =====================================================================
-- 4. ADD agenthub_delete_session TOOL
--    Archives/deletes a chat session and its messages.
--    Destructive — requires explicit confirmation.
-- =====================================================================

INSERT INTO tool (id, name, type, description, config, labels, read_only, should_defer, is_destructive, search_hint, always_load, created_at, updated_at)
VALUES (
    'c1000000-0000-0000-0000-000000000067',
    'agenthub_delete_session',
    'HTTP',
    'Permanently deletes a chat session and all its messages. This operation is irreversible. '
    'ALWAYS confirm with the user before calling this tool. '
    'Parameters: ''session_id'' (required UUID).',
    '{"method":"DELETE","url":"/api/chat/sessions/{session_id}","inputSchema":{"type":"object","properties":{"session_id":{"type":"string","description":"Session UUID to delete"}},"required":["session_id"]}}',
    ARRAY['session', 'chat', 'delete'],
    FALSE,  -- read_only
    TRUE,   -- should_defer
    TRUE,   -- is_destructive
    'delete session remove',
    FALSE,
    NOW(), NOW()
)
ON CONFLICT (id) DO NOTHING;

-- =====================================================================
-- 5. ADD agenthub_get_prompt_template TOOL
--    Gets full content of a specific prompt template.
-- =====================================================================

INSERT INTO tool (id, name, type, description, config, labels, read_only, should_defer, is_destructive, search_hint, always_load, created_at, updated_at)
VALUES (
    'c1000000-0000-0000-0000-000000000068',
    'agenthub_get_prompt_template',
    'HTTP',
    'Gets the full content and metadata of a specific prompt template. '
    'Returns: name, slug, category, content (full text), model_override, allowed_tools, is_builtin. '
    'Parameters: ''template_id'' (required UUID) or use agenthub_list_prompt_templates first to find the ID.',
    '{"method":"GET","url":"/api/prompt-templates/{template_id}","inputSchema":{"type":"object","properties":{"template_id":{"type":"string","description":"Prompt template UUID"}},"required":["template_id"]}}',
    ARRAY['prompt', 'template', 'get'],
    TRUE,   -- read_only
    FALSE,  -- should_defer
    FALSE,  -- is_destructive
    'get prompt template content',
    FALSE,
    NOW(), NOW()
)
ON CONFLICT (id) DO NOTHING;

-- =====================================================================
-- 6. ADD agenthub_update_prompt_template TOOL
--    Updates an existing prompt template's content or metadata.
--    Not allowed on is_builtin=true templates.
-- =====================================================================

INSERT INTO tool (id, name, type, description, config, labels, read_only, should_defer, is_destructive, search_hint, always_load, created_at, updated_at)
VALUES (
    'c1000000-0000-0000-0000-000000000069',
    'agenthub_update_prompt_template',
    'HTTP',
    'Updates a prompt template''s name, description, content, category, or model_override. '
    'Cannot update builtin templates (is_builtin=true) — clone them first via agenthub_create_prompt_template. '
    'Parameters: ''template_id'' (required), plus any of: ''name'', ''description'', ''content'', '
    '''category'', ''model_override'', ''allowed_tools'' (array of tool slugs).',
    '{"method":"PATCH","url":"/api/prompt-templates/{template_id}","inputSchema":{"type":"object","properties":{"template_id":{"type":"string","description":"Prompt template UUID"},"name":{"type":"string"},"description":{"type":"string"},"content":{"type":"string","description":"Full system prompt text"},"category":{"type":"string"},"model_override":{"type":"string"},"allowed_tools":{"type":"array","items":{"type":"string"}}},"required":["template_id"]}}',
    ARRAY['prompt', 'template', 'update'],
    FALSE,  -- read_only
    FALSE,  -- should_defer
    FALSE,  -- is_destructive
    'update edit prompt template',
    FALSE,
    NOW(), NOW()
)
ON CONFLICT (id) DO NOTHING;

-- =====================================================================
-- 7. ADD agenthub_delete_prompt_template TOOL
--    Deletes a custom prompt template (not allowed for is_builtin=true).
-- =====================================================================

INSERT INTO tool (id, name, type, description, config, labels, read_only, should_defer, is_destructive, search_hint, always_load, created_at, updated_at)
VALUES (
    'c1000000-0000-0000-0000-000000000070',
    'agenthub_delete_prompt_template',
    'HTTP',
    'Deletes a custom prompt template. Cannot delete builtin templates (is_builtin=true). '
    'ALWAYS confirm with the user before calling this tool. '
    'Parameters: ''template_id'' (required UUID).',
    '{"method":"DELETE","url":"/api/prompt-templates/{template_id}","inputSchema":{"type":"object","properties":{"template_id":{"type":"string","description":"Prompt template UUID to delete"}},"required":["template_id"]}}',
    ARRAY['prompt', 'template', 'delete'],
    FALSE,  -- read_only
    TRUE,   -- should_defer
    TRUE,   -- is_destructive
    'delete remove prompt template',
    FALSE,
    NOW(), NOW()
)
ON CONFLICT (id) DO NOTHING;

-- =====================================================================
-- 8. BIND NEW TOOLS TO SKILLS
-- =====================================================================

-- prompt-template-management skill: gets full CRUD now
-- (skill ID b1000000-0000-0000-0001-000000000009 = prompt-template-management from 000013)
INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at)
VALUES
    (gen_random_uuid(), 'b1000000-0000-0000-0001-000000000009', 'c1000000-0000-0000-0000-000000000068', 100, true, NOW()),  -- get_prompt_template
    (gen_random_uuid(), 'b1000000-0000-0000-0001-000000000009', 'c1000000-0000-0000-0000-000000000069', 101, true, NOW()),  -- update_prompt_template
    (gen_random_uuid(), 'b1000000-0000-0000-0001-000000000009', 'c1000000-0000-0000-0000-000000000070', 102, true, NOW())   -- delete_prompt_template
ON CONFLICT DO NOTHING;

-- chat-management skill: adds get/delete session
-- (skill ID b1000000-0000-0000-0001-000000000007 = chat-management from 000013)
INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at)
VALUES
    (gen_random_uuid(), 'b1000000-0000-0000-0001-000000000007', 'c1000000-0000-0000-0000-000000000066', 100, true, NOW()),  -- get_session
    (gen_random_uuid(), 'b1000000-0000-0000-0001-000000000007', 'c1000000-0000-0000-0000-000000000067', 101, true, NOW())   -- delete_session
ON CONFLICT DO NOTHING;

-- curate-memory skill: adds list_memories tool
-- (skill ID b1000000-0000-0000-0002-000000000004 = curate-memory from 000020)
INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at)
VALUES
    (gen_random_uuid(), 'b1000000-0000-0000-0002-000000000004', 'c1000000-0000-0000-0000-000000000065', 100, true, NOW())   -- list_memories
ON CONFLICT DO NOTHING;

-- AgentHub Assistant agent: bind new read tools (not destructive ones)
-- (agent ID d1000000-0000-0000-0001-000000000001 = AgentHub Assistant from 000013)
INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at)
SELECT
    gen_random_uuid(),
    st.skill_id,
    t.id,
    200,
    true,
    NOW()
FROM agent_skill ast
JOIN skill_tool st ON st.skill_id = ast.skill_id
JOIN tool t ON t.name IN ('agenthub_list_memories', 'agenthub_get_session', 'agenthub_get_prompt_template')
WHERE ast.agent_id = 'd1000000-0000-0000-0001-000000000001'
ON CONFLICT DO NOTHING;

-- =====================================================================
-- 9. UPDATE curate-memory SKILL DESCRIPTION WITH TYPED MEMORY CONTEXT
--    Inspired by CC's four-type memory taxonomy in memoryTypes.ts
-- =====================================================================

UPDATE skill
SET description =
    'Reviews and curates an agent''s long-term memories using the four-type taxonomy: '
    'user (preferences, expertise), feedback (corrections, confirmed approaches), '
    'project (deadlines, decisions, goals), and reference (pointers to external systems). '
    'Use when the user wants to clean up memories, resolve conflicts, remove outdated entries, '
    'or understand what the agent has learned about them. '
    'Parameters: ''agent_id'' (target agent UUID), ''action'' (optional: ''audit'', ''deduplicate'', ''clean'', ''list''). '
    'Returns a structured memory report with categories, counts, and suggested operations.',
    updated_at = NOW()
WHERE slug = 'curate-memory';

-- =====================================================================
-- 10. SET concurrency_safe ON NEW READ-ONLY TOOLS
--     These can safely run in parallel with other read operations.
-- =====================================================================

UPDATE tool
SET concurrency_safe = TRUE
WHERE name IN (
    'agenthub_list_memories',
    'agenthub_get_session',
    'agenthub_get_prompt_template'
);

-- =====================================================================
-- 11. SET max_result_chars FOR NEW TOOLS
--     Prompt templates can be large; cap at 30K to prevent context bloat.
-- =====================================================================

UPDATE tool
SET max_result_chars = 30000
WHERE name = 'agenthub_get_prompt_template';

UPDATE tool
SET max_result_chars = 20000
WHERE name = 'agenthub_list_memories';
