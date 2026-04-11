package pkg

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/apierror"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// packageService defines the methods used by Handler.
type packageService interface {
	ListPublic(ctx context.Context, req pagination.PageRequest) (pagination.Page[PackageResponse], error)
	GetByID(ctx context.Context, id uuid.UUID) (PackageResponse, error)
	GetBySlug(ctx context.Context, slug string) (PackageResponse, error)
	ListByTenant(ctx context.Context, tenantID string, req pagination.PageRequest) (pagination.Page[PackageResponse], error)
	Create(ctx context.Context, req CreatePackageRequest, tenantID string) (PackageResponse, error)
	Update(ctx context.Context, id uuid.UUID, req UpdatePackageRequest, tenantID string) (PackageResponse, error)
	Delete(ctx context.Context, id uuid.UUID, tenantID string) error
	Search(ctx context.Context, query string, pkgType *string, req pagination.PageRequest) (pagination.Page[PackageResponse], error)
}

// Handler exposes the HTTP interface for the package registry.
type Handler struct {
	svc packageService
}

// NewHandler creates a new package Handler.
func NewHandler(svc packageService) *Handler {
	return &Handler{svc: svc}
}

// RegisterPublicRoutes mounts read-only routes that require no authentication.
func (h *Handler) RegisterPublicRoutes(r chi.Router) {
	r.Get("/api/packages", h.listPublic)
	r.Get("/api/packages/search", h.search)
	r.Get("/api/packages/{id}", h.getByID)
	r.Get("/api/packages/slug/{slug}", h.getBySlug)
}

// RegisterProtectedRoutes mounts routes that require a valid JWT / tenant context.
func (h *Handler) RegisterProtectedRoutes(r chi.Router) {
	r.Get("/api/packages/mine", h.listMine)
	r.Post("/api/packages", h.create)
	r.Patch("/api/packages/{id}", h.update)
	r.Delete("/api/packages/{id}", h.delete)
}

// listPublic godoc — GET /api/packages
func (h *Handler) listPublic(w http.ResponseWriter, r *http.Request) {
	req := pagination.ParsePageRequest(r)
	page, err := h.svc.ListPublic(r.Context(), req)
	if err != nil {
		apierror.Write(w, http.StatusInternalServerError, "failed to list packages")
		return
	}
	apierror.WriteJSON(w, http.StatusOK, page)
}

// getByID godoc — GET /api/packages/{id}
func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		apierror.Write(w, http.StatusBadRequest, "invalid package id")
		return
	}
	p, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			apierror.Write(w, http.StatusNotFound, "package not found")
			return
		}
		apierror.Write(w, http.StatusInternalServerError, "failed to get package")
		return
	}
	apierror.WriteJSON(w, http.StatusOK, p)
}

// getBySlug godoc — GET /api/packages/slug/{slug}
func (h *Handler) getBySlug(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	p, err := h.svc.GetBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			apierror.Write(w, http.StatusNotFound, "package not found")
			return
		}
		apierror.Write(w, http.StatusInternalServerError, "failed to get package")
		return
	}
	apierror.WriteJSON(w, http.StatusOK, p)
}

// listMine godoc — GET /api/packages/mine
func (h *Handler) listMine(w http.ResponseWriter, r *http.Request) {
	tenantID := tenantFromContext(r)
	if tenantID == "" {
		apierror.Write(w, http.StatusUnauthorized, "missing tenant context")
		return
	}
	req := pagination.ParsePageRequest(r)
	page, err := h.svc.ListByTenant(r.Context(), tenantID, req)
	if err != nil {
		apierror.Write(w, http.StatusInternalServerError, "failed to list packages")
		return
	}
	apierror.WriteJSON(w, http.StatusOK, page)
}

// create godoc — POST /api/packages
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	tenantID := tenantFromContext(r)
	if tenantID == "" {
		apierror.Write(w, http.StatusUnauthorized, "missing tenant context")
		return
	}
	var req CreatePackageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.Write(w, http.StatusBadRequest, "invalid request body")
		return
	}
	p, err := h.svc.Create(r.Context(), req, tenantID)
	if err != nil {
		var ve *ValidationError
		if errors.As(err, &ve) {
			apierror.Write(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		apierror.Write(w, http.StatusInternalServerError, "failed to create package")
		return
	}
	apierror.WriteJSON(w, http.StatusCreated, p)
}

// update godoc — PATCH /api/packages/{id}
func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	tenantID := tenantFromContext(r)
	if tenantID == "" {
		apierror.Write(w, http.StatusUnauthorized, "missing tenant context")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		apierror.Write(w, http.StatusBadRequest, "invalid package id")
		return
	}
	var req UpdatePackageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.Write(w, http.StatusBadRequest, "invalid request body")
		return
	}
	p, err := h.svc.Update(r.Context(), id, req, tenantID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			apierror.Write(w, http.StatusNotFound, "package not found")
			return
		}
		var fe *ForbiddenError
		if errors.As(err, &fe) {
			apierror.Write(w, http.StatusForbidden, err.Error())
			return
		}
		apierror.Write(w, http.StatusInternalServerError, "failed to update package")
		return
	}
	apierror.WriteJSON(w, http.StatusOK, p)
}

// delete godoc — DELETE /api/packages/{id}
func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	tenantID := tenantFromContext(r)
	if tenantID == "" {
		apierror.Write(w, http.StatusUnauthorized, "missing tenant context")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		apierror.Write(w, http.StatusBadRequest, "invalid package id")
		return
	}
	if err := h.svc.Delete(r.Context(), id, tenantID); err != nil {
		if errors.Is(err, ErrNotFound) {
			apierror.Write(w, http.StatusNotFound, "package not found")
			return
		}
		var fe *ForbiddenError
		if errors.As(err, &fe) {
			apierror.Write(w, http.StatusForbidden, err.Error())
			return
		}
		apierror.Write(w, http.StatusInternalServerError, "failed to delete package")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// search godoc — GET /api/packages/search?q=...&type=...
func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		apierror.Write(w, http.StatusBadRequest, "q parameter is required")
		return
	}
	var pkgType *string
	if t := r.URL.Query().Get("type"); t != "" {
		pkgType = &t
	}
	req := pagination.ParsePageRequest(r)
	page, err := h.svc.Search(r.Context(), q, pkgType, req)
	if err != nil {
		apierror.Write(w, http.StatusInternalServerError, "failed to search packages")
		return
	}
	apierror.WriteJSON(w, http.StatusOK, page)
}

// tenantFromContext extracts the tenant ID from the request context.
func tenantFromContext(r *http.Request) string {
	return tenant.FromContext(r.Context())
}
