// Package prompttemplate manages reusable system prompt templates for agents.
package prompttemplate

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a prompt template is not found.
var ErrNotFound = fmt.Errorf("prompt template: not found")

// ErrDuplicateSlug is returned when a slug already exists for the agent.
var ErrDuplicateSlug = fmt.Errorf("prompt template: duplicate slug")

// Category represents the template category.
type Category string

const (
	CategoryGeneral Category = "general"
	CategoryRAG     Category = "rag"
	CategoryData    Category = "data"
	CategoryAPI     Category = "api"
	CategoryCustom  Category = "custom"
)

// PromptTemplate is the domain entity for a reusable system prompt template.
type PromptTemplate struct {
	ID            uuid.UUID
	AgentID       *uuid.UUID      // nil = global built-in
	Name          string
	Slug          string
	Description   string
	Content       string
	Category      Category
	ModelOverride *string
	AllowedTools  json.RawMessage // JSON array of tool slugs
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
