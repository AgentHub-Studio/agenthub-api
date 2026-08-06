package execution

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/httputil"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

// executionService defines the methods used by Handler.
type executionService interface {
	List(ctx context.Context, agentID *uuid.UUID, status *string, req pagination.PageRequest) (pagination.Page[AgentExecution], error)
	Start(ctx context.Context, req StartExecutionRequest) (AgentExecution, error)
	GetByID(ctx context.Context, id uuid.UUID) (AgentExecution, error)
	Cancel(ctx context.Context, id uuid.UUID) error
	GetDetails(ctx context.Context, id uuid.UUID) (ExecutionDetails, error)
	ListNodes(ctx context.Context, executionID uuid.UUID) ([]AgentExecutionNode, error)
	ListToolExecutions(ctx context.Context, executionID uuid.UUID, nodeExecutionID uuid.UUID) ([]ToolExecution, error)
}

// Handler exposes the HTTP interface for agent executions.
type Handler struct {
	svc executionService
}

// NewHandler creates a new execution Handler.
func NewHandler(svc executionService) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts execution endpoints on the router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireRole("admin"))
		r.Get("/api/executions", h.list)
		r.Post("/api/executions", h.start)
		r.Get("/api/executions/{id}", h.getByID)
		r.Delete("/api/executions/{id}", h.cancel)
		r.Get("/api/executions/{id}/nodes", h.listNodes)
		r.Get("/api/executions/{id}/nodes/{nodeId}/tools", h.listTools)
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	req := pagination.ParsePageRequest(r)

	var agentID *uuid.UUID
	if v := r.URL.Query().Get("agentId"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			respond.Error(w, http.StatusBadRequest, "invalid agentId")
			return
		}
		agentID = &id
	}

	var status *string
	if v := r.URL.Query().Get("status"); v != "" {
		status = &v
	}

	page, err := h.svc.List(r.Context(), agentID, status, req)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to list executions")
		return
	}
	for index := range page.Content {
		page.Content[index] = PublicExecutionFrom(page.Content[index])
	}
	respond.JSON(w, http.StatusOK, page)
}

func (h *Handler) start(w http.ResponseWriter, r *http.Request) {
	var req StartExecutionRequest
	if err := httputil.DecodeSingleJSON(r.Body, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	e, err := h.svc.Start(r.Context(), req)
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			respond.Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to start execution")
		return
	}
	respond.JSON(w, http.StatusCreated, PublicExecutionFrom(e))
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid execution id")
		return
	}
	// Return full hierarchy: execution + nodes + tool executions.
	e, err := h.svc.GetDetails(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "execution not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to get execution")
		return
	}
	respond.JSON(w, http.StatusOK, PublicExecutionDetailsFrom(e))
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid execution id")
		return
	}
	if err := h.svc.Cancel(r.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "execution not found")
			return
		}
		if errors.Is(err, ErrInvalidTransition) {
			respond.Error(w, http.StatusConflict, err.Error())
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to cancel execution")
		return
	}
	respond.NoContent(w)
}

func (h *Handler) listNodes(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid execution id")
		return
	}
	nodes, err := h.svc.ListNodes(r.Context(), id)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to list nodes")
		return
	}
	for index := range nodes {
		nodes[index] = PublicExecutionNodeFrom(nodes[index])
	}
	respond.JSON(w, http.StatusOK, nodes)
}

func (h *Handler) listTools(w http.ResponseWriter, r *http.Request) {
	executionID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid execution id")
		return
	}
	nodeID, err := uuid.Parse(chi.URLParam(r, "nodeId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid node id")
		return
	}
	tools, err := h.svc.ListToolExecutions(r.Context(), executionID, nodeID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "node execution not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to list tool executions")
		return
	}
	for index := range tools {
		tools[index] = PublicToolExecutionFrom(tools[index])
	}
	respond.JSON(w, http.StatusOK, tools)
}
