package listing

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

// listingService defines the methods used by Handler.
type listingService interface {
	ListAll(ctx context.Context, req pagination.PageRequest) (pagination.Page[ListingResponse], error)
	ListByType(ctx context.Context, t PackageType, req pagination.PageRequest) (pagination.Page[ListingResponse], error)
	ListByCategory(ctx context.Context, cat string, req pagination.PageRequest) (pagination.Page[ListingResponse], error)
	Create(ctx context.Context, tenantID string, req CreateListingRequest) (ListingResponse, error)
	GetByID(ctx context.Context, id uuid.UUID) (ListingResponse, error)
	Update(ctx context.Context, id uuid.UUID, tenantID string, req UpdateListingRequest) (ListingResponse, error)
	Delete(ctx context.Context, id uuid.UUID, tenantID string) error
}

// Handler exposes marketplace listing HTTP endpoints.
type Handler struct {
	svc listingService
}

// NewHandler creates a new Handler.
func NewHandler(svc listingService) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts listing routes on the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	h.RegisterReadRoutes(r)
	h.RegisterWriteRoutes(r)
}

// RegisterReadRoutes mounts the public listing read routes.
func (h *Handler) RegisterReadRoutes(r chi.Router) {
	r.Get("/api/marketplace/listings", h.list)
	r.Get("/api/marketplace/listings/{id}", h.getByID)
}

// RegisterWriteRoutes mounts listing mutations, which require a tenant.
func (h *Handler) RegisterWriteRoutes(r chi.Router) {
	r.Post("/api/marketplace/listings", h.create)
	r.Put("/api/marketplace/listings/{id}", h.update)
	r.Patch("/api/marketplace/listings/{id}", h.update)
	r.Delete("/api/marketplace/listings/{id}", h.delete)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	req := pagination.ParsePageRequest(r)
	typeFilter := r.URL.Query().Get("type")
	category := r.URL.Query().Get("category")

	var (
		page pagination.Page[ListingResponse]
		err  error
	)
	switch {
	case typeFilter != "":
		page, err = h.svc.ListByType(r.Context(), PackageType(typeFilter), req)
	case category != "":
		page, err = h.svc.ListByCategory(r.Context(), category, req)
	default:
		page, err = h.svc.ListAll(r.Context(), req)
	}
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusOK, page)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	var req CreateListingRequest
	if err := httputil.DecodeSingleJSON(r.Body, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.svc.Create(r.Context(), tenantID, req)
	if err != nil {
		if errors.Is(err, ErrDuplicateSlug) {
			respond.Error(w, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, ErrPackageNotFound) {
			respond.Error(w, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, ErrPackageNotPublic) {
			respond.Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		if errors.Is(err, ErrForbidden) {
			respond.Error(w, http.StatusForbidden, err.Error())
			return
		}
		if errors.Is(err, ErrValidation) {
			respond.Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusCreated, resp)
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	resp, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "listing not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusOK, resp)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req UpdateListingRequest
	if err := httputil.DecodeSingleJSON(r.Body, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.svc.Update(r.Context(), id, tenantID, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "listing not found")
			return
		}
		// Bug 114: ErrValidation no Update precisa de mapping 422.
		if errors.Is(err, ErrValidation) {
			respond.Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		if errors.Is(err, ErrForbidden) {
			respond.Error(w, http.StatusForbidden, err.Error())
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusOK, resp)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.svc.Delete(r.Context(), id, tenantID); err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "listing not found")
			return
		}
		if errors.Is(err, ErrForbidden) {
			respond.Error(w, http.StatusForbidden, err.Error())
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.NoContent(w)
}
