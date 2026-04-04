package multitenant

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-go-commons/database/migrate"
)

// MigrateAllTenants queries public.tenants for all ACTIVE tenants and applies
// the given migrations to each ah_{tenantID} schema.
func MigrateAllTenants(ctx context.Context, pool *pgxpool.Pool, migrationsPath string) error {
	rows, err := pool.Query(ctx, "SELECT id FROM public.tenants WHERE status = 'ACTIVE'")
	if err != nil {
		return fmt.Errorf("multitenant: query tenants: %w", err)
	}
	defer rows.Close()

	var tenantIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("multitenant: scan tenant id: %w", err)
		}
		tenantIDs = append(tenantIDs, id)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("multitenant: rows error: %w", err)
	}

	for _, tenantID := range tenantIDs {
		schema := "ah_" + tenantID
		if err := migrate.Up(ctx, pool, schema, migrationsPath); err != nil {
			return fmt.Errorf("multitenant: migrate tenant %q: %w", tenantID, err)
		}
	}
	return nil
}

// MigrateTenant applies migrations to a single tenant schema.
// Used when provisioning a new tenant.
func MigrateTenant(ctx context.Context, pool *pgxpool.Pool, tenantID, migrationsPath string) error {
	schema := "ah_" + tenantID
	if err := migrate.Up(ctx, pool, schema, migrationsPath); err != nil {
		return fmt.Errorf("multitenant: migrate tenant %q: %w", tenantID, err)
	}
	return nil
}
