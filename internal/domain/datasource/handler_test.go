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
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
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
	return setupDatasourceWithRoles("admin")
}

func setupDatasourceWithRoles(roles ...string) (*chi.Mux, *mockDatasourceSvc) {
	svc := newMockDatasourceSvc()
	h := datasource.NewHandler(svc)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := tenant.NewContext(r.Context(), "test-tenant")
			ctx = middleware.ContextWithRoles(ctx, roles...)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r.Mount("/api/datasources", h.Routes())
	r.Mount("/api/proxy/datasources", h.ProxyRoutes())
	return r, svc
}

func setupDatasourceWithRealService() *chi.Mux {
	svc := datasource.NewService(newMockRepo())
	h := datasource.NewHandler(svc)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := tenant.NewContext(r.Context(), "test-tenant")
			ctx = middleware.ContextWithRoles(ctx, "admin")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r.Mount("/api/datasources", h.Routes())
	return r
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
		Name:     "my-db",
		Type:     datasource.DataSourceTypePostgreSQL,
		Host:     "pg.internal",
		Port:     5432,
		Database: "appdb",
		DBUser:   "appuser",
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

func TestDatasourceHandler_Create_RejectsInternalHostWithRealService(t *testing.T) {
	r := setupDatasourceWithRealService()
	body, _ := json.Marshal(datasource.CreateRequest{
		Name:     "blocked-db",
		Type:     datasource.DataSourceTypePostgreSQL,
		Host:     "169.254.169.254",
		Port:     5432,
		Database: "appdb",
		DBUser:   "appuser",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/datasources/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestDatasourceHandler_Update_RejectsInternalHostWithRealService(t *testing.T) {
	r := setupDatasourceWithRealService()
	createBody, _ := json.Marshal(datasource.CreateRequest{
		Name:     "vpn-db",
		Type:     datasource.DataSourceTypePostgreSQL,
		Host:     "10.42.0.15",
		Port:     5432,
		Database: "appdb",
		DBUser:   "appuser",
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/datasources/", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createRes := httptest.NewRecorder()
	r.ServeHTTP(createRes, createReq)
	require.Equal(t, http.StatusCreated, createRes.Code)

	var created datasource.DataSourceResponse
	require.NoError(t, json.Unmarshal(createRes.Body.Bytes(), &created))

	updateBody, _ := json.Marshal(datasource.CreateRequest{
		Name:     "vpn-db",
		Type:     datasource.DataSourceTypePostgreSQL,
		Host:     "localhost.",
		Port:     5432,
		Database: "appdb",
		DBUser:   "appuser",
	})
	updateReq := httptest.NewRequest(http.MethodPut, "/api/datasources/"+created.ID.String(), bytes.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRes := httptest.NewRecorder()
	r.ServeHTTP(updateRes, updateReq)

	assert.Equal(t, http.StatusUnprocessableEntity, updateRes.Code)
}

func TestDatasourceHandler_Create_InvalidBody(t *testing.T) {
	r, _ := setupDatasource()
	req := httptest.NewRequest(http.MethodPost, "/api/datasources/", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDatasourceHandler_Patch_Success(t *testing.T) {
	r, svc := setupDatasource()
	id := uuid.New()
	svc.data[id] = datasource.DataSource{
		ID:   id,
		Name: "before",
		Type: datasource.DataSourceTypePostgreSQL,
		Host: "db.example.com",
		Port: 5432,
	}
	req := httptest.NewRequest(http.MethodPatch, "/api/datasources/"+id.String(), bytes.NewBufferString(`{"name":"after"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp datasource.DataSourceResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, id, resp.ID)
	assert.Equal(t, "after", resp.Name)
}

func TestDatasourceHandlerRejectsTrailingJSONWithoutServiceEffects(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		r, svc := setupDatasource()
		req := httptest.NewRequest(http.MethodPost, "/api/datasources/", bytes.NewBufferString(`{"name":"first","type":"POSTGRESQL","host":"db.example.com","port":5432}{"name":"ignored"}`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Empty(t, svc.data)
	})

	t.Run("update", func(t *testing.T) {
		r, svc := setupDatasource()
		id := uuid.New()
		svc.data[id] = datasource.DataSource{ID: id, Name: "original", Type: datasource.DataSourceTypePostgreSQL, Host: "db.example.com", Port: 5432}
		req := httptest.NewRequest(http.MethodPut, "/api/datasources/"+id.String(), bytes.NewBufferString(`{"name":"changed","type":"POSTGRESQL","host":"db.example.com","port":5432}{"name":"ignored"}`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Equal(t, "original", svc.data[id].Name)
	})
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

func TestDatasourceHandler_AdministrativeRoutesRequireAdminRole(t *testing.T) {
	r, _ := setupDatasourceWithRoles("user")
	id := uuid.NewString()
	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "list", method: http.MethodGet, path: "/api/datasources/"},
		{name: "create", method: http.MethodPost, path: "/api/datasources/", body: `{"name":"source","type":"POSTGRESQL","host":"db.example.com","port":5432,"database":"app","dbUser":"app"}`},
		{name: "get", method: http.MethodGet, path: "/api/datasources/" + id},
		{name: "put", method: http.MethodPut, path: "/api/datasources/" + id, body: `{}`},
		{name: "patch", method: http.MethodPatch, path: "/api/datasources/" + id, body: `{}`},
		{name: "delete", method: http.MethodDelete, path: "/api/datasources/" + id},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusForbidden, w.Code)
			assert.Contains(t, w.Body.String(), "missing required role")
		})
	}
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
