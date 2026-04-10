ALTER TABLE chat_session
    DROP COLUMN IF EXISTS system_prompt_snapshot,
    DROP COLUMN IF EXISTS model_config_snapshot;
