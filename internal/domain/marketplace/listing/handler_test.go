package listing_test

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

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/marketplace/listing"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	tenantpkg "github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// mockListingSvc is an in-memory implementation of the listingService interface.
type mockListingSvc struct {
	data      map[uuid.UUID]listing.Listing
	createErr error
	updateErr error
	deleteErr error
}

func newMockListingSvc() *mockListingSvc {
	return &mockListingSvc{data: make(map[uuid.UUID]listing.Listing)}
}

func (m *mockListingSvc) ListAll(_ context.Context, req pagination.PageRequest) (pagination.Page[listing.ListingResponse], error) {
	resp := listingPage(m.data, req)
	return resp, nil
}

func (m *mockListingSvc) ListByType(_ context.Context, t listing.PackageType, req pagination.PageRequest) (pagination.Page[listing.ListingResponse], error) {
	var filtered []listing.Listing
	for _, l := range m.data {
		if l.Type == t {
			filtered = append(filtered, l)
		}
	}
	return listingPageFrom(filtered, req), nil
}

func (m *mockListingSvc) ListByCategory(_ context.Context, cat string, req pagination.PageRequest) (pagination.Page[listing.ListingResponse], error) {
	var filtered []listing.Listing
	for _, l := range m.data {
		if l.Category == cat {
			filtered = append(filtered, l)
		}
	}
	return listingPageFrom(filtered, req), nil
}

func (m *mockListingSvc) GetByID(_ context.Context, id uuid.UUID) (listing.ListingResponse, error) {
	l, ok := m.data[id]
	if !ok {
		return listing.ListingResponse{}, listing.ErrNotFound
	}
	return listing.ResponseFrom(l), nil
}

func (m *mockListingSvc) Create(_ context.Context, tenantID string, req listing.CreateListingRequest) (listing.ListingResponse, error) {
	if m.createErr != nil {
		return listing.ListingResponse{}, m.createErr
	}
	if req.Name == "" {
		return listing.ListingResponse{}, listing.ErrNotFound
	}
	l := listing.Listing{
		ID:       uuid.New(),
		TenantID: tenantID,
		Name:     req.Name,
		Slug:     req.Slug,
		Type:     listing.PackageType(req.Type),
		Status:   listing.StatusActive,
	}
	m.data[l.ID] = l
	return listing.ResponseFrom(l), nil
}

func (m *mockListingSvc) Update(_ context.Context, id uuid.UUID, tenantID string, req listing.UpdateListingRequest) (listing.ListingResponse, error) {
	if m.updateErr != nil {
		return listing.ListingResponse{}, m.updateErr
	}
	l, ok := m.data[id]
	if !ok {
		return listing.ListingResponse{}, listing.ErrNotFound
	}
	if req.Name != nil {
		l.Name = *req.Name
	}
	m.data[id] = l
	return listing.ResponseFrom(l), nil
}

func (m *mockListingSvc) Delete(_ context.Context, id uuid.UUID, tenantID string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if _, ok := m.data[id]; !ok {
		return listing.ErrNotFound
	}
	delete(m.data, id)
	return nil
}

func listingPage(data map[uuid.UUID]listing.Listing, req pagination.PageRequest) pagination.Page[listing.ListingResponse] {
	items := make([]listing.Listing, 0, len(data))
	for _, l := range data {
		items = append(items, l)
	}
	return listingPageFrom(items, req)
}

func listingPageFrom(items []listing.Listing, req pagination.PageRequest) pagination.Page[listing.ListingResponse] {
	responses := make([]listing.ListingResponse, len(items))
	for i, l := range items {
		responses[i] = listing.ResponseFrom(l)
	}
	return pagination.NewPage(responses, int64(len(responses)), req)
}

