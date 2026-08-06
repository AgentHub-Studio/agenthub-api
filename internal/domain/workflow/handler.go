package workflow

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/httputil"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

type workflowService interface {
	Create(ctx context.Context, req CreateRequest) (Workflow, error)
	GetBySlug(ctx context.Context, slug string) (Workflow, error)
	Execute(ctx context.Context, slug string, req ExecuteRequest) (Execution, error)
	GetExecution(ctx context.Context, id uuid.UUID) (Execution, error)
	Resume(ctx context.Context, id uuid.UUID, req ResumeRequest) (Execution, error)
}

// Handler exposes workflow HTTP routes.
type Handler struct {
	svc workflowService
}

// NewHandler creates a workflow handler.
func NewHandler(svc workflowService) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts workflow endpoints.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireRole("admin"))
		r.Post("/api/workflows", h.create)
		r.Get("/api/workflows/executions/{id}", h.getExecution)
		r.Post("/api/workflows/executions/{id}/resume", h.resume)
		r.Get("/api/workflows/{slug}", h.getBySlug)
		r.Post("/api/workflows/{slug}/execute", h.execute)
	})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := decodeSingleJSON(r.Body, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	wf, err := h.svc.Create(r.Context(), req)
	if err != nil {
		h.writeError(w, err, "workflow create failed")
		return
	}
	respond.JSON(w, http.StatusCreated, wf)
}

func (h *Handler) getBySlug(w http.ResponseWriter, r *http.Request) {
	wf, err := h.svc.GetBySlug(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		h.writeError(w, err, "workflow get failed")
		return
	}
	respond.JSON(w, http.StatusOK, wf)
}

func (h *Handler) execute(w http.ResponseWriter, r *http.Request) {
	var req ExecuteRequest
	if err := decodeSingleJSON(r.Body, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ex, err := h.svc.Execute(r.Context(), chi.URLParam(r, "slug"), req)
	if err != nil {
		h.writeError(w, err, "workflow execute failed")
		return
	}
	respond.JSON(w, http.StatusAccepted, ex)
}

func (h *Handler) getExecution(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid execution id")
		return
	}
	ex, err := h.svc.GetExecution(r.Context(), id)
	if err != nil {
		h.writeError(w, err, "workflow execution get failed")
		return
	}
	respond.JSON(w, http.StatusOK, ex)
}

func (h *Handler) resume(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid execution id")
		return
	}
	var req ResumeRequest
	if err := decodeSingleJSON(r.Body, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ex, err := h.svc.Resume(r.Context(), id, req)
	if err != nil {
		h.writeError(w, err, "workflow resume failed")
		return
	}
	respond.JSON(w, http.StatusOK, ex)
}

func decodeSingleJSON(body io.Reader, dst any) error {
	return httputil.DecodeSingleJSON(body, dst)
}

func (h *Handler) writeError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, ErrValidation):
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, ErrConflict):
		respond.Error(w, http.StatusConflict, "workflow already exists")
	case errors.Is(err, ErrNotFound):
		respond.Error(w, http.StatusNotFound, "workflow not found")
	case errors.Is(err, ErrAlreadyResolved):
		respond.Error(w, http.StatusConflict, "workflow execution already resolved")
	default:
		slog.Error(fallback, "err", err)
		respond.Error(w, http.StatusInternalServerError, fallback)
	}
}
