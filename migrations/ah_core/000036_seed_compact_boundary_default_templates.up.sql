-- Seed platform-managed COMPACT BOUNDARY DEFAULT TEMPLATES in ah_core.
-- Templates are blueprints paired with CTX-011 CompactBoundaryRegistry.
-- Each row encodes when/how compaction triggers (turn count threshold,
-- token-pressure threshold, cost threshold, idle-time threshold) so
-- fresh tenants pick a profile without inventing thresholds.

CREATE TABLE IF NOT EXISTS ah_core.compact_boundary_default_template (
    id                          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    slug                        VARCHAR(96)  NOT NULL UNIQUE,
    name                        VARCHAR(160) NOT NULL,
    description                 TEXT         NOT NULL,
    -- trigger_kind identifies which signal triggers compaction.
    trigger_kind                VARCHAR(32)  NOT NULL,
    -- max_turns_before_compact: trigger when turn count crosses (0 = disabled).
    max_turns_before_compact    INTEGER      NOT NULL,
    -- token_pressure_pct: trigger when context_used / context_limit ≥ this (0-100, 0 = disabled).
    token_pressure_pct          INTEGER      NOT NULL,
    -- cost_threshold_usd: trigger when cumulative cost crosses (0 = disabled).
    cost_threshold_usd          DOUBLE PRECISION NOT NULL,
    -- idle_seconds: trigger after this many seconds of idle (0 = disabled).
    idle_seconds                INTEGER      NOT NULL,
    -- preserve_tail_messages: how many recent messages to KEEP post-compact.
    preserve_tail_messages      INTEGER      NOT NULL,
    target_use_case             VARCHAR(48)  NOT NULL,
    recommended_for_model_family VARCHAR(48) NOT NULL,
    requires_admin_review       BOOLEAN      NOT NULL DEFAULT FALSE,
    is_recommended              BOOLEAN      NOT NULL DEFAULT FALSE,
    is_active                   BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_order                  INTEGER      NOT NULL DEFAULT 0,
    created_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_cbdt_use_case ON ah_core.compact_boundary_default_template (target_use_case);
CREATE INDEX IF NOT EXISTS idx_ah_core_cbdt_kind     ON ah_core.compact_boundary_default_template (trigger_kind);
CREATE INDEX IF NOT EXISTS idx_ah_core_cbdt_active   ON ah_core.compact_boundary_default_template (is_active);
CREATE INDEX IF NOT EXISTS idx_ah_core_cbdt_rec      ON ah_core.compact_boundary_default_template (is_recommended) WHERE is_recommended = TRUE;

-- Seed catalog: 6 trigger profiles. Trigger kinds: turn_count /
-- token_pressure / cost / idle / hybrid_balanced / dev_debug.

INSERT INTO ah_core.compact_boundary_default_template
    (slug, name, description, trigger_kind, max_turns_before_compact,
     token_pressure_pct, cost_threshold_usd, idle_seconds,
     preserve_tail_messages, target_use_case, recommended_for_model_family,
     requires_admin_review, is_recommended, sort_order)
VALUES
    ('hybrid-balanced',
     'Hybrid Balanced',
     'Trigger compaction on the FIRST condition met across turns/token-pressure/cost — most robust default. Token-pressure 70% (mirrors CTX-010 default). Preserve last 8 messages.',
     'hybrid_balanced',
     30, 70, 5.00, 1800,
     8, 'general', 'mid_tier', FALSE, TRUE, 10),

    ('turn-count-strict',
     'Turn Count Strict',
     'Compact every 20 turns regardless of token pressure or cost. Predictable cadence; useful for predictable cost budgeting.',
     'turn_count',
     20, 0, 0, 0,
     8, 'general', 'mid_tier', FALSE, TRUE, 20),

    ('token-pressure-driven',
     'Token Pressure Driven',
     'Trigger only on token-pressure ≥ 70% (CTX-010 default mirror). Lets short conversations breathe; long ones get compacted at the right moment.',
     'token_pressure',
     0, 70, 0, 0,
     12, 'research', 'large_context', FALSE, TRUE, 30),

    ('cost-strict',
     'Cost Strict',
     'Compact aggressively on cost threshold ($1 USD per run). Trades context fidelity for cost. REQUIRES ADMIN REVIEW (degrades agent visibility for cost wins).',
     'cost',
     0, 0, 1.00, 0,
     5, 'general', 'mid_tier', TRUE, TRUE, 40),

    ('idle-driven',
     'Idle Driven',
     'Compact after 30 minutes of idle — useful for long-running multi-day sessions where the user comes back fresh and needs the prior context summarized.',
     'idle',
     0, 0, 0, 1800,
     10, 'general', 'mid_tier', FALSE, TRUE, 50),

    ('dev-debug-no-compact',
     'Dev Debug (no auto-compact)',
     'Disable all auto-compaction triggers. Use ONLY in dev — production conversations will eventually exceed model context windows. NOT recommended.',
     'dev_debug',
     0, 0, 0, 0,
     0, 'general', 'dev_local', FALSE, FALSE, 60)
ON CONFLICT (slug) DO NOTHING;
