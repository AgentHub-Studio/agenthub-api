CREATE TABLE IF NOT EXISTS public.tenant_package_installation (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       VARCHAR(255) NOT NULL,
    package_id      UUID         NOT NULL,
    package_version VARCHAR(50)  NOT NULL,
    installed_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    status          VARCHAR(20)  NOT NULL DEFAULT 'INSTALLED'
);

CREATE INDEX IF NOT EXISTS idx_tenant_pkg_install_tenant_id  ON public.tenant_package_installation (tenant_id);
CREATE INDEX IF NOT EXISTS idx_tenant_pkg_install_package_id ON public.tenant_package_installation (package_id);
