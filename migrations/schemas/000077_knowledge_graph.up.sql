ALTER TABLE knowledge_base
    ADD COLUMN IF NOT EXISTS graph_enabled BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS document_entity (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id UUID NOT NULL REFERENCES document(id) ON DELETE CASCADE,
    knowledge_base_id UUID NOT NULL REFERENCES knowledge_base(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    type TEXT NOT NULL DEFAULT 'entity',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_document_entity_document_name
    ON document_entity (document_id, lower(name));

CREATE INDEX IF NOT EXISTS idx_document_entity_kb_name
    ON document_entity (knowledge_base_id, lower(name));

CREATE TABLE IF NOT EXISTS document_entity_edge (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    knowledge_base_id UUID NOT NULL REFERENCES knowledge_base(id) ON DELETE CASCADE,
    source_entity_id UUID NOT NULL REFERENCES document_entity(id) ON DELETE CASCADE,
    target_entity_id UUID NOT NULL REFERENCES document_entity(id) ON DELETE CASCADE,
    relation TEXT NOT NULL,
    evidence TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_document_entity_edge_kb_relation
    ON document_entity_edge (knowledge_base_id, relation);

CREATE INDEX IF NOT EXISTS idx_document_entity_edge_source
    ON document_entity_edge (source_entity_id);

CREATE INDEX IF NOT EXISTS idx_document_entity_edge_target
    ON document_entity_edge (target_entity_id);
