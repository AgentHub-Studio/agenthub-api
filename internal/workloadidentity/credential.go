// Package workloadidentity stores the internal Keycloak service credential for
// each tenant. Credentials are never exposed through tenant-facing APIs.
package workloadidentity

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/crypto"
)

// ErrNotFound is returned when a tenant has no persisted workload credential.
var ErrNotFound = errors.New("workload identity: credential not found")

// Credential authenticates agenthub-api as a workload in one tenant realm.
// ClientSecret must only exist in memory while obtaining a Keycloak token.
type Credential struct {
	ClientID     string
	ClientSecret string
}

// Resolver loads one tenant-scoped workload credential.
type Resolver interface {
	Resolve(ctx context.Context, tenantID string) (Credential, error)
}

// Store persists a tenant-scoped workload credential.
type Store interface {
	Store(ctx context.Context, tenantID string, credential Credential) error
}

type storedCredential struct {
	ClientID         string
	SecretCiphertext string
}

// Repository persists encrypted workload credentials in the public schema.
type Repository interface {
	Upsert(ctx context.Context, tenantID string, credential storedCredential) error
	Find(ctx context.Context, tenantID string) (storedCredential, error)
}

type pgRepository struct {
	pool *pgxpool.Pool
}

// NewRepository creates the PostgreSQL repository for workload credentials.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

func (r *pgRepository) Upsert(ctx context.Context, tenantID string, credential storedCredential) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO public.tenant_workload_credential (tenant_id, client_id, client_secret_ciphertext)
		VALUES ($1, $2, $3)
		ON CONFLICT (tenant_id) DO UPDATE
		SET client_id = EXCLUDED.client_id,
			client_secret_ciphertext = EXCLUDED.client_secret_ciphertext,
			updated_at = NOW()`,
		tenantID, credential.ClientID, credential.SecretCiphertext,
	)
	if err != nil {
		return fmt.Errorf("workload identity: upsert credential: %w", err)
	}
	return nil
}

func (r *pgRepository) Find(ctx context.Context, tenantID string) (storedCredential, error) {
	var credential storedCredential
	err := r.pool.QueryRow(ctx, `
		SELECT client_id, client_secret_ciphertext
		FROM public.tenant_workload_credential
		WHERE tenant_id = $1`, tenantID,
	).Scan(&credential.ClientID, &credential.SecretCiphertext)
	if errors.Is(err, pgx.ErrNoRows) {
		return storedCredential{}, ErrNotFound
	}
	if err != nil {
		return storedCredential{}, fmt.Errorf("workload identity: find credential: %w", err)
	}
	return credential, nil
}

// Service encrypts credentials before persistence and decrypts them only for
// the internal client-credentials exchange.
type Service struct {
	repo          Repository
	encryptionKey string
}

// NewService creates a service backed by repo. The key must be a 32-byte
// AES-256 key; an empty key is rejected when storing or resolving credentials.
func NewService(repo Repository, encryptionKey string) *Service {
	normalized, err := NormalizeEncryptionKey(encryptionKey)
	if err != nil {
		return &Service{repo: repo, encryptionKey: encryptionKey}
	}
	return &Service{repo: repo, encryptionKey: string(normalized)}
}

// Store encrypts and upserts the tenant credential.
func (s *Service) Store(ctx context.Context, tenantID string, credential Credential) error {
	tenantID = strings.TrimSpace(tenantID)
	credential.ClientID = strings.TrimSpace(credential.ClientID)
	if tenantID == "" {
		return errors.New("workload identity: tenant is required")
	}
	if credential.ClientID == "" {
		return errors.New("workload identity: client ID is required")
	}
	if credential.ClientSecret == "" {
		return errors.New("workload identity: client secret is required")
	}
	if err := s.requireEncryptionKey(); err != nil {
		return err
	}
	ciphertext, err := crypto.Encrypt(s.encryptionKey, credential.ClientSecret)
	if err != nil {
		return fmt.Errorf("workload identity: encrypt client secret: %w", err)
	}
	return s.repo.Upsert(ctx, tenantID, storedCredential{
		ClientID:         credential.ClientID,
		SecretCiphertext: ciphertext,
	})
}

// Resolve returns the decrypted credential for one tenant. Callers must not
// persist or log the returned ClientSecret.
func (s *Service) Resolve(ctx context.Context, tenantID string) (Credential, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return Credential{}, errors.New("workload identity: tenant is required")
	}
	if err := s.requireEncryptionKey(); err != nil {
		return Credential{}, err
	}
	stored, err := s.repo.Find(ctx, tenantID)
	if err != nil {
		return Credential{}, err
	}
	secret, err := crypto.Decrypt(s.encryptionKey, stored.SecretCiphertext)
	if err != nil {
		return Credential{}, fmt.Errorf("workload identity: decrypt client secret: %w", err)
	}
	if strings.TrimSpace(stored.ClientID) == "" || secret == "" {
		return Credential{}, errors.New("workload identity: stored credential is incomplete")
	}
	return Credential{ClientID: stored.ClientID, ClientSecret: secret}, nil
}

func (s *Service) requireEncryptionKey() error {
	return ValidateEncryptionKey(s.encryptionKey)
}

// NormalizeEncryptionKey accepts a 32-byte raw key for local development or a
// base64-encoded 32-byte key for deployment, returning the raw AES-256 key.
func NormalizeEncryptionKey(encryptionKey string) ([]byte, error) {
	if len(encryptionKey) == 32 {
		return []byte(encryptionKey), nil
	}
	decoded, err := base64.StdEncoding.DecodeString(encryptionKey)
	if err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	return nil, errors.New("workload identity: MCP_RUNTIME_CREDENTIAL_ENCRYPTION_KEY must be exactly 32 raw bytes or base64-encode 32 bytes")
}

// ValidateEncryptionKey verifies that the deployment key is suitable for
// AES-256 encryption of tenant workload credentials.
func ValidateEncryptionKey(encryptionKey string) error {
	_, err := NormalizeEncryptionKey(encryptionKey)
	return err
}
