ALTER TABLE tool_execution
ADD COLUMN IF NOT EXISTS state JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE INDEX IF NOT EXISTS idx_tool_execution_suspend_state
ON tool_execution ((state->>'sessionId'), (state->>'requestId'))
WHERE state ? 'requestId';
