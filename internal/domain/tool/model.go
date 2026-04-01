package tool

import (
	"time"

	"github.com/google/uuid"
)

// ToolType represents the implementation type of a tool.
type ToolType = string

const (
	ToolTypeHTTP           ToolType = "HTTP"
	ToolTypeSQL            ToolType = "SQL"
	ToolTypeDocumentSearch ToolType = "DOCUMENT_SEARCH"
	ToolTypeCustom         ToolType = "CUSTOM"
	ToolTypeBlockly        ToolType = "BLOCKLY"
	ToolTypeComposite      ToolType = "COMPOSITE"
)

// validToolTypes is used for type validation.
var validToolTypes = map[ToolType]bool{
	ToolTypeHTTP:           true,
	ToolTypeSQL:            true,
	ToolTypeDocumentSearch: true,
	ToolTypeCustom:         true,
	ToolTypeBlockly:        true,
	ToolTypeComposite:      true,
}

// IsValidToolType returns true when the given type string is a known ToolType.
func IsValidToolType(t string) bool {
	return validToolTypes[t]
}

// Tool is a concrete implementation of a skill capability.
type Tool struct {
	ID          uuid.UUID `db:"id"`
	Name        string    `db:"name"`
	Type        ToolType  `db:"type"`
	Config      []byte    `db:"config"`
	Description string    `db:"description"`
	Labels      []string  `db:"labels"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// SkillTool represents the binding between a skill and a tool.
type SkillTool struct {
	ID        uuid.UUID `db:"id"`
	SkillID   uuid.UUID `db:"skill_id"`
	ToolID    uuid.UUID `db:"tool_id"`
	Priority  int       `db:"priority"`
	IsActive  bool      `db:"is_active"`
	CreatedAt time.Time `db:"created_at"`
}
