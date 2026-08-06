DROP INDEX IF EXISTS idx_document_metadata_gin;

ALTER TABLE document
    DROP COLUMN IF EXISTS metadata;
