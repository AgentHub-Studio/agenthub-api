-- Seed platform-managed DEFAULT WORKFLOW TEMPLATES in ah_core.
-- Templates are pre-configured agent + skill + KB bundles that
-- accomplish common end-to-end workflows (e.g. invoice processing,
-- PR review, customer onboarding).
--
-- Inspired by:
--   - PDF arXiv:2604.14228v1 §6.1 (workflow composition from primitives)
--   - PDF §11 (workflows are user-facing units of value)

CREATE TABLE IF NOT EXISTS ah_core.workflow_template (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    slug            VARCHAR(64)  NOT NULL UNIQUE,
    display_name    VARCHAR(255) NOT NULL,
    description     TEXT         NOT NULL,
    -- workflow_kind classifies the use case.
    workflow_kind   VARCHAR(32)  NOT NULL,
    -- target_agent_slug references the agent persona that drives this workflow.
    -- App-level FK to ah_core.agent.slug or ah_core.core_prompt_template.slug.
    target_agent_slug VARCHAR(64) NOT NULL,
    -- requires_skills is the comma-separated list of skill slugs required.
    requires_skills VARCHAR(512) NOT NULL DEFAULT '',
    -- requires_kb_kind is the optional KB template kind needed.
    requires_kb_kind VARCHAR(32) NOT NULL DEFAULT '',
    -- estimated_steps is the typical number of agentic loop iterations.
    estimated_steps INTEGER      NOT NULL DEFAULT 5,
    -- estimated_cost_usd is the typical cost per workflow execution.
    estimated_cost_usd NUMERIC(10,4) NOT NULL DEFAULT 0,
    -- requires_human_checkpoint: workflow includes a HUMAN-004
    -- understanding checkpoint before expensive operations.
    requires_human_checkpoint BOOLEAN NOT NULL DEFAULT FALSE,
    is_recommended  BOOLEAN      NOT NULL DEFAULT FALSE,
    is_active       BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_order      INTEGER      NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_workflow_template_slug      ON ah_core.workflow_template (slug);
CREATE INDEX IF NOT EXISTS idx_ah_core_workflow_template_kind      ON ah_core.workflow_template (workflow_kind);
CREATE INDEX IF NOT EXISTS idx_ah_core_workflow_template_is_active ON ah_core.workflow_template (is_active);

-- ============================
-- 8 workflows covering common end-to-end automations
-- ============================
INSERT INTO ah_core.workflow_template
    (slug, display_name, description, workflow_kind, target_agent_slug,
     requires_skills, requires_kb_kind,
     estimated_steps, estimated_cost_usd,
     requires_human_checkpoint, is_recommended, sort_order) VALUES

('faq-answer',
 'FAQ Answer',
 'User asks a question; agent searches FAQ KB and returns the answer with source links.',
 'qa', 'general-assistant',
 'knowledge_base_search',
 'faq',
 3, 0.01,
 FALSE, TRUE, 10),

('document-summary',
 'Document Summary',
 'User uploads a document; agent extracts key points + decisions + action items.',
 'extraction', 'data-extractor',
 'document_extract',
 '',
 4, 0.05,
 FALSE, TRUE, 20),

('code-review',
 'Code Review',
 'PR is opened; agent runs HUMAN-001 ReviewGuidance + HUMAN-002 ExplainedDiff. Human approves before merge.',
 'review', 'code-reviewer',
 'github_diff_fetch,run_linter',
 '',
 6, 0.12,
 TRUE, TRUE, 30),

('customer-onboarding',
 'Customer Onboarding',
 'New customer signs up; agent walks them through setup, creates initial config, schedules check-ins.',
 'onboarding', 'customer-support',
 'send_email,create_calendar_event,knowledge_base_search',
 'documentation',
 8, 0.15,
 FALSE, FALSE, 40),

('invoice-processing',
 'Invoice Processing',
 'Invoice arrives via email; agent extracts data, validates against PO, routes to approval queue.',
 'extraction', 'data-extractor',
 'parse_email_attachment,document_extract,create_ticket',
 '',
 7, 0.20,
 TRUE, FALSE, 50),

('research-brief',
 'Research Brief',
 'User asks for a research brief on a topic; agent searches web + internal KB, drafts brief with citations.',
 'research', 'researcher',
 'web_search,knowledge_base_search',
 'documentation',
 10, 0.30,
 FALSE, TRUE, 60),

('incident-triage',
 'Incident Triage',
 'Alert fires; agent classifies severity, identifies likely cause from KB, drafts incident summary.',
 'monitoring', 'general-assistant',
 'knowledge_base_search,query_metrics,create_incident',
 'documentation',
 6, 0.10,
 TRUE, FALSE, 70),

('compliance-export',
 'Compliance Export',
 'Auditor requests data export; agent runs governance report (GOV-005), packages CSV/JSON, requires admin approval.',
 'compliance', 'data-analyst',
 'execute_sql,export_data',
 '',
 5, 0.08,
 TRUE, FALSE, 80);
