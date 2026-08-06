CREATE TABLE IF NOT EXISTS workflow (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug VARCHAR(120) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    start_step_id VARCHAR(120),
    steps JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS workflow_step (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID NOT NULL REFERENCES workflow(id) ON DELETE CASCADE,
    step_id VARCHAR(120) NOT NULL,
    type VARCHAR(40) NOT NULL,
    config JSONB NOT NULL DEFAULT '{}'::jsonb,
    next_step_id VARCHAR(120),
    cases JSONB NOT NULL DEFAULT '{}'::jsonb,
    position INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (workflow_id, step_id)
);

CREATE INDEX IF NOT EXISTS idx_workflow_step_workflow_id ON workflow_step(workflow_id);

CREATE TABLE IF NOT EXISTS workflow_execution (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID NOT NULL REFERENCES workflow(id) ON DELETE CASCADE,
    workflow_slug VARCHAR(120) NOT NULL,
    state VARCHAR(40) NOT NULL,
    input JSONB NOT NULL DEFAULT '{}'::jsonb,
    current_step_id VARCHAR(120),
    suspended_at TIMESTAMPTZ,
    resumed_at TIMESTAMPTZ,
    resume_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    output JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_workflow_execution_workflow_id ON workflow_execution(workflow_id);
CREATE INDEX IF NOT EXISTS idx_workflow_execution_state ON workflow_execution(state);
