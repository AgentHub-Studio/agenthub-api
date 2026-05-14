-- Seed capability-specific webhook notification templates in ah_core
-- (migration 000097). These 3 templates are outbound webhook notification
-- definitions designed for the three capability agents introduced in
-- migration 000091 (core-researcher, core-analyst, core-planner).
--
-- Each template describes a notification event that a capability agent can
-- fire, including the expected payload schema and the event type key:
--
--   capability-research-complete — fires when core-researcher finishes a task
--   capability-analysis-done     — fires when core-analyst completes analysis
--   capability-tasks-updated     — fires when core-planner updates task list
--
-- All 3:
--   - are is_recommended = TRUE (surfaces in the UI notification picker)
--   - carry a JSONB payload_schema describing the fields consumers may receive
--   - have sort_order ≥ 100 (capability-layer range, above any future platform rows)
--
-- The webhook_notification_template table does not exist before this migration;
-- it is created here in ah_core (schema created by migration 000001).
-- This table is distinct from ah_core.webhook_endpoint_template (migration 000021)
-- which describes outbound endpoint destinations. This table describes the
-- notification events themselves — what fires, not where it goes.

CREATE TABLE IF NOT EXISTS ah_core.webhook_notification_template (
    id             UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    slug           VARCHAR(64)  NOT NULL UNIQUE,
    name           VARCHAR(255) NOT NULL,
    description    TEXT         NOT NULL,
    -- event_type is the machine-readable event key consumers subscribe to.
    -- Convention: snake_case, e.g. research_complete, analysis_done.
    event_type     VARCHAR(64)  NOT NULL UNIQUE,
    -- payload_schema is the JSONB schema of the notification payload.
    -- Follows JSON Schema draft-07 conventions for field documentation.
    payload_schema JSONB        NOT NULL DEFAULT '{}',
    is_recommended BOOLEAN      NOT NULL DEFAULT FALSE,
    is_active      BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_order     INTEGER      NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_webhook_notif_tmpl_slug       ON ah_core.webhook_notification_template (slug);
CREATE INDEX IF NOT EXISTS idx_ah_core_webhook_notif_tmpl_event_type ON ah_core.webhook_notification_template (event_type);
CREATE INDEX IF NOT EXISTS idx_ah_core_webhook_notif_tmpl_active     ON ah_core.webhook_notification_template (is_active);

-- ============================
-- 3 capability webhook notification templates (one per capability agent)
-- ============================
INSERT INTO ah_core.webhook_notification_template
    (slug, name, description, event_type, payload_schema,
     is_recommended, sort_order)
VALUES

-- 1. Research Complete — fires when core-researcher finishes a research task.
('capability-research-complete',
 'Research Complete',
 'Fired when the Web Researcher capability agent completes a research task. Carries a summary of findings, the number of sources consulted, and the knowledge base that was populated.',
 'research_complete',
 '{
   "$schema": "http://json-schema.org/draft-07/schema#",
   "type": "object",
   "required": ["research_summary", "sources_count", "knowledge_base_id"],
   "properties": {
     "research_summary": {
       "type": "string",
       "description": "Brief narrative summary of the research findings (max 500 chars)."
     },
     "sources_count": {
       "type": "integer",
       "minimum": 0,
       "description": "Number of distinct sources referenced in the research output."
     },
     "knowledge_base_id": {
       "type": "string",
       "format": "uuid",
       "description": "UUID of the knowledge base populated with research findings."
     }
   }
 }',
 TRUE, 100),

-- 2. Analysis Done — fires when core-analyst completes document analysis.
('capability-analysis-done',
 'Analysis Done',
 'Fired when the Document Analyst capability agent completes its analysis. Carries a brief of the analysis, a confidence score (0.0–1.0), and the count of documents analyzed.',
 'analysis_done',
 '{
   "$schema": "http://json-schema.org/draft-07/schema#",
   "type": "object",
   "required": ["analysis_brief", "confidence_score", "doc_count"],
   "properties": {
     "analysis_brief": {
       "type": "string",
       "description": "Short executive summary of the analysis outcome (max 300 chars)."
     },
     "confidence_score": {
       "type": "number",
       "minimum": 0.0,
       "maximum": 1.0,
       "description": "Model confidence in the analysis result, from 0.0 (low) to 1.0 (high)."
     },
     "doc_count": {
       "type": "integer",
       "minimum": 0,
       "description": "Number of documents analyzed in this run."
     }
   }
 }',
 TRUE, 101),

-- 3. Tasks Updated — fires when core-planner updates the task list.
('capability-tasks-updated',
 'Tasks Updated',
 'Fired when the Task Planner capability agent modifies the task list (adds, completes, or removes tasks). Carries the count of tasks added, tasks completed, and currently pending tasks.',
 'tasks_updated',
 '{
   "$schema": "http://json-schema.org/draft-07/schema#",
   "type": "object",
   "required": ["tasks_added", "tasks_completed", "pending_count"],
   "properties": {
     "tasks_added": {
       "type": "integer",
       "minimum": 0,
       "description": "Number of new tasks added in this update."
     },
     "tasks_completed": {
       "type": "integer",
       "minimum": 0,
       "description": "Number of tasks marked complete in this update."
     },
     "pending_count": {
       "type": "integer",
       "minimum": 0,
       "description": "Total number of pending (incomplete) tasks after this update."
     }
   }
 }',
 TRUE, 102)

ON CONFLICT (slug) DO NOTHING;
