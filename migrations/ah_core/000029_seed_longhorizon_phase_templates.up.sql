-- Seed platform-managed LONG-HORIZON PHASE TEMPLATES in ah_core.
-- Templates are blueprints paired with FUTURE-004 LongHorizonTask
-- (multi-week umbrella tasks composed of dependent phases). Tenants
-- instantiate these to spin up multi-week initiatives without inventing
-- phase decomposition + dependency graphs.

CREATE TABLE IF NOT EXISTS ah_core.longhorizon_phase_template (
    id                          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    -- task_template_slug groups phases that belong to the SAME multi-week
    -- task template. (template, phase_slug) is unique.
    task_template_slug          VARCHAR(96)  NOT NULL,
    -- phase_slug is unique within a task template.
    phase_slug                  VARCHAR(96)  NOT NULL,
    name                        VARCHAR(160) NOT NULL,
    description                 TEXT         NOT NULL,
    -- depends_on is comma-separated list of phase_slugs in the SAME
    -- task template that must be completed before this phase starts.
    -- Empty string = no dependencies (entry phase).
    depends_on                  TEXT         NOT NULL DEFAULT '',
    -- estimated_days is the typical duration; helps tenants plan deadlines.
    estimated_days              INTEGER      NOT NULL,
    -- expected_deliverables is human-readable list of phase outputs.
    expected_deliverables       TEXT         NOT NULL,
    -- requires_admin_review marks phases gated on admin sign-off
    -- (e.g. compliance phase, regulatory phase, decision-go-live phase).
    requires_admin_review       BOOLEAN      NOT NULL DEFAULT FALSE,
    -- sort_order is used to render the phases in canonical UI order.
    sort_order                  INTEGER      NOT NULL DEFAULT 0,
    is_active                   BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (task_template_slug, phase_slug)
);

CREATE INDEX IF NOT EXISTS idx_ah_core_lhph_template ON ah_core.longhorizon_phase_template (task_template_slug);
CREATE INDEX IF NOT EXISTS idx_ah_core_lhph_active   ON ah_core.longhorizon_phase_template (is_active);

-- Seed catalog: 8 phases distributed across 3 multi-week task templates
-- (typical AgentHub tenant initiatives). Each task template has 2-3
-- phases with explicit dependency graph between them.
--
-- Task templates:
--   1. customer-30day-monitoring (3 phases): baseline → daily-checkin → final-report
--   2. quarterly-product-research (3 phases): scoping → drafting → review
--   3. compliance-annual-recert  (2 phases): evidence-collection → admin-attestation

INSERT INTO ah_core.longhorizon_phase_template
    (task_template_slug, phase_slug, name, description, depends_on, estimated_days, expected_deliverables, requires_admin_review, sort_order)
VALUES
    -- customer-30day-monitoring (3 phases).
    ('customer-30day-monitoring', 'baseline-snapshot',
     'Phase 1: Baseline Snapshot',
     'Capture initial state of customer interaction patterns, sentiment, and key metrics on day 1. Provides reference values for the 30-day delta calculation.',
     '', 1, 'baseline_metrics.json + interaction_summary.md', FALSE, 10),

    ('customer-30day-monitoring', 'daily-checkin-loop',
     'Phase 2: Daily Check-in Loop (28 days)',
     'Run a daily agent check-in for 28 days; surface any anomalies (sentiment drop, support ticket spike, escalation events). Depends on baseline being captured first.',
     'baseline-snapshot', 28, 'daily_logs/*.json + anomaly_alerts.json', FALSE, 20),

    ('customer-30day-monitoring', 'final-report',
     'Phase 3: Final 30-Day Report',
     'Aggregate the 28 daily check-ins into a final report with trend analysis, recommendations, and admin sign-off on continuing/concluding the engagement. Depends on daily loop completing.',
     'daily-checkin-loop', 1, 'final_report.pdf + recommendations.md', TRUE, 30),

    -- quarterly-product-research (3 phases).
    ('quarterly-product-research', 'scoping-and-questions',
     'Phase 1: Scoping & Research Questions',
     'Define research questions, target market segment, success metrics. Output: scoped research brief approved by stakeholder.',
     '', 3, 'research_brief.md + question_list.md', FALSE, 40),

    ('quarterly-product-research', 'drafting-and-evidence',
     'Phase 2: Drafting & Evidence Collection (~2 weeks)',
     'Conduct research, collect evidence (KB queries, web fetches, customer interviews), produce iterative drafts every 2 days. Depends on scoping.',
     'scoping-and-questions', 14, 'evidence_log.md + draft_v[1-7].md', FALSE, 50),

    ('quarterly-product-research', 'final-review-and-publish',
     'Phase 3: Final Review & Publish',
     'Stakeholder review, revisions, final publication. Depends on drafting.',
     'drafting-and-evidence', 5, 'final_report.pdf + executive_summary.md', TRUE, 60),

    -- compliance-annual-recert (2 phases).
    ('compliance-annual-recert', 'evidence-collection',
     'Phase 1: Evidence Collection',
     'Collect annual compliance evidence (audit logs, access reports, exception register). Output: evidence package per compliance_evidence_kit blueprint.',
     '', 14, 'evidence_package.zip + checklist_v1.json', FALSE, 70),

    ('compliance-annual-recert', 'admin-attestation',
     'Phase 2: Admin Attestation & Filing',
     'Admin reviews collected evidence, signs attestation, files with compliance authority. Requires admin signature (legal binding).',
     'evidence-collection', 5, 'attestation_signed.pdf + filing_receipt.pdf', TRUE, 80)
ON CONFLICT (task_template_slug, phase_slug) DO NOTHING;
