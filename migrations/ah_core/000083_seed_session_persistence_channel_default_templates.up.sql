CREATE TABLE IF NOT EXISTS ah_core.session_persistence_channel_template (
    id                  UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    slug                VARCHAR(64)  NOT NULL UNIQUE,
    label               VARCHAR(128) NOT NULL,
    description         TEXT         NOT NULL,
    storage_format      VARCHAR(16)  NOT NULL CHECK (storage_format IN ('jsonl', 'json', 'db_row')),
    is_append_only      BOOLEAN      NOT NULL DEFAULT true,
    is_project_scoped   BOOLEAN      NOT NULL DEFAULT false,
    is_always_active    BOOLEAN      NOT NULL DEFAULT true,
    sort_order          INTEGER      NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- §9.1: three persistence channels for session state in the Claude Code architecture.
-- AgentHub maps each channel to its PostgreSQL-backed equivalent.
--
-- storage_format: the original format before db persistence
--   jsonl   — append-only JSONL files (conversation transcripts, sidechains)
--   json    — single JSON document (meta files)
--   db_row  — native AgentHub DB record (no file equivalent)
--
-- is_append_only: true for transcript/history channels (never mutate past entries)
-- is_project_scoped: true for channels whose scope matches a project/session pair
-- is_always_active: false = only present when the feature is in use (e.g. sidechains)
INSERT INTO ah_core.session_persistence_channel_template
    (slug, label, description, storage_format,
     is_append_only, is_project_scoped, is_always_active, sort_order)
VALUES
    ('session_transcripts',
     'Session Transcripts',
     'Complete conversation records for a session: user messages, assistant turns, attachment messages, system messages, and compaction events. Project-scoped (one record per session). Append-only — compaction never deletes previously written entries, only appends boundary markers and summary events. In AgentHub, persisted as rows in agent_execution and agent_execution_node tables.',
     'jsonl', true, true, true, 0),

    ('global_prompt_history',
     'Global Prompt History',
     'User prompts only, stored independently of full conversation transcripts. Enables reverse-chronological navigation (history.jsonl via readLinesReverse()). Not project-scoped — shared across sessions for the user. In AgentHub, persisted as a separate audit/history log per tenant user.',
     'jsonl', true, false, true, 1),

    ('subagent_sidechains',
     'Subagent Sidechains',
     'Per-subagent conversation transcripts (.jsonl) paired with metadata files (.meta.json). Summary-only return model: only the subagent final response and metadata return to the parent; the full sidechain never enters the parent context window. Conditional: only present when subagents are invoked. In AgentHub, modeled as nested agent_execution records linked to the parent run.',
     'jsonl', true, true, false, 2)
ON CONFLICT (slug) DO NOTHING;
