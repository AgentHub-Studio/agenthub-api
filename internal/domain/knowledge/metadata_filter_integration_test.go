//go:build integration

package knowledge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

const (
	metadataSearchTenantA = "metadatafiltera"
	metadataSearchTenantB = "metadatafilterb"
)

func TestIntegration_DocumentSearchMetadataFilter_VectorLexicalAndTenantIsolation(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	setupMetadataSearchSchema(t, pool, metadataSearchTenantA)
	setupMetadataSearchSchema(t, pool, metadataSearchTenantB)

	vectorKB := uuid.New()
	lexicalKB := uuid.New()
	seedMetadataSearchKB(t, pool, metadataSearchTenantA, vectorKB, []metadataSearchDocument{{
		name:     "manual-vector.txt",
		content:  "release policy vector",
		metadata: `{"source":"manual","tags":["release","api"],"year":2026}`,
		vector:   true,
	}})
	seedMetadataSearchKB(t, pool, metadataSearchTenantA, lexicalKB, []metadataSearchDocument{
		{name: "manual-lexical.txt", content: "release policy manual", metadata: `{"source":"manual","tags":["release","api"],"year":2026,"customer":{"region":"BR","tier":2,"labels":["priority","release"]}}`},
		{name: "generated-lexical.txt", content: "release policy generated", metadata: `{"source":"generated","tags":["internal"],"year":2025,"customer":{"region":"US","tier":1,"labels":["internal"]}}`},
		{name: "retired-lexical.txt", content: "release policy retired", metadata: `{"source":"manual","tags":["release"],"retired":true,"customer":{"region":"BR","tier":3,"labels":["priority"]}}`},
	})
	foreignKB := uuid.New()
	seedMetadataSearchKB(t, pool, metadataSearchTenantB, foreignKB, []metadataSearchDocument{{
		name:     "foreign.txt",
		content:  "release policy foreign tenant",
		metadata: `{"source":"manual","tags":["release"]}`,
	}})

	embedding := make([]float32, 1024)
	embedding[0] = 1
	embeddingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/embed", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(embedResponse{Embedding: embedding}))
	}))
	defer embeddingServer.Close()

	client := NewPgDocumentSearchClient(pool, embeddingServer.URL)
	ctx := tenant.NewContext(context.Background(), metadataSearchTenantA)
	manualRelease, err := ParseMetadataFilter(json.RawMessage(`{
  "all":[
    {"field":"source","op":"eq","value":"manual"},
    {"field":"tags","op":"containsAll","value":["release"]},
    {"not":{"field":"retired","op":"exists"}}
  ]
}`))
	require.NoError(t, err)

	vectorResults, err := client.Search(ctx, "release policy", SearchOptions{
		KBIDs:          []uuid.UUID{vectorKB},
		TopK:           5,
		MetadataFilter: manualRelease,
	})
	require.NoError(t, err)
	require.Len(t, vectorResults, 1)
	assert.Equal(t, "manual-vector.txt", vectorResults[0].DocumentName)

	lexicalResults, err := client.Search(ctx, "release policy", SearchOptions{
		KBIDs:          []uuid.UUID{lexicalKB},
		TopK:           5,
		MetadataFilter: manualRelease,
	})
	require.NoError(t, err)
	require.Len(t, lexicalResults, 1)
	assert.Equal(t, "manual-lexical.txt", lexicalResults[0].DocumentName)

	internalTags, err := ParseMetadataFilter(json.RawMessage(`{"field":"tags","op":"containsAny","value":["internal","review"]}`))
	require.NoError(t, err)
	internalResults, err := client.Search(ctx, "release policy", SearchOptions{
		KBIDs:          []uuid.UUID{lexicalKB},
		TopK:           5,
		MetadataFilter: internalTags,
	})
	require.NoError(t, err)
	require.Len(t, internalResults, 1)
	assert.Equal(t, "generated-lexical.txt", internalResults[0].DocumentName)

	emptyContainsAny, err := ParseMetadataFilter(json.RawMessage(`{"field":"tags","op":"containsAny","value":[]}`))
	require.NoError(t, err)
	emptyContainsAnyResults, err := client.Search(ctx, "release policy", SearchOptions{
		KBIDs:          []uuid.UUID{lexicalKB},
		TopK:           5,
		MetadataFilter: emptyContainsAny,
	})
	require.NoError(t, err)
	assert.Empty(t, emptyContainsAnyResults)

	emptyContainsAll, err := ParseMetadataFilter(json.RawMessage(`{"field":"tags","op":"containsAll","value":[]}`))
	require.NoError(t, err)
	emptyContainsAllResults, err := client.Search(ctx, "release policy", SearchOptions{
		KBIDs:          []uuid.UUID{lexicalKB},
		TopK:           5,
		MetadataFilter: emptyContainsAll,
	})
	require.NoError(t, err)
	require.Len(t, emptyContainsAllResults, 3)
	assert.Equal(t, []string{"manual-lexical.txt", "generated-lexical.txt", "retired-lexical.txt"}, documentNames(emptyContainsAllResults))

	unfiltered, err := client.Search(ctx, "release policy", SearchOptions{KBIDs: []uuid.UUID{lexicalKB}, TopK: 5})
	require.NoError(t, err)
	assert.Len(t, unfiltered, 3, "unfiltered lexical behavior must remain unchanged")

	unknownField, err := ParseMetadataFilter(json.RawMessage(`{"field":"missing","op":"eq","value":"value"}`))
	require.NoError(t, err)
	unknownResults, err := client.Search(ctx, "release policy", SearchOptions{
		KBIDs:          []uuid.UUID{lexicalKB},
		TopK:           5,
		MetadataFilter: unknownField,
	})
	require.NoError(t, err)
	assert.Empty(t, unknownResults)

	nestedRegion, err := ParseMetadataFilter(json.RawMessage(`{"field":"customer.region","op":"ilike","value":"br"}`))
	require.NoError(t, err)
	nestedRegionResults, err := client.Search(ctx, "release policy", SearchOptions{
		KBIDs:          []uuid.UUID{lexicalKB},
		TopK:           5,
		MetadataFilter: nestedRegion,
	})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"manual-lexical.txt", "retired-lexical.txt"}, documentNames(nestedRegionResults))

	nestedTier, err := ParseMetadataFilter(json.RawMessage(`{"field":"customer.tier","op":"gte","value":2}`))
	require.NoError(t, err)
	nestedTierResults, err := client.Search(ctx, "release policy", SearchOptions{
		KBIDs:          []uuid.UUID{lexicalKB},
		TopK:           5,
		MetadataFilter: nestedTier,
	})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"manual-lexical.txt", "retired-lexical.txt"}, documentNames(nestedTierResults))

	tenantResults, err := client.Search(ctx, "release policy", SearchOptions{TopK: 10})
	require.NoError(t, err)
	assert.Len(t, tenantResults, 1)
	for _, result := range tenantResults {
		assert.NotEqual(t, "foreign.txt", result.DocumentName)
	}
}

