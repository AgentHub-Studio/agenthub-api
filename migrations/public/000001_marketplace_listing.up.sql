CREATE TABLE IF NOT EXISTS public.marketplace_listing (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       VARCHAR(255) NOT NULL,
    package_id      UUID        NOT NULL,
    package_name    VARCHAR(255) NOT NULL,
    package_slug    VARCHAR(255) NOT NULL UNIQUE,
    package_type    VARCHAR(50)  NOT NULL,
    description     TEXT,
    version         VARCHAR(50)  NOT NULL,
    author_name     VARCHAR(255),
    tags            TEXT[]       NOT NULL DEFAULT '{}',
    category        VARCHAR(255),
    visibility      VARCHAR(20)  NOT NULL DEFAULT 'PUBLIC',
    status          VARCHAR(20)  NOT NULL DEFAULT 'ACTIVE',
    download_count  INTEGER      NOT NULL DEFAULT 0,
    avg_rating      NUMERIC(3,2) NOT NULL DEFAULT 0.0,
    review_count    INTEGER      NOT NULL DEFAULT 0,
    published_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_marketplace_listing_tenant_id   ON public.marketplace_listing (tenant_id);
CREATE INDEX IF NOT EXISTS idx_marketplace_listing_package_type ON public.marketplace_listing (package_type);
CREATE INDEX IF NOT EXISTS idx_marketplace_listing_category     ON public.marketplace_listing (category);
CREATE INDEX IF NOT EXISTS idx_marketplace_listing_status       ON public.marketplace_listing (status);