func setupListingHandler() (*chi.Mux, *mockListingSvc) {
	svc := newMockListingSvc()
	h := listing.NewHandler(svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r, svc
}

func withTenant(req *http.Request, tenantID string) *http.Request {
	ctx := tenantpkg.NewContext(req.Context(), tenantID)
	return req.WithContext(ctx)
}

// Tests

func TestListingHandler_List_OK(t *testing.T) {
	r, svc := setupListingHandler()
	svc.data[uuid.New()] = listing.Listing{ID: uuid.New(), Name: "A", Type: listing.PackageTypeAgent, Status: listing.StatusActive}

	req := httptest.NewRequest(http.MethodGet, "/api/marketplace/listings", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var page pagination.Page[listing.ListingResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.Equal(t, int64(1), page.TotalElements)
}

func TestListingHandler_List_Empty(t *testing.T) {
	r, _ := setupListingHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/marketplace/listings", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var page pagination.Page[listing.ListingResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.True(t, page.Empty)
}

func TestListingHandler_Create_Success(t *testing.T) {
	r, _ := setupListingHandler()
	body, _ := json.Marshal(listing.CreateListingRequest{
		Name:      "Agent Alpha",
		PackageID: uuid.New(),
		Type:      "AGENT",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/marketplace/listings", bytes.NewReader(body))
	req = withTenant(req, "tenant-1")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp listing.ListingResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "Agent Alpha", resp.Name)
}

func TestListingHandler_Create_BadBody(t *testing.T) {
	r, _ := setupListingHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/marketplace/listings", bytes.NewReader([]byte("not-json")))
	req = withTenant(req, "tenant-1")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListingHandler_Create_MapsPackageAndAuthorizationErrors(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{name: "missing package", err: listing.ErrPackageNotFound, want: http.StatusNotFound},
		{name: "other tenant package", err: listing.ErrForbidden, want: http.StatusForbidden},
		{name: "private package", err: listing.ErrPackageNotPublic, want: http.StatusUnprocessableEntity},
		{name: "invalid package", err: listing.ErrValidation, want: http.StatusUnprocessableEntity},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, svc := setupListingHandler()
			svc.createErr = tc.err
			req := httptest.NewRequest(http.MethodPost, "/api/marketplace/listings", bytes.NewBufferString(`{"name":"listing","packageId":"`+uuid.NewString()+`","type":"AGENT"}`))
			req = withTenant(req, "tenant-1")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, tc.want, w.Code)
			assert.Empty(t, svc.data)
		})
	}
}

func TestListingHandler_CreateAndUpdateRejectTrailingJSONWithoutServiceEffects(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		r, svc := setupListingHandler()
		req := httptest.NewRequest(http.MethodPost, "/api/marketplace/listings", bytes.NewBufferString(`{"name":"first","type":"AGENT"} {"name":"ignored"}`))
		req = withTenant(req, "tenant-1")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Empty(t, svc.data)
	})

	t.Run("update", func(t *testing.T) {
		r, svc := setupListingHandler()
		id := uuid.New()
		original := listing.Listing{ID: id, TenantID: "tenant-1", Name: "unchanged", Status: listing.StatusActive}
		svc.data[id] = original
		req := httptest.NewRequest(http.MethodPut, "/api/marketplace/listings/"+id.String(), bytes.NewBufferString(`{"name":"changed"} {"name":"ignored"}`))
		req = withTenant(req, "tenant-1")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, original, svc.data[id])
	})
}

func TestListingHandler_GetByID_OK(t *testing.T) {
	r, svc := setupListingHandler()
	id := uuid.New()
	svc.data[id] = listing.Listing{ID: id, Name: "Beta", Status: listing.StatusActive}

	req := httptest.NewRequest(http.MethodGet, "/api/marketplace/listings/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestListingHandler_GetByID_NotFound(t *testing.T) {
	r, _ := setupListingHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/marketplace/listings/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestListingHandler_GetByID_InvalidID(t *testing.T) {
	r, _ := setupListingHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/marketplace/listings/not-a-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListingHandler_Delete_NoContent(t *testing.T) {
	r, svc := setupListingHandler()
	id := uuid.New()
	svc.data[id] = listing.Listing{ID: id, TenantID: "t1", Status: listing.StatusActive}

	req := httptest.NewRequest(http.MethodDelete, "/api/marketplace/listings/"+id.String(), nil)
	req = withTenant(req, "t1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestListingHandler_Delete_NotFound(t *testing.T) {
	r, _ := setupListingHandler()
	req := httptest.NewRequest(http.MethodDelete, "/api/marketplace/listings/"+uuid.New().String(), nil)
	req = withTenant(req, "t1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestListingHandler_Delete_Forbidden(t *testing.T) {
	r, svc := setupListingHandler()
	svc.deleteErr = listing.ErrForbidden
	req := withTenant(httptest.NewRequest(http.MethodDelete, "/api/marketplace/listings/"+uuid.NewString(), nil), "tenant-1")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestListingHandler_Update_OK(t *testing.T) {
	r, svc := setupListingHandler()
	id := uuid.New()
	svc.data[id] = listing.Listing{ID: id, TenantID: "t1", Name: "Old", Status: listing.StatusActive}

	newName := "New Name"
	body, _ := json.Marshal(listing.UpdateListingRequest{Name: &newName})
	req := httptest.NewRequest(http.MethodPut, "/api/marketplace/listings/"+id.String(), bytes.NewReader(body))
	req = withTenant(req, "t1")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestListingHandler_Update_Forbidden(t *testing.T) {
	r, svc := setupListingHandler()
	svc.updateErr = listing.ErrForbidden
	name := "not applied"
	body, err := json.Marshal(listing.UpdateListingRequest{Name: &name})
	require.NoError(t, err)
	req := withTenant(httptest.NewRequest(http.MethodPatch, "/api/marketplace/listings/"+uuid.NewString(), bytes.NewReader(body)), "tenant-1")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}
