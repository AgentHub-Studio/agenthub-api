-- Semantic Registry Search keeps the pgvector type globally visible. The
-- conditional move also repairs older installations that created it in a
-- tenant schema before public.package_registry started using vectors.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_extension e
        JOIN pg_namespace n ON n.oid = e.extnamespace
        WHERE e.extname = 'vector' AND n.nspname <> 'public'
    ) THEN
        ALTER EXTENSION vector SET SCHEMA public;
    ELSE
        CREATE EXTENSION IF NOT EXISTS vector WITH SCHEMA public;
    END IF;
END $$;

ALTER TABLE public.package_registry
    ADD COLUMN IF NOT EXISTS tags TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS embedding public.vector(1024),
    ADD COLUMN IF NOT EXISTS embedding_model VARCHAR(255),
    ADD COLUMN IF NOT EXISTS embedding_source_hash TEXT,
    ADD COLUMN IF NOT EXISTS embedding_status VARCHAR(16) NOT NULL DEFAULT 'PENDING',
    ADD COLUMN IF NOT EXISTS embedding_error TEXT,
    ADD COLUMN IF NOT EXISTS embedded_at TIMESTAMPTZ;

ALTER TABLE public.package_registry
    DROP CONSTRAINT IF EXISTS chk_package_registry_embedding_status;

ALTER TABLE public.package_registry
    ADD CONSTRAINT chk_package_registry_embedding_status
    CHECK (embedding_status IN ('PENDING', 'PROCESSING', 'COMPLETED', 'FAILED'));

UPDATE public.package_registry
   SET embedding_status = 'PENDING'
 WHERE embedding IS NULL;

CREATE INDEX IF NOT EXISTS idx_package_registry_tags
    ON public.package_registry USING GIN (tags);

CREATE INDEX IF NOT EXISTS idx_package_registry_embedding_pending
    ON public.package_registry (embedding_status, updated_at)
    WHERE embedding_status IN ('PENDING', 'FAILED');

CREATE INDEX IF NOT EXISTS idx_package_registry_embedding_cosine
    ON public.package_registry USING HNSW (embedding vector_cosine_ops)
    WHERE embedding IS NOT NULL;

CREATE OR REPLACE FUNCTION public.mark_package_registry_embedding_pending()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.embedding = NULL;
    NEW.embedding_model = NULL;
    NEW.embedding_source_hash = NULL;
    NEW.embedding_status = 'PENDING';
    NEW.embedding_error = NULL;
    NEW.embedded_at = NULL;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_package_registry_embedding_pending ON public.package_registry;

CREATE TRIGGER trg_package_registry_embedding_pending
BEFORE INSERT OR UPDATE OF name, slug, description, tags, latest_version
ON public.package_registry
FOR EACH ROW
EXECUTE FUNCTION public.mark_package_registry_embedding_pending();
