package knowledge

import (
	"context"

	"github.com/google/uuid"
)

// SearchResult represents a single document chunk match from semantic search.
type SearchResult struct {
	DocumentID      uuid.UUID `json:"documentId"`
	ChunkID         uuid.UUID `json:"chunkId"`
	Content         string    `json:"content"`
	Score           float64   `json:"score"`
	DocumentName    string    `json:"documentName"`
	KnowledgeBaseID uuid.UUID `json:"knowledgeBaseId"`
}

// DocumentSearchClient performs semantic search against indexed document chunks
// using pgvector cosine similarity. Implementations must be safe for concurrent use.
type DocumentSearchClient interface {
	Search(ctx context.Context, query string, kbIDs []uuid.UUID, topK int) ([]SearchResult, error)
}
