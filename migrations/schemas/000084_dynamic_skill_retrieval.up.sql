-- Dynamic Skill Retrieval metadata and chat-session routing state.

ALTER TABLE skill
    ADD COLUMN IF NOT EXISTS embedding             vector(1024),
    ADD COLUMN IF NOT EXISTS embedding_source_hash VARCHAR(64),
    ADD COLUMN IF NOT EXISTS embedded_at           TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_skill_embedding_source_hash
    ON skill (embedding_source_hash)
    WHERE embedding_source_hash IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_skill_embedded_at
    ON skill (embedded_at)
    WHERE embedded_at IS NOT NULL;

ALTER TABLE chat_session
    ADD COLUMN IF NOT EXISTS mode             VARCHAR(32) NOT NULL DEFAULT 'AGENT',
    ADD COLUMN IF NOT EXISTS persona_id       UUID,
    ADD COLUMN IF NOT EXISTS sticky_skill_set JSONB       NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE chat_session
    ALTER COLUMN agent_id DROP NOT NULL;

CREATE INDEX IF NOT EXISTS idx_chat_session_mode
    ON chat_session (mode);

CREATE INDEX IF NOT EXISTS idx_chat_session_persona_id
    ON chat_session (persona_id)
    WHERE persona_id IS NOT NULL;
