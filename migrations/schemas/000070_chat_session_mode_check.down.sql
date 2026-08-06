ALTER TABLE chat_session
    DROP CONSTRAINT IF EXISTS chk_chat_session_mode_agent;

ALTER TABLE chat_session
    ALTER COLUMN mode SET DEFAULT 'AGENT';
