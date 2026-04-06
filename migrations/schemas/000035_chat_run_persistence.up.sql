-- chat_run table tracks background agentic runs.
CREATE TABLE IF NOT EXISTS chat_run (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES chat_session(id) ON DELETE CASCADE,
    tenant_id VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'active', -- 'active', 'completed', 'failed', 'cancelled'
    last_event_id VARCHAR(255),
    metadata JSONB, -- store things like model config, current depth, etc.
    started_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_chat_run_session_status ON chat_run(session_id, status);
CREATE INDEX IF NOT EXISTS idx_chat_run_tenant ON chat_run(tenant_id);

-- Add run_id to chat_message to associate assistant messages with specific runs.
ALTER TABLE chat_message ADD COLUMN IF NOT EXISTS run_id UUID REFERENCES chat_run(id) ON DELETE SET NULL;
