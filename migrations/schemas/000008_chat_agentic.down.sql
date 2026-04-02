-- 000008_chat_agentic.down.sql
-- Revert agentic mode changes.

-- ─── agent: restore pipeline_id, remove agentic fields ──────────────────────

ALTER TABLE agent
    ADD COLUMN pipeline_id UUID;

ALTER TABLE agent
    DROP COLUMN IF EXISTS system_prompt,
    DROP COLUMN IF EXISTS model_config;

-- ─── chat_message: remove agentic fields ─────────────────────────────────────

DROP INDEX IF EXISTS idx_chat_message_tool_call_id;

ALTER TABLE chat_message
    DROP COLUMN IF EXISTS message_type,
    DROP COLUMN IF EXISTS tool_calls,
    DROP COLUMN IF EXISTS tool_call_id,
    DROP COLUMN IF EXISTS metadata,
    DROP COLUMN IF EXISTS token_usage,
    DROP COLUMN IF EXISTS finish_reason,
    DROP COLUMN IF EXISTS turn_index;
