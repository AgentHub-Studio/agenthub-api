-- Seed a default "Meu Assistente" agent in every tenant schema so a fresh
-- tenant is never met with an empty list. Also seeds the onboarding.completed
-- flag so the admin can render a welcome experience on first login.
--
-- Runs per-tenant via the schemas/ migrator, under the ah_{tenantID} schema's
-- search_path, so no tenant_id column is needed and the row only lands in
-- the tenant's own agent table.

-- Insert default agent only if the tenant has zero agents yet. Tenants with
-- pre-existing agents are left untouched so we don't clutter existing setups.
INSERT INTO agent (name, slug, description, status, config)
SELECT
    'Meu Assistente',
    'meu-assistente',
    'Assistente padrão pronto para conversar com seus sistemas conectados. Personalize as instruções a qualquer momento.',
    'PUBLISHED',
    jsonb_build_object(
        'systemPrompt',
        'Você é um assistente amigável em português do Brasil. Use as ferramentas disponíveis para responder perguntas sobre os sistemas conectados do usuário. Se não souber algo, diga com honestidade.',
        'modelConfig',
        jsonb_build_object(
            'provider', '',
            'model', '',
            'temperature', 0.3
        )
    )
WHERE NOT EXISTS (SELECT 1 FROM agent);

-- Seed sane defaults for LLM provider and onboarding flag. Upserts so existing
-- tenants keep whatever they already have.
INSERT INTO settings (key, value, description) VALUES
    ('openai.model', '"gpt-4o-mini"'::jsonb, 'Default OpenAI model for new tenants — cheap and fast.')
ON CONFLICT (key) DO NOTHING;

INSERT INTO settings (key, value, description) VALUES
    ('onboarding.completed', 'false'::jsonb, 'True once the tenant has connected at least one system and opened the chat.')
ON CONFLICT (key) DO NOTHING;
