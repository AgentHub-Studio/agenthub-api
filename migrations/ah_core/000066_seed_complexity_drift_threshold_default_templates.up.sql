-- HUMAN-006-paired: complexity drift threshold default templates.
-- 6 threshold templates, one per ComplexityDriftSignal enum value, so
-- fresh tenants don't have to invent warn/critical bands. Each row
-- declares the unit of measure so admin UI can render bands in context.
--
-- DB-level CHECK constraints encode HUMAN-006 ComplexityDriftThreshold
-- invariants: warn_at >= 0 AND critical_at > warn_at.

CREATE TABLE IF NOT EXISTS ah_core.complexity_drift_threshold_default_template (
    id                          UUID PRIMARY KEY,
    slug                        TEXT NOT NULL UNIQUE,
    signal                      TEXT NOT NULL UNIQUE, -- matches HUMAN-006 ComplexityDriftSignal byte-for-byte
    warn_at                     DOUBLE PRECISION NOT NULL,
    critical_at                 DOUBLE PRECISION NOT NULL,
    unit                        TEXT NOT NULL,
    description                 TEXT NOT NULL,
    applies_to_subject_kind     TEXT NOT NULL DEFAULT 'run',
    typical_dedup_window_seconds INTEGER NOT NULL DEFAULT 300,
    is_recommended              BOOLEAN NOT NULL DEFAULT TRUE,
    is_active                   BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order                  INTEGER NOT NULL DEFAULT 0,
    CONSTRAINT chk_drift_threshold_warn_nonneg CHECK (warn_at >= 0),
    CONSTRAINT chk_drift_threshold_critical_above_warn CHECK (critical_at > warn_at)
);

CREATE INDEX IF NOT EXISTS idx_drift_threshold_dt_signal
    ON ah_core.complexity_drift_threshold_default_template(signal);
CREATE INDEX IF NOT EXISTS idx_drift_threshold_dt_active
    ON ah_core.complexity_drift_threshold_default_template(is_active);

INSERT INTO ah_core.complexity_drift_threshold_default_template
    (id, slug, signal, warn_at, critical_at, unit, description,
     applies_to_subject_kind, typical_dedup_window_seconds,
     is_recommended, is_active, sort_order)
VALUES
    ('00000008-0000-0000-0000-000000000001',
     'scope-creep-default',
     'scope_creep',
     50.0, 100.0,
     'sub_tasks',
     'Number of sub-tasks accumulated beyond the original plan. Warn at 50, critical at 100. Beyond critical, parent should re-plan or hand off.',
     'run', 300, TRUE, TRUE, 10),

    ('00000008-0000-0000-0000-000000000002',
     'dependency-explosion-default',
     'dependency_explosion',
     30.0, 80.0,
     'modules_touched',
     'Number of distinct modules/files the run has touched. Warn at 30 (likely too wide a scope), critical at 80 (almost certainly drifted).',
     'run', 300, TRUE, TRUE, 20),

    ('00000008-0000-0000-0000-000000000003',
     'test-decay-default',
     'test_decay',
     0.05, 0.15,
     'coverage_drop_ratio',
     'Drop ratio in test coverage since run start. Warn at 5% drop, critical at 15%. Negative drops (improvements) never trigger.',
     'run', 600, TRUE, TRUE, 30),

    ('00000008-0000-0000-0000-000000000004',
     'churn-spike-default',
     'churn_spike',
     10.0, 25.0,
     'reedits_per_file',
     'Average re-edits on the same file (compulsive churn). Warn at 10, critical at 25. Indicates the agent is stuck.',
     'run', 300, TRUE, TRUE, 40),

    ('00000008-0000-0000-0000-000000000005',
     'goal-drift-default',
     'goal_drift',
     0.4, 0.7,
     'cosine_distance',
     'Cosine distance between current run output and original prompt embedding. Warn at 0.4, critical at 0.7. Higher means more divergence.',
     'run', 600, TRUE, TRUE, 50),

    ('00000008-0000-0000-0000-000000000006',
     'cognitive-load-default',
     'cognitive_load',
     0.7, 0.9,
     'context_fill_ratio',
     'Context window fill ratio. Warn at 70% (compact soon), critical at 90% (forced compact imminent). Captures turn-by-turn complexity pressure.',
     'run', 120, TRUE, TRUE, 60)
ON CONFLICT (slug) DO NOTHING;
