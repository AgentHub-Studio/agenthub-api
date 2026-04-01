package review

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// reviewService defines the methods used by Handler.
type reviewService interface {
	ListByListing(ctx context.Context, listingID uuid.UUID, req pagination.PageRequest) (pagination.Page[ReviewResponse], error)
	Create(ctx context.Context, listingID uuid.UUID, tenantID string, req CreateRequest) (ReviewResponse, error)
	Delete(ctx context.Context, listingID uuid.UUID, reviewID uuid.UUID, tenantID string) error
}

// Handler exposes review HTTP endpoints.
type Handler struct {
	svc reviewService
}

// NewHandler creates a new Handler.
func NewHandler(svc reviewService) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts review routes on the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/marketplace/listings/{listingId}/reviews", h.list)
	r.Post("/api/marketplace/listings/{listingId}/reviews", h.create)
	r.Delete("/api/marketplace/listings/{listingId}/reviews/{id}", h.delete)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	listingID, err := uuid.Parse(chi.URLParam(r, "listingId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid listingId")
		return
	}
	req := pagination.ParsePageRequest(r)
	page, err := h.svc.ListByListing(r.Context(), listingID, req)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, page)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	listingID, err := uuid.Parse(chi.URLParam(r, "listingId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid listingId")
		return
	}
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.svc.Create(r.Context(), listingID, tenantID, req)
	if err != nil {
		if errors.Is(err, ErrDuplicate) {
			respond.Error(w, http.StatusConflict, "review already exists")
			return
		}
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	respond.JSON(w, http.StatusCreated, resp)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	listingID, err := uuid.Parse(chi.URLParam(r, "listingId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid listingId")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.svc.Delete(r.Context(), listingID, id, tenantID); err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "review not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.NoContent(w)
}
