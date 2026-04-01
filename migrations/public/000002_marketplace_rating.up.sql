CREATE TABLE IF NOT EXISTS public.marketplace_rating (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  VARCHAR(255) NOT NULL,
    listing_id UUID         NOT NULL REFERENCES public.marketplace_listing(id) ON DELETE CASCADE,
    rating     INTEGER      NOT NULL CHECK (rating BETWEEN 1 AND 5),
    comment    TEXT,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, listing_id)
);

CREATE INDEX IF NOT EXISTS idx_marketplace_rating_listing_id ON public.marketplace_rating (listing_id);
CREATE INDEX IF NOT EXISTS idx_marketplace_rating_tenant_id  ON public.marketplace_rating (tenant_id);
