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

var ErrNotFound = errors.New("workload identity: credential not found")

type Credential struct {
	ClientID     string
	ClientSecret string
}

type Resolver interface {
	Resolve(ctx context.Context, tenantID string) (Credential, error)
}

type Store interface {
	Store(ctx context.Context, tenantID string, credential Credential) error
}

type storedCredential struct {
	ClientID         string
	SecretCiphertext string
}

type Repository interface {
	Upsert(ctx context.Context, tenantID string, credential storedCredential) error
	Find(ctx context.Context, tenantID string) (storedCredential, error)
}

type pgRepository struct {
	pool *pgxpool.Pool
}

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

type Service struct {
	repo          Repository
	encryptionKey string
}

func NewService(repo Repository, encryptionKey string) *Service {
	normalized, err := NormalizeEncryptionKey(encryptionKey)
	if err != nil {
		return &Service{repo: repo, encryptionKey: encryptionKey}
	}
	return &Service{repo: repo, encryptionKey: string(normalized)}
}

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
	return s.repo.Upsert(ctx, tenantID, storedCredential{ClientID: credential.ClientID, SecretCiphertext: ciphertext})
}

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

func ValidateEncryptionKey(encryptionKey string) error {
	_, err := NormalizeEncryptionKey(encryptionKey)
	return err
}
