package suggest_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/suggest"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/integration"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

type fakeIntegrations struct {
	items []integration.Response
	err   error
}

func (f *fakeIntegrations) List(_ context.Context, _ pagination.PageRequest, filters integration.ListFilters) (pagination.Page[integration.Response], error) {
	if f.err != nil {
		return pagination.Page[integration.Response]{}, f.err
	}
	var out []integration.Response
	for _, it := range f.items {
		if filters.Enabled != nil && *filters.Enabled != it.Enabled {
			continue
		}
		out = append(out, it)
	}
	return pagination.Page[integration.Response]{Content: out, TotalElements: int64(len(out))}, nil
}

func integ(name string, t integration.IntegrationType, enabled bool) integration.Response {
	return integration.Response{
		ID:      uuid.New(),
		Name:    name,
		Slug:    name + "-slug",
		Type:    t,
		Enabled: enabled,
	}
}

func TestGenerate_ReturnsEmpty_WhenTenantHasNoIntegrations(t *testing.T) {
	svc := suggest.NewService(&fakeIntegrations{})
	got, err := svc.Generate(context.Background(), 3)

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestGenerate_PicksOneSuggestionPerIntegration(t *testing.T) {
	svc := suggest.NewService(&fakeIntegrations{items: []integration.Response{
		integ("HubSpot", integration.IntegrationTypeHTTPAPI, true),
		integ("PG", integration.IntegrationTypeDatabaseQuery, true),
		integ("GitHub MCP", integration.IntegrationTypeMCP, true),
	}})
	got, err := svc.Generate(context.Background(), 5)

	require.NoError(t, err)
	assert.Len(t, got, 3)
	assert.Contains(t, got[0].Prompt, "HubSpot")
	assert.Contains(t, got[1].Prompt, "registros")
	assert.Contains(t, got[1].Prompt, "PG")
	assert.Contains(t, got[2].Prompt, "recursos")
	assert.Contains(t, got[2].Prompt, "GitHub MCP")
}

func TestGenerate_RespectsLimit(t *testing.T) {
	svc := suggest.NewService(&fakeIntegrations{items: []integration.Response{
		integ("A", integration.IntegrationTypeHTTPAPI, true),
		integ("B", integration.IntegrationTypeHTTPAPI, true),
		integ("C", integration.IntegrationTypeHTTPAPI, true),
		integ("D", integration.IntegrationTypeHTTPAPI, true),
	}})
	got, err := svc.Generate(context.Background(), 2)

	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestGenerate_AppliesDefaultLimit_WhenZero(t *testing.T) {
	svc := suggest.NewService(&fakeIntegrations{items: []integration.Response{
		integ("A", integration.IntegrationTypeHTTPAPI, true),
		integ("B", integration.IntegrationTypeHTTPAPI, true),
		integ("C", integration.IntegrationTypeHTTPAPI, true),
		integ("D", integration.IntegrationTypeHTTPAPI, true),
	}})
	got, err := svc.Generate(context.Background(), 0)

	require.NoError(t, err)
	assert.Len(t, got, 3)
}

func TestGenerate_CapsLimitAt6(t *testing.T) {
	items := make([]integration.Response, 10)
	for i := range items {
		items[i] = integ("N"+string(rune('0'+i)), integration.IntegrationTypeHTTPAPI, true)
	}
	svc := suggest.NewService(&fakeIntegrations{items: items})
	got, err := svc.Generate(context.Background(), 99)

	require.NoError(t, err)
	assert.Len(t, got, 6)
}

func TestHandler_Get_ReturnsJSONArray(t *testing.T) {
	svc := suggest.NewService(&fakeIntegrations{items: []integration.Response{
		integ("HubSpot", integration.IntegrationTypeHTTPAPI, true),
	}})
	r := chi.NewRouter()
	suggest.NewHandler(svc).RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/chat/suggestions", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got []suggest.Suggestion
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Len(t, got, 1)
	assert.Equal(t, "HubSpot", got[0].IntegrationName)
}

func TestHandler_Get_RespectsLimitQueryParam(t *testing.T) {
	svc := suggest.NewService(&fakeIntegrations{items: []integration.Response{
		integ("A", integration.IntegrationTypeHTTPAPI, true),
		integ("B", integration.IntegrationTypeHTTPAPI, true),
		integ("C", integration.IntegrationTypeHTTPAPI, true),
	}})
	r := chi.NewRouter()
	suggest.NewHandler(svc).RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/chat/suggestions?limit=1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got []suggest.Suggestion
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Len(t, got, 1)
}
