DROP INDEX IF EXISTS idx_eval_run_chat_run;
DROP INDEX IF EXISTS idx_eval_run_session_created_at;
DROP INDEX IF EXISTS idx_eval_run_agent_created_at;
DROP TABLE IF EXISTS eval_run;

ALTER TABLE agent
  DROP COLUMN IF EXISTS eval_config;
