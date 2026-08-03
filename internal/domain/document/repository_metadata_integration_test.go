//go:build integration

package document_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/document"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

const documentMetadataTenant = "documentmetadata"

func TestIntegration_DocumentMetadataMigrationAndRepositoryPersistence(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	setupDocumentMetadataSchema(t, pool)
	ctx := tenant.NewContext(context.Background(), documentMetadataTenant)
	repo := document.NewRepository(pool)

	created, err := repo.Create(ctx, document.Document{
		KnowledgeBaseID: uuid.New(),
		FileName:        "release-notes.txt",
		ContentType:     "text/plain",
		Status:          document.StatusPending,
		StoragePath:     "documents/release-notes.txt",
		FileSize:        42,
		Metadata:        json.RawMessage(`{"source":"manual","tags":["release","api"],"year":2026}`),
	})
	require.NoError(t, err)

	found, err := repo.FindByID(ctx, created.ID)
	require.NoError(t, err)
	assert.JSONEq(t, `{"source":"manual","tags":["release","api"],"year":2026}`, string(found.Metadata))

	page, total, err := repo.FindByKnowledgeBase(ctx, created.KnowledgeBaseID, pagination.PageRequest{Page: 0, Size: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, page, 1)
	assert.JSONEq(t, string(created.Metadata), string(page[0].Metadata))

	updated, err := repo.UpdateStatus(ctx, created.ID, document.StatusIndexed)
	require.NoError(t, err)
	assert.Equal(t, document.StatusIndexed, updated.Status)
	assert.JSONEq(t, string(created.Metadata), string(updated.Metadata))

	var indexed bool
	err = pool.QueryRow(context.Background(), `
SELECT EXISTS (
    SELECT 1
    FROM pg_indexes
    WHERE schemaname = $1 AND indexname = 'idx_document_metadata_gin'
)`, "ah_"+documentMetadataTenant).Scan(&indexed)
	require.NoError(t, err)
	assert.True(t, indexed)
}

func setupDocumentMetadataSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	schema := "ah_" + documentMetadataTenant
	_, err := pool.Exec(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err)

	conn, release, err := database.AcquireWithTenant(ctx, pool, documentMetadataTenant)
	require.NoError(t, err)
	defer release()
	_, err = conn.Exec(ctx, `
CREATE TABLE document (
    id UUID PRIMARY KEY,
    knowledge_base_id UUID NOT NULL,
    file_name VARCHAR(500) NOT NULL,
    content_type VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL,
    storage_path TEXT NOT NULL,
    file_size BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
)`)
	require.NoError(t, err)

	path := filepath.Join("..", "..", "..", "migrations", "schemas", "000083_document_metadata.up.sql")
	migration, err := os.ReadFile(path)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, string(migration))
	require.NoError(t, err)
}
