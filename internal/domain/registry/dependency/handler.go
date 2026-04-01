package dependency

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/apierror"
)

// Handler exposes the HTTP interface for package dependencies.
type Handler struct {
	svc *Service
}

// NewHandler creates a new dependency Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts dependency endpoints on the router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/packages/{packageId}/dependencies", h.list)
	r.Post("/api/packages/{packageId}/dependencies", h.add)
	r.Delete("/api/packages/{packageId}/dependencies/{depId}", h.remove)
	r.Get("/api/packages/{packageId}/dependencies/resolved", h.resolve)
}

// list godoc — GET /api/packages/{packageId}/dependencies
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	packageID, err := uuid.Parse(chi.URLParam(r, "packageId"))
	if err != nil {
		apierror.Write(w, http.StatusBadRequest, "invalid package id")
		return
	}
	deps, err := h.svc.List(r.Context(), packageID)
	if err != nil {
		apierror.Write(w, http.StatusInternalServerError, "failed to list dependencies")
		return
	}
	apierror.WriteJSON(w, http.StatusOK, deps)
}

// add godoc — POST /api/packages/{packageId}/dependencies
func (h *Handler) add(w http.ResponseWriter, r *http.Request) {
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
	var req AddDependencyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.Write(w, http.StatusBadRequest, "invalid request body")
		return
	}
	dep, err := h.svc.Add(r.Context(), packageID, req, tenantID)
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
		apierror.Write(w, http.StatusInternalServerError, "failed to add dependency")
		return
	}
	apierror.WriteJSON(w, http.StatusCreated, dep)
}

// remove godoc — DELETE /api/packages/{packageId}/dependencies/{depId}
func (h *Handler) remove(w http.ResponseWriter, r *http.Request) {
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
	depID, err := uuid.Parse(chi.URLParam(r, "depId"))
	if err != nil {
		apierror.Write(w, http.StatusBadRequest, "invalid dependency id")
		return
	}
	if err := h.svc.Remove(r.Context(), packageID, depID, tenantID); err != nil {
		if errors.Is(err, ErrNotFound) {
			apierror.Write(w, http.StatusNotFound, "dependency not found")
			return
		}
		var fe *ForbiddenError
		if errors.As(err, &fe) {
			apierror.Write(w, http.StatusForbidden, err.Error())
			return
		}
		apierror.Write(w, http.StatusInternalServerError, "failed to remove dependency")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// resolve godoc — GET /api/packages/{packageId}/dependencies/resolved
func (h *Handler) resolve(w http.ResponseWriter, r *http.Request) {
	packageID, err := uuid.Parse(chi.URLParam(r, "packageId"))
	if err != nil {
		apierror.Write(w, http.StatusBadRequest, "invalid package id")
		return
	}
	tree, err := h.svc.Resolve(r.Context(), packageID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			apierror.Write(w, http.StatusNotFound, "package not found")
			return
		}
		apierror.Write(w, http.StatusInternalServerError, "failed to resolve dependencies")
		return
	}
	apierror.WriteJSON(w, http.StatusOK, tree)
}

type tenantContextKey struct{}

func tenantFromContext(ctx context.Context) string {
	v := ctx.Value(tenantContextKey{})
	if v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}
