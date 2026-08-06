package review

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/httputil"
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

// listingExister verifies parent marketplace listing existence (bug 211 batch).
type listingExister interface {
	GetByID(ctx context.Context, id uuid.UUID) error
}

// Handler exposes review HTTP endpoints.
type Handler struct {
	svc     reviewService
	listing listingExister
}

// NewHandler creates a new Handler.
func NewHandler(svc reviewService) *Handler {
	return &Handler{svc: svc}
}

// WithListingExister wires parent-listing existence checker.
func (h *Handler) WithListingExister(l listingExister) *Handler {
	h.listing = l
	return h
}

// RegisterRoutes mounts review routes on the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	h.RegisterReadRoutes(r)
	h.RegisterWriteRoutes(r)
}

// RegisterReadRoutes mounts public review reads for public listings.
func (h *Handler) RegisterReadRoutes(r chi.Router) {
	r.Get("/api/marketplace/listings/{listingId}/reviews", h.list)
}

// RegisterWriteRoutes mounts review mutations, which require a tenant.
func (h *Handler) RegisterWriteRoutes(r chi.Router) {
	r.Post("/api/marketplace/listings/{listingId}/reviews", h.create)
	r.Delete("/api/marketplace/listings/{listingId}/reviews/{id}", h.delete)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	listingID, err := uuid.Parse(chi.URLParam(r, "listingId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid listingId")
		return
	}
	if h.listing != nil {
		if err := h.listing.GetByID(r.Context(), listingID); err != nil {
			respond.Error(w, http.StatusNotFound, "marketplace listing not found")
			return
		}
	}
	req := pagination.ParsePageRequest(r)
	page, err := h.svc.ListByListing(r.Context(), listingID, req)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "internal error")
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
	// Bug 215: valida listing antes de criar review (evita 422 com
	// "listing not found" — semântica REST esperada é 404).
	if h.listing != nil {
		if err := h.listing.GetByID(r.Context(), listingID); err != nil {
			respond.Error(w, http.StatusNotFound, "marketplace listing not found")
			return
		}
	}
	var req CreateRequest
	if err := httputil.DecodeSingleJSON(r.Body, &req); err != nil {
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
		if errors.Is(err, ErrForbidden) {
			respond.Error(w, http.StatusForbidden, "review forbidden")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.NoContent(w)
}
