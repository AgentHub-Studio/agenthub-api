ALTER TABLE chat_session
    ADD COLUMN IF NOT EXISTS cloned_from_session_id UUID REFERENCES chat_session (id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS cloned_from_session_title VARCHAR(500);

CREATE INDEX IF NOT EXISTS idx_chat_session_cloned_from_session_id
    ON chat_session (cloned_from_session_id);
