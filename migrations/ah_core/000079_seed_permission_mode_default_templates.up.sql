CREATE TABLE IF NOT EXISTS ah_core.permission_mode_template (
    id                    UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    slug                  VARCHAR(64) NOT NULL UNIQUE,
    label                 VARCHAR(128) NOT NULL,
    description           TEXT        NOT NULL,
    gradient_index        INTEGER     NOT NULL,
    safety_score          INTEGER     NOT NULL CHECK (safety_score >= 0 AND safety_score <= 100),
    requires_confirmation BOOLEAN     NOT NULL DEFAULT true,
    auto_accepts_edits    BOOLEAN     NOT NULL DEFAULT false,
    allows_background_run BOOLEAN     NOT NULL DEFAULT false,
    allows_tool_bypass    BOOLEAN     NOT NULL DEFAULT false,
    recommended_for       JSONB       NOT NULL DEFAULT '[]',
    sort_order            INTEGER     NOT NULL DEFAULT 0,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- §11.3: five-mode gradient from safest (plan, index 0) to most autonomous (bypassPermissions, index 4).
-- Safety scores are strictly decreasing: 100 → 80 → 60 → 40 → 0.
INSERT INTO ah_core.permission_mode_template
    (slug, label, description, gradient_index, safety_score,
     requires_confirmation, auto_accepts_edits, allows_background_run, allows_tool_bypass,
     recommended_for, sort_order)
VALUES
    ('plan',
     'Plan',
     'User approves all actions before execution. Maximum human oversight. Mutating tools are recorded as planned rather than executed; the user batch-approves or rejects.',
     0, 100, true, false, false, false,
     '["regulated environments","GDPR/HIPAA/SOX compliance","onboarding","high-risk tasks","governed tier agents"]',
     0),

    ('default',
     'Default',
     'Deny-first per-action rule evaluation. User approves each potentially dangerous operation. Safe default for most interactive agents.',
     1, 80, true, false, false, false,
     '["general-purpose agents","interactive workflows","standard tier agents","customer-facing assistants"]',
     1),

    ('acceptEdits',
     'Accept Edits',
     'File edits auto-accepted; shell commands and destructive operations still prompt. Balanced posture for trusted development sessions.',
     2, 60, false, true, false, false,
     '["trusted development sessions","frequent editing workflows","code-generation agents","background tier agents"]',
     2),

    ('auto',
     'Auto',
     'ML classifier evaluates safety; most actions proceed automatically. Background-capable. Compatible with KAIROS heartbeat scheduling.',
     3, 40, false, true, true, false,
     '["autonomous workflows","KAIROS heartbeat agents","background monitoring","autonomous tier agents"]',
     3),

    ('bypassPermissions',
     'Bypass Permissions',
     'Skips most permission prompts. Safety-critical checks and deny rules still apply. Reserved for trusted automation pipelines and service agents with well-defined scope.',
     4, 0, false, true, true, true,
     '["trusted automation","CI/CD pipelines","service agents","autonomous tier agents with scoped authority"]',
     4)
ON CONFLICT (slug) DO NOTHING;
