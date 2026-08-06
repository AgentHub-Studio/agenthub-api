CREATE TABLE IF NOT EXISTS public.tenant_workload_credential (
    tenant_id                VARCHAR(255) PRIMARY KEY REFERENCES public.tenants(id) ON DELETE CASCADE,
    client_id                VARCHAR(255) NOT NULL,
    client_secret_ciphertext TEXT NOT NULL,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
