// Package core provides access to the global ah_core schema — platform-managed
// agents, skills, and tools that are available to all tenants.
package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreTool represents a platform-managed tool from ah_core.tool.
// These tools use use_caller_token=true to execute REST calls in the
// caller's tenant context (not a service account).
type CoreTool struct {
	ID          uuid.UUID
	Name        string
	Slug        string
	Description string
	Type        string
	Config      json.RawMessage
	IsActive    bool
}

// CoreToolLoader loads tools from the global ah_core schema.
// It is non-fatal: if the schema does not exist (e.g. during migration or test)
// it returns an empty slice with a warning log.
type CoreToolLoader struct {
	pool *pgxpool.Pool
}

// NewCoreToolLoader creates a CoreToolLoader backed by the given connection pool.
func NewCoreToolLoader(pool *pgxpool.Pool) *CoreToolLoader {
	return &CoreToolLoader{pool: pool}
}

// LoadAll returns all active tools from ah_core.tool.
// Returns an empty slice (not an error) if the ah_core schema does not exist.
func (l *CoreToolLoader) LoadAll(ctx context.Context) ([]CoreTool, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, name, slug, description, type, config, is_active
		  FROM ah_core.tool
		 WHERE is_active = true
		 ORDER BY name`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		// Schema may not exist in fresh deployments — non-fatal.
		slog.WarnContext(ctx, "core: ah_core.tool not accessible, core tools unavailable", "err", err)
		return nil, nil
	}
	defer rows.Close()

	var tools []CoreTool
	for rows.Next() {
		var t CoreTool
		if err := rows.Scan(&t.ID, &t.Name, &t.Slug, &t.Description, &t.Type, &t.Config, &t.IsActive); err != nil {
			return nil, fmt.Errorf("core: scan tool: %w", err)
		}
		tools = append(tools, t)
	}
	return tools, rows.Err()
}
