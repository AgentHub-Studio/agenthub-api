-- Public defaults for tenant-level chat routing.

CREATE TABLE IF NOT EXISTS public.tenant_chat_default (
    tenant_id        VARCHAR(255) PRIMARY KEY REFERENCES public.tenants (id) ON DELETE CASCADE,
    mode             VARCHAR(32)  NOT NULL DEFAULT 'AGENT',
    agent_id         UUID,
    persona_id       UUID,
    sticky_skill_set JSONB        NOT NULL DEFAULT '[]'::jsonb,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tenant_chat_default_mode
    ON public.tenant_chat_default (mode);

CREATE INDEX IF NOT EXISTS idx_tenant_chat_default_agent_id
    ON public.tenant_chat_default (agent_id)
    WHERE agent_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_tenant_chat_default_persona_id
    ON public.tenant_chat_default (persona_id)
    WHERE persona_id IS NOT NULL;

INSERT INTO public.tenant_chat_default (tenant_id)
SELECT id
FROM public.tenants
ON CONFLICT (tenant_id) DO NOTHING;
