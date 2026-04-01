package skill

import (
	"time"

	"github.com/google/uuid"
)

// Skill represents an abstract AI capability that can be bound to one or more tools.
type Skill struct {
	ID           uuid.UUID `db:"id"`
	Name         string    `db:"name"`
	Slug         string    `db:"slug"`
	Description  string    `db:"description"`
	Category     string    `db:"category"`
	InputSchema  []byte    `db:"input_schema"`
	OutputSchema []byte    `db:"output_schema"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}
