ALTER TABLE knowledge_base
    DROP COLUMN IF EXISTS embedding_model,
    DROP COLUMN IF EXISTS search_mode,
    DROP COLUMN IF EXISTS context_window;
