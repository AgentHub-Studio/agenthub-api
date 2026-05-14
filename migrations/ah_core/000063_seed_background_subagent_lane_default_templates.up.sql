-- SUB-008-paired: background subagent lane default templates.
-- 4 lane configurations that fresh tenants can route SUB-008
-- BackgroundSubagentJobs to without inventing per-task-class operational
-- tuning (timeout budget, concurrency, watchdog cadence, parent-cancel
-- propagation). Each lane references a typical_task_class from the
-- SUB-002 builtin roster vocabulary so admin UI can suggest lanes
-- based on the subagent role.

CREATE TABLE IF NOT EXISTS ah_core.background_subagent_lane_default_template (
    id                              UUID PRIMARY KEY,
    slug                            TEXT NOT NULL UNIQUE,
    lane_name                       TEXT NOT NULL,
    description                     TEXT NOT NULL,
    typical_task_class              TEXT NOT NULL,
    timeout_budget_seconds          INTEGER NOT NULL,
    watchdog_check_interval_seconds INTEGER NOT NULL,
    max_concurrent_per_parent       INTEGER NOT NULL,
    priority                        TEXT NOT NULL DEFAULT 'normal',
    auto_cancel_on_parent_terminate BOOLEAN NOT NULL DEFAULT TRUE,
    retry_posture                   TEXT NOT NULL DEFAULT 'none',
    recommended_for_tenant_kind     TEXT NOT NULL DEFAULT 'general',
    is_recommended                  BOOLEAN NOT NULL DEFAULT TRUE,
    is_active                       BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order                      INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_bg_subagent_lane_dt_task_class
    ON ah_core.background_subagent_lane_default_template(typical_task_class);
CREATE INDEX IF NOT EXISTS idx_bg_subagent_lane_dt_priority
    ON ah_core.background_subagent_lane_default_template(priority);
CREATE INDEX IF NOT EXISTS idx_bg_subagent_lane_dt_active
    ON ah_core.background_subagent_lane_default_template(is_active);

INSERT INTO ah_core.background_subagent_lane_default_template
    (id, slug, lane_name, description, typical_task_class,
     timeout_budget_seconds, watchdog_check_interval_seconds,
     max_concurrent_per_parent, priority,
     auto_cancel_on_parent_terminate, retry_posture,
     recommended_for_tenant_kind,
     is_recommended, is_active, sort_order)
VALUES
    ('00000005-0000-0000-0000-000000000001',
     'quick-glance',
     'Quick Glance Lane',
     'Sub-minute read-only lookups (status checks, citation fetches, single-doc lookups). Short timeout so stuck jobs surface fast. High concurrency per parent because each is cheap. Used for fan-out queries that the parent will join later.',
     'exploration',
     30, 5, 20, 'high',
     TRUE, 'none', 'general',
     TRUE, TRUE, 10),

    ('00000005-0000-0000-0000-000000000002',
     'planner-lane',
     'Planner Lane',
     'Short read-only planning runs (2-3 minutes max). Single concurrent job per parent — sequential planning avoids divergent plans landing in parallel. Normal priority. Used by SUB-002 planner-baseline and SUB-003 dual-loop-planner templates.',
     'planning',
     180, 15, 1, 'normal',
     TRUE, 'none', 'general',
     TRUE, TRUE, 20),

    ('00000005-0000-0000-0000-000000000003',
     'research-lane',
     'Research Lane',
     'Long-running read-only investigation (up to 10 min). Up to 4 concurrent per parent for parallel topic coverage. Watchdog checks every 30s. Used by SUB-002 researcher-baseline and SUB-003 researcher-derived.',
     'investigation',
     600, 30, 4, 'normal',
     TRUE, 'none', 'general',
     TRUE, TRUE, 30),

    ('00000005-0000-0000-0000-000000000004',
     'coder-lane',
     'Coder Lane',
     'Code edits scoped to parent task (5 min budget). Single concurrent per parent — serial writes prevent conflicting edits on the same files. Watchdog every 20s. Used by SUB-002 coder-baseline and SUB-003 coder-derived. Retry posture is none — failed edits surface to parent for human-in-the-loop.',
     'implementation',
     300, 20, 1, 'normal',
     TRUE, 'none', 'general',
     TRUE, TRUE, 40)
ON CONFLICT (slug) DO NOTHING;
