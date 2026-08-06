package workloadidentity

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type memoryRepository struct {
	values map[string]storedCredential
}

func (r *memoryRepository) Upsert(_ context.Context, tenantID string, credential storedCredential) error {
	if r.values == nil {
		r.values = make(map[string]storedCredential)
	}
	r.values[tenantID] = credential
	return nil
}

func (r *memoryRepository) Find(_ context.Context, tenantID string) (storedCredential, error) {
	credential, ok := r.values[tenantID]
	if !ok {
		return storedCredential{}, ErrNotFound
	}
	return credential, nil
}

func TestService_StoresEncryptedCredentialAndResolvesByTenant(t *testing.T) {
	repo := &memoryRepository{}
	service := NewService(repo, "12345678901234567890123456789012")

	err := service.Store(context.Background(), "tenant-a", Credential{
		ClientID:     "agenthub-api",
		ClientSecret: "tenant-a-secret",
	})
	require.NoError(t, err)
	assert.NotEqual(t, "tenant-a-secret", repo.values["tenant-a"].SecretCiphertext)

	credential, err := service.Resolve(context.Background(), "tenant-a")
	require.NoError(t, err)
	assert.Equal(t, "agenthub-api", credential.ClientID)
	assert.Equal(t, "tenant-a-secret", credential.ClientSecret)
}

func TestService_RejectsMissingOrUnsafeEncryptionKey(t *testing.T) {
	service := NewService(&memoryRepository{}, "")
	err := service.Store(context.Background(), "tenant-a", Credential{ClientID: "agenthub-api", ClientSecret: "secret"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MCP_RUNTIME_CREDENTIAL_ENCRYPTION_KEY")
}

func TestService_AcceptsBase64EncodedDeploymentKey(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte("12345678901234567890123456789012"))
	service := NewService(&memoryRepository{}, key)

	err := service.Store(context.Background(), "tenant-a", Credential{ClientID: "agenthub-api", ClientSecret: "secret"})

	require.NoError(t, err)
}

func TestService_ResolveDoesNotCrossTenantBoundary(t *testing.T) {
	repo := &memoryRepository{}
	service := NewService(repo, "12345678901234567890123456789012")
	require.NoError(t, service.Store(context.Background(), "tenant-a", Credential{ClientID: "agenthub-api", ClientSecret: "tenant-a-secret"}))

	_, err := service.Resolve(context.Background(), "tenant-b")
	require.ErrorIs(t, err, ErrNotFound)
}
