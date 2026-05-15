-- Out-of-the-box usability fixes for fresh tenants.
--
-- Runs per-tenant via the schemas/ migrator, under the ah_{tenantID} schema's
-- search_path.
--
-- 1. Defensive slug cleanup. The canonical default agent (slug
--    'agenthub-assistant') is seeded by migration 000014. Migration 000063
--    additionally tries to seed an agent with slug 'meu-assistente', but its
--    WHERE NOT EXISTS (SELECT 1 FROM agent) guard means it only fires for a
--    tenant that had ZERO agents when 000063 ran — which normally cannot
--    happen because 000014 runs first. The one exception is a tenant that
--    skipped 000014 due to the golang-migrate high-water-mark behaviour (see
--    000061's comment): such a tenant could end up with a stray
--    'meu-assistente' agent and no 'agenthub-assistant'. This UPDATE realigns
--    that stray row with the well-known slug the chat routing queries prefer.
--    For the normal case (000014 applied) it is a guarded no-op.
UPDATE agent SET slug = 'agenthub-assistant'
WHERE slug = 'meu-assistente'
  AND NOT EXISTS (SELECT 1 FROM agent WHERE slug = 'agenthub-assistant');

-- 2. Seed the default LLM provider so the model resolution chain can actually
--    reach a configured model for a fresh tenant. Migration 000063 seeded
--    openai.model but never seeded llm.defaultProvider, so ResolveDefaultProvider
--    returned empty and the seeded model setting was never read. OpenRouter is
--    the chosen out-of-the-box provider — a single API key unlocks many models.
--    The API key itself is a secret and must still be configured by the admin.
--    Upserts so existing tenants keep whatever they already have.
INSERT INTO settings (key, value, description) VALUES
    ('llm.defaultProvider', '"openrouter"'::jsonb, 'Provedor de LLM padrão para tenants novos (out-of-the-box).'),
    ('openrouter.model', '"mistralai/mistral-nemo"'::jsonb, 'Modelo OpenRouter padrão para tenants novos — estável e validado.')
ON CONFLICT (key) DO NOTHING;
