// Command migrate-workload-identities backfills the tenant-local Keycloak
// workload identity introduced after some realms already existed. It is an
// operator command and must only run after the public schema migration exists.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/AgentHub-Studio/agenthub-api/internal/config"
	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	apikc "github.com/AgentHub-Studio/agenthub-api/internal/keycloak"
	"github.com/AgentHub-Studio/agenthub-api/internal/workloadidentity"
)

const tenantBackfillTimeout = 2 * time.Minute

type workloadIdentityProvisioner interface {
	EnsureWorkloadIdentity(ctx context.Context, tenantID string) (workloadidentity.Credential, error)
}

type tenantCredentialStore interface {
	Store(ctx context.Context, tenantID string, credential workloadidentity.Credential) error
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("workload identity migration: configuration is invalid", "error", err)
		os.Exit(1)
	}
	if err := workloadidentity.ValidateEncryptionKey(cfg.MCPRuntimeCredentialEncryptionKey); err != nil {
		slog.Error("workload identity migration: configuration is invalid", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("workload identity migration: database connection failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	provisioner := apikc.NewProvisioner(apikc.Config{
		BaseURL:          cfg.KeycloakBaseURL,
		AdminUsername:    cfg.KeycloakAdmin.AdminUsername,
		AdminPassword:    cfg.KeycloakAdmin.AdminPassword,
		AdminClientID:    cfg.KeycloakAdmin.AdminClientID,
		AdminRealm:       cfg.KeycloakAdmin.AdminRealm,
		FrontendClient:   cfg.KeycloakAdmin.FrontendClient,
		WorkloadClientID: cfg.MCPRuntimeClientID,
		WorkloadAudience: cfg.MCPRuntimeAudience,
	})
	store := workloadidentity.NewService(
		workloadidentity.NewRepository(pool),
		cfg.MCPRuntimeCredentialEncryptionKey,
	)

	rows, err := pool.Query(ctx, `SELECT id FROM public.tenants WHERE status = 'ACTIVE' ORDER BY id`)
	if err != nil {
		slog.Error("workload identity migration: list tenants failed", "error", err)
		os.Exit(1)
	}
	if err := backfill(ctx, rows, provisioner, store); err != nil {
		slog.Error("workload identity migration failed", "error", err)
		os.Exit(1)
	}
}

type tenantRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}

func backfill(ctx context.Context, tenants tenantRows, provisioner workloadIdentityProvisioner, store tenantCredentialStore) error {
	defer tenants.Close()

	var failed int
	for tenants.Next() {
		var tenantID string
		if err := tenants.Scan(&tenantID); err != nil {
			return fmt.Errorf("scan tenant: %w", err)
		}
		tenantCtx, cancel := context.WithTimeout(ctx, tenantBackfillTimeout)
		credential, err := provisioner.EnsureWorkloadIdentity(tenantCtx, tenantID)
		if err == nil {
			err = store.Store(tenantCtx, tenantID, credential)
		}
		cancel()
		if err != nil {
			failed++
			slog.Error("workload identity migration: tenant failed", "tenantID", tenantID, "error", err)
			continue
		}
		slog.Info("workload identity migration: tenant synchronized", "tenantID", tenantID)
	}
	if err := tenants.Err(); err != nil {
		return fmt.Errorf("iterate tenants: %w", err)
	}
	if failed > 0 {
		return fmt.Errorf("workload identity migration failed for %d tenant(s)", failed)
	}
	return nil
}
