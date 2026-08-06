DROP TRIGGER IF EXISTS trg_package_registry_embedding_pending ON public.package_registry;
DROP FUNCTION IF EXISTS public.mark_package_registry_embedding_pending();

DROP INDEX IF EXISTS public.idx_package_registry_embedding_cosine;
DROP INDEX IF EXISTS public.idx_package_registry_embedding_pending;
DROP INDEX IF EXISTS public.idx_package_registry_tags;

ALTER TABLE public.package_registry
    DROP CONSTRAINT IF EXISTS chk_package_registry_embedding_status;

ALTER TABLE public.package_registry
    DROP COLUMN IF EXISTS embedded_at,
    DROP COLUMN IF EXISTS embedding_error,
    DROP COLUMN IF EXISTS embedding_status,
    DROP COLUMN IF EXISTS embedding_source_hash,
    DROP COLUMN IF EXISTS embedding_model,
    DROP COLUMN IF EXISTS embedding,
    DROP COLUMN IF EXISTS tags;
