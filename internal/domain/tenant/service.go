package tenant

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// provisionTimeout caps the per-tenant Keycloak realm provisioning. Realm
// creation is dominated by Keycloak cold-path cost (observed 97–360s in dev
// cluster) — well over any edge proxy timeout. We detach from the request
// context to survive client disconnect / proxy cap, but still bound the
// background work so a stuck Keycloak can't leak goroutines.
const provisionTimeout = 6 * time.Minute

var slugRegexp = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,61}[a-z0-9]$`)

// Service defines business logic operations for Tenant.
type Service interface {
	Create(ctx context.Context, req CreateTenantRequest) (TenantResponse, error)
	GetByID(ctx context.Context, id string) (TenantResponse, error)
	List(ctx context.Context, req pagination.PageRequest) (pagination.Page[TenantResponse], error)
	Exists(ctx context.Context, id string) (bool, error)
	UpdateStatus(ctx context.Context, id string, status Status) error
}

type service struct {
	repo               Repository
	provisioningClient ProvisioningClient
	presetSeeder       PresetSeeder
	schemaMigrator     SchemaMigrator
}

// ProvisioningClient is a placeholder interface for Keycloak realm provisioning.
// Wire a real implementation when Keycloak integration is ready.
type ProvisioningClient interface {
	ProvisionRealm(ctx context.Context, tenantID string, tenantName string) error
}

// PresetSeeder seeds default LLM presets for a newly-created tenant.
type PresetSeeder interface {
	SeedDefaults(ctx context.Context, tenantID string) error
}

// SchemaMigrator creates the PostgreSQL schema for a new tenant and applies
// all pending tenant-scoped migrations. Implementations are expected to be
// idempotent (CREATE SCHEMA IF NOT EXISTS + migrate ErrNoChange is ok).
type SchemaMigrator interface {
	MigrateTenant(ctx context.Context, tenantID string) error
}

// NewService creates a new tenant Service.
// The returned *service also satisfies the Service interface; callers that
// need to attach optional collaborators (e.g. SchemaMigrator) should use
// the concrete type returned by this constructor.
func NewService(repo Repository, pc ProvisioningClient, ps PresetSeeder) *service {
	return &service{repo: repo, provisioningClient: pc, presetSeeder: ps}
}

// WithSchemaMigrator attaches a schema migrator that is called during tenant creation.
func (s *service) WithSchemaMigrator(sm SchemaMigrator) *service {
	s.schemaMigrator = sm
	return s
}

func (s *service) Create(ctx context.Context, req CreateTenantRequest) (TenantResponse, error) {
	if req.ID == "" {
		return TenantResponse{}, fmt.Errorf("%w: id is required", ErrValidation)
	}
	if req.Name == "" {
		return TenantResponse{}, fmt.Errorf("%w: name is required", ErrValidation)
	}
	if !slugRegexp.MatchString(req.ID) {
		return TenantResponse{}, fmt.Errorf("%w: id must be a kebab-case slug (^[a-z0-9][a-z0-9-]{0,61}[a-z0-9]$)", ErrValidation)
	}

	t := Tenant{
		ID:     req.ID,
		Name:   req.Name,
		Status: StatusActive,
	}
	created, err := s.repo.Create(ctx, t)
	if err != nil {
		return TenantResponse{}, err
	}

	// Detach provisioning + migration from request context so client disconnect
	// or edge-proxy cancellation (Cloudflare/nginx default ~30-60s) doesn't
	// leave the tenant in a half-provisioned state. Caller still observes the
	// final outcome via the synchronous return below; the new context has its
	// own bound to prevent goroutine leaks on stuck Keycloak.
	provisionCtx, provisionCancel := context.WithTimeout(context.Background(), provisionTimeout)
	defer provisionCancel()

	// Run schema migration BEFORE Keycloak provisioning. The schema is independent
	// of the realm and is fast (<30s). By migrating first, any caller that waits for
	// the KC realm to appear (e.g. BDD harness WaitForRealmRoles) is guaranteed to
	// see a fully-migrated schema when it makes the first tenant-scoped API call.
	// Doing it after ProvisionRealm (which can take 3-6 min) would create a race
	// where the realm is ready but the tables are not. See [project_postgres_dirty_schemas].
	if s.schemaMigrator != nil {
		if mErr := s.schemaMigrator.MigrateTenant(provisionCtx, created.ID); mErr != nil {
			slog.Warn("tenant: schema migration failed",
				"tenantID", created.ID,
				"error", mErr.Error(),
			)
			// Non-fatal: schema can be created on next server restart via MigrateAllTenants.
		}
	}

	// Attempt Keycloak provisioning; on failure mark status but do not rollback.
	if s.provisioningClient != nil {
		if pErr := s.provisioningClient.ProvisionRealm(provisionCtx, created.ID, created.Name); pErr != nil {
			slog.Warn("tenant: keycloak provisioning failed",
				"tenantID", created.ID,
				"error", pErr.Error(),
			)
			if updErr := s.repo.UpdateStatus(provisionCtx, created.ID, StatusProvisioningFailed); updErr != nil {
				slog.Error("tenant: failed to mark status provisioning_failed", "tenantID", created.ID, "err", updErr)
			}
			created.Status = StatusProvisioningFailed
		}
	}

	// Seed default LLM presets; non-fatal — log only.
	if s.presetSeeder != nil {
		_ = s.presetSeeder.SeedDefaults(ctx, created.ID)
	}

	return ResponseFrom(created), nil
}

func (s *service) GetByID(ctx context.Context, id string) (TenantResponse, error) {
	t, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return TenantResponse{}, err
	}
	return ResponseFrom(t), nil
}

func (s *service) List(ctx context.Context, req pagination.PageRequest) (pagination.Page[TenantResponse], error) {
	tenants, total, err := s.repo.FindAll(ctx, req)
	if err != nil {
		return pagination.Page[TenantResponse]{}, err
	}
	responses := make([]TenantResponse, len(tenants))
	for i, t := range tenants {
		responses[i] = ResponseFrom(t)
	}
	return pagination.NewPage(responses, total, req), nil
}

func (s *service) Exists(ctx context.Context, id string) (bool, error) {
	return s.repo.Exists(ctx, id)
}

func (s *service) UpdateStatus(ctx context.Context, id string, status Status) error {
	return s.repo.UpdateStatus(ctx, id, status)
}
