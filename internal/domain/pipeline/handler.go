package pipeline

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

// pipelineService defines the read-only methods used by Handler.
// Write operations have been removed — pipelines are deprecated in favour of the agentic architecture.
type pipelineService interface {
	List(ctx context.Context, req pagination.PageRequest) (pagination.Page[Response], error)
	GetByID(ctx context.Context, id uuid.UUID) (Response, error)
	GetGraph(ctx context.Context, id uuid.UUID) (GraphResponse, error)
}

// Handler exposes pipeline HTTP endpoints.
type Handler struct {
	svc pipelineService
}

// NewHandler creates a new Handler.
func NewHandler(svc pipelineService) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts read-only pipeline routes on the given router.
// Write operations (POST, PUT, PATCH, DELETE) have been removed — pipelines are
// deprecated in favour of the agentic architecture (POST /api/chat/sessions/{id}/run).
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.With(deprecated).Get("/api/pipelines", h.list)
	r.With(deprecated).Get("/api/pipelines/{id}", h.getByID)
	r.With(deprecated).Get("/api/pipelines/{id}/graph", h.getGraph)
}

// deprecated is a middleware that sets the Deprecated header and logs a warning.
func deprecated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Deprecated", "true")
		w.Header().Set("Sunset", "2026-07-01")
		slog.Warn("deprecated pipeline endpoint called",
			"method", r.Method,
			"path", r.URL.Path,
		)
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	req := pagination.ParsePageRequest(r)
	page, err := h.svc.List(r.Context(), req)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, page)
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
			respond.Error(w, http.StatusNotFound, "pipeline not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, resp)
}

func (h *Handler) getGraph(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	graph, err := h.svc.GetGraph(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "pipeline not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, graph)
}

