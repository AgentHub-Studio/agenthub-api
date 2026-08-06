ALTER TABLE knowledge_base
    ADD COLUMN IF NOT EXISTS rerank_strategy VARCHAR(50) NOT NULL DEFAULT 'none';
