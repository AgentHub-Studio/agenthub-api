package knowledgebase

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a knowledge base cannot be found.
var ErrNotFound = errors.New("knowledgebase: not found")

// ErrDuplicateName é retornado quando já existe uma KB com o
// mesmo name no tenant. Mapeado para 409 no handler.
var ErrDuplicateName = errors.New("knowledgebase: a knowledge base with this name already exists")

// KnowledgeBaseStatus represents the lifecycle status of a KnowledgeBase.
type KnowledgeBaseStatus string

const (
	StatusActive KnowledgeBaseStatus = "ACTIVE"
	StatusPaused KnowledgeBaseStatus = "PAUSED"
)

// KnowledgeBase is the domain entity.
type KnowledgeBase struct {
	ID             uuid.UUID           `db:"id"`
	Name           string              `db:"name"`
	Description    string              `db:"description"`
	Status         KnowledgeBaseStatus `db:"status"`
	EmbeddingModel string              `db:"embedding_model"`
	SearchMode     string              `db:"search_mode"`
	ContextWindow  int                 `db:"context_window"`
	DocumentCount  int64               `db:"document_count"`
	CreatedAt      time.Time           `db:"created_at"`
	UpdatedAt      time.Time           `db:"updated_at"`
}

// KnowledgeBaseResponse is the DTO returned by the API.
type KnowledgeBaseResponse struct {
	ID             uuid.UUID           `json:"id"`
	Name           string              `json:"name"`
	Description    string              `json:"description"`
	Status         KnowledgeBaseStatus `json:"status"`
	EmbeddingModel string              `json:"embeddingModel"`
	SearchMode     string              `json:"searchMode"`
	ContextWindow  int                 `json:"contextWindow"`
	DocumentCount  int64               `json:"documentCount"`
	CreatedAt      time.Time           `json:"createdAt"`
	UpdatedAt      time.Time           `json:"updatedAt"`
}

// ResponseFrom maps a KnowledgeBase entity to a KnowledgeBaseResponse DTO.
func ResponseFrom(k KnowledgeBase) KnowledgeBaseResponse {
	return KnowledgeBaseResponse(k)
}

// CreateRequest is the payload for creating a KnowledgeBase.
type CreateRequest struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	EmbeddingModel string `json:"embeddingModel"`
	SearchMode     string `json:"searchMode"`
	ContextWindow  int    `json:"contextWindow"`
}

// UpdateRequest is the payload for updating a KnowledgeBase (all fields optional).
type UpdateRequest struct {
	Name           *string `json:"name"`
	Description    *string `json:"description"`
	EmbeddingModel *string `json:"embeddingModel"`
	SearchMode     *string `json:"searchMode"`
	ContextWindow  *int    `json:"contextWindow"`
}
