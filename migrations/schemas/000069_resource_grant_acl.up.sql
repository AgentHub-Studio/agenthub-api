CREATE TABLE IF NOT EXISTS resource_grant (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_type  VARCHAR(32) NOT NULL,
    subject_id    TEXT        NOT NULL,
    resource_type TEXT        NOT NULL,
    resource_id   TEXT        NOT NULL,
    actions       TEXT[]      NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_resource_grant_subject_resource UNIQUE (subject_type, subject_id, resource_type, resource_id)
);

CREATE INDEX IF NOT EXISTS idx_resource_grant_resource
    ON resource_grant (resource_type, resource_id);

CREATE INDEX IF NOT EXISTS idx_resource_grant_subject
    ON resource_grant (subject_type, subject_id);
