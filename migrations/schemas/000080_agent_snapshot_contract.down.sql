ALTER TABLE chat_session
    DROP COLUMN IF EXISTS agent_snapshot_hash,
    DROP COLUMN IF EXISTS agent_snapshot;
