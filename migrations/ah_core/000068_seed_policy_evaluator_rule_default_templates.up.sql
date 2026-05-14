-- PERM-007-paired: policy evaluator rule default templates.
-- 6 starter fallback rules covering common dangerous patterns so fresh
-- tenants get sensible classifier behavior before authoring their own.
--
-- DB-level CHECK constraints encode PERM-007 invariants:
--   - decision in (allow, deny, escalate, abstain)
--   - confidence in (low, medium, high)
--   - priority >= 0

CREATE TABLE IF NOT EXISTS ah_core.policy_evaluator_rule_default_template (
    id                          UUID PRIMARY KEY,
    slug                        TEXT NOT NULL UNIQUE,
    rule_id                     TEXT NOT NULL UNIQUE, -- matches PERM-007 RuleID kebab regex
    tool_name_pattern           TEXT NOT NULL,
    description                 TEXT NOT NULL,
    decision                    TEXT NOT NULL,
    confidence                  TEXT NOT NULL,
    priority                    INTEGER NOT NULL,
    risk_kind                   TEXT NOT NULL,
    is_recommended              BOOLEAN NOT NULL DEFAULT TRUE,
    is_active                   BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order                  INTEGER NOT NULL DEFAULT 0,
    CONSTRAINT chk_per_decision  CHECK (decision IN ('allow','deny','escalate','abstain')),
    CONSTRAINT chk_per_confidence CHECK (confidence IN ('low','medium','high')),
    CONSTRAINT chk_per_priority   CHECK (priority >= 0)
);

CREATE INDEX IF NOT EXISTS idx_per_rule_dt_decision
    ON ah_core.policy_evaluator_rule_default_template(decision);
CREATE INDEX IF NOT EXISTS idx_per_rule_dt_priority
    ON ah_core.policy_evaluator_rule_default_template(priority);
CREATE INDEX IF NOT EXISTS idx_per_rule_dt_active
    ON ah_core.policy_evaluator_rule_default_template(is_active);

INSERT INTO ah_core.policy_evaluator_rule_default_template
    (id, slug, rule_id, tool_name_pattern, description,
     decision, confidence, priority, risk_kind,
     is_recommended, is_active, sort_order)
VALUES
    ('0000000a-0000-0000-0000-000000000001',
     'deny-destructive-shell',
     'deny-destructive-shell',
     '^shell\.(rm|drop|truncate|force-push).*$',
     'Destructive shell commands deny by default at classifier fallback. Includes rm, drop, truncate, force-push variants.',
     'deny', 'high', 10, 'destructive_io',
     TRUE, TRUE, 10),

    ('0000000a-0000-0000-0000-000000000002',
     'escalate-prod-db-writes',
     'escalate-prod-db-writes',
     '^(db|sql)\.prod\..*(delete|drop|update|insert).*$',
     'Production DB writes escalate to human review. Includes prod schemas across db and sql tools.',
     'escalate', 'high', 20, 'prod_data_mutation',
     TRUE, TRUE, 20),

    ('0000000a-0000-0000-0000-000000000003',
     'deny-secret-path-access',
     'deny-secret-path-access',
     '^(fs|file|secret)\..*(\.env|/secrets/|credentials|private_key|id_rsa).*$',
     'Touching secret paths denied by default. Covers .env files, /secrets/ paths, credentials, private keys.',
     'deny', 'high', 30, 'secret_exposure',
     TRUE, TRUE, 30),

    ('0000000a-0000-0000-0000-000000000004',
     'escalate-network-egress',
     'escalate-network-egress',
     '^(http|https|net)\.(post|put|delete)\..*$',
     'Outbound network writes (POST/PUT/DELETE) escalate to human. GET reads abstain.',
     'escalate', 'medium', 40, 'network_egress',
     TRUE, TRUE, 40),

    ('0000000a-0000-0000-0000-000000000005',
     'escalate-pii-read',
     'escalate-pii-read',
     '^(pii|user|customer)\.read\..*$',
     'PII reads escalate to human review for GDPR/LGPD compliance.',
     'escalate', 'medium', 50, 'pii_compliance',
     TRUE, TRUE, 50),

    ('0000000a-0000-0000-0000-000000000006',
     'escalate-installation-privileges',
     'escalate-installation-privileges',
     '^(install|extension|plugin)\.(install|enable|grant).*$',
     'Installing extensions or granting privileges escalates to admin approval.',
     'escalate', 'high', 60, 'privilege_escalation',
     TRUE, TRUE, 60)
ON CONFLICT (slug) DO NOTHING;
