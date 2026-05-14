CREATE TABLE IF NOT EXISTS ah_core.query_pipeline_step_template (
    id                  UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    slug                VARCHAR(64)  NOT NULL UNIQUE,
    label               VARCHAR(128) NOT NULL,
    description         TEXT         NOT NULL,
    step_order          INTEGER      NOT NULL CHECK (step_order >= 1 AND step_order <= 9),
    phase               VARCHAR(32)  NOT NULL,
    can_block           BOOLEAN      NOT NULL DEFAULT false,
    is_retryable        BOOLEAN      NOT NULL DEFAULT false,
    is_per_iteration    BOOLEAN      NOT NULL DEFAULT true,
    sort_order          INTEGER      NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- §4.1: nine fixed steps in the agentic query loop (Figure 2, query.ts).
-- Phases: setup (1-2) | context (3-4) | reasoning (5) | execution (6-8) | termination (9)
-- can_block: step can halt the pipeline (permission denied, context overflow, etc.)
-- is_retryable: recovery mechanism may re-attempt this step (§4.4)
-- is_per_iteration: true = runs every loop iteration; false = one-time turn setup
INSERT INTO ah_core.query_pipeline_step_template
    (slug, label, description, step_order, phase, can_block, is_retryable, is_per_iteration, sort_order)
VALUES
    ('settings_resolution',
     'Settings Resolution',
     'Destructures immutable parameters for the turn: system prompt, active permission set, model configuration, and tool definitions.',
     1, 'setup', false, false, false, 0),

    ('mutable_state_init',
     'Mutable State Init',
     'Creates a single State object that holds the message history, tool execution context, and recovery counters for the current turn.',
     2, 'setup', false, false, false, 1),

    ('context_assembly',
     'Context Assembly',
     'Retrieves messages via getMessagesAfterCompactBoundary(), assembling the conversation window that will be sent to the model.',
     3, 'context', false, false, true, 2),

    ('pre_model_shapers',
     'Pre-Model Shapers',
     'Executes the five sequential context shapers in order: budget, snip, micro, collapse, and auto. May block if context cannot be reduced below the model limit.',
     4, 'context', true, false, true, 3),

    ('model_call',
     'Model Call',
     'Streams the model response via deps.callModel() with the fully assembled context. Blocking and retryable; handles transient API errors and rate limits.',
     5, 'reasoning', true, true, true, 4),

    ('tool_use_dispatch',
     'Tool-Use Dispatch',
     'Routes tool_use content blocks from the model response to the tool orchestration layer (§4.2). Non-blocking; always proceeds when tool use is present.',
     6, 'execution', false, false, true, 5),

    ('permission_gate',
     'Permission Gate',
     'Evaluates each pending tool request through the seven-layer permission system (§5). Blocks the pipeline if any tool is denied by the active permission mode.',
     7, 'execution', true, false, true, 6),

    ('tool_execution',
     'Tool Execution',
     'Executes all approved tools and appends tool_result messages to the conversation. Retryable for transient skill errors; loop continues after results are added.',
     8, 'execution', false, true, true, 7),

    ('stop_condition',
     'Stop Condition',
     'Checks all termination conditions: no tool use in model response, maxTurns reached, context overflow, hook-triggered abort, or user abort signal.',
     9, 'termination', true, false, true, 8)
ON CONFLICT (slug) DO NOTHING;