func TestIntegration_DocumentSearchMetadataFilter_PreservesJSONScalarTypes(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	setupMetadataSearchSchema(t, pool, metadataSearchTenantA)

	kbID := uuid.New()
	seedMetadataSearchKB(t, pool, metadataSearchTenantA, kbID, []metadataSearchDocument{
		{name: "year-number.txt", content: "typed metadata policy", metadata: `{"year":2026,"enabled":true}`},
		{name: "year-string.txt", content: "typed metadata policy", metadata: `{"year":"2026","enabled":false}`},
		{name: "year-previous.txt", content: "typed metadata policy", metadata: `{"year":2025}`},
	})

	client := NewPgDocumentSearchClient(pool, "")
	ctx := tenant.NewContext(context.Background(), metadataSearchTenantA)

	filter, err := ParseMetadataFilter(json.RawMessage(`{"field":"year","op":"eq","value":2026}`))
	require.NoError(t, err)
	results, err := client.Search(ctx, "typed metadata policy", SearchOptions{KBIDs: []uuid.UUID{kbID}, TopK: 10, MetadataFilter: filter})
	require.NoError(t, err)
	require.Equal(t, []string{"year-number.txt"}, documentNames(results))

	filter, err = ParseMetadataFilter(json.RawMessage(`{"field":"year","op":"eq","value":"2026"}`))
	require.NoError(t, err)
	results, err = client.Search(ctx, "typed metadata policy", SearchOptions{KBIDs: []uuid.UUID{kbID}, TopK: 10, MetadataFilter: filter})
	require.NoError(t, err)
	require.Equal(t, []string{"year-string.txt"}, documentNames(results))

	filter, err = ParseMetadataFilter(json.RawMessage(`{"field":"year","op":"in","value":[2025,2026]}`))
	require.NoError(t, err)
	results, err = client.Search(ctx, "typed metadata policy", SearchOptions{KBIDs: []uuid.UUID{kbID}, TopK: 10, MetadataFilter: filter})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"year-number.txt", "year-previous.txt"}, documentNames(results))

	filter, err = ParseMetadataFilter(json.RawMessage(`{"field":"enabled","op":"eq","value":false}`))
	require.NoError(t, err)
	results, err = client.Search(ctx, "typed metadata policy", SearchOptions{KBIDs: []uuid.UUID{kbID}, TopK: 10, MetadataFilter: filter})
	require.NoError(t, err)
	require.Equal(t, []string{"year-string.txt"}, documentNames(results))

	filter, err = ParseMetadataFilter(json.RawMessage(`{"field":"enabled","op":"exists"}`))
	require.NoError(t, err)
	results, err = client.Search(ctx, "typed metadata policy", SearchOptions{KBIDs: []uuid.UUID{kbID}, TopK: 10, MetadataFilter: filter})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"year-number.txt", "year-string.txt"}, documentNames(results))
}

