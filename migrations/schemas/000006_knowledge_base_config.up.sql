ALTER TABLE knowledge_base
    ADD COLUMN IF NOT EXISTS embedding_model VARCHAR(255) NOT NULL DEFAULT 'intfloat/multilingual-e5-large',
    ADD COLUMN IF NOT EXISTS search_mode     VARCHAR(20)  NOT NULL DEFAULT 'HYBRID',
    ADD COLUMN IF NOT EXISTS context_window  INTEGER      NOT NULL DEFAULT 0;
