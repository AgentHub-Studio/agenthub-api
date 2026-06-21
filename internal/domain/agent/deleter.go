package agent

import (
	"context"

	"github.com/google/uuid"
)

// Deleter deletes agents through the service layer so binding checks and audit
// logging are enforced. Used by ManagementExecutor (SEC-01).
type Deleter interface {
	Delete(ctx context.Context, id uuid.UUID) error
}
