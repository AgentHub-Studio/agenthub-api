package knowledgebase

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a knowledge base cannot be found.
var ErrNotFound = errors.New("knowledgebase: not found")

// KnowledgeBaseStatus represents the lifecycle status of a KnowledgeBase.
type KnowledgeBaseStatus string

const (
	StatusActive KnowledgeBaseStatus = "ACTIVE"
	StatusPaused KnowledgeBaseStatus = "PAUSED"
)

// KnowledgeBase is the domain entity.
type KnowledgeBase struct {
	ID          uuid.UUID           `db:"id"`
	Name        string              `db:"name"`
	Description string              `db:"description"`
	Status      KnowledgeBaseStatus `db:"status"`
	CreatedAt   time.Time           `db:"created_at"`
	UpdatedAt   time.Time           `db:"updated_at"`
}

// KnowledgeBaseResponse is the DTO returned by the API.
type KnowledgeBaseResponse struct {
	ID          uuid.UUID           `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Status      KnowledgeBaseStatus `json:"status"`
	CreatedAt   time.Time           `json:"createdAt"`
	UpdatedAt   time.Time           `json:"updatedAt"`
}

// ResponseFrom maps a KnowledgeBase entity to a KnowledgeBaseResponse DTO.
func ResponseFrom(k KnowledgeBase) KnowledgeBaseResponse {
	return KnowledgeBaseResponse{
		ID:          k.ID,
		Name:        k.Name,
		Description: k.Description,
		Status:      k.Status,
		CreatedAt:   k.CreatedAt,
		UpdatedAt:   k.UpdatedAt,
	}
}

// CreateRequest is the payload for creating a KnowledgeBase.
type CreateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// UpdateRequest is the payload for updating a KnowledgeBase (all fields optional).
type UpdateRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}
