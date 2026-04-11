-- Track whether an installation has been fully hydrated (agent/skills/tools provisioned).
ALTER TABLE tenant_package_installation
    ADD COLUMN IF NOT EXISTS hydrated BOOLEAN NOT NULL DEFAULT false;
