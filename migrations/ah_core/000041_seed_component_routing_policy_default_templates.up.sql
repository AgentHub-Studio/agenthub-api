-- Seed platform-managed COMPONENT ROUTING POLICY DEFAULT TEMPLATES in ah_core.
-- Templates are blueprints paired with EXT-005 ComponentRouter. Each
-- row encodes a (conflict_policy + use_case) preset so fresh tenants
-- pick routing behavior without inventing semantics.

CREATE TABLE IF NOT EXISTS ah_core.component_routing_policy_default_template (
    id                          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    slug                        VARCHAR(96)  NOT NULL UNIQUE,
    name                        VARCHAR(160) NOT NULL,
    description                 TEXT         NOT NULL,
    -- conflict_policy MATCHES EXT-005 RoutingConflictPolicy enum byte-for-byte.
    conflict_policy             VARCHAR(48)  NOT NULL,
    -- target_use_case classifies the deployment scenario.
    target_use_case             VARCHAR(48)  NOT NULL,
    -- recommended_for_tenant_kind hints the audience.
    recommended_for_tenant_kind VARCHAR(48)  NOT NULL,
    requires_admin_review       BOOLEAN      NOT NULL DEFAULT FALSE,
    is_recommended              BOOLEAN      NOT NULL DEFAULT FALSE,
    is_active                   BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_order                  INTEGER      NOT NULL DEFAULT 0,
    created_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_crpd_policy   ON ah_core.component_routing_policy_default_template (conflict_policy);
CREATE INDEX IF NOT EXISTS idx_ah_core_crpd_active   ON ah_core.component_routing_policy_default_template (is_active);
CREATE INDEX IF NOT EXISTS idx_ah_core_crpd_rec      ON ah_core.component_routing_policy_default_template (is_recommended) WHERE is_recommended = TRUE;

-- Seed catalog: 5 routing policy presets covering common deployment shapes.

INSERT INTO ah_core.component_routing_policy_default_template
    (slug, name, description, conflict_policy, target_use_case,
     recommended_for_tenant_kind, requires_admin_review, is_recommended, sort_order)
VALUES
    ('stable-incumbent',
     'Stable Incumbent (first install wins)',
     'Conservative routing: when two extensions provide the same component, the EARLIEST installed extension wins. Avoids surprises during marketplace updates. Recommended default for most production tenants.',
     'first_install_wins', 'general', 'general', FALSE, TRUE, 10),

    ('optimistic-latest',
     'Optimistic Latest (newest install wins)',
     'Optimistic routing: most recent install wins on conflict. Useful when platform team trusts marketplace updates and wants automatic upgrades. May surprise users when newer extensions change behavior.',
     'latest_install_wins', 'general', 'general', FALSE, TRUE, 20),

    ('strict-pinned',
     'Strict Pinned (require explicit pin)',
     'Strict routing: any conflict requires an explicit admin pin before runtime resolution. Forces operations team to make the decision visibly. Recommended for regulated tenants. Requires admin review.',
     'require_explicit_pin', 'regulated', 'regulated', TRUE, TRUE, 30),

    ('fail-fast',
     'Fail Fast (error on conflict)',
     'Defensive routing: any conflict throws ErrRoutingConflictUnresolved. Useful for staging environments where conflicts indicate misconfiguration that must be fixed before production deploy.',
     'error_on_conflict', 'staging', 'general', FALSE, TRUE, 40),

    ('dev-debug-incumbent',
     'Dev Debug (incumbent default)',
     'Dev-mode preset using first-install-wins to keep behavior predictable while iterating. NOT recommended as a production posture (inherits stale routes silently).',
     'first_install_wins', 'general', 'dev_local', FALSE, FALSE, 50)
ON CONFLICT (slug) DO NOTHING;
