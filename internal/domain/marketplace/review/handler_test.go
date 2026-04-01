package review_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/marketplace/review"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	tenantpkg "github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// mockReviewSvc is an in-memory implementation of the reviewService interface.
type mockReviewSvc struct {
	reviews map[uuid.UUID]review.Review
}

func newMockReviewSvc() *mockReviewSvc {
	return &mockReviewSvc{reviews: make(map[uuid.UUID]review.Review)}
}

func (m *mockReviewSvc) ListByListing(_ context.Context, listingID uuid.UUID, req pagination.PageRequest) (pagination.Page[review.ReviewResponse], error) {
	var items []review.ReviewResponse
	for _, r := range m.reviews {
		if r.ListingID == listingID {
			items = append(items, review.ResponseFrom(r))
		}
	}
	return pagination.NewPage(items, int64(len(items)), req), nil
}

func (m *mockReviewSvc) Create(_ context.Context, listingID uuid.UUID, tenantID string, req review.CreateRequest) (review.ReviewResponse, error) {
	if req.Rating < 1 || req.Rating > 5 {
		return review.ReviewResponse{}, review.ErrNotFound
	}
	r := review.Review{
		ID:        uuid.New(),
		ListingID: listingID,
		TenantID:  tenantID,
		Rating:    req.Rating,
		Comment:   req.Comment,
		CreatedAt: time.Now(),
	}
	m.reviews[r.ID] = r
	return review.ResponseFrom(r), nil
}

func (m *mockReviewSvc) Delete(_ context.Context, listingID uuid.UUID, reviewID uuid.UUID, tenantID string) error {
	r, ok := m.reviews[reviewID]
	if !ok {
		return review.ErrNotFound
	}
	if r.ListingID != listingID || r.TenantID != tenantID {
		return review.ErrNotFound
	}
	delete(m.reviews, reviewID)
	return nil
}

func setupReviewHandler() (*chi.Mux, *mockReviewSvc) {
	svc := newMockReviewSvc()
	h := review.NewHandler(svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r, svc
}

func withTenant(req *http.Request, tenantID string) *http.Request {
	ctx := tenantpkg.NewContext(req.Context(), tenantID)
	return req.WithContext(ctx)
}

// Tests

func TestReviewHandler_List_OK(t *testing.T) {
	r, svc := setupReviewHandler()
	listingID := uuid.New()
	svc.reviews[uuid.New()] = review.Review{ID: uuid.New(), ListingID: listingID, TenantID: "t1", Rating: 5}

	req := httptest.NewRequest(http.MethodGet, "/api/marketplace/listings/"+listingID.String()+"/reviews", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var page pagination.Page[review.ReviewResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.Equal(t, int64(1), page.TotalElements)
}

func TestReviewHandler_List_InvalidListingID(t *testing.T) {
	r, _ := setupReviewHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/marketplace/listings/not-a-uuid/reviews", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestReviewHandler_Create_Success(t *testing.T) {
	r, _ := setupReviewHandler()
	listingID := uuid.New()
	body, _ := json.Marshal(review.CreateRequest{Rating: 4, Comment: "good"})
	req := httptest.NewRequest(http.MethodPost, "/api/marketplace/listings/"+listingID.String()+"/reviews", bytes.NewReader(body))
	req = withTenant(req, "tenant-1")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp review.ReviewResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 4, resp.Rating)
}

func TestReviewHandler_Create_BadBody(t *testing.T) {
	r, _ := setupReviewHandler()
	listingID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/marketplace/listings/"+listingID.String()+"/reviews", bytes.NewReader([]byte("not-json")))
	req = withTenant(req, "tenant-1")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestReviewHandler_Delete_NoContent(t *testing.T) {
	r, svc := setupReviewHandler()
	listingID := uuid.New()
	reviewID := uuid.New()
	svc.reviews[reviewID] = review.Review{ID: reviewID, ListingID: listingID, TenantID: "t1", Rating: 3}

	req := httptest.NewRequest(http.MethodDelete, "/api/marketplace/listings/"+listingID.String()+"/reviews/"+reviewID.String(), nil)
	req = withTenant(req, "t1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestReviewHandler_Delete_NotFound(t *testing.T) {
	r, _ := setupReviewHandler()
	listingID := uuid.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/marketplace/listings/"+listingID.String()+"/reviews/"+uuid.New().String(), nil)
	req = withTenant(req, "t1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestReviewHandler_Delete_InvalidID(t *testing.T) {
	r, _ := setupReviewHandler()
	listingID := uuid.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/marketplace/listings/"+listingID.String()+"/reviews/bad-id", nil)
	req = withTenant(req, "t1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
