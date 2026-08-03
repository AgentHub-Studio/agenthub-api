DROP INDEX IF EXISTS idx_chat_session_persona_id;
DROP INDEX IF EXISTS idx_chat_session_mode;

ALTER TABLE chat_session
    DROP COLUMN IF EXISTS sticky_skill_set,
    DROP COLUMN IF EXISTS persona_id,
    DROP COLUMN IF EXISTS mode;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM chat_session WHERE agent_id IS NULL LIMIT 1) THEN
        ALTER TABLE chat_session ALTER COLUMN agent_id SET NOT NULL;
    END IF;
END $$;

DROP INDEX IF EXISTS idx_skill_embedded_at;
DROP INDEX IF EXISTS idx_skill_embedding_source_hash;

ALTER TABLE skill
    DROP COLUMN IF EXISTS embedded_at,
    DROP COLUMN IF EXISTS embedding_source_hash,
    DROP COLUMN IF EXISTS embedding;
