// Package database provides pgxpool setup and health check helpers
// for AgentHub Go services.
//
// Usage:
//
//	pool, err := database.NewPool(ctx, database.Config{DSN: cfg.DatabaseDSN})
//	conn, err := database.AcquireWithTenant(ctx, pool) // sets search_path
package database
