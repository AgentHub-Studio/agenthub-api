CREATE INDEX IF NOT EXISTS idx_chat_message_session_created_at
    ON chat_message (session_id, created_at ASC);

CREATE INDEX IF NOT EXISTS idx_chat_message_session_role_created_at_desc
    ON chat_message (session_id, role, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_chat_message_session_compact_summary_created_at_desc
    ON chat_message (session_id, created_at DESC)
    WHERE message_type = 'compact_summary';
