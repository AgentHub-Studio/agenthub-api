ALTER TABLE chat_session ADD COLUMN IF NOT EXISTS config_hash VARCHAR(64);

COMMENT ON COLUMN chat_session.config_hash IS
    'SHA-256 hash of the agent modelConfig at the time of the last run. '
    'Used to detect config changes between turns and inject a system notification.';
