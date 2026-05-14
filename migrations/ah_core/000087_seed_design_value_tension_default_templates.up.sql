-- ah_core: seed design value tension default templates
-- Source: arXiv:2604.14228v1 Table 4 (§11.2 — Design Value Tensions)
-- Five tensions observed empirically in production coding-agent deployments.

CREATE TABLE IF NOT EXISTS ah_core.design_value_tension_template (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    slug              VARCHAR(64) NOT NULL UNIQUE,
    label             VARCHAR(128) NOT NULL,
    description       TEXT,
    value1            VARCHAR(32) NOT NULL CHECK (value1 IN ('human_authority','safety','reliability','capability','adaptability')),
    value2            VARCHAR(32) NOT NULL CHECK (value2 IN ('human_authority','safety','reliability','capability','adaptability')),
    tension_label     VARCHAR(128) NOT NULL,
    evidence_summary  TEXT,
    sort_order        INT         NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO ah_core.design_value_tension_template
    (slug, label, description, value1, value2, tension_label, evidence_summary, sort_order)
VALUES
    (
        'authority_safety',
        'Human Authority × Safety',
        'Tension between requiring human approval (authority) and deny-first safety mechanisms.',
        'human_authority',
        'safety',
        'Approval fatigue vs. protection',
        '93% approval fatigue rate — frequent confirmations erode oversight quality while deny-first safety requires them.',
        10
    ),
    (
        'safety_capability',
        'Safety × Capability',
        'Tension between deny-first safety defaults and the reachable capability of the agent.',
        'safety',
        'capability',
        'Performance vs. defense depth',
        'Deny-first defaults block >50 subcommands that bypass deny checks; safe defaults directly reduce reachable capability.',
        20
    ),
    (
        'adaptability_safety',
        'Adaptability × Safety',
        'Tension between adaptive trust grants and the expanded prompt-injection attack surface they create.',
        'adaptability',
        'safety',
        'Extensibility vs. attack surface',
        'Pre-trust window exploits — adaptive trust grants enable prompt-injection attacks before deny rules engage.',
        30
    ),
    (
        'capability_adaptability',
        'Capability × Adaptability',
        'Tension between proactive autonomous actions and the risk of disrupting user context.',
        'capability',
        'adaptability',
        'Proactivity vs. disruption',
        'Proactive autonomous actions increase task completion but risk disrupting user context or session state unexpectedly.',
        40
    ),
    (
        'capability_reliability',
        'Capability × Reliability',
        'Tension between maximising task-completion velocity and maintaining cross-session coherence.',
        'capability',
        'reliability',
        'Velocity vs. coherence',
        'Maximising autonomous task completion degrades cross-session consistency and predictable behaviour.',
        50
    )
ON CONFLICT (slug) DO NOTHING;
