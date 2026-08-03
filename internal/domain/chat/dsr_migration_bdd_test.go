package chat_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDSR01Migrations_CoverRequiredUpSchemaChanges(t *testing.T) {
	schemaUp := readMigrationFile(t, "schemas", "000084_dynamic_skill_retrieval.up.sql")
	publicUp := readMigrationFile(t, "public", "000017_tenant_chat_default.up.sql")

	assert.Contains(t, schemaUp, "ADD COLUMN IF NOT EXISTS embedding             vector(1024)")
	assert.Contains(t, schemaUp, "ADD COLUMN IF NOT EXISTS embedding_source_hash VARCHAR(64)")
	assert.Contains(t, schemaUp, "ADD COLUMN IF NOT EXISTS embedded_at           TIMESTAMPTZ")
	assert.Contains(t, schemaUp, "ADD COLUMN IF NOT EXISTS mode             VARCHAR(32) NOT NULL DEFAULT 'AGENT'")
	assert.Contains(t, schemaUp, "ADD COLUMN IF NOT EXISTS persona_id       UUID")
	assert.Contains(t, schemaUp, "ADD COLUMN IF NOT EXISTS sticky_skill_set JSONB       NOT NULL DEFAULT '{}'::jsonb")
	assert.Contains(t, schemaUp, "ALTER COLUMN agent_id DROP NOT NULL")

	assert.Contains(t, publicUp, "CREATE TABLE IF NOT EXISTS public.tenant_chat_default")
	assert.Contains(t, publicUp, "tenant_id        VARCHAR(255) PRIMARY KEY REFERENCES public.tenants (id) ON DELETE CASCADE")
	assert.Contains(t, publicUp, "mode             VARCHAR(32)  NOT NULL DEFAULT 'AGENT'")
	assert.Contains(t, publicUp, "sticky_skill_set JSONB        NOT NULL DEFAULT '[]'::jsonb")
	assert.Contains(t, publicUp, "INSERT INTO public.tenant_chat_default (tenant_id)")
	assert.Contains(t, publicUp, "ON CONFLICT (tenant_id) DO NOTHING")
}

func TestDSR01Migrations_CoverDownReversalWithoutDeletingTenantData(t *testing.T) {
	schemaDown := readMigrationFile(t, "schemas", "000084_dynamic_skill_retrieval.down.sql")
	publicDown := readMigrationFile(t, "public", "000017_tenant_chat_default.down.sql")

	assert.Contains(t, schemaDown, "DROP COLUMN IF EXISTS sticky_skill_set")
	assert.Contains(t, schemaDown, "DROP COLUMN IF EXISTS persona_id")
	assert.Contains(t, schemaDown, "DROP COLUMN IF EXISTS mode")
	assert.Contains(t, schemaDown, "WHERE agent_id IS NULL LIMIT 1")
	assert.Contains(t, schemaDown, "ALTER TABLE chat_session ALTER COLUMN agent_id SET NOT NULL")
	assert.Contains(t, schemaDown, "DROP COLUMN IF EXISTS embedded_at")
	assert.Contains(t, schemaDown, "DROP COLUMN IF EXISTS embedding_source_hash")
	assert.Contains(t, schemaDown, "DROP COLUMN IF EXISTS embedding")

	assert.Equal(t, "DROP TABLE IF EXISTS public.tenant_chat_default;", strings.TrimSpace(publicDown))
}

func readMigrationFile(t *testing.T, scope, name string) string {
	t.Helper()

	path := filepath.Join("..", "..", "..", "migrations", scope, name)
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(content)
}
