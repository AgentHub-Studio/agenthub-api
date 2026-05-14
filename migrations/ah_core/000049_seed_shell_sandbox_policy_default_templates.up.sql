-- PERM-008-paired: shell sandbox policy default templates.
-- 5 sandbox policy blueprints spanning the safety ladder so fresh
-- tenants do not invent AllowedPathRoots/BlockedCommands/ceilings.

CREATE TABLE IF NOT EXISTS ah_core.shell_sandbox_policy_default_template (
    id                              UUID PRIMARY KEY,
    slug                            TEXT NOT NULL UNIQUE,
    name                            TEXT NOT NULL,
    description                     TEXT NOT NULL,
    target_safety_posture           TEXT NOT NULL,
    target_use_case                 TEXT NOT NULL,
    allowed_path_roots              JSONB NOT NULL DEFAULT '[]'::jsonb,
    blocked_commands                JSONB NOT NULL DEFAULT '[]'::jsonb,
    blocked_arg_patterns            JSONB NOT NULL DEFAULT '[]'::jsonb,
    max_runtime_secs                INTEGER NOT NULL DEFAULT 0,
    max_output_bytes                INTEGER NOT NULL DEFAULT 0,
    allow_network                   BOOLEAN NOT NULL DEFAULT FALSE,
    recommended_for_tenant_kind     TEXT NOT NULL DEFAULT 'general',
    requires_admin_review           BOOLEAN NOT NULL DEFAULT FALSE,
    is_recommended                  BOOLEAN NOT NULL DEFAULT FALSE,
    is_active                       BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order                      INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_shell_sandbox_policy_dt_posture
    ON ah_core.shell_sandbox_policy_default_template(target_safety_posture);
CREATE INDEX IF NOT EXISTS idx_shell_sandbox_policy_dt_use_case
    ON ah_core.shell_sandbox_policy_default_template(target_use_case);
CREATE INDEX IF NOT EXISTS idx_shell_sandbox_policy_dt_active
    ON ah_core.shell_sandbox_policy_default_template(is_active);
CREATE INDEX IF NOT EXISTS idx_shell_sandbox_policy_dt_recommended
    ON ah_core.shell_sandbox_policy_default_template(is_recommended)
    WHERE is_recommended = TRUE;

INSERT INTO ah_core.shell_sandbox_policy_default_template
    (id, slug, name, description, target_safety_posture, target_use_case,
     allowed_path_roots, blocked_commands, blocked_arg_patterns,
     max_runtime_secs, max_output_bytes, allow_network,
     recommended_for_tenant_kind, requires_admin_review,
     is_recommended, is_active, sort_order)
VALUES
    ('66666666-6666-6666-6666-000000000001',
     'locked-down',
     'Locked Down',
     'Maximum restriction: no filesystem access, no network, every shell command rejected by virtue of empty allowed path roots. Used for read-only audit sessions where the agent must never execute a side effect.',
     'strict', 'audit_session',
     '[]'::jsonb,
     '["sudo","su","ssh","scp","rsync","mount","systemctl","docker","kubectl"]'::jsonb,
     '["--privileged","--no-sandbox","; rm","| sh","| bash","curl * | "]'::jsonb,
     10, 16384, FALSE,
     'general', TRUE, TRUE, TRUE, 10),

    ('66666666-6666-6666-6666-000000000002',
     'web-safe-default',
     'Web-Safe Default',
     'Recommended default for web-first tenants: confines shell to a per-session sandbox directory, blocks privilege escalation and network exfiltration shells, and applies modest runtime/output ceilings.',
     'balanced', 'standard_chat',
     '["/var/agenthub/session"]'::jsonb,
     '["sudo","su","ssh","scp","systemctl","mount","umount"]'::jsonb,
     '["--privileged","--no-sandbox","; rm","| sh","| bash","curl * | "]'::jsonb,
     60, 1048576, FALSE,
     'general', FALSE, TRUE, TRUE, 20),

    ('66666666-6666-6666-6666-000000000003',
     'dev-workstation',
     'Developer Workstation',
     'Engineering tenants who run builds and tests need broader paths and longer runtimes. Network is allowed so package managers and git can fetch dependencies; sudo and privileged container flags remain blocked.',
     'progressive', 'engineering',
     '["/workspace","/tmp"]'::jsonb,
     '["sudo","su"]'::jsonb,
     '["--privileged","--no-sandbox"]'::jsonb,
     900, 16777216, TRUE,
     'general', TRUE, TRUE, TRUE, 30),

    ('66666666-6666-6666-6666-000000000004',
     'cicd-runner',
     'CI/CD Runner',
     'Designed for CI/CD agents: longer runtime (build pipelines), large output budget (test logs), network allowed for artifact fetch. Adds destructive-arg blocking to defend against runaway pipelines.',
     'permissive', 'cicd_pipeline',
     '["/workspace","/tmp","/build"]'::jsonb,
     '["sudo","su"]'::jsonb,
     '["--privileged","; rm -rf","| sh","| bash"]'::jsonb,
     1800, 67108864, TRUE,
     'general', TRUE, TRUE, TRUE, 40),

    ('66666666-6666-6666-6666-000000000005',
     'incident-response-readonly',
     'Incident Response Read-Only',
     'During a security incident the agent should observe state without altering it. Filesystem read scope is broad (so the agent can inspect /var/log, /etc) but every mutating command is blocked.',
     'conservative', 'incident_response',
     '["/var/log","/etc","/var/agenthub/session"]'::jsonb,
     '["sudo","su","rm","mv","chmod","chown","tee","dd","systemctl","mount"]'::jsonb,
     '["; rm","| sh","| bash",">> ","--force"]'::jsonb,
     120, 4194304, FALSE,
     'general', TRUE, TRUE, TRUE, 50)
ON CONFLICT (slug) DO NOTHING;
