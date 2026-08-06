ALTER TABLE document
    ADD COLUMN IF NOT EXISTS metadata JSONB NOT NULL DEFAULT '{}';

CREATE INDEX IF NOT EXISTS idx_document_metadata_gin
    ON document USING GIN (metadata);