func documentNames(results []SearchResult) []string {
	names := make([]string, len(results))
	for i, result := range results {
		names[i] = result.DocumentName
	}
	return names
}

type metadataSearchDocument struct {
	name     string
	content  string
	metadata string
	vector   bool
}

func setupMetadataSearchSchema(t *testing.T, pool *pgxpool.Pool, tenantID string) {
	t.Helper()
	ctx := context.Background()
	schema := "ah_" + tenantID
	_, err := pool.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS vector WITH SCHEMA public`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err)

	conn, release, err := database.AcquireWithTenant(ctx, pool, tenantID)
	require.NoError(t, err)
	defer release()
	_, err = conn.Exec(ctx, `
CREATE TABLE knowledge_base (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'ACTIVE'
);
CREATE TABLE document (
    id UUID PRIMARY KEY,
    knowledge_base_id UUID NOT NULL REFERENCES knowledge_base(id) ON DELETE CASCADE,
    file_name VARCHAR(500) NOT NULL,
    content_type VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL,
    storage_path TEXT NOT NULL,
    file_size BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE document_chunk (
    id UUID PRIMARY KEY,
    document_id UUID NOT NULL REFERENCES document(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    chunk_index INTEGER NOT NULL
);
CREATE TABLE document_chunk_embedding (
    chunk_id UUID PRIMARY KEY REFERENCES document_chunk(id) ON DELETE CASCADE,
    embedding public.vector(1024)
)`)
	require.NoError(t, err)

	migrationPath := filepath.Join("..", "..", "..", "migrations", "schemas", "000083_document_metadata.up.sql")
	migration, err := os.ReadFile(migrationPath)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, string(migration))
	require.NoError(t, err)
}

func seedMetadataSearchKB(t *testing.T, pool *pgxpool.Pool, tenantID string, kbID uuid.UUID, documents []metadataSearchDocument) {
	t.Helper()
	ctx := context.Background()
	conn, release, err := database.AcquireWithTenant(ctx, pool, tenantID)
	require.NoError(t, err)
	defer release()

	_, err = conn.Exec(ctx, `INSERT INTO knowledge_base (id, name, status) VALUES ($1, $2, 'ACTIVE')`, kbID, "Metadata search")
	require.NoError(t, err)
	for index, document := range documents {
		documentID := uuid.New()
		chunkID := uuid.New()
		_, err = conn.Exec(ctx, `
INSERT INTO document (id, knowledge_base_id, file_name, content_type, status, storage_path, file_size, metadata)
VALUES ($1, $2, $3, 'text/plain', 'INDEXED', $4, $5, $6::jsonb)`,
			documentID, kbID, document.name, "documents/"+documentID.String(), len(document.content), document.metadata)
		require.NoError(t, err)
		_, err = conn.Exec(ctx, `INSERT INTO document_chunk (id, document_id, content, chunk_index) VALUES ($1, $2, $3, $4)`, chunkID, documentID, document.content, index)
		require.NoError(t, err)
		if document.vector {
			_, err = conn.Exec(ctx, `INSERT INTO document_chunk_embedding (chunk_id, embedding) VALUES ($1, $2::vector)`, chunkID, metadataSearchVector())
			require.NoError(t, err)
		}
	}
}

func metadataSearchVector() string {
	values := make([]string, 1024)
	values[0] = "1"
	for i := 1; i < len(values); i++ {
		values[i] = "0"
	}
	return "[" + strings.Join(values, ",") + "]"
}
