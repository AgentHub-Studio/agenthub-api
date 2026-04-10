ALTER TABLE chat_session
    ADD COLUMN IF NOT EXISTS system_prompt_snapshot TEXT,
    ADD COLUMN IF NOT EXISTS model_config_snapshot   JSONB;

COMMENT ON COLUMN chat_session.system_prompt_snapshot IS
    'Snapshot of the agent''s systemPrompt at session creation time. '
    'Ensures persona consistency throughout the conversation even if the agent is updated.';

COMMENT ON COLUMN chat_session.model_config_snapshot IS
    'Snapshot of the agent''s modelConfig at session creation time. '
    'Ensures the same model/provider is used across all runs in the session.';
