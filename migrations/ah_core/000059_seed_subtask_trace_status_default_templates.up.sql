-- OBS-006-paired: subtask trace status default templates.
-- 3 templates 1:1 with SubtaskStatus enum so fresh tenants have
-- proven trace shapes matching parent/child correlation envelope.

CREATE TABLE IF NOT EXISTS ah_core.subtask_trace_status_default_template (
    id                              UUID PRIMARY KEY,
    slug                            TEXT NOT NULL UNIQUE,
    name                            TEXT NOT NULL,
    description                     TEXT NOT NULL,
    target_status                   TEXT NOT NULL,
    target_use_case                 TEXT NOT NULL,
    expected_event_sequence         JSONB NOT NULL DEFAULT '[]'::jsonb,
    propagate_cost_to_parent        BOOLEAN NOT NULL DEFAULT FALSE,
    emit_error_envelope             BOOLEAN NOT NULL DEFAULT FALSE,
    typical_min_turns               INTEGER NOT NULL DEFAULT 0,
    recommended_for_tenant_kind     TEXT NOT NULL DEFAULT 'general',
    requires_admin_review           BOOLEAN NOT NULL DEFAULT FALSE,
    is_recommended                  BOOLEAN NOT NULL DEFAULT FALSE,
    is_active                       BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order                      INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_subtask_trace_dt_status
    ON ah_core.subtask_trace_status_default_template(target_status);
CREATE INDEX IF NOT EXISTS idx_subtask_trace_dt_use_case
    ON ah_core.subtask_trace_status_default_template(target_use_case);
CREATE INDEX IF NOT EXISTS idx_subtask_trace_dt_active
    ON ah_core.subtask_trace_status_default_template(is_active);
CREATE INDEX IF NOT EXISTS idx_subtask_trace_dt_recommended
    ON ah_core.subtask_trace_status_default_template(is_recommended)
    WHERE is_recommended = TRUE;

INSERT INTO ah_core.subtask_trace_status_default_template
    (id, slug, name, description, target_status, target_use_case,
     expected_event_sequence, propagate_cost_to_parent,
     emit_error_envelope, typical_min_turns,
     recommended_for_tenant_kind, requires_admin_review,
     is_recommended, is_active, sort_order)
VALUES
    ('00000001-0000-0000-0000-000000000001',
     'completed-routine-trace',
     'Completed — Routine Trace',
     'Routine successful subagent: emits subtask_start, then 1+ tool calls and text deltas, then subtask_complete with totalTurns + totalTokens + totalCost + summary populated. No error envelope. Used for normal task completion across all use cases.',
     'completed', 'routine_completion',
     '["subtask_start","text_delta","tool_call_start","tool_result","subtask_complete"]'::jsonb,
     TRUE, FALSE, 1,
     'general', FALSE, TRUE, TRUE, 10),

    ('00000001-0000-0000-0000-000000000002',
     'failed-error-context-trace',
     'Failed — Error Context Trace',
     'Subagent could not complete: emits subtask_start, partial events, ErrorData event with Message, then subtask_complete carrying the Error pointer. Status maps to failed; parent diagnoses without reading transcript. Admin review since failures may signal misconfiguration.',
     'failed', 'error_diagnosis',
     '["subtask_start","text_delta","error","subtask_complete"]'::jsonb,
     TRUE, TRUE, 1,
     'general', TRUE, TRUE, TRUE, 20),

    ('00000001-0000-0000-0000-000000000003',
     'killed-budget-or-depth-trace',
     'Killed — Budget/Depth Trace',
     'Subagent killed by harness due to depth limit, budget exhaustion, or explicit abort. Emits subtask_start then subtask_complete with Error pointer carrying the kill reason. typical_min_turns=0 since the kill can happen before the first LLM turn (depth check). Admin review.',
     'killed', 'harness_kill',
     '["subtask_start","subtask_complete"]'::jsonb,
     FALSE, TRUE, 0,
     'general', TRUE, TRUE, TRUE, 30)
ON CONFLICT (slug) DO NOTHING;
