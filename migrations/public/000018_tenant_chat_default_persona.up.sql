-- DSR-03: tenant default persona and retrieval defaults.
-- Depends on 000017_tenant_chat_default.

ALTER TABLE public.tenant_chat_default
    ADD COLUMN IF NOT EXISTS id UUID,
    ADD COLUMN IF NOT EXISTS name VARCHAR(200) NOT NULL DEFAULT 'Assistente',
    ADD COLUMN IF NOT EXISTS system_prompt TEXT NOT NULL DEFAULT 'Você é o assistente padrão do AgentHub. Responda em português do Brasil, seja direto e use as ferramentas disponíveis quando elas forem relevantes para a solicitação do usuário.',
    ADD COLUMN IF NOT EXISTS model_config JSONB NOT NULL DEFAULT '{"provider":"","model":"","temperature":0.3}'::jsonb,
    ADD COLUMN IF NOT EXISTS retrieval_config JSONB NOT NULL DEFAULT '{"topK":8,"driftThreshold":0.55,"maxStickySize":15,"allowDrift":true,"refreshPolicy":"drift_or_invalidation","minScore":0.30}'::jsonb,
    ADD COLUMN IF NOT EXISTS enable_management BOOLEAN NOT NULL DEFAULT false;

UPDATE public.tenant_chat_default
SET id = gen_random_uuid()
WHERE id IS NULL;

ALTER TABLE public.tenant_chat_default
    ALTER COLUMN id SET DEFAULT gen_random_uuid(),
    ALTER COLUMN id SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_tenant_chat_default_id
    ON public.tenant_chat_default (id);

UPDATE public.tenant_chat_default
SET retrieval_config = '{"topK":8,"driftThreshold":0.55,"maxStickySize":15,"allowDrift":true,"refreshPolicy":"drift_or_invalidation","minScore":0.30}'::jsonb
WHERE retrieval_config IS NULL
   OR retrieval_config = '{}'::jsonb;
