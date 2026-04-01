package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AcquireWithTenant acquires a connection from the pool and sets the search_path
// to the tenant schema so that all subsequent queries target ah_{tenantID}.
// The caller MUST invoke the returned release function when done.
func AcquireWithTenant(ctx context.Context, pool *pgxpool.Pool, tenantID string) (*pgxpool.Conn, func(), error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("database: acquire: %w", err)
	}
	schema := "ah_" + tenantID
	if _, err := conn.Exec(ctx, "SET search_path TO "+schema+", public"); err != nil {
		conn.Release()
		return nil, nil, fmt.Errorf("database: set search_path: %w", err)
	}
	return conn, conn.Release, nil
}
