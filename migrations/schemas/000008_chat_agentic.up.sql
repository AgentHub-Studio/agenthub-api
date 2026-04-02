-- 000008_chat_agentic.up.sql
-- Add agentic mode support to chat_message and agent tables.

-- ─── chat_message: agentic fields ────────────────────────────────────────────

ALTER TABLE chat_message
    ADD COLUMN message_type  VARCHAR(20)  NOT NULL DEFAULT 'text',
    ADD COLUMN tool_calls    JSONB,
    ADD COLUMN tool_call_id  VARCHAR(64),
    ADD COLUMN metadata      JSONB,
    ADD COLUMN token_usage   JSONB,
    ADD COLUMN finish_reason VARCHAR(20),
    ADD COLUMN turn_index    INTEGER      NOT NULL DEFAULT 0;

COMMENT ON COLUMN chat_message.message_type  IS 'text | tool_use | tool_result | system | compact_summary';
COMMENT ON COLUMN chat_message.tool_calls    IS 'Array of tool calls [{id, name, input}] when finish_reason=tool_calls';
COMMENT ON COLUMN chat_message.tool_call_id  IS 'References the tool_call that produced this tool_result';
COMMENT ON COLUMN chat_message.metadata      IS 'Provider, model, latency and other execution metadata';
COMMENT ON COLUMN chat_message.token_usage   IS '{input_tokens, output_tokens, cache_read, cache_write}';
COMMENT ON COLUMN chat_message.finish_reason IS 'stop | tool_calls | max_tokens | content_filter';
COMMENT ON COLUMN chat_message.turn_index    IS 'Groups messages belonging to the same agentic turn';

CREATE INDEX idx_chat_message_tool_call_id
    ON chat_message (tool_call_id)
    WHERE tool_call_id IS NOT NULL;

-- ─── agent: agentic configuration ────────────────────────────────────────────

ALTER TABLE agent
    ADD COLUMN system_prompt TEXT,
    ADD COLUMN model_config  JSONB;

COMMENT ON COLUMN agent.system_prompt IS 'System prompt that defines the agent personality and instructions';
COMMENT ON COLUMN agent.model_config  IS '{"provider","model","temperature","maxTokens"}';

-- Drop the pipeline_id column (pipelines are deprecated in favor of agentic mode).
ALTER TABLE agent DROP COLUMN IF EXISTS pipeline_id;
