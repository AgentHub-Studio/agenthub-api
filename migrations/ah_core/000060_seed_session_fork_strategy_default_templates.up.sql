-- PERSIST-005a-paired: session fork strategy default templates.
-- 3 templates 1:1 with SessionForkStrategy enum so fresh tenants
-- pick proven branching patterns without inventing cost vs safety
-- trade-offs.

CREATE TABLE IF NOT EXISTS ah_core.session_fork_strategy_default_template (
    id                              UUID PRIMARY KEY,
    slug                            TEXT NOT NULL UNIQUE,
    name                            TEXT NOT NULL,
    description                     TEXT NOT NULL,
    target_strategy                 TEXT NOT NULL,
    target_use_case                 TEXT NOT NULL,
    safety_posture                  TEXT NOT NULL,
    computes_transcript_hash        BOOLEAN NOT NULL DEFAULT FALSE,
    allows_fork_past_compaction     BOOLEAN NOT NULL DEFAULT FALSE,
    typical_storage_overhead        TEXT NOT NULL DEFAULT 'low',
    recommended_for_tenant_kind     TEXT NOT NULL DEFAULT 'general',
    requires_admin_review           BOOLEAN NOT NULL DEFAULT FALSE,
    is_recommended                  BOOLEAN NOT NULL DEFAULT FALSE,
    is_active                       BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order                      INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_session_fork_dt_strategy
    ON ah_core.session_fork_strategy_default_template(target_strategy);
CREATE INDEX IF NOT EXISTS idx_session_fork_dt_use_case
    ON ah_core.session_fork_strategy_default_template(target_use_case);
CREATE INDEX IF NOT EXISTS idx_session_fork_dt_active
    ON ah_core.session_fork_strategy_default_template(is_active);
CREATE INDEX IF NOT EXISTS idx_session_fork_dt_recommended
    ON ah_core.session_fork_strategy_default_template(is_recommended)
    WHERE is_recommended = TRUE;

INSERT INTO ah_core.session_fork_strategy_default_template
    (id, slug, name, description, target_strategy, target_use_case,
     safety_posture, computes_transcript_hash, allows_fork_past_compaction,
     typical_storage_overhead,
     recommended_for_tenant_kind, requires_admin_review,
     is_recommended, is_active, sort_order)
VALUES
    ('00000002-0000-0000-0000-000000000001',
     'full-copy-explore',
     'Full Copy — Explore',
     'Full physical copy of every turn up to the fork point. New session is fully independent — mutations to original do not affect the fork. Highest storage cost but safest semantics. Used for routine "/branch" exploration where the user wants a fully isolated playground.',
     'full_copy', 'routine_branch_exploration',
     'balanced',
     FALSE, FALSE, 'high',
     'general', FALSE, TRUE, TRUE, 10),

    ('00000002-0000-0000-0000-000000000002',
     'branch-pointer-cheap',
     'Branch Pointer — Cheap',
     'Stores a pointer to the original session transcript up to ForkedAtTurn; subsequent turns live only in the fork. Cheapest but requires the original to be immutable up to that turn. Used when storage cost matters more than divergence detection (most tenants with active sessions).',
     'branch_pointer', 'storage_optimized_branch',
     'permissive',
     FALSE, FALSE, 'low',
     'general', FALSE, TRUE, TRUE, 20),

    ('00000002-0000-0000-0000-000000000003',
     'snapshot-isolated-compliance',
     'Snapshot Isolated — Compliance',
     'Like branch_pointer but additionally records a SHA-256 hash of the shared transcript prefix so divergence can be detected if the original is mutated. Required for forking inside compacted prefix. Used by audit-strict tenants and any /branch operation that crosses a compact boundary.',
     'snapshot_isolated', 'compliance_branch',
     'strict',
     TRUE, TRUE, 'medium',
     'general', TRUE, TRUE, TRUE, 30)
ON CONFLICT (slug) DO NOTHING;
