package installation

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

// installationService defines the methods used by Handler.
type installationService interface {
	ListByTenant(ctx context.Context, tenantID string, req pagination.PageRequest) (pagination.Page[InstallResponse], error)
	Install(ctx context.Context, tenantID string, req InstallRequest) (InstallResponse, error)
	Uninstall(ctx context.Context, id uuid.UUID, tenantID string) error
}

// Handler exposes installation HTTP endpoints.
type Handler struct {
	svc installationService
}

// NewHandler creates a new Handler.
func NewHandler(svc installationService) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts installation routes on the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/marketplace/installations", h.list)
	r.Post("/api/marketplace/installations", h.install)
	r.Delete("/api/marketplace/installations/{id}", h.uninstall)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	req := pagination.ParsePageRequest(r)
	page, err := h.svc.ListByTenant(r.Context(), tenantID, req)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, page)
}

func (h *Handler) install(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	var req InstallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.svc.Install(r.Context(), tenantID, req)
	if err != nil {
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	respond.JSON(w, http.StatusCreated, resp)
}

func (h *Handler) uninstall(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.svc.Uninstall(r.Context(), id, tenantID); err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "installation not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.NoContent(w)
}
