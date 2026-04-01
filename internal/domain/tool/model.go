package tool

import (
	"time"

	"github.com/google/uuid"
)

// Tool is a concrete implementation of a skill capability.
type Tool struct {
	ID          uuid.UUID `db:"id"`
	Name        string    `db:"name"`
	Type        string    `db:"type"`
	Config      []byte    `db:"config"`
	Description string    `db:"description"`
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
