-- Seed platform-managed CAPABILITY ASSESSMENT TEMPLATES in ah_core.
-- Templates are blueprints for measuring human capability over time
-- (paired with FUTURE-006 architecture: track if platform helps users
-- grow vs creates dependence). Tenants instantiate these to record
-- HumanCapabilitySnapshot rows without inventing definitions.

CREATE TABLE IF NOT EXISTS ah_core.capability_assessment_template (
    id                       UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    -- slug is dotted-path namespace (per-dimension, holistic, cadence-driven).
    slug                     VARCHAR(96)  NOT NULL UNIQUE,
    name                     VARCHAR(160) NOT NULL,
    description              TEXT         NOT NULL,
    -- target_dimension is the FUTURE-006 CapabilityDimension covered
    -- ("multi" when template covers all 5 dimensions in one snapshot).
    target_dimension         VARCHAR(48)  NOT NULL,
    -- cadence is the recommended measurement interval.
    cadence                  VARCHAR(32)  NOT NULL,
    -- recommended_period_days is the snapshot reporting period.
    recommended_period_days  INTEGER      NOT NULL DEFAULT 30,
    -- min_sample_size guards against noisy snapshots (≥ this many obs).
    min_sample_size          INTEGER      NOT NULL DEFAULT 10,
    -- delta_alert_threshold is absolute trend delta that triggers an
    -- admin alert (matches classifyTrend ±0.05 in FUTURE-006 by default).
    delta_alert_threshold    DOUBLE PRECISION NOT NULL DEFAULT 0.05,
    -- evaluator_kind is how this assessment is computed.
    evaluator_kind           VARCHAR(48)  NOT NULL,
    -- requires_admin_review is true when evidence must be admin-vetted
    -- before becoming part of the user's capability record.
    requires_admin_review    BOOLEAN      NOT NULL DEFAULT FALSE,
    -- is_recommended marks safe one-click defaults for fresh tenants.
    is_recommended           BOOLEAN      NOT NULL DEFAULT FALSE,
    is_active                BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_order               INTEGER      NOT NULL DEFAULT 0,
    created_at               TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_capability_template_dim     ON ah_core.capability_assessment_template (target_dimension);
CREATE INDEX IF NOT EXISTS idx_ah_core_capability_template_cadence ON ah_core.capability_assessment_template (cadence);
CREATE INDEX IF NOT EXISTS idx_ah_core_capability_template_active  ON ah_core.capability_assessment_template (is_active);
CREATE INDEX IF NOT EXISTS idx_ah_core_capability_template_rec     ON ah_core.capability_assessment_template (is_recommended) WHERE is_recommended = TRUE;

-- Seed catalog: one template per FUTURE-006 dimension (5) + 1 holistic
-- + 1 onboarding baseline + 1 incident-driven post-mortem = 8 rows.

INSERT INTO ah_core.capability_assessment_template
    (slug, name, description, target_dimension, cadence, recommended_period_days, min_sample_size, delta_alert_threshold, evaluator_kind, requires_admin_review, is_recommended, sort_order)
VALUES
    -- Per-dimension assessments (cover the 5 FUTURE-006 CapabilityDimensions).
    ('domain-knowledge-monthly',
     'Domain Knowledge — Monthly Snapshot',
     'Monthly measurement of subject-matter understanding via question-answer accuracy and unprompted explanation depth across the user''s active workspaces.',
     'domain_knowledge', 'monthly', 30, 20, 0.05, 'rubric_scored', FALSE, TRUE, 10),

    ('decision-independence-monthly',
     'Decision Independence — Monthly Snapshot',
     'Monthly measurement of fraction of decisions made without delegating to the agent. A degrading trend signals growing dependence and warrants admin review.',
     'decision_independence', 'monthly', 30, 30, 0.05, 'behavior_log', FALSE, TRUE, 20),

    ('task-throughput-weekly',
     'Task Throughput — Weekly Snapshot',
     'Weekly measurement of task volume per active hour and time-to-completion. Captures fast feedback for productivity changes.',
     'task_throughput', 'weekly', 7, 5, 0.10, 'metric_aggregate', FALSE, TRUE, 30),

    ('quality-output-monthly',
     'Quality Output — Monthly Snapshot',
     'Monthly measurement of error rate, rework count, and downstream review pass-rate on artifacts produced by the user.',
     'quality_output', 'monthly', 30, 15, 0.05, 'metric_aggregate', FALSE, TRUE, 40),

    ('collaboration-quarterly',
     'Collaboration — Quarterly Snapshot',
     'Quarterly measurement of peer feedback, hand-off success rate, and joint-task throughput. Rare cadence reflects slow feedback loop of social signals.',
     'collaboration', 'quarterly', 90, 5, 0.05, 'peer_review', TRUE, FALSE, 50),

    -- Multi-dimensional aggregate.
    ('holistic-capability-quarterly',
     'Holistic Capability — Quarterly Review',
     'Quarterly multi-dimensional snapshot covering all 5 FUTURE-006 capability dimensions in one assessment. Used for promotion / role-change decisions.',
     'multi', 'quarterly', 90, 50, 0.05, 'rubric_scored', TRUE, TRUE, 60),

    -- Cadence-driven kits (paired with onboarding + incident workflows).
    ('onboarding-baseline',
     'Onboarding Baseline Capability',
     'One-shot baseline taken in the user''s first 14 days. Establishes reference values for all 5 dimensions before agent-assisted growth begins.',
     'multi', 'one_shot', 14, 5, 0.10, 'rubric_scored', FALSE, TRUE, 70),

    ('incident-postmortem',
     'Incident-Driven Post-Mortem',
     'Triggered after a user-affecting incident (production outage, governance violation, escalation). Captures decision_independence and quality_output changes attributable to the incident.',
     'multi', 'event_driven', 1, 1, 0.10, 'incident_review', TRUE, FALSE, 80)
ON CONFLICT (slug) DO NOTHING;
