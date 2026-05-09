package tenant

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/AgentHub-Studio/agenthub-api/internal/httputil"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// Handler holds HTTP handlers for the tenant domain.
type Handler struct {
	svc Service
}

// NewHandler creates a new tenant Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterPublicRoutes mounts unauthenticated tenant routes onto r.
func (h *Handler) RegisterPublicRoutes(r chi.Router) {
	r.Post("/public/tenants", h.create)
	r.Get("/public/tenants", h.list)
	r.Get("/public/tenants/{id}/exists", h.exists)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateTenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	resp, err := h.svc.Create(r.Context(), req)
	if errors.Is(err, ErrAlreadyExists) {
		httputil.Conflict(w, "tenant already exists")
		return
	}
	if errors.Is(err, ErrValidation) {
		httputil.BadRequest(w, err.Error())
		return
	}
	if err != nil {
		// Bug 189: endpoint público — não vazar repo wrapper (`tenant.Create:
		// %w`), SQL state ou Keycloak details. Log completo internamente.
		slog.Error("tenant: internal create failure", "err", err)
		httputil.InternalServerError(w, "tenant create failed")
		return
	}
	httputil.JSON(w, http.StatusCreated, resp)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	req := pagination.ParsePageRequest(r)
	page, err := h.svc.List(r.Context(), req)
	if err != nil {
		// Bug 189: endpoint público — repo error pode conter SQLSTATE.
		slog.Error("tenant: list failed", "err", err)
		httputil.InternalServerError(w, "list failed")
		return
	}
	httputil.JSON(w, http.StatusOK, page)
}

func (h *Handler) exists(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ok, err := h.svc.Exists(r.Context(), id)
	if err != nil {
		// Bug 189: endpoint público de login flow — não vazar repo error.
		slog.Error("tenant: exists check failed", "id", id, "err", err)
		httputil.InternalServerError(w, "exists check failed")
		return
	}
	httputil.JSON(w, http.StatusOK, ExistsResponse{Exists: ok})
}
