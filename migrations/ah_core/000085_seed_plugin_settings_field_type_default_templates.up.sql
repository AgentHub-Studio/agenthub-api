CREATE TABLE IF NOT EXISTS ah_core.plugin_settings_field_type_template (
    id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    slug             VARCHAR(32)  NOT NULL UNIQUE,
    label            VARCHAR(64)  NOT NULL,
    description      TEXT         NOT NULL,
    is_maskable      BOOLEAN      NOT NULL DEFAULT false,
    requires_options BOOLEAN      NOT NULL DEFAULT false,
    is_numeric       BOOLEAN      NOT NULL DEFAULT false,
    ui_widget        VARCHAR(32)  NOT NULL,
    sort_order       INTEGER      NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- §6.1 plugin manifest "settings" component type: five field types for
-- structured tenant and user configuration declarations.
-- is_maskable: true = value hidden in UI and excluded from audit logs (secret only)
-- requires_options: true = manifest must declare an options list (select only)
-- is_numeric: true = value must be parseable as a number (number only)
-- ui_widget: frontend widget hint
INSERT INTO ah_core.plugin_settings_field_type_template
    (slug, label, description, is_maskable, requires_options, is_numeric, ui_widget, sort_order)
VALUES
    ('string',
     'Text',
     'Free-text input; stored as a string. Use for URLs, labels, or any short textual configuration value.',
     false, false, false, 'text_input', 0),

    ('boolean',
     'Toggle',
     'Binary on/off switch; stored as true or false. Use for feature enablement flags and simple yes/no choices.',
     false, false, false, 'toggle', 1),

    ('number',
     'Number',
     'Numeric input with optional min/max validation; stored as a number. Use for limits, thresholds, and counts.',
     false, false, true, 'number_input', 2),

    ('select',
     'Dropdown',
     'Single-choice selection from a plugin-declared option list. The manifest MUST include an options array for this field type.',
     false, true, false, 'dropdown', 3),

    ('secret',
     'Secret',
     'Like text but masked in the UI and excluded from logs and audit exports. Use for API keys, tokens, and passwords.',
     true, false, false, 'password_input', 4)
ON CONFLICT (slug) DO NOTHING;
