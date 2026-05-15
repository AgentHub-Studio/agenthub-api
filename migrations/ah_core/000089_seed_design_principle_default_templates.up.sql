-- ah_core: seed design principle default templates
-- Source: arXiv:2604.14228v1 Table 1 (§2.2 — "Design Principles")
-- Thirteen principles, each answering a recurring design question for production coding agents.

CREATE TABLE IF NOT EXISTS ah_core.design_principle_template (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    slug                VARCHAR(64) NOT NULL UNIQUE,
    label               VARCHAR(128) NOT NULL,
    design_question     TEXT        NOT NULL,
    values_served       TEXT[]      NOT NULL DEFAULT '{}',
    referenced_sections TEXT[]      NOT NULL DEFAULT '{}',
    sort_order          INT         NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO ah_core.design_principle_template
    (slug, label, design_question, values_served, referenced_sections, sort_order)
VALUES
    (
        'deny_first_human_escalation',
        'Deny-first with human escalation',
        'Should unrecognized actions be allowed, blocked, or escalated to the human?',
        ARRAY['human_authority','safety'],
        ARRAY['5','8','9'],
        10
    ),
    (
        'graduated_trust_spectrum',
        'Graduated trust spectrum',
        'Fixed permission level, or a spectrum users traverse over time?',
        ARRAY['human_authority','adaptability'],
        ARRAY['5'],
        20
    ),
    (
        'defense_in_depth_layered',
        'Defense in depth with layered mechanisms',
        'Single safety boundary, or multiple overlapping ones using different techniques?',
        ARRAY['safety','human_authority','reliability'],
        ARRAY['3','5'],
        30
    ),
    (
        'externalized_programmable_policy',
        'Externalized programmable policy',
        'Hardcoded policy, or externalized configs with lifecycle hooks?',
        ARRAY['safety','human_authority','adaptability'],
        ARRAY['5','6'],
        40
    ),
    (
        'context_as_scarce_resource',
        'Context as scarce resource with progressive management',
        'What is the binding resource constraint, and how to manage it: single-pass truncation or graduated pipeline?',
        ARRAY['reliability','capability'],
        ARRAY['4','6','7','8'],
        50
    ),
    (
        'append_only_durable_state',
        'Append-only durable state',
        'Mutable state, checkpoint snapshots, or append-only logs?',
        ARRAY['reliability','human_authority'],
        ARRAY['4','9'],
        60
    ),
    (
        'minimal_scaffolding_maximal_harness',
        'Minimal scaffolding, maximal operational harness',
        'Invest in scaffolding-side reasoning, or operational infrastructure that lets the model reason freely?',
        ARRAY['capability','reliability'],
        ARRAY['3','4'],
        70
    ),
    (
        'values_over_rules',
        'Values over rules',
        'Rigid decision procedures, or contextual judgment backed by deterministic guardrails?',
        ARRAY['capability','human_authority'],
        ARRAY['3','5','7'],
        80
    ),
    (
        'composable_multi_mechanism',
        'Composable multi-mechanism extensibility',
        'One unified extension API, or layered mechanisms at different context costs?',
        ARRAY['capability','adaptability'],
        ARRAY['6'],
        90
    ),
    (
        'reversibility_weighted_risk',
        'Reversibility-weighted risk assessment',
        'Same oversight for all actions, or lighter for reversible and read-only ones?',
        ARRAY['capability','safety'],
        ARRAY['4','5','8'],
        100
    ),
    (
        'transparent_file_based',
        'Transparent file-based configuration and memory',
        'Opaque database, embedding-based retrieval, or user-visible version-controllable files?',
        ARRAY['adaptability','human_authority'],
        ARRAY['7'],
        110
    ),
    (
        'isolated_subagent_boundaries',
        'Isolated subagent boundaries',
        'Subagents share the parent''s context and permissions, or operate in isolation?',
        ARRAY['reliability','safety','capability'],
        ARRAY['8'],
        120
    ),
    (
        'graceful_recovery_resilience',
        'Graceful recovery and resilience',
        'Fail hard on errors, or silently recover and reserve human attention for unrecoverable situations?',
        ARRAY['reliability','capability'],
        ARRAY['4','5'],
        130
    )
ON CONFLICT (slug) DO NOTHING;
