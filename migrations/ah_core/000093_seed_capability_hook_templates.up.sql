-- Seed capability-layer hooks in ah_core.hook (migration 000093).
-- These four hooks instrument the capability agent layer introduced in
-- migrations 000090-000092 (skills/tools → agents → commands).
--
-- Design rationale:
--   - PostToolUse hooks enforce citation discipline after web/doc tool calls —
--     they inject a reminder WITHOUT blocking the run (prompt type, no side effects).
--   - PreToolUse hook validates the query shape BEFORE the web-search tool fires —
--     catches empty/gibberish queries early and saves an LLM round-trip.
--   - SessionStart hook pre-loads existing task context so capability agents
--     resume with full awareness of pending work items.
--
-- All four are prompt-type only (zero external calls). Tenants may layer their
-- own HTTP hooks on top. None require admin to disable — they are advisory.
--
-- Hook table is created by migration 000010_seed_hooks.up.sql.

INSERT INTO ah_core.hook (slug, name, description, event, hook_type, inject_text, matcher, priority, requires_admin_to_disable, sort_order)
VALUES

-- 1. PostToolUse — cite web sources after every core-web-search / core-web-fetch call.
('capability-posttooluse-cite-web-sources',
 'Capability Post Tool Use — Cite Web Sources',
 'After a web search or fetch tool returns, remind the agent to include source URLs in its response so users can verify findings. Applies to core-web-search and core-web-fetch.',
 'PostToolUse', 'prompt',
 'You just received web content from a search or fetch tool. ALWAYS include at least one source URL in your response so the user can verify the information. Format citations as inline references or a brief "Sources:" section. Never omit source attribution for web-retrieved content.',
 'core-web-search,core-web-fetch', 75, FALSE, 500),

-- 2. PostToolUse — index document IDs into citations after core-doc-search.
('capability-posttooluse-index-doc-citations',
 'Capability Post Tool Use — Index Document Citations',
 'After a document search tool returns, remind the agent to include document IDs or names in its response so users can trace results back to source documents in the knowledge base.',
 'PostToolUse', 'prompt',
 'You just retrieved content from a knowledge base document search. Include the document name or ID for each key finding so the user can locate the source document. Format as brief inline references (e.g. "[Doc: report-q4-2025]"). Never present document-search results as unsourced assertions.',
 'core-doc-search', 70, FALSE, 510),

-- 3. PreToolUse — validate the search query before executing core-web-search.
('capability-pretooluse-validate-search-query',
 'Capability Pre Tool Use — Validate Search Query',
 'Before a web search executes, check that the query is well-formed: non-empty, not a single stop-word, and specific enough to return useful results. Saves a wasted tool round-trip on vague queries.',
 'PreToolUse', 'prompt',
 'You are about to call a web search tool. STOP and verify the query: (1) it must be non-empty, (2) it must not be a single common stop-word (e.g. "the", "a", "it"), (3) it must be specific enough to retrieve relevant results. If the query fails any check, refine it before executing. A precise query returns better results with fewer follow-up calls.',
 'core-web-search', 80, FALSE, 520),

-- 4. SessionStart — load task context when a capability session begins.
('capability-sessionstart-load-task-context',
 'Capability Session Start — Load Task Context',
 'When a capability agent session starts, remind the agent to surface any existing tasks or in-progress work items before responding to the user. Prevents the capability layer from starting blind when prior tasks exist.',
 'SessionStart', 'prompt',
 'A new capability session is starting. Before responding: check if there are existing tasks or in-progress work items the user may be continuing. If tasks exist, briefly acknowledge them (1-2 sentences) and ask whether the user wants to resume or start fresh. If no tasks exist, proceed normally with the user request.',
 '', 65, FALSE, 530)

ON CONFLICT (slug) DO NOTHING;
