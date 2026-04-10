ALTER TABLE chat_session
    ADD COLUMN IF NOT EXISTS skill_bindings_snapshot JSONB;

COMMENT ON COLUMN chat_session.skill_bindings_snapshot IS
    'Array of skill UUIDs bound to the agent at session creation time. '
    'Format: {"skillIds": ["uuid1", "uuid2"]}. '
    'Guarantees the same tool set is available across all runs of the session '
    'even if the agent''s bindings change mid-conversation. P-C115-1.';
