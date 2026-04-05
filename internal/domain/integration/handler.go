package integration

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/datasource"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/mcp"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tool"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

type integrationService interface {
	List(ctx context.Context, req pagination.PageRequest, filters ListFilters) (pagination.Page[Response], error)
	CreateHTTP(ctx context.Context, req HTTPCreateRequest) (HTTPResponse, error)
	GetHTTP(ctx context.Context, id uuid.UUID) (HTTPResponse, error)
	UpdateHTTP(ctx context.Context, id uuid.UUID, req HTTPCreateRequest) (HTTPResponse, error)
	DeleteHTTP(ctx context.Context, id uuid.UUID) error
	CreateDatabase(ctx context.Context, req DatabaseCreateRequest) (DatabaseResponse, error)
	GetDatabase(ctx context.Context, id uuid.UUID) (DatabaseResponse, error)
	UpdateDatabase(ctx context.Context, id uuid.UUID, req DatabaseCreateRequest) (DatabaseResponse, error)
	DeleteDatabase(ctx context.Context, id uuid.UUID) error
	CreateMCP(ctx context.Context, req mcp.CreateRequest) (mcp.McpServerConfigResponse, error)
	GetMCP(ctx context.Context, id uuid.UUID) (mcp.McpServerConfigResponse, error)
	UpdateMCP(ctx context.Context, id uuid.UUID, req mcp.UpdateRequest) (mcp.McpServerConfigResponse, error)
	DeleteMCP(ctx context.Context, id uuid.UUID) error
}

// Handler exposes integration catalog endpoints.
type Handler struct {
	svc integrationService
}

// NewHandler creates a new integration handler.
func NewHandler(svc integrationService) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts integration routes on the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/integrations", h.list)
	r.Post("/api/integrations/http", h.createHTTP)
	r.Get("/api/integrations/http/{id}", h.getHTTP)
	r.Put("/api/integrations/http/{id}", h.updateHTTP)
	r.Patch("/api/integrations/http/{id}", h.updateHTTP)
	r.Delete("/api/integrations/http/{id}", h.deleteHTTP)
	r.Post("/api/integrations/database", h.createDatabase)
	r.Get("/api/integrations/database/{id}", h.getDatabase)
	r.Put("/api/integrations/database/{id}", h.updateDatabase)
	r.Patch("/api/integrations/database/{id}", h.updateDatabase)
	r.Delete("/api/integrations/database/{id}", h.deleteDatabase)
	r.Post("/api/integrations/mcp", h.createMCP)
	r.Get("/api/integrations/mcp/{id}", h.getMCP)
	r.Put("/api/integrations/mcp/{id}", h.updateMCP)
	r.Patch("/api/integrations/mcp/{id}", h.updateMCP)
	r.Delete("/api/integrations/mcp/{id}", h.deleteMCP)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	filters, err := parseFilters(r)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	page, err := h.svc.List(r.Context(), pagination.ParsePageRequest(r), filters)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	respond.JSON(w, http.StatusOK, page)
}

func parseFilters(r *http.Request) (ListFilters, error) {
	var filters ListFilters
	query := r.URL.Query()

	if rawType := query.Get("type"); rawType != "" {
		parsed := IntegrationType(rawType)
		switch parsed {
		case IntegrationTypeHTTPAPI, IntegrationTypeDatabaseQuery, IntegrationTypeMCP:
			filters.Type = &parsed
		default:
			return ListFilters{}, errInvalidFilter("type")
		}
	}

	if rawEnabled := query.Get("enabled"); rawEnabled != "" {
		parsed, err := strconv.ParseBool(rawEnabled)
		if err != nil {
			return ListFilters{}, errInvalidFilter("enabled")
		}
		filters.Enabled = &parsed
	}

	if rawOrigin := query.Get("origin"); rawOrigin != "" {
		parsed := IntegrationOrigin(rawOrigin)
		switch parsed {
		case IntegrationOriginLegacy, IntegrationOriginGenerated, IntegrationOriginManual:
			filters.Origin = &parsed
		default:
			return ListFilters{}, errInvalidFilter("origin")
		}
	}

	return filters, nil
}

func errInvalidFilter(name string) error {
	return &filterError{name: name}
}

type filterError struct{ name string }

func (e *filterError) Error() string { return "invalid " + e.name + " filter" }

func (h *Handler) createHTTP(w http.ResponseWriter, r *http.Request) {
	var req HTTPCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.svc.CreateHTTP(r.Context(), req)
	if err != nil {
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	respond.JSON(w, http.StatusCreated, resp)
}

func (h *Handler) getHTTP(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	resp, err := h.svc.GetHTTP(r.Context(), id)
	if err != nil {
		if errors.Is(err, tool.ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "integration not found")
			return
		}
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, resp)
}

func (h *Handler) updateHTTP(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req HTTPCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.svc.UpdateHTTP(r.Context(), id, req)
	if err != nil {
		if errors.Is(err, tool.ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "integration not found")
			return
		}
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, resp)
}

func (h *Handler) deleteHTTP(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.svc.DeleteHTTP(r.Context(), id); err != nil {
		if errors.Is(err, tool.ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "integration not found")
			return
		}
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	respond.NoContent(w)
}

func (h *Handler) createDatabase(w http.ResponseWriter, r *http.Request) {
	var req DatabaseCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.svc.CreateDatabase(r.Context(), req)
	if err != nil {
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	respond.JSON(w, http.StatusCreated, resp)
}

func (h *Handler) getDatabase(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	resp, err := h.svc.GetDatabase(r.Context(), id)
	if err != nil {
		if errors.Is(err, datasource.ErrNotFound) || errors.Is(err, tool.ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "integration not found")
			return
		}
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, resp)
}

func (h *Handler) updateDatabase(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req DatabaseCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.svc.UpdateDatabase(r.Context(), id, req)
	if err != nil {
		if errors.Is(err, datasource.ErrNotFound) || errors.Is(err, tool.ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "integration not found")
			return
		}
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, resp)
}

func (h *Handler) deleteDatabase(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.svc.DeleteDatabase(r.Context(), id); err != nil {
		if errors.Is(err, datasource.ErrNotFound) || errors.Is(err, tool.ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "integration not found")
			return
		}
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	respond.NoContent(w)
}

func (h *Handler) createMCP(w http.ResponseWriter, r *http.Request) {
	var req mcp.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.svc.CreateMCP(r.Context(), req)
	if err != nil {
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	respond.JSON(w, http.StatusCreated, resp)
}

func (h *Handler) getMCP(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	resp, err := h.svc.GetMCP(r.Context(), id)
	if err != nil {
		if errors.Is(err, mcp.ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "integration not found")
			return
		}
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, resp)
}

func (h *Handler) updateMCP(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req mcp.UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.svc.UpdateMCP(r.Context(), id, req)
	if err != nil {
		if errors.Is(err, mcp.ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "integration not found")
			return
		}
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, resp)
}

func (h *Handler) deleteMCP(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.svc.DeleteMCP(r.Context(), id); err != nil {
		if errors.Is(err, mcp.ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "integration not found")
			return
		}
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	respond.NoContent(w)
}
