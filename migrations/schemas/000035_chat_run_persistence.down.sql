DROP INDEX IF EXISTS idx_chat_run_tenant;
DROP INDEX IF EXISTS idx_chat_run_session_status;
ALTER TABLE chat_message DROP COLUMN IF EXISTS run_id;
DROP TABLE IF EXISTS chat_run;
