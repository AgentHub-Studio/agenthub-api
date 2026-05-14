-- Seed platform-managed DYNAMIC SKILL HOOK DEFAULT TEMPLATES in ah_core.
-- Templates are blueprints paired with EXT-007 DynamicSkillHookRegistry.
-- Each row provides a (phase + handler_pattern + priority) preset
-- fresh extension authors copy when registering hooks for their skills.

CREATE TABLE IF NOT EXISTS ah_core.dynamic_skill_hook_default_template (
    id                          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    slug                        VARCHAR(96)  NOT NULL UNIQUE,
    name                        VARCHAR(160) NOT NULL,
    description                 TEXT         NOT NULL,
    -- target_phase MATCHES EXT-007 DynamicSkillHookPhase enum byte-for-byte.
    target_phase                VARCHAR(48)  NOT NULL,
    -- handler_pattern is a documentary template (vendor copies + customizes).
    handler_pattern             VARCHAR(160) NOT NULL,
    -- default_priority is the priority value the template suggests
    -- when a vendor instantiates (vendor can override).
    default_priority            INTEGER      NOT NULL DEFAULT 50,
    -- target_use_case classifies the hook's purpose.
    target_use_case             VARCHAR(48)  NOT NULL,
    -- recommended_for_tenant_kind hints the audience.
    recommended_for_tenant_kind VARCHAR(48)  NOT NULL,
    requires_admin_review       BOOLEAN      NOT NULL DEFAULT FALSE,
    is_recommended              BOOLEAN      NOT NULL DEFAULT FALSE,
    is_active                   BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_order                  INTEGER      NOT NULL DEFAULT 0,
    created_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_dshd_phase    ON ah_core.dynamic_skill_hook_default_template (target_phase);
CREATE INDEX IF NOT EXISTS idx_ah_core_dshd_active   ON ah_core.dynamic_skill_hook_default_template (is_active);
CREATE INDEX IF NOT EXISTS idx_ah_core_dshd_rec      ON ah_core.dynamic_skill_hook_default_template (is_recommended) WHERE is_recommended = TRUE;

-- Seed catalog: 6 hook templates covering common skill-defined patterns.
-- All 5 EXT-007 phases represented, plus an additional before_invocation
-- entry (validate-input + redact-pii both target same phase but different
-- semantics).

INSERT INTO ah_core.dynamic_skill_hook_default_template
    (slug, name, description, target_phase, handler_pattern,
     default_priority, target_use_case, recommended_for_tenant_kind,
     requires_admin_review, is_recommended, sort_order)
VALUES
    ('validate-input',
     'Validate Input',
     'Before-invocation hook that validates skill arguments against a JSON schema. Aborts the skill if validation fails — catches caller bugs before tool calls fire.',
     'before_invocation', 'hooks/validators/validate-input.yaml',
     90, 'input_validation', 'general', FALSE, TRUE, 10),

    ('redact-pii',
     'Redact PII Before Invocation',
     'Before-invocation hook that scrubs PII (emails, phone numbers, SSN) from skill arguments before they hit any tool. RECOMMENDED for regulated tenants. Requires admin review (privacy posture change).',
     'before_invocation', 'hooks/privacy/redact-pii.yaml',
     95, 'privacy_compliance', 'regulated', TRUE, TRUE, 20),

    ('audit-tool-call',
     'Audit Tool Call',
     'Before-tool-call hook that emits a GOV-001 audit event for every tool invocation within the skill. Creates per-skill audit trail without polluting global hook registry.',
     'before_tool_call', 'hooks/audit/audit-tool-call.yaml',
     80, 'audit_trail', 'general', FALSE, TRUE, 30),

    ('cost-track',
     'Cost Track After Tool Call',
     'After-tool-call hook that records token usage + dollar cost per tool invocation. Feeds cost dashboards (mirror of CTX-008 ToolResultBudget signals).',
     'after_tool_call', 'hooks/cost/track-tokens.yaml',
     60, 'cost_analytics', 'general', FALSE, TRUE, 40),

    ('sanitize-output',
     'Sanitize Output After Invocation',
     'After-invocation hook that strips internal IDs / debug data / vendor-specific fields from skill output before user sees it. Production polish for end-user-facing skills.',
     'after_invocation', 'hooks/sanitizers/sanitize-output.yaml',
     70, 'output_sanitization', 'general', FALSE, TRUE, 50),

    ('error-recovery',
     'Error Recovery on Skill Failure',
     'On-error hook that captures skill failure context (last tool call, args, error message) into a recovery store. Drives the "retry with adjustments" flow when admin intervenes.',
     'on_error', 'hooks/recovery/capture-context.yaml',
     85, 'error_recovery', 'general', FALSE, TRUE, 60)
ON CONFLICT (slug) DO NOTHING;
