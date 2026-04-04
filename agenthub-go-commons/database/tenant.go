package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-go-commons/tenant"
)

// AcquireWithTenant acquires a connection from the pool and executes
// SET search_path TO ah_{tenantID}, public so all subsequent queries
// on this connection target the tenant's schema.
//
// The caller MUST release the connection when done:
//
//	conn, release, err := database.AcquireWithTenant(ctx, pool)
//	if err != nil { ... }
//	defer release()
func AcquireWithTenant(ctx context.Context, pool *pgxpool.Pool) (*pgxpool.Conn, func(), error) {
	tenantID := tenant.FromContext(ctx)
	if tenantID == "" {
		return nil, nil, fmt.Errorf("database: tenantID not found in context")
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("database: acquire connection: %w", err)
	}

	schema := "ah_" + tenantID
	if _, err := conn.Exec(ctx, fmt.Sprintf("SET search_path TO %s, public", schema)); err != nil {
		conn.Release()
		return nil, nil, fmt.Errorf("database: set search_path to %s: %w", schema, err)
	}

	return conn, conn.Release, nil
}
