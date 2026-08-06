DROP TABLE IF EXISTS document_entity_edge;
DROP TABLE IF EXISTS document_entity;

ALTER TABLE knowledge_base
    DROP COLUMN IF EXISTS graph_enabled;
