DROP INDEX IF EXISTS idx_tool_execution_suspend_state;

ALTER TABLE tool_execution
DROP COLUMN IF EXISTS state;
