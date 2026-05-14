-- SUB-011-paired: multi-agent coordination plan default templates.
-- 5 pre-vetted coordination shapes so fresh tenants pick proven
-- orchestration patterns without inventing strategy + failure +
-- parallelism trade-offs.

CREATE TABLE IF NOT EXISTS ah_core.multi_agent_coordination_plan_default_template (
    id                              UUID PRIMARY KEY,
    slug                            TEXT NOT NULL UNIQUE,
    name                            TEXT NOT NULL,
    description                     TEXT NOT NULL,
    target_strategy                 TEXT NOT NULL,
    target_failure_policy           TEXT NOT NULL,
    target_use_case                 TEXT NOT NULL,
    max_parallelism                 INTEGER NOT NULL DEFAULT 0,
    sample_task_count               INTEGER NOT NULL DEFAULT 0,
    has_dag_dependencies            BOOLEAN NOT NULL DEFAULT FALSE,
    recommended_for_tenant_kind     TEXT NOT NULL DEFAULT 'general',
    requires_admin_review           BOOLEAN NOT NULL DEFAULT FALSE,
    is_recommended                  BOOLEAN NOT NULL DEFAULT FALSE,
    is_active                       BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order                      INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_mac_plan_dt_strategy
    ON ah_core.multi_agent_coordination_plan_default_template(target_strategy);
CREATE INDEX IF NOT EXISTS idx_mac_plan_dt_failure_policy
    ON ah_core.multi_agent_coordination_plan_default_template(target_failure_policy);
CREATE INDEX IF NOT EXISTS idx_mac_plan_dt_use_case
    ON ah_core.multi_agent_coordination_plan_default_template(target_use_case);
CREATE INDEX IF NOT EXISTS idx_mac_plan_dt_active
    ON ah_core.multi_agent_coordination_plan_default_template(is_active);
CREATE INDEX IF NOT EXISTS idx_mac_plan_dt_recommended
    ON ah_core.multi_agent_coordination_plan_default_template(is_recommended)
    WHERE is_recommended = TRUE;

INSERT INTO ah_core.multi_agent_coordination_plan_default_template
    (id, slug, name, description, target_strategy, target_failure_policy,
     target_use_case, max_parallelism, sample_task_count, has_dag_dependencies,
     recommended_for_tenant_kind, requires_admin_review,
     is_recommended, is_active, sort_order)
VALUES
    ('ffffffff-ffff-ffff-ffff-000000000001',
     'sequential-pipeline',
     'Sequential Pipeline',
     'Step-by-step orchestration: each subagent finishes before the next begins. Routine choice for linear workflows (extract → transform → load). Failure aborts the rest.',
     'sequential', 'abort_on_failure',
     'linear_workflow', 1, 3, FALSE,
     'general', FALSE, TRUE, TRUE, 10),

    ('ffffffff-ffff-ffff-ffff-000000000002',
     'parallel-fanout',
     'Parallel Fan-Out',
     'Fan out 5+ independent searches/analyses in parallel; continue on failure so a single broken branch does not kill the batch. Bounded at MaxParallelism=5 to avoid resource exhaustion.',
     'parallel', 'continue_on_failure',
     'independent_research', 5, 5, FALSE,
     'general', FALSE, TRUE, TRUE, 20),

    ('ffffffff-ffff-ffff-ffff-000000000003',
     'dag-build-test-deploy',
     'DAG — Build / Test / Deploy',
     'Classic CI/CD diamond DAG: build → (test, lint) → deploy. Skip-downstream on failure so failed test does not waste deploy capacity. Admin review since deployment automation has blast radius.',
     'dag', 'skip_downstream_on_failure',
     'cicd_pipeline', 3, 4, TRUE,
     'general', TRUE, TRUE, TRUE, 30),

    ('ffffffff-ffff-ffff-ffff-000000000004',
     'pipeline-extract-summarize',
     'Pipeline — Extract / Summarize',
     'Two-stage pipeline where extract emits artifacts the summarizer consumes. Abort on failure (a failed extract leaves the summarizer with nothing to do).',
     'pipeline', 'abort_on_failure',
     'data_pipeline', 1, 2, TRUE,
     'general', FALSE, TRUE, TRUE, 40),

    ('ffffffff-ffff-ffff-ffff-000000000005',
     'dag-with-skip-downstream',
     'DAG — Skip Downstream',
     'General DAG template for complex workflows: 6 tasks across 3 stages. Skip-downstream policy lets independent branches finish even when one branch fails. Higher parallelism (4) for throughput; admin review for non-trivial topology.',
     'dag', 'skip_downstream_on_failure',
     'complex_orchestration', 4, 6, TRUE,
     'general', TRUE, TRUE, TRUE, 50)
ON CONFLICT (slug) DO NOTHING;
