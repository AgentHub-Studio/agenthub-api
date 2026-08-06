package main

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/workloadidentity"
)

type fakeTenantRows struct {
	tenantIDs []string
	index     int
	err       error
	closed    bool
}

func (r *fakeTenantRows) Next() bool {
	return r.index < len(r.tenantIDs)
}

func (r *fakeTenantRows) Scan(dest ...any) error {
	if len(dest) != 1 {
		return errors.New("unexpected scan destination")
	}
	id, ok := dest[0].(*string)
	if !ok {
		return errors.New("unexpected scan type")
	}
	*id = r.tenantIDs[r.index]
	r.index++
	return nil
}

func (r *fakeTenantRows) Err() error { return r.err }
func (r *fakeTenantRows) Close()     { r.closed = true }

type fakeWorkloadProvisioner struct {
	credentials map[string]workloadidentity.Credential
	errors      map[string]error
}

func (p fakeWorkloadProvisioner) EnsureWorkloadIdentity(_ context.Context, tenantID string) (workloadidentity.Credential, error) {
	if err := p.errors[tenantID]; err != nil {
		return workloadidentity.Credential{}, err
	}
	return p.credentials[tenantID], nil
}

type fakeTenantCredentialStore struct {
	stored map[string]workloadidentity.Credential
	errors map[string]error
}

func (s *fakeTenantCredentialStore) Store(_ context.Context, tenantID string, credential workloadidentity.Credential) error {
	if err := s.errors[tenantID]; err != nil {
		return err
	}
	if s.stored == nil {
		s.stored = map[string]workloadidentity.Credential{}
	}
	s.stored[tenantID] = credential
	return nil
}

func TestBackfill_SynchronizesEveryTenant(t *testing.T) {
	rows := &fakeTenantRows{tenantIDs: []string{"tenant-a", "tenant-b"}}
	provisioner := fakeWorkloadProvisioner{credentials: map[string]workloadidentity.Credential{
		"tenant-a": {ClientID: "agenthub-api", ClientSecret: "secret-a"},
		"tenant-b": {ClientID: "agenthub-api", ClientSecret: "secret-b"},
	}}
	store := &fakeTenantCredentialStore{}

	err := backfill(context.Background(), rows, provisioner, store)

	require.NoError(t, err)
	assert.True(t, rows.closed)
	assert.Equal(t, "secret-a", store.stored["tenant-a"].ClientSecret)
	assert.Equal(t, "secret-b", store.stored["tenant-b"].ClientSecret)
}

func TestBackfill_ContinuesAndFailsWhenOneTenantCannotSynchronize(t *testing.T) {
	rows := &fakeTenantRows{tenantIDs: []string{"tenant-a", "tenant-b"}}
	provisioner := fakeWorkloadProvisioner{
		credentials: map[string]workloadidentity.Credential{
			"tenant-b": {ClientID: "agenthub-api", ClientSecret: "secret-b"},
		},
		errors: map[string]error{"tenant-a": errors.New("keycloak unavailable")},
	}
	store := &fakeTenantCredentialStore{}

	err := backfill(context.Background(), rows, provisioner, store)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "1 tenant")
	assert.Equal(t, "secret-b", store.stored["tenant-b"].ClientSecret)
}
