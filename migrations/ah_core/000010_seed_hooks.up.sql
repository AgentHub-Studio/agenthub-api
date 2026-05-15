-- Seed platform-managed DEFAULT HOOK CONFIGURATIONS in ah_core.
-- These are baseline hooks every tenant inherits — adapted from Claude
-- Code's 27 lifecycle hooks (PDF Section 6.1) for AgentHub's web reality.
--
-- A platform-managed hook is a default behaviour the runner applies
-- automatically when no tenant-specific hook overrides it. Tenants opt-in
-- by NOT registering their own hook for the same event; opt-out by
-- registering a no-op or counter-rule.
--
-- We seed PROMPT-TYPE hooks (zero side effects, no external HTTP) so the
-- baseline is safe for every deployment. Tenants can layer HTTP hooks of
-- their own on top.
--
-- Inspired by:
--   - PDF arXiv:2604.14228v1 Section 6.1 (27 lifecycle hooks; 5 safety hooks)
--   - PDF Section 5.3 (PreToolUse / PostToolUse / PermissionRequest /
--     PermissionDenied / PostToolUseFailure are first-class safety hooks)

CREATE TABLE IF NOT EXISTS ah_core.hook (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(255) NOT NULL,
    -- slug is the stable identifier for this default hook.
    slug        VARCHAR(64)  NOT NULL UNIQUE,
    description TEXT,
    -- event is the lifecycle event the hook attaches to. Must match the
    -- ExtendedHookEvent enum in agenthub-api (PreToolUse / PostToolUse /
    -- PermissionRequest / SessionStart / etc).
    event       VARCHAR(64)  NOT NULL,
    -- hook_type controls how the runner dispatches it:
    --   "prompt" — inject prompt template into context (zero side effect)
    --   "http"   — call an HTTP endpoint (managed hooks: NEVER seeded
    --              http to avoid surprising deployments)
    hook_type   VARCHAR(32)  NOT NULL DEFAULT 'prompt',
    -- inject_text is the static text injected when hook_type='prompt'.
    -- Mirrors the {inject:"…"} shorthand from PromptHookConfig.
    inject_text TEXT,
    -- matcher narrows the hook to a subset of tools/skills (optional).
    -- Empty string = applies to ALL invocations of the event.
    matcher     VARCHAR(255) NOT NULL DEFAULT '',
    priority    INTEGER      NOT NULL DEFAULT 0,
    -- requires_admin_to_disable: tenants cannot disable safety-critical
    -- baseline hooks without admin role.
    requires_admin_to_disable BOOLEAN NOT NULL DEFAULT FALSE,
    is_active   BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_order  INTEGER      NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_hook_slug      ON ah_core.hook (slug);
CREATE INDEX IF NOT EXISTS idx_ah_core_hook_is_active ON ah_core.hook (is_active);
CREATE INDEX IF NOT EXISTS idx_ah_core_hook_event     ON ah_core.hook (event);

-- ============================
-- SAFETY HOOKS (4 — admin-only disable, PreToolUse/PostToolUseFailure/PermissionDenied)
-- ============================
INSERT INTO ah_core.hook (slug, name, description, event, hook_type, inject_text, matcher, priority, requires_admin_to_disable, sort_order) VALUES
('safety-pretooluse-confirm-destructive',
 'Pre Tool Use — Confirm Destructive',
 'Inject a reminder before destructive tool calls to prefer the safer alternative.',
 'PreToolUse', 'prompt',
 'REMEMBER: this tool may have irreversible effects. Confirm with the user explicitly before invoking destructive operations (DROP, DELETE without WHERE, force-push, file removal).',
 'execute-sql,shell,agenthub_manage', 100, TRUE, 10),

('safety-pretooluse-redact-secrets',
 'Pre Tool Use — Redact Secrets in Input',
 'Remind the agent to redact secret-like values before sending them to external tools.',
 'PreToolUse', 'prompt',
 'REMEMBER: redact API keys, passwords, tokens, and other secret-like strings before passing them as tool arguments. Treat any value matching common secret patterns as confidential.',
 '', 95, TRUE, 20),

('safety-permissiondenied-explain',
 'Permission Denied — Explain to User',
 'When a permission is denied, explain to the user what was attempted and why it was blocked.',
 'PermissionDenied', 'prompt',
 'A tool call was blocked by permission rules. Explain to the user WHAT was attempted and WHY it was blocked (cite the matched rule). Suggest a safer alternative if available.',
 '', 90, TRUE, 30),

('safety-posttooluseFailure-acknowledge',
 'Post Tool Use Failure — Acknowledge',
 'On tool failure, acknowledge the failure to the user and avoid silent retries.',
 'PostToolUseFailure', 'prompt',
 'A tool call failed. Acknowledge the failure to the user, do NOT silently retry. State the error briefly and ask the user how to proceed.',
 '', 85, TRUE, 40);

-- ============================
-- LIFECYCLE HOOKS (3)
-- ============================
INSERT INTO ah_core.hook (slug, name, description, event, hook_type, inject_text, matcher, priority, sort_order) VALUES
('lifecycle-sessionstart-greeting',
 'Session Start — Greeting Context',
 'On session start, inject a brief context note about the current agent and capabilities.',
 'SessionStart', 'prompt',
 'You are starting a new session. Begin with a brief, professional greeting and a one-sentence summary of your capabilities. Do not over-explain — get to the user goal.',
 '', 60, 100),

('lifecycle-userpromptsubmit-clarify-if-vague',
 'User Prompt Submit — Clarify If Vague',
 'When a user prompt is vague, ask one clarifying question before invoking expensive tools.',
 'UserPromptSubmit', 'prompt',
 'If the user prompt is vague (single word, ambiguous reference, missing context for a tool call), ask ONE focused clarifying question before invoking any tool that costs tokens or time.',
 '', 55, 110),

('lifecycle-stop-summarize-if-long',
 'Stop — Summarize If Long Run',
 'On run stop after many turns, offer a brief summary of what was accomplished.',
 'Stop', 'prompt',
 'If the run took multiple turns and produced multiple deliverables, end with a brief 2-3 line summary of what was accomplished and what remains open.',
 '', 50, 120);

-- ============================
-- CONTEXT HOOKS (2)
-- ============================
INSERT INTO ah_core.hook (slug, name, description, event, hook_type, inject_text, matcher, priority, sort_order) VALUES
('context-precompact-preserve-decisions',
 'Pre Compact — Preserve Decisions',
 'Before compaction, ensure key decisions and open action items are preserved in the summary.',
 'PreCompact', 'prompt',
 'Before compaction, ensure the summary preserves: (1) key decisions made, (2) open action items, (3) any errors or blockers the user is waiting on. Do not lose user-blocking context.',
 '', 70, 200),

('context-postcompact-acknowledge',
 'Post Compact — Acknowledge',
 'After compaction, briefly note that older context was summarized so the user knows what state they are in.',
 'PostCompact', 'prompt',
 'Older conversation context was just summarized to free space. The summary is in the system context — refer to it as needed. Do not announce this to the user unless they ask.',
 '', 65, 210);

-- ============================
-- COORDINATION HOOKS (2)
-- ============================
INSERT INTO ah_core.hook (slug, name, description, event, hook_type, inject_text, matcher, priority, sort_order) VALUES
('coord-subagentstop-summarize-result',
 'Subagent Stop — Summarize Result',
 'When a subagent completes, summarize its result back to the parent in 1-2 sentences.',
 'SubagentStop', 'prompt',
 'A subagent just finished. Return a 1-2 sentence summary of its result to the parent context. Do NOT include the subagent full transcript.',
 '', 45, 300),

('coord-taskcompleted-acknowledge',
 'Task Completed — Acknowledge',
 'When a task completes, acknowledge it briefly so the user knows it is done.',
 'TaskCompleted', 'prompt',
 'A task just completed. Acknowledge briefly with a single line stating what was finished. Do not list every sub-step.',
 '', 40, 310);
