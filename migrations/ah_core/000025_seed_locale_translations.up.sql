-- Seed platform-managed LOCALE TRANSLATIONS in ah_core.
-- Translations are i18n strings the agent uses for tone-appropriate
-- responses in user's preferred language (linked via FUTURE-001 user
-- memory + FUTURE-002 communication style).

CREATE TABLE IF NOT EXISTS ah_core.locale_translation (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    -- locale_code is BCP 47 (e.g. "en-US", "pt-BR", "es-ES").
    locale_code     VARCHAR(16)  NOT NULL,
    -- key is the translation key (dotted-path namespace).
    key             VARCHAR(128) NOT NULL,
    -- value is the localized string.
    value           TEXT         NOT NULL,
    -- category groups related translations.
    category        VARCHAR(32)  NOT NULL,
    is_active       BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (locale_code, key)
);

CREATE INDEX IF NOT EXISTS idx_ah_core_locale_translation_locale ON ah_core.locale_translation (locale_code);
CREATE INDEX IF NOT EXISTS idx_ah_core_locale_translation_key    ON ah_core.locale_translation (key);
CREATE INDEX IF NOT EXISTS idx_ah_core_locale_translation_active ON ah_core.locale_translation (is_active);
CREATE INDEX IF NOT EXISTS idx_ah_core_locale_translation_cat    ON ah_core.locale_translation (category);

-- ============================
-- 4 locales × 6 translation keys = 24 rows
-- Locales: en-US (default), pt-BR, es-ES, fr-FR
-- Categories: greeting / acknowledgement / error / confirmation /
--             progress / closing
-- ============================

-- en-US (English, US)
INSERT INTO ah_core.locale_translation (locale_code, key, value, category) VALUES
('en-US', 'greeting.hello', 'Hello! How can I help you today?', 'greeting'),
('en-US', 'acknowledgement.understood', 'Got it. Working on that now.', 'acknowledgement'),
('en-US', 'error.generic', 'Something went wrong. Let me try a different approach.', 'error'),
('en-US', 'confirmation.proceed', 'Are you sure you want to proceed?', 'confirmation'),
('en-US', 'progress.thinking', 'Thinking through this...', 'progress'),
('en-US', 'closing.farewell', 'Done. Let me know if you need anything else.', 'closing');

-- pt-BR (Portuguese, Brazil)
INSERT INTO ah_core.locale_translation (locale_code, key, value, category) VALUES
('pt-BR', 'greeting.hello', 'Olá! Como posso ajudar você hoje?', 'greeting'),
('pt-BR', 'acknowledgement.understood', 'Entendi. Vou trabalhar nisso agora.', 'acknowledgement'),
('pt-BR', 'error.generic', 'Algo deu errado. Vou tentar uma abordagem diferente.', 'error'),
('pt-BR', 'confirmation.proceed', 'Tem certeza que deseja continuar?', 'confirmation'),
('pt-BR', 'progress.thinking', 'Analisando isso...', 'progress'),
('pt-BR', 'closing.farewell', 'Concluído. Me avise se precisar de algo mais.', 'closing');

-- es-ES (Spanish, Spain)
INSERT INTO ah_core.locale_translation (locale_code, key, value, category) VALUES
('es-ES', 'greeting.hello', '¡Hola! ¿Cómo puedo ayudarte hoy?', 'greeting'),
('es-ES', 'acknowledgement.understood', 'Entendido. Trabajando en eso ahora.', 'acknowledgement'),
('es-ES', 'error.generic', 'Algo salió mal. Voy a intentar un enfoque diferente.', 'error'),
('es-ES', 'confirmation.proceed', '¿Estás seguro de que quieres continuar?', 'confirmation'),
('es-ES', 'progress.thinking', 'Analizando esto...', 'progress'),
('es-ES', 'closing.farewell', 'Listo. Avísame si necesitas algo más.', 'closing');

-- fr-FR (French, France)
INSERT INTO ah_core.locale_translation (locale_code, key, value, category) VALUES
('fr-FR', 'greeting.hello', 'Bonjour ! Comment puis-je vous aider aujourd''hui ?', 'greeting'),
('fr-FR', 'acknowledgement.understood', 'Compris. Je travaille sur cela maintenant.', 'acknowledgement'),
('fr-FR', 'error.generic', 'Quelque chose s''est mal passé. Je vais essayer une approche différente.', 'error'),
('fr-FR', 'confirmation.proceed', 'Êtes-vous sûr de vouloir continuer ?', 'confirmation'),
('fr-FR', 'progress.thinking', 'Réflexion en cours...', 'progress'),
('fr-FR', 'closing.farewell', 'Terminé. Faites-moi savoir si vous avez besoin d''autre chose.', 'closing');
