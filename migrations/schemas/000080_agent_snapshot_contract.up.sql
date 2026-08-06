ALTER TABLE chat_session
    ADD COLUMN IF NOT EXISTS agent_snapshot JSONB,
    ADD COLUMN IF NOT EXISTS agent_snapshot_hash VARCHAR(64);

COMMENT ON COLUMN chat_session.agent_snapshot IS
    'Canonical RT-01 snapshot of the agent configuration captured for a chat session.';

COMMENT ON COLUMN chat_session.agent_snapshot_hash IS
    'SHA-256 hash of agent_snapshot for snapshot integrity and config-change detection.';
