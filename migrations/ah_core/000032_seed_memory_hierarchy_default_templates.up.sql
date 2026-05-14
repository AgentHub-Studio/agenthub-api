-- Seed platform-managed MEMORY HIERARCHY DEFAULT TEMPLATES in ah_core.
-- Templates are blueprints paired with CTX-003 MemoryHierarchy. Each
-- row declares a baseline (key, value, scope, max_age) that fresh
-- tenants instantiate on bootstrap so first-run lookups have something
-- to return at the global/tenant layer instead of NotFound.

CREATE TABLE IF NOT EXISTS ah_core.memory_hierarchy_default_template (
    id                          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    -- slug is the unique template identifier (kebab-case).
    slug                        VARCHAR(96)  NOT NULL UNIQUE,
    -- target_scope MATCHES CTX-003 MemoryHierarchyScope enum byte-for-byte.
    -- Templates only target widely-applicable scopes: global / tenant.
    target_scope                VARCHAR(32)  NOT NULL,
    -- key is the lookup key under which the value will be stored.
    key                         VARCHAR(96)  NOT NULL,
    -- default_value is the initial value the template seeds.
    default_value               TEXT         NOT NULL,
    -- max_age_seconds is how long the seeded fact stays live (0 = never expire).
    max_age_seconds             INTEGER      NOT NULL DEFAULT 0,
    -- description explains why this default exists.
    description                 TEXT         NOT NULL,
    -- recommended_for_tenant_kind hints which tenant type benefits most
    -- (general/regulated/dev_local).
    recommended_for_tenant_kind VARCHAR(48)  NOT NULL,
    -- requires_admin_review flags templates the admin must vet before
    -- bootstrap (e.g. compliance_mode default has audit implications).
    requires_admin_review       BOOLEAN      NOT NULL DEFAULT FALSE,
    -- is_recommended marks safe one-click defaults.
    is_recommended              BOOLEAN      NOT NULL DEFAULT FALSE,
    is_active                   BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_order                  INTEGER      NOT NULL DEFAULT 0,
    created_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_mhdt_scope    ON ah_core.memory_hierarchy_default_template (target_scope);
CREATE INDEX IF NOT EXISTS idx_ah_core_mhdt_key      ON ah_core.memory_hierarchy_default_template (key);
CREATE INDEX IF NOT EXISTS idx_ah_core_mhdt_active   ON ah_core.memory_hierarchy_default_template (is_active);
CREATE INDEX IF NOT EXISTS idx_ah_core_mhdt_rec      ON ah_core.memory_hierarchy_default_template (is_recommended) WHERE is_recommended = TRUE;

-- Seed catalog: 8 baseline defaults covering platform constants
-- (global) + tenant-tunable defaults that fresh tenants typically want.
-- target_scope MUST be one of CTX-003 scopes (5 valid; we only seed
-- global + tenant since session/user/agent are subject-specific).

INSERT INTO ah_core.memory_hierarchy_default_template
    (slug, target_scope, key, default_value, max_age_seconds, description,
     recommended_for_tenant_kind, requires_admin_review, is_recommended, sort_order)
VALUES
    -- global platform constants (4): never expire.
    ('global-platform-name',
     'global', 'platform_name', 'AgentHub', 0,
     'Platform brand name. Used in system prompts and UI greeting templates.',
     'general', FALSE, TRUE, 10),

    ('global-platform-version',
     'global', 'platform_version', '1.0.0', 0,
     'Platform schema/feature-set version. LLM uses to gate version-specific behavior.',
     'general', FALSE, TRUE, 20),

    ('global-default-locale',
     'global', 'default_locale', 'en-US', 0,
     'Fallback BCP 47 locale when user/session has none specified. Mirrors locale_translations seed default.',
     'general', FALSE, TRUE, 30),

    ('global-support-email',
     'global', 'support_email', 'support@agenthub.example', 0,
     'Platform-wide support contact. Override per-tenant for white-label deployments.',
     'general', FALSE, TRUE, 40),

    -- tenant defaults (4): expire after a reasonable window so the
    -- agent re-evaluates if tenant ops change something.
    ('tenant-default-tone',
     'tenant', 'tone', 'professional', 2592000,
     'Default communication tone for tenant agents. Override per-user via memory hierarchy.',
     'general', FALSE, TRUE, 50),

    ('tenant-default-timezone',
     'tenant', 'timezone', 'America/Sao_Paulo', 2592000,
     'Default IANA timezone for tenant scheduling and date rendering. Tenants in other regions override.',
     'general', FALSE, TRUE, 60),

    ('tenant-support-window',
     'tenant', 'support_window', '08:00-18:00', 2592000,
     'Default tenant support hours window. Customer-support agents use to set expectations.',
     'general', FALSE, TRUE, 70),

    ('tenant-compliance-mode',
     'tenant', 'compliance_mode', 'standard', 7776000,
     'Tenant compliance posture: standard / strict. Strict enables additional audit + admin-review gates per GOV-002.',
     'regulated', TRUE, FALSE, 80)
ON CONFLICT (slug) DO NOTHING;
