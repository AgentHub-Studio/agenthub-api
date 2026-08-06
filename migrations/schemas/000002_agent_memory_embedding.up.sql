-- Add pgvector embedding and last_accessed_at to agent_memory for semantic recall (issue #43).
-- Requires pgvector extension to be loaded on the database (included in the initial schema via
-- public schema bootstrap, so it is available in tenant schemas automatically).

ALTER TABLE agent_memory
    ADD COLUMN IF NOT EXISTS embedding      public.vector(1024),
    ADD COLUMN IF NOT EXISTS last_accessed_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- IVFFlat index for approximate nearest-neighbour search via cosine distance (<=>).
-- lists=100 is a reasonable starting point for tables up to ~1M rows.
CREATE INDEX IF NOT EXISTS idx_agent_memory_embedding
    ON agent_memory USING ivfflat (embedding public.vector_cosine_ops)
    WITH (lists = 100)
    WHERE embedding IS NOT NULL;
