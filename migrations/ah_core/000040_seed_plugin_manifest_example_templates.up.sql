-- Seed platform-managed PLUGIN MANIFEST EXAMPLE TEMPLATES in ah_core.
-- Templates are skeleton/example manifests paired with EXT-004
-- PluginManifest. Each row provides a JSON template (one per kind)
-- that fresh extension authors copy + customize instead of inventing
-- the manifest shape from scratch.

CREATE TABLE IF NOT EXISTS ah_core.plugin_manifest_example_template (
    id                          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    slug                        VARCHAR(96)  NOT NULL UNIQUE,
    name                        VARCHAR(160) NOT NULL,
    description                 TEXT         NOT NULL,
    -- manifest_kind MATCHES EXT-004 PluginManifestKind enum byte-for-byte.
    manifest_kind               VARCHAR(48)  NOT NULL,
    -- example_manifest_json is the literal JSON skeleton authors copy.
    example_manifest_json       TEXT         NOT NULL,
    -- target_audience identifies who the example is for.
    target_audience             VARCHAR(48)  NOT NULL,
    -- recommended_for_tenant_kind hints the audience.
    recommended_for_tenant_kind VARCHAR(48)  NOT NULL,
    requires_admin_review       BOOLEAN      NOT NULL DEFAULT FALSE,
    is_recommended              BOOLEAN      NOT NULL DEFAULT FALSE,
    is_active                   BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_order                  INTEGER      NOT NULL DEFAULT 0,
    created_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_pmet_kind     ON ah_core.plugin_manifest_example_template (manifest_kind);
CREATE INDEX IF NOT EXISTS idx_ah_core_pmet_active   ON ah_core.plugin_manifest_example_template (is_active);
CREATE INDEX IF NOT EXISTS idx_ah_core_pmet_rec      ON ah_core.plugin_manifest_example_template (is_recommended) WHERE is_recommended = TRUE;

-- Seed catalog: 7 example manifests, one per EXT-004 PluginManifestKind.

INSERT INTO ah_core.plugin_manifest_example_template
    (slug, name, description, manifest_kind, example_manifest_json,
     target_audience, recommended_for_tenant_kind,
     requires_admin_review, is_recommended, sort_order)
VALUES
    ('example-agent-pack',
     'Example: Agent Pack',
     'Skeleton manifest for a plugin that bundles a single agent persona. Copy + customize id, name, version, then declare the agent component.',
     'agent',
     '{"schemaVersion":"1.0.0","id":"example-vendor/agent-pack","name":"Example Agent Pack","version":"0.1.0","description":"Replace with your agent description.","author":"your-name","kind":"agent","minPlatformVersion":"1.0.0","components":[{"kind":"agents","slug":"example-agent","entryPath":"agents/example.yaml"}],"license":"Apache-2.0"}',
     'extension_author', 'general', FALSE, TRUE, 10),

    ('example-skill-pack',
     'Example: Skill Pack',
     'Skeleton for a plugin bundling multiple skills (with their tool bindings). Most common manifest kind in marketplaces.',
     'skill_pack',
     '{"schemaVersion":"1.0.0","id":"example-vendor/skill-pack","name":"Example Skill Pack","version":"0.1.0","description":"Replace with your skill-pack description.","author":"your-name","kind":"skill_pack","minPlatformVersion":"1.0.0","components":[{"kind":"skills","slug":"skill-one","entryPath":"skills/one.yaml"},{"kind":"skills","slug":"skill-two","entryPath":"skills/two.yaml"}],"license":"Apache-2.0"}',
     'extension_author', 'general', FALSE, TRUE, 20),

    ('example-tool-pack',
     'Example: Tool Pack',
     'Skeleton for a plugin bundling only tools (no skills). Useful for utility libraries (HTTP fetchers, math, JSON manipulators).',
     'tool_pack',
     '{"schemaVersion":"1.0.0","id":"example-vendor/tool-pack","name":"Example Tool Pack","version":"0.1.0","description":"Replace with your tool-pack description.","author":"your-name","kind":"tool_pack","minPlatformVersion":"1.0.0","components":[{"kind":"tools","slug":"example-tool","entryPath":"tools/example.yaml"}],"license":"Apache-2.0"}',
     'extension_author', 'general', FALSE, TRUE, 30),

    ('example-hook-pack',
     'Example: Hook Pack',
     'Skeleton for a plugin bundling hooks (lifecycle event handlers). Authors declare which event(s) to listen on and provide handler entry points.',
     'hook_pack',
     '{"schemaVersion":"1.0.0","id":"example-vendor/hook-pack","name":"Example Hook Pack","version":"0.1.0","description":"Replace with your hook-pack description.","author":"your-name","kind":"hook_pack","minPlatformVersion":"1.0.0","components":[{"kind":"hooks","slug":"on-pretooluse","entryPath":"hooks/pre-tool-use.yaml"}],"license":"Apache-2.0"}',
     'extension_author', 'general', FALSE, TRUE, 40),

    ('example-rule-pack',
     'Example: Rule Pack',
     'Skeleton for a plugin bundling path-scoped or global rules. Authors declare rule scope + content + path glob.',
     'rule_pack',
     '{"schemaVersion":"1.0.0","id":"example-vendor/rule-pack","name":"Example Rule Pack","version":"0.1.0","description":"Replace with your rule-pack description.","author":"your-name","kind":"rule_pack","minPlatformVersion":"1.0.0","components":[{"kind":"rules","slug":"example-rule","entryPath":"rules/example.yaml"}],"license":"Apache-2.0"}',
     'extension_author', 'general', FALSE, TRUE, 50),

    ('example-theme-pack',
     'Example: Theme Pack (Output Styles)',
     'Skeleton for a plugin bundling output styles + UI presets. Authors declare style slugs and entry templates.',
     'theme_pack',
     '{"schemaVersion":"1.0.0","id":"example-vendor/theme-pack","name":"Example Theme Pack","version":"0.1.0","description":"Replace with your theme-pack description.","author":"your-name","kind":"theme_pack","minPlatformVersion":"1.0.0","components":[{"kind":"output_styles","slug":"example-style","entryPath":"styles/example.yaml"}],"license":"Apache-2.0"}',
     'extension_author', 'general', FALSE, TRUE, 60),

    ('example-bundle-meta',
     'Example: Bundle (Composition Meta-Plugin)',
     'Skeleton for a bundle plugin that composes dependencies without its own components. Useful for meta-distributions ("recommended starter pack").',
     'bundle',
     '{"schemaVersion":"1.0.0","id":"example-vendor/starter-bundle","name":"Example Starter Bundle","version":"0.1.0","description":"Composition of recommended plugins; no own components.","author":"your-name","kind":"bundle","minPlatformVersion":"1.0.0","components":[],"dependencies":[{"slug":"example-vendor/skill-pack","minVersion":"0.1.0"},{"slug":"example-vendor/tool-pack","minVersion":"0.1.0"}],"license":"Apache-2.0"}',
     'extension_author', 'general', FALSE, TRUE, 70)
ON CONFLICT (slug) DO NOTHING;
