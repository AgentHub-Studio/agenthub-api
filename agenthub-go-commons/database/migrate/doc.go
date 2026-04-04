// Package migrate wraps golang-migrate to apply SQL migrations
// against a PostgreSQL schema. Compatible with Flyway naming conventions.
//
// Usage:
//
//	err := migrate.Up(ctx, pool, "public", "migrations/public")
package migrate
