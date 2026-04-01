package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AcquireWithTenant acquires a connection from pool and sets the search_path to
// the tenant schema (ah_{tenantID}) plus public. Returns the connection and a
// release function that must be called when done.
func AcquireWithTenant(ctx context.Context, pool *pgxpool.Pool, tenantID string) (*pgxpool.Conn, func(), error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("database: acquire connection: %w", err)
	}

	if tenantID != "" {
		schema := "ah_" + tenantID
		_, err = conn.Exec(ctx, fmt.Sprintf("SET search_path TO %s, public", pgx.Identifier{schema}.Sanitize()))
		if err != nil {
			conn.Release()
			return nil, nil, fmt.Errorf("database: set search_path: %w", err)
		}
	}

	return conn, conn.Release, nil
}
