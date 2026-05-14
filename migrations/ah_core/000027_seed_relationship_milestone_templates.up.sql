-- Seed platform-managed RELATIONSHIP MILESTONE TEMPLATES in ah_core.
-- Templates are blueprints for FUTURE-002 user-agent relationship state
-- transitions: which interaction-count + rapport-score combinations
-- promote/demote trust level, and which rapport-event types trigger
-- automated recording. Tenants instantiate these to drive longitudinal
-- relationship tracking without inventing thresholds.

CREATE TABLE IF NOT EXISTS ah_core.relationship_milestone_template (
    id                          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    -- slug is dotted-path namespace (kebab-case).
    slug                        VARCHAR(96)  NOT NULL UNIQUE,
    name                        VARCHAR(160) NOT NULL,
    description                 TEXT         NOT NULL,
    -- target_trust_level is the FUTURE-002 TrustLevel reached on success.
    target_trust_level          VARCHAR(32)  NOT NULL,
    -- min_interaction_count is the floor count to reach this trust level.
    min_interaction_count       INTEGER      NOT NULL,
    -- min_rapport_score is the floor rapport ([-1,+1]) to reach this level.
    min_rapport_score           DOUBLE PRECISION NOT NULL,
    -- triggers_event_kinds is a comma-separated list of FUTURE-002
    -- RapportEvent kinds whose recording is automated by this template.
    triggers_event_kinds        TEXT         NOT NULL,
    -- on_reach_action is the platform-side action triggered on milestone
    -- reach (e.g. "send_admin_notification", "elevate_communication_style",
    -- "open_review_ticket", "no_action").
    on_reach_action             VARCHAR(48)  NOT NULL,
    -- requires_admin_review marks promotions that need admin vetting
    -- (typically trusted level + mistrusted detection).
    requires_admin_review       BOOLEAN      NOT NULL DEFAULT FALSE,
    -- is_recommended marks safe one-click defaults.
    is_recommended              BOOLEAN      NOT NULL DEFAULT FALSE,
    is_active                   BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_order                  INTEGER      NOT NULL DEFAULT 0,
    created_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_relmilestone_trust    ON ah_core.relationship_milestone_template (target_trust_level);
CREATE INDEX IF NOT EXISTS idx_ah_core_relmilestone_action   ON ah_core.relationship_milestone_template (on_reach_action);
CREATE INDEX IF NOT EXISTS idx_ah_core_relmilestone_active   ON ah_core.relationship_milestone_template (is_active);
CREATE INDEX IF NOT EXISTS idx_ah_core_relmilestone_rec      ON ah_core.relationship_milestone_template (is_recommended) WHERE is_recommended = TRUE;

-- Seed catalog: 8 milestone templates aligned with FUTURE-002 promotion
-- rules (1+ event = probationary, 10+ rapport≥0 = established,
-- 50+ rapport≥0.5 = trusted; sticky mistrusted) plus event-recording
-- automation templates.

INSERT INTO ah_core.relationship_milestone_template
    (slug, name, description, target_trust_level, min_interaction_count, min_rapport_score, triggers_event_kinds, on_reach_action, requires_admin_review, is_recommended, sort_order)
VALUES
    -- Trust ladder promotions (ordered low→high).
    ('first-interaction-probationary',
     'First Interaction → Probationary',
     'Promotes a fresh user from unknown to probationary on the first recorded interaction. Establishes baseline tracking before any positive/negative signal accumulates.',
     'probationary', 1, -1.0,
     'positive,neutral,negative,conflict_resolved,escalation',
     'no_action', FALSE, TRUE, 10),

    ('established-after-ten-positive',
     'Established → After 10 Interactions Without Negative Drift',
     'Promotes from probationary to established after 10 interactions provided rapport stayed non-negative. Aligns with FUTURE-002 default: 10 interactions + rapport≥0.',
     'established', 10, 0.0,
     'positive,neutral,conflict_resolved',
     'elevate_communication_style', FALSE, TRUE, 20),

    ('trusted-after-fifty-strong-rapport',
     'Trusted → After 50 Interactions With Strong Rapport',
     'Promotes to trusted after 50 interactions and sustained rapport ≥0.5. Triggers admin notification for autonomy expansion review (admin signs off on increased privileges).',
     'trusted', 50, 0.5,
     'positive,conflict_resolved',
     'send_admin_notification', TRUE, TRUE, 30),

    ('mistrusted-on-repeated-escalation',
     'Mistrusted → On Repeated Escalation Pattern',
     'Marks user as mistrusted on 3+ escalation events within 30 days OR a single severe abuse pattern (e.g. jailbreak attempt). Sticky — clearing requires admin reset.',
     'mistrusted', 3, -0.4,
     'escalation,negative',
     'open_review_ticket', TRUE, TRUE, 40),

    -- Event-recording automation templates (do not promote; just record).
    ('record-positive-acknowledgements',
     'Record Positive Acknowledgements',
     'Auto-records rapport_event=positive whenever the user expresses thanks/satisfaction. Drives the rapport score upward without admin intervention.',
     'probationary', 0, -1.0,
     'positive',
     'no_action', FALSE, TRUE, 50),

    ('record-conflict-resolutions',
     'Record Conflict Resolutions',
     'Auto-records rapport_event=conflict_resolved when a previously-negative interaction terminates positively. Counters past negative drift (delta +0.10 vs negative -0.10).',
     'probationary', 0, -1.0,
     'conflict_resolved',
     'no_action', FALSE, TRUE, 60),

    ('record-negative-and-alert',
     'Record Negative + Alert Admin',
     'Auto-records rapport_event=negative AND surfaces an alert to admin when rapport drops below 0 (early intervention before mistrust threshold).',
     'probationary', 0, -1.0,
     'negative',
     'send_admin_notification', FALSE, TRUE, 70),

    ('record-escalation-and-open-ticket',
     'Record Escalation + Open Review Ticket',
     'Auto-records rapport_event=escalation AND opens a review ticket for admin investigation. Critical signal preceding mistrusted promotion.',
     'probationary', 0, -1.0,
     'escalation',
     'open_review_ticket', TRUE, TRUE, 80)
ON CONFLICT (slug) DO NOTHING;
