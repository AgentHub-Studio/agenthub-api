CREATE TABLE IF NOT EXISTS ah_core.context_assembly_source_template (
    id                  UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    slug                VARCHAR(64)  NOT NULL UNIQUE,
    label               VARCHAR(128) NOT NULL,
    description         TEXT         NOT NULL,
    source_order        INTEGER      NOT NULL CHECK (source_order >= 1 AND source_order <= 9),
    domain              VARCHAR(32)  NOT NULL,
    is_memoized         BOOLEAN      NOT NULL DEFAULT false,
    is_asynchronous     BOOLEAN      NOT NULL DEFAULT false,
    is_always_included  BOOLEAN      NOT NULL DEFAULT true,
    sort_order          INTEGER      NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- §7.1: nine sources assembled in fixed order before each model call.
-- source_order: canonical position 1..9
-- domain: prompt_construction|platform_context|instruction_files|memory|tool_definitions|conversation
-- is_memoized: true = computed once per session and cached
-- is_asynchronous: true = prefetched asynchronously without blocking assembly
-- is_always_included: false = conditional / lazy (path-scoped rules, auto_memory, compact_summaries)
INSERT INTO ah_core.context_assembly_source_template
    (slug, label, description, source_order, domain,
     is_memoized, is_asynchronous, is_always_included, sort_order)
VALUES
    ('system_prompt',
     'System Prompt',
     'Agent system prompt with output-style modifications and any appended system content. The foundational context block present in every turn.',
     1, 'prompt_construction', false, false, true, 0),

    ('environment_info',
     'Environment Info',
     'Platform metadata from getSystemContext(): git status (skipped in remote/web mode), optional cache-break injection. Memoized once per session to avoid redundant computation.',
     2, 'platform_context', true, false, true, 1),

    ('claude_md_hierarchy',
     'CLAUDE.md Hierarchy',
     'Four-level instruction file hierarchy loaded by getUserContext(): platform policy, user profile, project instructions, and local overrides. Memoized once per session.',
     3, 'instruction_files', true, false, true, 2),

    ('path_scoped_rules',
     'Path-Scoped Rules',
     'Directory-matched instruction files loaded lazily when the agent reads files in directories that have associated rule files. Conditional: only present when matching files exist.',
     4, 'instruction_files', false, false, false, 3),

    ('auto_memory',
     'Auto Memory',
     'Contextually relevant memory entries prefetched asynchronously. Uses LLM-based header scanning (file granularity, up to 5 files, no vector similarity). Conditional: only injected when relevant entries are found.',
     5, 'memory', false, true, false, 4),

    ('tool_metadata',
     'Tool Metadata',
     'Skill descriptions, MCP tool names, and deferred tool definitions resolved via ToolSearch on demand. Prefetched asynchronously; always included since at least built-in tools are always present.',
     6, 'tool_definitions', false, true, true, 5),

    ('conversation_history',
     'Conversation History',
     'Full message history from prior turns, carried forward subject to compaction. Always present; may be a reduced slice after compaction stages run.',
     7, 'conversation', false, false, true, 6),

    ('tool_results',
     'Tool Results',
     'Outputs from tool executions: file reads, command outputs, subagent summaries, and tool_result content blocks. Always present when any tool was called in prior turns.',
     8, 'conversation', false, false, true, 7),

    ('compact_summaries',
     'Compact Summaries',
     'LLM-generated summaries that replace older history segments after compaction events. Conditional: only present in sessions where at least one compaction has occurred.',
     9, 'conversation', false, false, false, 8)
ON CONFLICT (slug) DO NOTHING;
