package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AcquireWithTenant acquires a connection from the pool and sets the search_path
// to the tenant's schema (ah_{tenantID}) so that all queries run in the correct
// multi-tenant context. The caller MUST invoke the returned release function (e.g.
// via defer) to return the connection to the pool.
func AcquireWithTenant(ctx context.Context, pool *pgxpool.Pool, tenantID string) (*pgxpool.Conn, func(), error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("database: acquire connection: %w", err)
	}

	schema := fmt.Sprintf("ah_%s", tenantID)
	if _, err := conn.Exec(ctx, fmt.Sprintf("SET search_path TO %s, public", schema)); err != nil {
		conn.Release()
		return nil, nil, fmt.Errorf("database: set search_path to %s: %w", schema, err)
	}

	return conn, conn.Release, nil
}

// NewPool creates and validates a pgxpool.Pool.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("database: parse DSN: %w", err)
	}

	cfg.MaxConns = 25
	cfg.MinConns = 2
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("database: create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database: ping: %w", err)
	}

	return pool, nil
}
