package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TenantConn wraps a pgxpool.Conn with a release method.
type TenantConn struct {
	*pgxpool.Conn
}

// AcquireWithTenant acquires a connection from the pool and sets the search_path
// to the tenant schema (ah_{tenantID}) plus public.
func AcquireWithTenant(ctx context.Context, pool *pgxpool.Pool, tenantID string) (*pgxpool.Conn, error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("database: acquire connection: %w", err)
	}

	schema := fmt.Sprintf("ah_%s", tenantID)
	_, err = conn.Exec(ctx, fmt.Sprintf("SET search_path TO %s, public", schema))
	if err != nil {
		conn.Release()
		return nil, fmt.Errorf("database: set search_path to %s: %w", schema, err)
	}

	return conn, nil
}
