package datasource_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/datasource"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// mockDatasourceSvc satisfies the private datasourceService interface in datasource.Handler.
type mockDatasourceSvc struct {
	data map[uuid.UUID]datasource.DataSource
}

func newMockDatasourceSvc() *mockDatasourceSvc {
	return &mockDatasourceSvc{data: make(map[uuid.UUID]datasource.DataSource)}
}

func (m *mockDatasourceSvc) ListAll(_ context.Context, _ string, pr pagination.PageRequest) ([]datasource.DataSource, int, error) {
	items := make([]datasource.DataSource, 0, len(m.data))
	for _, d := range m.data {
		items = append(items, d)
	}
	return items, len(items), nil
}

func (m *mockDatasourceSvc) GetByID(_ context.Context, _ string, id uuid.UUID) (datasource.DataSource, error) {
	d, ok := m.data[id]
	if !ok {
		return datasource.DataSource{}, datasource.ErrNotFound
	}
	return d, nil
}

func (m *mockDatasourceSvc) Create(_ context.Context, _ string, req datasource.CreateRequest) (datasource.DataSource, error) {
	id := uuid.New()
	d := datasource.DataSource{
		ID:   id,
		Name: req.Name,
		Type: req.Type,
		Host: req.Host,
		Port: req.Port,
	}
	m.data[id] = d
	return d, nil
}

func (m *mockDatasourceSvc) Update(_ context.Context, _ string, id uuid.UUID, req datasource.CreateRequest) (datasource.DataSource, error) {
	d, ok := m.data[id]
	if !ok {
		return datasource.DataSource{}, datasource.ErrNotFound
	}
	d.Name = req.Name
	m.data[id] = d
	return d, nil
}

func (m *mockDatasourceSvc) Delete(_ context.Context, _ string, id uuid.UUID) error {
	if _, ok := m.data[id]; !ok {
		return datasource.ErrNotFound
	}
	delete(m.data, id)
	return nil
}

func (m *mockDatasourceSvc) GetCredentials(_ context.Context, _ string, id uuid.UUID) (datasource.DataSourceCredentials, error) {
	d, ok := m.data[id]
	if !ok {
		return datasource.DataSourceCredentials{}, datasource.ErrNotFound
	}
	return datasource.DataSourceCredentials{
		ID:   d.ID,
		Name: d.Name,
		Type: d.Type,
		Host: d.Host,
		Port: d.Port,
	}, nil
}

func setupDatasource() (*chi.Mux, *mockDatasourceSvc) {
	svc := newMockDatasourceSvc()
	h := datasource.NewHandler(svc)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := tenant.NewContext(r.Context(), "test-tenant")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r.Mount("/api/datasources", h.Routes())
	r.Mount("/api/proxy/datasources", h.ProxyRoutes())
	return r, svc
}

func TestDatasourceHandler_List_Success(t *testing.T) {
	r, svc := setupDatasource()
	id := uuid.New()
	svc.data[id] = datasource.DataSource{ID: id, Name: "prod-pg", Type: datasource.DataSourceTypePostgreSQL}

	req := httptest.NewRequest(http.MethodGet, "/api/datasources/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var page pagination.Page[datasource.DataSourceResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.Equal(t, int64(1), page.TotalElements)
}

func TestDatasourceHandler_Create_Success(t *testing.T) {
	r, _ := setupDatasource()
	body, _ := json.Marshal(datasource.CreateRequest{
		Name: "my-db",
		Type: datasource.DataSourceTypePostgreSQL,
		Host: "localhost",
		Port: 5432,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/datasources/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp datasource.DataSourceResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "my-db", resp.Name)
}

func TestDatasourceHandler_Create_InvalidBody(t *testing.T) {
	r, _ := setupDatasource()
	req := httptest.NewRequest(http.MethodPost, "/api/datasources/", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDatasourceHandler_GetByID_NotFound(t *testing.T) {
	r, _ := setupDatasource()
	req := httptest.NewRequest(http.MethodGet, "/api/datasources/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDatasourceHandler_Delete_Success(t *testing.T) {
	r, svc := setupDatasource()
	id := uuid.New()
	svc.data[id] = datasource.DataSource{ID: id, Name: "to-delete"}

	req := httptest.NewRequest(http.MethodDelete, "/api/datasources/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestDatasourceHandler_Delete_NotFound(t *testing.T) {
	r, _ := setupDatasource()
	req := httptest.NewRequest(http.MethodDelete, "/api/datasources/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDatasourceHandler_GetCredentials_Success(t *testing.T) {
	r, svc := setupDatasource()
	id := uuid.New()
	svc.data[id] = datasource.DataSource{ID: id, Name: "prod-db", Type: datasource.DataSourceTypePostgreSQL}

	req := httptest.NewRequest(http.MethodGet, "/api/proxy/datasources/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDatasourceHandler_GetCredentials_NotFound(t *testing.T) {
	r, _ := setupDatasource()
	req := httptest.NewRequest(http.MethodGet, "/api/proxy/datasources/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
