-- Bug 241: webhook_config tinha campo Token no model Go mas a coluna
-- nunca existiu no schema. Service gerava uuid.New() e o repo INSERT
-- silenciosamente descartava, deixando o endpoint
-- /api/webhooks/{token}/ingest inutilizável.
--
-- Adiciona coluna token + UNIQUE constraint para evitar colisões.
ALTER TABLE webhook_config
    ADD COLUMN IF NOT EXISTS token TEXT NOT NULL DEFAULT '';

-- Backfill de webhooks existentes com tokens distintos.
-- gen_random_uuid() é determinístico aqui — se rodar de novo, novos
-- webhooks ainda recebem token novo via o INSERT (service gera).
UPDATE webhook_config
   SET token = gen_random_uuid()::text
 WHERE token = '';

-- UNIQUE constraint depois do backfill para não quebrar.
CREATE UNIQUE INDEX IF NOT EXISTS webhook_config_token_unique ON webhook_config(token);
