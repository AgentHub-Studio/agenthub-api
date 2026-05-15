-- Seed platform-managed LAZY INSTRUCTION DEFAULT TEMPLATES in ah_core.
-- Templates are blueprints paired with CTX-005 LazyInstructionLoader.
-- Each row encodes a (slug, ttl, source_kind, target_use_case) profile
-- so fresh tenants pick reasonable TTL + source-kind defaults without
-- inventing thresholds.

CREATE TABLE IF NOT EXISTS ah_core.lazy_instruction_default_template (
    id                          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    slug                        VARCHAR(96)  NOT NULL UNIQUE,
    name                        VARCHAR(160) NOT NULL,
    description                 TEXT         NOT NULL,
    -- source_kind classifies WHERE the loader fn fetches content from.
    source_kind                 VARCHAR(48)  NOT NULL,
    -- ttl_seconds mirrors CTX-005 LazyInstruction.TTL (0 = never expire).
    ttl_seconds                 INTEGER      NOT NULL,
    target_use_case             VARCHAR(48)  NOT NULL,
    recommended_for_tenant_kind VARCHAR(48)  NOT NULL,
    requires_admin_review       BOOLEAN      NOT NULL DEFAULT FALSE,
    is_recommended              BOOLEAN      NOT NULL DEFAULT FALSE,
    is_active                   BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_order                  INTEGER      NOT NULL DEFAULT 0,
    created_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_lidt_source   ON ah_core.lazy_instruction_default_template (source_kind);
CREATE INDEX IF NOT EXISTS idx_ah_core_lidt_active   ON ah_core.lazy_instruction_default_template (is_active);
CREATE INDEX IF NOT EXISTS idx_ah_core_lidt_rec      ON ah_core.lazy_instruction_default_template (is_recommended) WHERE is_recommended = TRUE;

-- Seed catalog: 6 templates covering common source kinds + TTL patterns.

INSERT INTO ah_core.lazy_instruction_default_template
    (slug, name, description, source_kind, ttl_seconds,
     target_use_case, recommended_for_tenant_kind,
     requires_admin_review, is_recommended, sort_order)
VALUES
    ('stable-rule-catalog',
     'Stable Rule Catalog',
     'Platform-stable rule catalog from ah_core seed. TTL=0 (never expires) — rule catalog only changes on platform deploy. Load once, cache forever until explicit Invalidate.',
     'ah_core_seed', 0,
     'rule_lookup', 'general', FALSE, TRUE, 10),

    ('tenant-config-medium-ttl',
     'Tenant Config (Medium TTL)',
     'Tenant platform settings that change occasionally (e.g. tone defaults, compliance mode). TTL=1h to pick up updates within an hour without hammering DB on every Load.',
     'tenant_db', 3600,
     'config_lookup', 'general', FALSE, TRUE, 20),

    ('external-kb-short-ttl',
     'External KB (Short TTL)',
     'External knowledge base content that changes frequently (live docs, wikis). TTL=5min ensures fresh-enough content without wasting load cost.',
     'external_http', 300,
     'kb_lookup', 'general', FALSE, TRUE, 30),

    ('regulated-policy-strict-ttl',
     'Regulated Policy (Strict TTL)',
     'Compliance policy documents (GDPR, HIPAA, SOX) that must always reflect latest version. TTL=10min — short enough to catch policy updates quickly. REQUIRES ADMIN REVIEW (regulatory binding).',
     'compliance_store', 600,
     'policy_lookup', 'regulated', TRUE, TRUE, 40),

    ('user-memory-session-ttl',
     'User Memory (Session TTL)',
     'Per-user memory snapshots that change as user interacts. TTL=30min — matches typical session duration. Invalidates manually on user-driven updates.',
     'memory_hierarchy', 1800,
     'user_lookup', 'general', FALSE, TRUE, 50),

    ('dev-debug-no-cache',
     'Dev Debug (No Cache)',
     'Dev profile: TTL=1s effectively disables caching — every Load() invokes fresh. Use ONLY in dev — production tenants would hammer upstream. NOT recommended.',
     'tenant_db', 1,
     'config_lookup', 'dev_local', FALSE, FALSE, 60)
ON CONFLICT (slug) DO NOTHING;
