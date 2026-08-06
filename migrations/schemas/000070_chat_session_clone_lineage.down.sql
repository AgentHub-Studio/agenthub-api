DROP INDEX IF EXISTS idx_chat_session_cloned_from_session_id;

ALTER TABLE chat_session
    DROP COLUMN IF EXISTS cloned_from_session_title,
    DROP COLUMN IF EXISTS cloned_from_session_id;
