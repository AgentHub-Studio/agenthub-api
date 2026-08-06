package knowledgebase

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a knowledge base cannot be found.
var ErrNotFound = errors.New("knowledgebase: not found")

// ErrDuplicateName é retornado quando já existe uma KB com o
// mesmo name no tenant. Mapeado para 409 no handler.
var ErrDuplicateName = errors.New("knowledgebase: a knowledge base with this name already exists")

// ErrValidation is returned when the create/update request fails server-side
// validation (e.g. contextWindow < 0). Mapped to HTTP 422 by the handler.
var ErrValidation = errors.New("knowledgebase: validation failed")

// KnowledgeBaseStatus represents the lifecycle status of a KnowledgeBase.
type KnowledgeBaseStatus string

const (
	StatusActive KnowledgeBaseStatus = "ACTIVE"
	StatusPaused KnowledgeBaseStatus = "PAUSED"
)

// RerankStrategy controls how retrieved RAG candidates are re-scored.
type RerankStrategy string

const (
	RerankStrategyNone           RerankStrategy = "none"
	RerankStrategyLLM            RerankStrategy = "llm"
	RerankStrategyCrossEncoder   RerankStrategy = "cross_encoder"
	RerankStrategyRRF            RerankStrategy = "rrf"
	RerankStrategyBrokenReranker RerankStrategy = "broken_reranker"
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
	RerankStrategy RerankStrategy      `db:"rerank_strategy"`
	GraphEnabled   bool                `db:"graph_enabled"`
	DocumentCount  int64               `db:"document_count"`
	CreatedAt      time.Time           `db:"created_at"`
	UpdatedAt      time.Time           `db:"updated_at"`
}

// ListFilters contains optional filters accepted by the list endpoint.
type ListFilters struct {
	Status *KnowledgeBaseStatus
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
	RerankStrategy RerankStrategy      `json:"rerankStrategy"`
	GraphEnabled   bool                `json:"graphEnabled"`
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
	Name                string `json:"name"`
	Description         string `json:"description"`
	EmbeddingModel      string `json:"embeddingModel"`
	SearchMode          string `json:"searchMode"`
	ContextWindow       int    `json:"contextWindow"`
	RerankStrategy      string `json:"rerankStrategy"`
	RerankStrategySnake string `json:"rerank_strategy"`
	GraphEnabled        bool   `json:"graphEnabled"`
	GraphEnabledSnake   bool   `json:"graph_enabled"`
}

// UnmarshalJSON accepts the legacy snake_case configuration aliases only when
// they describe the same configuration as their canonical camelCase fields.
func (r *CreateRequest) UnmarshalJSON(data []byte) error {
	var wire struct {
		Name                string  `json:"name"`
		Description         string  `json:"description"`
		EmbeddingModel      string  `json:"embeddingModel"`
		SearchMode          string  `json:"searchMode"`
		ContextWindow       int     `json:"contextWindow"`
		RerankStrategy      *string `json:"rerankStrategy"`
		RerankStrategySnake *string `json:"rerank_strategy"`
		GraphEnabled        *bool   `json:"graphEnabled"`
		GraphEnabledSnake   *bool   `json:"graph_enabled"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	if err := validateConfigurationAliases(wire.RerankStrategy, wire.RerankStrategySnake, wire.GraphEnabled, wire.GraphEnabledSnake); err != nil {
		return err
	}

	*r = CreateRequest{
		Name:           wire.Name,
		Description:    wire.Description,
		EmbeddingModel: wire.EmbeddingModel,
		SearchMode:     wire.SearchMode,
		ContextWindow:  wire.ContextWindow,
	}
	if wire.RerankStrategy != nil {
		r.RerankStrategy = *wire.RerankStrategy
	}
	if wire.RerankStrategySnake != nil {
		r.RerankStrategySnake = *wire.RerankStrategySnake
	}
	if wire.GraphEnabled != nil {
		r.GraphEnabled = *wire.GraphEnabled
	}
	if wire.GraphEnabledSnake != nil {
		r.GraphEnabledSnake = *wire.GraphEnabledSnake
	}
	return nil
}

// UpdateRequest is the payload for updating a KnowledgeBase (all fields optional).
type UpdateRequest struct {
	Name                *string `json:"name"`
	Description         *string `json:"description"`
	EmbeddingModel      *string `json:"embeddingModel"`
	SearchMode          *string `json:"searchMode"`
	ContextWindow       *int    `json:"contextWindow"`
	RerankStrategy      *string `json:"rerankStrategy"`
	RerankStrategySnake *string `json:"rerank_strategy"`
	GraphEnabled        *bool   `json:"graphEnabled"`
	GraphEnabledSnake   *bool   `json:"graph_enabled"`
}

// UnmarshalJSON accepts the legacy snake_case configuration aliases only when
// they describe the same configuration as their canonical camelCase fields.
func (r *UpdateRequest) UnmarshalJSON(data []byte) error {
	type updateRequestWire UpdateRequest
	var wire updateRequestWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	if err := validateConfigurationAliases(wire.RerankStrategy, wire.RerankStrategySnake, wire.GraphEnabled, wire.GraphEnabledSnake); err != nil {
		return err
	}
	*r = UpdateRequest(wire)
	return nil
}

func validateConfigurationAliases(rerankStrategy, rerankStrategySnake *string, graphEnabled, graphEnabledSnake *bool) error {
	if rerankStrategy != nil && rerankStrategySnake != nil && strings.TrimSpace(*rerankStrategy) != strings.TrimSpace(*rerankStrategySnake) {
		return fmt.Errorf("knowledgebase: conflicting rerank strategy aliases")
	}
	if graphEnabled != nil && graphEnabledSnake != nil && *graphEnabled != *graphEnabledSnake {
		return fmt.Errorf("knowledgebase: conflicting graph enabled aliases")
	}
	return nil
}
