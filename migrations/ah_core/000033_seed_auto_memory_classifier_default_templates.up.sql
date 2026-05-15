-- Seed platform-managed AUTO MEMORY CLASSIFIER DEFAULT TEMPLATES in ah_core.
-- Templates are blueprints paired with CTX-006 AutoMemoryConfig. Each
-- row declares (min_confidence, max_per_turn, admin_blocked_keys) so
-- fresh tenants pick a posture (strict / balanced / lenient + privacy/
-- pii / dev-debug) without inventing thresholds.

CREATE TABLE IF NOT EXISTS ah_core.auto_memory_classifier_default_template (
    id                          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    -- slug is the unique template identifier (kebab-case).
    slug                        VARCHAR(96)  NOT NULL UNIQUE,
    name                        VARCHAR(160) NOT NULL,
    description                 TEXT         NOT NULL,
    -- min_confidence in [0, 1] mirrors AutoMemoryConfig.MinConfidence.
    min_confidence              DOUBLE PRECISION NOT NULL,
    -- max_per_turn caps decisions per turn (0 = unlimited).
    max_per_turn                INTEGER      NOT NULL,
    -- admin_blocked_keys is a comma-separated list of keys NEVER stored.
    admin_blocked_keys          TEXT         NOT NULL,
    -- target_posture classifies the profile (strict/balanced/lenient/
    -- privacy_first/pii_strict/dev_debug).
    target_posture              VARCHAR(48)  NOT NULL,
    -- recommended_for_tenant_kind hints the audience.
    recommended_for_tenant_kind VARCHAR(48)  NOT NULL,
    -- requires_admin_review marks templates with audit implications.
    requires_admin_review       BOOLEAN      NOT NULL DEFAULT FALSE,
    is_recommended              BOOLEAN      NOT NULL DEFAULT FALSE,
    is_active                   BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_order                  INTEGER      NOT NULL DEFAULT 0,
    created_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_amcd_posture  ON ah_core.auto_memory_classifier_default_template (target_posture);
CREATE INDEX IF NOT EXISTS idx_ah_core_amcd_active   ON ah_core.auto_memory_classifier_default_template (is_active);
CREATE INDEX IF NOT EXISTS idx_ah_core_amcd_rec      ON ah_core.auto_memory_classifier_default_template (is_recommended) WHERE is_recommended = TRUE;

-- Seed catalog: 6 profiles covering strictness ladder + privacy-first +
-- pii-strict + dev-debug. Block lists progressively widen.

INSERT INTO ah_core.auto_memory_classifier_default_template
    (slug, name, description, min_confidence, max_per_turn, admin_blocked_keys,
     target_posture, recommended_for_tenant_kind, requires_admin_review, is_recommended, sort_order)
VALUES
    ('balanced-default',
     'Balanced Default',
     'Mirrors CTX-006 DefaultAutoMemoryConfig — safe one-click for fresh tenants. min_confidence=0.5 / max_per_turn=10 / standard sensitive-keys block list.',
     0.5, 10,
     'password,credit_card,ssn,api_key,secret',
     'balanced', 'general', FALSE, TRUE, 10),

    ('strict-conservative',
     'Strict Conservative (high signal only)',
     'Strict posture: only high-confidence decisions stored, low per-turn cap. Use for production tenants where memory store quality outweighs coverage.',
     0.8, 5,
     'password,credit_card,ssn,api_key,secret,token,bearer',
     'strict', 'general', FALSE, TRUE, 20),

    ('lenient-exploration',
     'Lenient Exploration',
     'Lenient posture for low-stakes tenants exploring auto-memory: low threshold + larger per-turn cap. May produce some noise; useful for early product discovery.',
     0.3, 25,
     'password,credit_card,ssn,api_key,secret',
     'lenient', 'general', FALSE, TRUE, 30),

    ('privacy-first',
     'Privacy First',
     'Privacy-first profile for tenants prioritizing user data minimization. Higher confidence threshold + extended block list (PII categories beyond sensitive auth secrets). REQUIRES ADMIN REVIEW (org-wide privacy implications).',
     0.7, 5,
     'password,credit_card,ssn,api_key,secret,token,bearer,email,phone,address,birthdate,national_id,health,medication,diagnosis',
     'privacy_first', 'regulated', TRUE, TRUE, 40),

    ('pii-strict',
     'PII-Strict (HIPAA/GDPR-aligned)',
     'PII-strict posture for HIPAA/GDPR-regulated tenants: aggressive block list including PHI + PII categories; very high confidence to avoid mis-classification. REQUIRES ADMIN REVIEW (compliance binding).',
     0.85, 3,
     'password,credit_card,ssn,api_key,secret,token,bearer,email,phone,address,birthdate,national_id,health,medication,diagnosis,phi,patient_id,medical_record,insurance_number,salary,compensation,bank_account',
     'pii_strict', 'regulated', TRUE, TRUE, 50),

    ('dev-debug',
     'Dev Debug (verbose)',
     'Development/debug posture: very low threshold + no per-turn cap + minimal block list. Use ONLY in dev environments — exposes much, including low-quality decisions. NOT recommended for production.',
     0.1, 0,
     'password',
     'dev_debug', 'dev_local', FALSE, FALSE, 60)
ON CONFLICT (slug) DO NOTHING;
