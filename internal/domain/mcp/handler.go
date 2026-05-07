package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

// mcpService defines the methods used by Handler.
type mcpService interface {
	List(ctx context.Context) ([]McpServerConfigResponse, error)
	ListAutoStart(ctx context.Context) ([]McpServerConfigResponse, error)
	ListAllEnabled(ctx context.Context) ([]McpServerConfigResponse, error)
	ListBootstrap(ctx context.Context) ([]McpServerConfigBootstrapResponse, error)
	Create(ctx context.Context, req CreateRequest) (McpServerConfigResponse, error)
	GetByID(ctx context.Context, id uuid.UUID) (McpServerConfigResponse, error)
	Update(ctx context.Context, id uuid.UUID, req UpdateRequest) (McpServerConfigResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
	GetAuthStatus(ctx context.Context, id uuid.UUID) (AuthStatusResponse, error)
	GetConnectURL(ctx context.Context, id uuid.UUID, redirectURL string) (ConnectURLResponse, error)
	HandleOAuthCallback(ctx context.Context, mcpServerID uuid.UUID, code string) error
	ListTools(ctx context.Context, id uuid.UUID) ([]ToolResponse, error)
}

// listResponse wraps the flat slice in the Page envelope expected by the frontend.
func toPage(items []McpServerConfigResponse, req pagination.PageRequest) pagination.Page[McpServerConfigResponse] {
	return pagination.NewPage(items, int64(len(items)), req)
}

// Handler handles HTTP requests for MCP server configs.
type Handler struct {
	svc mcpService
}

// NewHandler creates a new Handler.
func NewHandler(svc mcpService) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts MCP server config routes onto the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/mcp-server-configs", h.list)
	r.Post("/api/mcp-server-configs", h.create)
	// Bootstrap endpoint: requires mcp-client-runtime role (P-C351-1 / ACT-F3-12).
	// Returns all auto-start configs for the runtime service to load at startup.
	r.With(middleware.RequireRole("mcp-client-runtime")).Get("/api/mcp-server-configs/bootstrap", h.bootstrap)
	r.Get("/api/mcp-server-configs/{id}", h.getByID)
	r.Put("/api/mcp-server-configs/{id}", h.update)
	r.Patch("/api/mcp-server-configs/{id}", h.update)
	r.Delete("/api/mcp-server-configs/{id}", h.delete)
	r.Get("/api/mcp-server-configs/{id}/auth-status", h.getAuthStatus)
	r.Get("/api/mcp-server-configs/{id}/connect", h.getConnectURL)
	r.Post("/api/mcp-server-configs/{id}/callback", h.handleCallback)
	r.Get("/api/mcp-server-configs/{id}/tools", h.listTools)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.List(r.Context())
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to list MCP servers")
		return
	}
	req := pagination.ParsePageRequest(r)
	respond.JSON(w, http.StatusOK, toPage(items, req))
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.svc.Create(r.Context(), req)
	if err != nil {
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
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
			respond.Error(w, http.StatusNotFound, "MCP server not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to get MCP server")
		return
	}

	respond.JSON(w, http.StatusOK, resp)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.svc.Update(r.Context(), id, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "MCP server not found")
			return
		}
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	respond.JSON(w, http.StatusOK, resp)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "MCP server not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to delete MCP server")
		return
	}

	respond.NoContent(w)
}

func (h *Handler) getAuthStatus(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	resp, err := h.svc.GetAuthStatus(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "MCP server not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to get auth status")
		return
	}

	respond.JSON(w, http.StatusOK, resp)
}

func (h *Handler) handleCallback(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Code == "" {
		respond.Error(w, http.StatusBadRequest, "code is required")
		return
	}

	if err := h.svc.HandleOAuthCallback(r.Context(), id, body.Code); err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "MCP server not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	respond.JSON(w, http.StatusOK, map[string]string{"status": "connected"})
}

func (h *Handler) listTools(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	resp, err := h.svc.ListTools(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "MCP server not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	respond.JSON(w, http.StatusOK, resp)
}

// bootstrap returns all enabled MCP server configs for the mcp-client-runtime service.
// BUG-MCP-RUNTIME-STALE fix: previously returned only auto_start=true configs, causing
// servers with auto_start=false to be invisible to agent runs after startup. Now returns
// all enabled configs so the runtime registers them for lazy connection.
// Protected by RequireRole("mcp-client-runtime") — only service accounts may call this.
func (h *Handler) bootstrap(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListBootstrap(r.Context())
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to load bootstrap configs")
		return
	}
	respond.JSON(w, http.StatusOK, items)
}

func (h *Handler) getConnectURL(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	redirectURL := r.URL.Query().Get("redirectUrl")
	if redirectURL == "" {
		respond.Error(w, http.StatusBadRequest, "redirectUrl query param is required")
		return
	}

	resp, err := h.svc.GetConnectURL(r.Context(), id, redirectURL)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "MCP server not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	respond.JSON(w, http.StatusOK, resp)
}
