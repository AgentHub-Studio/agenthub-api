package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// PgDocumentSearchClient implements DocumentSearchClient using pgvector cosine
// similarity search. It embeds the query via an HTTP embedding service and
// queries the document_chunk / document_chunk_embedding tables.
type PgDocumentSearchClient struct {
	pool         *pgxpool.Pool
	embeddingURL string
	httpClient   *http.Client
}

// NewPgDocumentSearchClient creates a PgDocumentSearchClient.
// embeddingURL is the base URL of the embedding service (e.g. "http://agenthub-embedding:8092").
func NewPgDocumentSearchClient(pool *pgxpool.Pool, embeddingURL string) *PgDocumentSearchClient {
	return &PgDocumentSearchClient{
		pool:         pool,
		embeddingURL: embeddingURL,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

type embedRequest struct {
	Text string `json:"text"`
}

type embedResponse struct {
	Embedding []float32 `json:"embedding"`
}

// embed calls the embedding service and returns the query vector.
func (c *PgDocumentSearchClient) embed(ctx context.Context, text string) ([]float32, error) {
	body, err := json.Marshal(embedRequest{Text: text})
	if err != nil {
		return nil, fmt.Errorf("knowledge: marshal embed request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.embeddingURL+"/embed", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("knowledge: build embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("knowledge: embed request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("knowledge: embedding service returned HTTP %d", resp.StatusCode)
	}

	var result embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("knowledge: decode embed response: %w", err)
	}
	if len(result.Embedding) == 0 {
		return nil, fmt.Errorf("knowledge: embedding service returned empty vector")
	}
	return result.Embedding, nil
}

// pgQuerier is a minimal interface satisfied by both *pgxpool.Pool and *pgxpool.Conn.
type pgQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Search embeds the query and returns the top-K most similar chunks filtered by kbIDs.
// If kbIDs is empty all active KBs in the current tenant schema are searched.
// When ctx carries a tenant ID (via tenant.FromContext) a dedicated connection is
// acquired with the correct search_path so the per-tenant schema tables are visible.
func (c *PgDocumentSearchClient) Search(ctx context.Context, query string, kbIDs []uuid.UUID, topK int) ([]SearchResult, error) {
	if topK <= 0 {
		topK = 5
	}

	// P-KB2-2: if context carries a tenant ID, acquire a tenant-scoped connection so
	// that the document_chunk / document_chunk_embedding tables in ah_{tenantID} are
	// visible. Without this the pool returns connections with the default search_path
	// (public), causing "relation does not exist" errors.
	if tenantID := tenant.FromContext(ctx); tenantID != "" {
		conn, release, err := database.AcquireWithTenant(ctx, c.pool, tenantID)
		if err != nil {
			return nil, fmt.Errorf("knowledge: acquire tenant connection: %w", err)
		}
		defer release()
		return c.searchWithQuerier(ctx, conn, query, kbIDs, topK)
	}

	return c.searchWithQuerier(ctx, c.pool, query, kbIDs, topK)
}

// searchWithQuerier performs the actual vector search using the given querier.
// The querier must already have the correct search_path set if per-tenant isolation is required.
func (c *PgDocumentSearchClient) searchWithQuerier(ctx context.Context, db pgQuerier, query string, kbIDs []uuid.UUID, topK int) ([]SearchResult, error) {
	queryVec, err := c.embed(ctx, query)
	if err != nil {
		return nil, err
	}

	// Build a pgvector-compatible string representation: '[f1,f2,...]'
	vecStr := floatSliceToVector(queryVec)

	// P-E1-2: search document_chunk_embedding (per-tenant schema) using cosine distance.
	// Join with document_chunk to get content, document_id, and knowledge_base_id.
	// Filter by kbIDs when provided; otherwise search all KBs.
	var rows []struct {
		DocumentID      uuid.UUID
		ChunkID         uuid.UUID
		Content         string
		Score           float64
		DocumentName    string
		KnowledgeBaseID uuid.UUID
	}

	// knowledge_base_id lives on the document table, not document_chunk.
	// The join path is: document_chunk_embedding → document_chunk → document.
	if len(kbIDs) > 0 {
		// Build IN clause manually — pgx doesn't support uuid[] binding for this shape.
		inClause := "("
		for i, id := range kbIDs {
			if i > 0 {
				inClause += ","
			}
			inClause += "'" + id.String() + "'"
		}
		inClause += ")"

		// P-KB1-1: join knowledge_base to exclude PAUSED KBs — even when the caller
		// explicitly provides a kbID, respect the KB's pause status so that paused
		// KBs never return search results.
		sqlQuery := fmt.Sprintf(`
SELECT
    dc.id                  AS chunk_id,
    dc.document_id,
    dc.content,
    1 - (dce.embedding <=> $1::vector) AS score,
    d.file_name            AS document_name,
    d.knowledge_base_id
FROM document_chunk_embedding dce
JOIN document_chunk dc ON dc.id = dce.chunk_id
JOIN document d ON d.id = dc.document_id
JOIN knowledge_base kb ON kb.id = d.knowledge_base_id
WHERE d.knowledge_base_id IN %s AND kb.status = 'ACTIVE'
ORDER BY dce.embedding <=> $1::vector
LIMIT $2`, inClause)

		dbRows, err := db.Query(ctx, sqlQuery, vecStr, topK)
		if err != nil {
			return nil, fmt.Errorf("knowledge: vector search query: %w", err)
		}
		defer dbRows.Close()

		for dbRows.Next() {
			var row struct {
				DocumentID      uuid.UUID
				ChunkID         uuid.UUID
				Content         string
				Score           float64
				DocumentName    string
				KnowledgeBaseID uuid.UUID
			}
			if err := dbRows.Scan(&row.ChunkID, &row.DocumentID, &row.Content, &row.Score, &row.DocumentName, &row.KnowledgeBaseID); err != nil {
				return nil, fmt.Errorf("knowledge: scan search row: %w", err)
			}
			rows = append(rows, row)
		}
		if err := dbRows.Err(); err != nil {
			return nil, fmt.Errorf("knowledge: iterate search rows: %w", err)
		}
	} else {
		// No kbID filter — search all document chunks in the tenant schema.
		// P-KB1-1: always exclude chunks from PAUSED KBs regardless of whether
		// the caller specified kbIDs — pause means no search results.
		sqlQuery := `
SELECT
    dc.id                  AS chunk_id,
    dc.document_id,
    dc.content,
    1 - (dce.embedding <=> $1::vector) AS score,
    d.file_name            AS document_name,
    d.knowledge_base_id
FROM document_chunk_embedding dce
JOIN document_chunk dc ON dc.id = dce.chunk_id
JOIN document d ON d.id = dc.document_id
JOIN knowledge_base kb ON kb.id = d.knowledge_base_id
WHERE kb.status = 'ACTIVE'
ORDER BY dce.embedding <=> $1::vector
LIMIT $2`

		dbRows, err := db.Query(ctx, sqlQuery, vecStr, topK)
		if err != nil {
			return nil, fmt.Errorf("knowledge: vector search query (all KBs): %w", err)
		}
		defer dbRows.Close()

		for dbRows.Next() {
			var row struct {
				DocumentID      uuid.UUID
				ChunkID         uuid.UUID
				Content         string
				Score           float64
				DocumentName    string
				KnowledgeBaseID uuid.UUID
			}
			if err := dbRows.Scan(&row.ChunkID, &row.DocumentID, &row.Content, &row.Score, &row.DocumentName, &row.KnowledgeBaseID); err != nil {
				return nil, fmt.Errorf("knowledge: scan search row (all KBs): %w", err)
			}
			rows = append(rows, row)
		}
		if err := dbRows.Err(); err != nil {
			return nil, fmt.Errorf("knowledge: iterate search rows (all KBs): %w", err)
		}
	}

	results := make([]SearchResult, len(rows))
	for i, r := range rows {
		results[i] = SearchResult{
			DocumentID:      r.DocumentID,
			ChunkID:         r.ChunkID,
			Content:         r.Content,
			Score:           r.Score,
			DocumentName:    r.DocumentName,
			KnowledgeBaseID: r.KnowledgeBaseID,
		}
	}
	return results, nil
}

// floatSliceToVector converts a float32 slice to a pgvector-compatible string.
func floatSliceToVector(v []float32) string {
	if len(v) == 0 {
		return "[]"
	}
	buf := make([]byte, 0, 2+len(v)*12)
	buf = append(buf, '[')
	for i, f := range v {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = fmt.Appendf(buf, "%g", f)
	}
	buf = append(buf, ']')
	return string(buf)
}
