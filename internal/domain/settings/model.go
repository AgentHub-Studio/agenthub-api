// Package settings implements the per-tenant system settings domain.
package settings

import (
	"encoding/json"
	"time"
)

// Setting is the domain entity for a per-tenant key-value configuration entry.
type Setting struct {
	Key         string
	Value       json.RawMessage
	Description string
	UpdatedAt   time.Time
}
