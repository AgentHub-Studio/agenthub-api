-- Package Registry tables (Go schema — not present in Java migration).
-- Created as migration 000007 so golang-migrate applies them on first startup.

CREATE TABLE IF NOT EXISTS public.package_registry (
    id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name             VARCHAR(255) NOT NULL,
    slug             VARCHAR(255) NOT NULL UNIQUE,
    description      TEXT,
    type             VARCHAR(50)  NOT NULL,
    visibility       VARCHAR(20)  NOT NULL DEFAULT 'PUBLIC',
    author_tenant_id VARCHAR(255) NOT NULL,
    download_count   INTEGER      NOT NULL DEFAULT 0,
    latest_version   VARCHAR(50),
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_package_registry_slug            ON public.package_registry (slug);
CREATE INDEX IF NOT EXISTS idx_package_registry_type            ON public.package_registry (type);
CREATE INDEX IF NOT EXISTS idx_package_registry_author_tenant   ON public.package_registry (author_tenant_id);
CREATE INDEX IF NOT EXISTS idx_package_registry_visibility      ON public.package_registry (visibility);

CREATE TABLE IF NOT EXISTS public.package_version (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    package_id   UUID         NOT NULL REFERENCES public.package_registry (id) ON DELETE CASCADE,
    version      VARCHAR(50)  NOT NULL,
    changelog    TEXT,
    storage_path TEXT,
    checksum     VARCHAR(255),
    download_count INTEGER     NOT NULL DEFAULT 0,
    published_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    published_by VARCHAR(255) NOT NULL DEFAULT '',
    UNIQUE (package_id, version)
);

CREATE INDEX IF NOT EXISTS idx_package_version_package_id ON public.package_version (package_id);

CREATE TABLE IF NOT EXISTS public.package_dependency (
    id                 UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    package_id         UUID         NOT NULL REFERENCES public.package_registry (id) ON DELETE CASCADE,
    dependency_id      UUID         NOT NULL REFERENCES public.package_registry (id) ON DELETE CASCADE,
    version_constraint VARCHAR(100) NOT NULL DEFAULT '*',
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (package_id, dependency_id)
);

CREATE INDEX IF NOT EXISTS idx_package_dependency_package_id    ON public.package_dependency (package_id);
CREATE INDEX IF NOT EXISTS idx_package_dependency_dependency_id ON public.package_dependency (dependency_id);

CREATE TABLE IF NOT EXISTS public.package_asset (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    package_id   UUID         NOT NULL REFERENCES public.package_registry (id) ON DELETE CASCADE,
    version_id   UUID         REFERENCES public.package_version (id) ON DELETE CASCADE,
    filename     VARCHAR(500) NOT NULL,
    content_type VARCHAR(255) NOT NULL,
    storage_path TEXT         NOT NULL,
    size_bytes   BIGINT       NOT NULL DEFAULT 0,
    checksum     VARCHAR(255),
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_package_asset_package_id ON public.package_asset (package_id);
CREATE INDEX IF NOT EXISTS idx_package_asset_version_id ON public.package_asset (version_id);
