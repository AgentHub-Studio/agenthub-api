package trigger

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

// triggerService defines the methods used by Handler.
type triggerService interface {
	Create(ctx context.Context, agentID uuid.UUID, req CreateTriggerRequest) (AgentTrigger, error)
	GetByID(ctx context.Context, id uuid.UUID) (AgentTrigger, error)
	List(ctx context.Context, agentID uuid.UUID, page pagination.PageRequest) (pagination.Page[AgentTrigger], error)
	Update(ctx context.Context, id uuid.UUID, req UpdateTriggerRequest) (AgentTrigger, error)
	Delete(ctx context.Context, id uuid.UUID) error
	ListRuns(ctx context.Context, triggerID uuid.UUID, page pagination.PageRequest) (pagination.Page[AgentTriggerRun], error)
}

// agentExister verifies parent agent existence (bug 209 batch).
type agentExister interface {
	GetByID(ctx context.Context, id uuid.UUID) error
}

// Handler exposes HTTP endpoints for agent triggers.
type Handler struct {
	svc   triggerService
	agent agentExister
}

// NewHandler creates a new trigger Handler.
func NewHandler(svc triggerService) *Handler {
	return &Handler{svc: svc}
}

// WithAgentExister wires agent existence checker for parent-resource validation.
func (h *Handler) WithAgentExister(a agentExister) *Handler {
	h.agent = a
	return h
}

// RegisterRoutes mounts trigger endpoints on the router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireRole("admin"))
		r.Post("/api/agents/{agentId}/triggers", h.create)
		r.Get("/api/agents/{agentId}/triggers", h.list)
		r.Get("/api/agents/{agentId}/triggers/{triggerId}", h.getByID)
		r.Put("/api/agents/{agentId}/triggers/{triggerId}", h.update)
		r.Patch("/api/agents/{agentId}/triggers/{triggerId}", h.update)
		r.Delete("/api/agents/{agentId}/triggers/{triggerId}", h.delete)
		r.Get("/api/agents/{agentId}/triggers/{triggerId}/runs", h.listRuns)
	})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	agentID, err := uuid.Parse(chi.URLParam(r, "agentId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agent id")
		return
	}
	var req CreateTriggerRequest
	if err := httputil.DecodeSingleJSON(r.Body, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	t, err := h.svc.Create(r.Context(), agentID, req)
	if err != nil {
		if errors.Is(err, ErrAgentNotFound) {
			respond.Error(w, http.StatusNotFound, "agent not found")
			return
		}
		if errors.Is(err, ErrDuplicateName) {
			respond.Error(w, http.StatusConflict, err.Error())
			return
		}
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	respond.JSON(w, http.StatusCreated, t)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	agentID, err := uuid.Parse(chi.URLParam(r, "agentId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agent id")
		return
	}
	if h.agent != nil {
		if err := h.agent.GetByID(r.Context(), agentID); err != nil {
			respond.Error(w, http.StatusNotFound, "agent not found")
			return
		}
	}
	page := pagination.ParsePageRequest(r)
	result, err := h.svc.List(r.Context(), agentID, page)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to list triggers")
		return
	}
	respond.JSON(w, http.StatusOK, result)
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	t, ok := h.triggerForRoute(w, r)
	if !ok {
		return
	}
	respond.JSON(w, http.StatusOK, t)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	t, ok := h.triggerForRoute(w, r)
	if !ok {
		return
	}
	var req UpdateTriggerRequest
	if err := httputil.DecodeSingleJSON(r.Body, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	t, err := h.svc.Update(r.Context(), t.ID, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "trigger not found")
			return
		}
		// Bug 117: erros de validação (cron inválido, name vazio,
		// etc.) devem ser 422 — Create já mapeia assim. Mantemos
		// 422 como default; reservamos 500 para erros de repo.
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, t)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	t, ok := h.triggerForRoute(w, r)
	if !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), t.ID); err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "trigger not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to delete trigger")
		return
	}
	respond.NoContent(w)
}

func (h *Handler) listRuns(w http.ResponseWriter, r *http.Request) {
	trigger, ok := h.triggerForRoute(w, r)
	if !ok {
		return
	}
	page := pagination.ParsePageRequest(r)
	result, err := h.svc.ListRuns(r.Context(), trigger.ID, page)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to list runs")
		return
	}
	for index := range result.Content {
		result.Content[index] = PublicRunResponseFrom(result.Content[index])
	}
	respond.JSON(w, http.StatusOK, result)
}

// triggerForRoute loads a trigger only after validating its parent agent from
// the nested route. This prevents a trigger identifier from being reused under
// a different agent path in the same tenant.
func (h *Handler) triggerForRoute(w http.ResponseWriter, r *http.Request) (AgentTrigger, bool) {
	agentID, err := uuid.Parse(chi.URLParam(r, "agentId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agent id")
		return AgentTrigger{}, false
	}
	if h.agent != nil {
		if err := h.agent.GetByID(r.Context(), agentID); err != nil {
			respond.Error(w, http.StatusNotFound, "agent not found")
			return AgentTrigger{}, false
		}
	}

	triggerID, err := uuid.Parse(chi.URLParam(r, "triggerId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid trigger id")
		return AgentTrigger{}, false
	}
	trigger, err := h.svc.GetByID(r.Context(), triggerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "trigger not found")
			return AgentTrigger{}, false
		}
		respond.Error(w, http.StatusInternalServerError, "failed to get trigger")
		return AgentTrigger{}, false
	}
	if trigger.AgentID != agentID {
		respond.Error(w, http.StatusNotFound, "trigger not found")
		return AgentTrigger{}, false
	}

	return trigger, true
}
