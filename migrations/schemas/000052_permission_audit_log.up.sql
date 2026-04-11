-- Creates the permission_audit_log table for recording per-tool permission decisions.
-- Each row captures one EvaluatePermission call during an agentic run.
CREATE TABLE IF NOT EXISTS permission_audit_log (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id  UUID         NOT NULL,
    run_id      UUID,
    tool_name   VARCHAR(255) NOT NULL,
    -- decision: 'allow', 'deny', 'confirm_approved', 'confirm_denied', 'confirm_escalated'
    decision    VARCHAR(50)  NOT NULL,
    matched_rule TEXT,           -- the rule pattern that triggered this decision (nullable)
    input_snippet TEXT,          -- truncated tool input (first 300 chars)
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_perm_audit_session ON permission_audit_log (session_id);
CREATE INDEX IF NOT EXISTS idx_perm_audit_run     ON permission_audit_log (run_id) WHERE run_id IS NOT NULL;
