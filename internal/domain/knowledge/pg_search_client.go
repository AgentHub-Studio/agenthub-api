package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/ssrf"
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

var errDocumentSearchEmbeddingRedirectNotAllowed = errors.New("knowledge: embedding redirect target is not allowed")

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

	// #nosec G704 -- the configured embedding service is infrastructure-owned and every redirect is checked against the SSRF policy.
	resp, err := c.validatedHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("knowledge: embed request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

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

func (c *PgDocumentSearchClient) validatedHTTPClient() *http.Client {
	client := c.httpClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	protected := *client
	previousCheckRedirect := client.CheckRedirect
	protected.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if err := ssrf.ValidateURL(req.URL.String()); err != nil {
			return errDocumentSearchEmbeddingRedirectNotAllowed
		}
		if previousCheckRedirect != nil {
			return previousCheckRedirect(req, via)
		}
		return nil
	}
	return &protected
}

// pgQuerier is a minimal interface satisfied by both *pgxpool.Pool and *pgxpool.Conn.
type pgQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Search embeds the query and returns the top-K most similar chunks filtered by options.
// If options.KBIDs is empty all active KBs in the current tenant schema are searched.
// When ctx carries a tenant ID (via tenant.FromContext) a dedicated connection is
// acquired with the correct search_path so the per-tenant schema tables are visible.
func (c *PgDocumentSearchClient) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	if opts.TopK <= 0 {
		opts.TopK = 5
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
		return c.searchWithQuerier(ctx, conn, query, opts)
	}

	return c.searchWithQuerier(ctx, c.pool, query, opts)
}

// searchWithQuerier performs the actual vector search using the given querier.
// The querier must already have the correct search_path set if per-tenant isolation is required.
func (c *PgDocumentSearchClient) searchWithQuerier(ctx context.Context, db pgQuerier, query string, opts SearchOptions) ([]SearchResult, error) {
	hasEmbeddings, err := hasSearchableEmbeddings(ctx, db, opts.KBIDs)
	if err != nil {
		return nil, err
	}
	if !hasEmbeddings {
		return c.textSearchWithQuerier(ctx, db, query, opts)
	}

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
	metadataClause, metadataArgs := opts.MetadataFilter.SQL(2)
	metadataWhere := ""
	if metadataClause != "" {
		metadataWhere = " AND " + metadataClause
	}

	if len(opts.KBIDs) > 0 {
		// Build IN clause manually — pgx doesn't support uuid[] binding for this shape.
		inClause := "("
		for i, id := range opts.KBIDs {
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
WHERE d.knowledge_base_id IN %s AND kb.status = 'ACTIVE'%s
ORDER BY dce.embedding <=> $1::vector
LIMIT $2`, inClause, metadataWhere)

		args := append([]any{vecStr, opts.TopK}, metadataArgs...)
		dbRows, err := db.Query(ctx, sqlQuery, args...)
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
WHERE kb.status = 'ACTIVE'%s
ORDER BY dce.embedding <=> $1::vector
LIMIT $2`, metadataWhere)

		args := append([]any{vecStr, opts.TopK}, metadataArgs...)
		dbRows, err := db.Query(ctx, sqlQuery, args...)
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

func (c *PgDocumentSearchClient) textSearchWithQuerier(ctx context.Context, db pgQuerier, query string, opts SearchOptions) ([]SearchResult, error) {
	tokens := queryTokens(query)
	if len(tokens) == 0 {
		return []SearchResult{}, nil
	}
	if len(tokens) > 8 {
		tokens = tokens[:8]
	}

	var conditions []string
	args := make([]any, 0, len(tokens)+1)
	for _, token := range tokens {
		args = append(args, "%"+token+"%")
		conditions = append(conditions, fmt.Sprintf("LOWER(dc.content) LIKE $%d", len(args)))
	}

	kbFilter := ""
	if len(opts.KBIDs) > 0 {
		inClause := "("
		for i, id := range opts.KBIDs {
			if i > 0 {
				inClause += ","
			}
			inClause += "'" + id.String() + "'"
		}
		inClause += ")"
		kbFilter = "AND d.knowledge_base_id IN " + inClause
	}
	metadataFilter := ""
	if opts.MetadataFilter != nil {
		clause, metadataArgs := opts.MetadataFilter.SQL(len(args))
		args = append(args, metadataArgs...)
		metadataFilter = "AND " + clause
	}

	args = append(args, opts.TopK)
	limitArg := len(args)
	sqlQuery := fmt.Sprintf(`
SELECT
    dc.id,
    dc.document_id,
    dc.content,
    d.file_name,
    d.knowledge_base_id,
    dc.chunk_index
FROM document_chunk dc
JOIN document d ON d.id = dc.document_id
JOIN knowledge_base kb ON kb.id = d.knowledge_base_id
WHERE kb.status = 'ACTIVE'
  %s
  %s
  AND (%s)
ORDER BY d.created_at ASC, dc.chunk_index ASC, dc.id ASC
LIMIT $%d`, kbFilter, metadataFilter, strings.Join(conditions, " OR "), limitArg)

	rows, err := db.Query(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("knowledge: text search query: %w", err)
	}
	defer rows.Close()

	results := make([]SearchResult, 0, opts.TopK)
	for rows.Next() {
		var result SearchResult
		var chunkIndex int
		if err := rows.Scan(
			&result.ChunkID,
			&result.DocumentID,
			&result.Content,
			&result.DocumentName,
			&result.KnowledgeBaseID,
			&chunkIndex,
		); err != nil {
			return nil, fmt.Errorf("knowledge: scan text search row: %w", err)
		}
		result.Score = 1.0 / float64(chunkIndex+1)
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("knowledge: iterate text search rows: %w", err)
	}

	return results, nil
}

func queryTokens(query string) []string {
	seen := map[string]bool{}
	tokens := []string{}
	for _, token := range strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if token == "" || seen[token] {
			continue
		}
		seen[token] = true
		tokens = append(tokens, token)
	}
	return tokens
}

func hasSearchableEmbeddings(ctx context.Context, db pgQuerier, kbIDs []uuid.UUID) (bool, error) {
	if len(kbIDs) > 0 {
		inClause := "("
		for i, id := range kbIDs {
			if i > 0 {
				inClause += ","
			}
			inClause += "'" + id.String() + "'"
		}
		inClause += ")"

		sqlQuery := fmt.Sprintf(`
SELECT EXISTS (
	SELECT 1
	FROM document_chunk_embedding dce
	JOIN document_chunk dc ON dc.id = dce.chunk_id
	JOIN document d ON d.id = dc.document_id
	JOIN knowledge_base kb ON kb.id = d.knowledge_base_id
	WHERE d.knowledge_base_id IN %s AND kb.status = 'ACTIVE'
)`, inClause)

		var exists bool
		if err := db.QueryRow(ctx, sqlQuery).Scan(&exists); err != nil {
			return false, fmt.Errorf("knowledge: check searchable embeddings: %w", err)
		}
		return exists, nil
	}

	const sqlQuery = `
SELECT EXISTS (
	SELECT 1
	FROM document_chunk_embedding dce
	JOIN document_chunk dc ON dc.id = dce.chunk_id
	JOIN document d ON d.id = dc.document_id
	JOIN knowledge_base kb ON kb.id = d.knowledge_base_id
	WHERE kb.status = 'ACTIVE'
)`
	var exists bool
	if err := db.QueryRow(ctx, sqlQuery).Scan(&exists); err != nil {
		return false, fmt.Errorf("knowledge: check searchable embeddings: %w", err)
	}
	return exists, nil
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
