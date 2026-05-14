-- EXT-001-paired: extension descriptor default templates.
-- 5 pre-vetted extension shapes spanning source × component variety
-- so fresh tenants can install reference extensions without inventing
-- descriptors from scratch.

CREATE TABLE IF NOT EXISTS ah_core.extension_descriptor_default_template (
    id                              UUID PRIMARY KEY,
    slug                            TEXT NOT NULL UNIQUE,
    name                            TEXT NOT NULL,
    description                     TEXT NOT NULL,
    target_source                   TEXT NOT NULL,
    target_version                  TEXT NOT NULL,
    target_use_case                 TEXT NOT NULL,
    safety_posture                  TEXT NOT NULL,
    components_offered              JSONB NOT NULL DEFAULT '[]'::jsonb,
    checksum_required               BOOLEAN NOT NULL DEFAULT FALSE,
    auto_enable_after_install       BOOLEAN NOT NULL DEFAULT FALSE,
    recommended_for_tenant_kind     TEXT NOT NULL DEFAULT 'general',
    requires_admin_review           BOOLEAN NOT NULL DEFAULT FALSE,
    is_recommended                  BOOLEAN NOT NULL DEFAULT FALSE,
    is_active                       BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order                      INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_extension_desc_dt_source
    ON ah_core.extension_descriptor_default_template(target_source);
CREATE INDEX IF NOT EXISTS idx_extension_desc_dt_use_case
    ON ah_core.extension_descriptor_default_template(target_use_case);
CREATE INDEX IF NOT EXISTS idx_extension_desc_dt_active
    ON ah_core.extension_descriptor_default_template(is_active);
CREATE INDEX IF NOT EXISTS idx_extension_desc_dt_recommended
    ON ah_core.extension_descriptor_default_template(is_recommended)
    WHERE is_recommended = TRUE;

INSERT INTO ah_core.extension_descriptor_default_template
    (id, slug, name, description, target_source, target_version, target_use_case,
     safety_posture, components_offered, checksum_required,
     auto_enable_after_install,
     recommended_for_tenant_kind, requires_admin_review,
     is_recommended, is_active, sort_order)
VALUES
    ('dddddddd-dddd-dddd-dddd-000000000001',
     'builtin-essentials',
     'Builtin Essentials',
     'Bundled-with-platform extension shipping the baseline agents, tools, and skills every tenant needs out of the box. Source=builtin means no marketplace round-trip; auto-enabled at install since the platform signs and ships it.',
     'builtin', '1.0.0', 'platform_baseline',
     'balanced',
     '["agents","tools","skills","commands"]'::jsonb,
     FALSE, TRUE,
     'general', FALSE, TRUE, TRUE, 10),

    ('dddddddd-dddd-dddd-dddd-000000000002',
     'marketplace-rag-pack',
     'Marketplace — RAG Pack',
     'Knowledge-base and document search bundle from the curated marketplace: ships document_search skill, kb_query tool, and an indexer hook. Checksum verification required since source is external.',
     'marketplace', '2.1.0', 'rag_search',
     'balanced',
     '["skills","tools","hooks"]'::jsonb,
     TRUE, FALSE,
     'general', FALSE, TRUE, TRUE, 20),

    ('dddddddd-dddd-dddd-dddd-000000000003',
     'marketplace-engineering-pack',
     'Marketplace — Engineering Pack',
     'Engineering toolset bundle: code-review agent, test-runner skill, lint hooks, refactor commands. Marketplace-sourced with checksum and admin review (broader surface than RAG pack).',
     'marketplace', '1.5.0', 'engineering',
     'balanced',
     '["agents","skills","hooks","commands"]'::jsonb,
     TRUE, FALSE,
     'general', TRUE, TRUE, TRUE, 30),

    ('dddddddd-dddd-dddd-dddd-000000000004',
     'git-internal-tools',
     'Git — Internal Tools',
     'Reference template for tenants installing extensions from their own private git repos. Source=git requires checksum; admin review since the platform cannot vet internal code at install time.',
     'git', '0.1.0', 'internal_tooling',
     'conservative',
     '["tools","skills"]'::jsonb,
     TRUE, FALSE,
     'general', TRUE, TRUE, TRUE, 40),

    ('dddddddd-dddd-dddd-dddd-000000000005',
     'url-vendor-skills',
     'URL — Vendor Skills',
     'Reference template for skills delivered via signed-URL distribution (most-conservative posture). Checksum mandatory; admin review; never auto-enabled. Used for ad-hoc vendor deliveries before they reach the marketplace.',
     'url', '0.0.1', 'vendor_delivery',
     'strict',
     '["skills"]'::jsonb,
     TRUE, FALSE,
     'general', TRUE, TRUE, TRUE, 50)
ON CONFLICT (slug) DO NOTHING;
