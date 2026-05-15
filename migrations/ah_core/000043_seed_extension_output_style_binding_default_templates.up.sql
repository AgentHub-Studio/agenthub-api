-- Seed platform-managed EXTENSION OUTPUT STYLE BINDING DEFAULT TEMPLATES in ah_core.
-- Templates are blueprints paired with EXT-009 ExtensionOutputStyleRegistry.
-- Each row encodes a (scope + format + style_slug) preset so fresh tenants
-- pick output-style bindings without inventing scope semantics.

CREATE TABLE IF NOT EXISTS ah_core.extension_output_style_binding_default_template (
    id                          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    slug                        VARCHAR(96)  NOT NULL UNIQUE,
    name                        VARCHAR(160) NOT NULL,
    description                 TEXT         NOT NULL,
    -- target_scope MATCHES EXT-009 OutputStyleScope enum byte-for-byte.
    target_scope                VARCHAR(32)  NOT NULL,
    -- target_format MATCHES EXT-009 OutputStyleFormat enum byte-for-byte.
    target_format               VARCHAR(48)  NOT NULL,
    -- style_slug references the underlying ah_core.output_style row OR
    -- an extension-contributed style slug.
    style_slug                  VARCHAR(96)  NOT NULL,
    default_priority            INTEGER      NOT NULL DEFAULT 50,
    target_use_case             VARCHAR(48)  NOT NULL,
    recommended_for_tenant_kind VARCHAR(48)  NOT NULL,
    requires_admin_review       BOOLEAN      NOT NULL DEFAULT FALSE,
    is_recommended              BOOLEAN      NOT NULL DEFAULT FALSE,
    is_active                   BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_order                  INTEGER      NOT NULL DEFAULT 0,
    created_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_eosbd_scope   ON ah_core.extension_output_style_binding_default_template (target_scope);
CREATE INDEX IF NOT EXISTS idx_ah_core_eosbd_format  ON ah_core.extension_output_style_binding_default_template (target_format);
CREATE INDEX IF NOT EXISTS idx_ah_core_eosbd_active  ON ah_core.extension_output_style_binding_default_template (is_active);
CREATE INDEX IF NOT EXISTS idx_ah_core_eosbd_rec     ON ah_core.extension_output_style_binding_default_template (is_recommended) WHERE is_recommended = TRUE;

-- Seed catalog: 6 binding templates covering scope cascade + format spectrum.

INSERT INTO ah_core.extension_output_style_binding_default_template
    (slug, name, description, target_scope, target_format, style_slug,
     default_priority, target_use_case, recommended_for_tenant_kind,
     requires_admin_review, is_recommended, sort_order)
VALUES
    ('platform-conversational-default',
     'Platform Default: Conversational Markdown',
     'Platform-scope binding for the conversational style (markdown). Catches every tenant that has not configured anything else — guarantees no tenant ever has zero rendering options.',
     'platform', 'markdown', 'conversational',
     10, 'general', 'general', FALSE, TRUE, 10),

    ('tenant-technical-default',
     'Tenant Default: Technical (markdown code-first)',
     'Tenant-scope binding for the technical style. Engineering-focused tenants apply this once and every agent picks it up by default.',
     'tenant', 'markdown', 'technical',
     30, 'engineering', 'general', FALSE, TRUE, 20),

    ('agent-extractor-json',
     'Agent Override: Extractor → JSON Only',
     'Agent-scope binding forcing the data-extractor agent to output strict JSON. Overrides tenant default for that one agent — example pattern for vendor-specific output contracts.',
     'agent', 'json', 'json_only',
     50, 'data_extraction', 'general', FALSE, TRUE, 30),

    ('explicit-debug-verbose',
     'Explicit: Debug Verbose (request-bound)',
     'Explicit-scope binding caller can attach to a debug request. Verbose markdown style with full reasoning. Use only when caller requests inspection — overrides everything.',
     'explicit', 'markdown', 'verbose',
     90, 'debugging', 'general', FALSE, TRUE, 40),

    ('tenant-html-sanitized-ui',
     'Tenant: HTML-Sanitized for End-User UI',
     'Tenant-scope binding for end-user-facing UI rendering. Uses html_sanitized format (XSS guard built in). REQUIRES ADMIN REVIEW (rendering security posture).',
     'tenant', 'html_sanitized', 'structured',
     40, 'user_facing_ui', 'general', TRUE, TRUE, 50),

    ('platform-plain-fallback',
     'Platform: Plain Text Fallback',
     'Platform-scope binding for the absolute fallback (plain text, no markdown). Used by clients that cannot render markdown (legacy, screen readers, raw text logs). Lowest priority on purpose.',
     'platform', 'plain', 'concise',
     5, 'fallback', 'general', FALSE, TRUE, 60)
ON CONFLICT (slug) DO NOTHING;
