package provider_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/provider"
)

type memRepo struct {
	data map[string]provider.Provider
}

func newMemRepo() *memRepo {
	return &memRepo{data: make(map[string]provider.Provider)}
}

func (r *memRepo) ListAll(_ context.Context) ([]provider.Provider, error) {
	out := make([]provider.Provider, 0, len(r.data))
	for _, p := range r.data {
		out = append(out, p)
	}
	return out, nil
}

func (r *memRepo) ListByKind(_ context.Context, kind provider.Kind) ([]provider.Provider, error) {
	var out []provider.Provider
	for _, p := range r.data {
		if p.Kind == kind {
			out = append(out, p)
		}
	}
	return out, nil
}

func (r *memRepo) GetBySlug(_ context.Context, slug string) (provider.Provider, error) {
	p, ok := r.data[slug]
	if !ok {
		return provider.Provider{}, provider.ErrNotFound
	}
	return p, nil
}

func seed(r *memRepo, slug string, kind provider.Kind) {
	r.data[slug] = provider.Provider{
		Slug:         slug,
		Name:         slug + "-name",
		Kind:         kind,
		TemplateJSON: json.RawMessage(`{"k":"v"}`),
		IsBuiltin:    true,
		Enabled:      true,
	}
}

func TestService_ListAll_NoFilter(t *testing.T) {
	repo := newMemRepo()
	seed(repo, "github-mcp", provider.KindMCP)
	seed(repo, "postgresql", provider.KindDatabase)
	seed(repo, "slack-api", provider.KindHTTP)

	svc := provider.NewService(repo)
	got, err := svc.ListAll(context.Background(), "")

	require.NoError(t, err)
	assert.Len(t, got, 3)
}

func TestService_ListAll_FilterByKind(t *testing.T) {
	repo := newMemRepo()
	seed(repo, "github-mcp", provider.KindMCP)
	seed(repo, "postgresql", provider.KindDatabase)
	seed(repo, "mysql", provider.KindDatabase)

	svc := provider.NewService(repo)
	got, err := svc.ListAll(context.Background(), "database")

	require.NoError(t, err)
	assert.Len(t, got, 2)
	for _, p := range got {
		assert.Equal(t, provider.KindDatabase, p.Kind)
	}
}

func TestService_GetBySlug_Hit(t *testing.T) {
	repo := newMemRepo()
	seed(repo, "slack-api", provider.KindHTTP)

	svc := provider.NewService(repo)
	got, err := svc.GetBySlug(context.Background(), "slack-api")

	require.NoError(t, err)
	assert.Equal(t, "slack-api", got.Slug)
	assert.Equal(t, provider.KindHTTP, got.Kind)
}

func TestService_GetBySlug_NotFound(t *testing.T) {
	repo := newMemRepo()

	svc := provider.NewService(repo)
	_, err := svc.GetBySlug(context.Background(), "does-not-exist")

	assert.ErrorIs(t, err, provider.ErrNotFound)
}

func TestResponseFrom_PreservesTemplateJSON(t *testing.T) {
	p := provider.Provider{
		Slug:         "generic-http",
		Kind:         provider.KindHTTP,
		TemplateJSON: json.RawMessage(`{"baseUrl":""}`),
	}
	got := provider.ResponseFrom(p)

	assert.Equal(t, "generic-http", got.Slug)
	assert.JSONEq(t, `{"baseUrl":""}`, string(got.Template))
}
