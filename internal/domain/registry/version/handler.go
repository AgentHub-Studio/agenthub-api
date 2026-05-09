package version

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/apierror"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// versionService defines the methods used by Handler.
type versionService interface {
	ListByPackage(ctx context.Context, packageID uuid.UUID) ([]VersionResponse, error)
	GetByVersion(ctx context.Context, packageID uuid.UUID, versionStr string) (VersionResponse, error)
	Publish(ctx context.Context, packageID uuid.UUID, req PublishVersionRequest, tenantID string) (VersionResponse, error)
	Delete(ctx context.Context, packageID uuid.UUID, versionStr string, tenantID string) error
}

// packageExister verifies parent package existence (bug 210 batch).
type packageExister interface {
	GetByID(ctx context.Context, id uuid.UUID) error
}

// Handler exposes the HTTP interface for package versions.
type Handler struct {
	svc versionService
	pkg packageExister
}

// NewHandler creates a new version Handler.
func NewHandler(svc versionService) *Handler {
	return &Handler{svc: svc}
}

// WithPackageExister wires the parent-package existence checker.
func (h *Handler) WithPackageExister(p packageExister) *Handler {
	h.pkg = p
	return h
}

// RegisterRoutes mounts version endpoints on the router.
// Version routes cover both public reads and authenticated writes.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/packages/{packageId}/versions", h.list)
	r.Get("/api/packages/{packageId}/versions/{version}", h.getByVersion)
	r.Post("/api/packages/{packageId}/versions", h.publish)
	r.Delete("/api/packages/{packageId}/versions/{version}", h.delete)
}

// list godoc — GET /api/packages/{packageId}/versions
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	packageID, err := uuid.Parse(chi.URLParam(r, "packageId"))
	if err != nil {
		apierror.Write(w, http.StatusBadRequest, "invalid package id")
		return
	}
	if h.pkg != nil {
		if err := h.pkg.GetByID(r.Context(), packageID); err != nil {
			apierror.Write(w, http.StatusNotFound, "package not found")
			return
		}
	}
	versions, err := h.svc.ListByPackage(r.Context(), packageID)
	if err != nil {
		apierror.Write(w, http.StatusInternalServerError, "failed to list versions")
		return
	}
	apierror.WriteJSON(w, http.StatusOK, versions)
}

// getByVersion godoc — GET /api/packages/{packageId}/versions/{version}
func (h *Handler) getByVersion(w http.ResponseWriter, r *http.Request) {
	packageID, err := uuid.Parse(chi.URLParam(r, "packageId"))
	if err != nil {
		apierror.Write(w, http.StatusBadRequest, "invalid package id")
		return
	}
	versionStr := chi.URLParam(r, "version")
	v, err := h.svc.GetByVersion(r.Context(), packageID, versionStr)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			apierror.Write(w, http.StatusNotFound, "version not found")
			return
		}
		apierror.Write(w, http.StatusInternalServerError, "failed to get version")
		return
	}
	apierror.WriteJSON(w, http.StatusOK, v)
}

// publish godoc — POST /api/packages/{packageId}/versions
func (h *Handler) publish(w http.ResponseWriter, r *http.Request) {
	tenantID := tenantFromContext(r.Context())
	if tenantID == "" {
		apierror.Write(w, http.StatusUnauthorized, "missing tenant context")
		return
	}
	packageID, err := uuid.Parse(chi.URLParam(r, "packageId"))
	if err != nil {
		apierror.Write(w, http.StatusBadRequest, "invalid package id")
		return
	}
	var req PublishVersionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.Write(w, http.StatusBadRequest, "invalid request body")
		return
	}
	v, err := h.svc.Publish(r.Context(), packageID, req, tenantID)
	if err != nil {
		var ve *ValidationError
		if errors.As(err, &ve) {
			apierror.Write(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		var fe *ForbiddenError
		if errors.As(err, &fe) {
			apierror.Write(w, http.StatusForbidden, err.Error())
			return
		}
		if errors.Is(err, ErrDuplicateVersion) {
			apierror.Write(w, http.StatusConflict, err.Error())
			return
		}
		apierror.Write(w, http.StatusInternalServerError, "failed to publish version")
		return
	}
	apierror.WriteJSON(w, http.StatusCreated, v)
}

// delete godoc — DELETE /api/packages/{packageId}/versions/{version}
func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	tenantID := tenantFromContext(r.Context())
	if tenantID == "" {
		apierror.Write(w, http.StatusUnauthorized, "missing tenant context")
		return
	}
	packageID, err := uuid.Parse(chi.URLParam(r, "packageId"))
	if err != nil {
		apierror.Write(w, http.StatusBadRequest, "invalid package id")
		return
	}
	versionStr := chi.URLParam(r, "version")
	if err := h.svc.Delete(r.Context(), packageID, versionStr, tenantID); err != nil {
		if errors.Is(err, ErrNotFound) {
			apierror.Write(w, http.StatusNotFound, "version not found")
			return
		}
		var fe *ForbiddenError
		if errors.As(err, &fe) {
			apierror.Write(w, http.StatusForbidden, err.Error())
			return
		}
		apierror.Write(w, http.StatusInternalServerError, "failed to delete version")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func tenantFromContext(ctx context.Context) string {
	return tenant.FromContext(ctx)
}
