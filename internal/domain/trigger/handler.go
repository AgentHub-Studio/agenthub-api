package trigger

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

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

// Handler exposes HTTP endpoints for agent triggers.
type Handler struct {
	svc triggerService
}

// NewHandler creates a new trigger Handler.
func NewHandler(svc triggerService) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts trigger endpoints on the router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/api/agents/{agentId}/triggers", h.create)
	r.Get("/api/agents/{agentId}/triggers", h.list)
	r.Get("/api/agents/{agentId}/triggers/{triggerId}", h.getByID)
	r.Put("/api/agents/{agentId}/triggers/{triggerId}", h.update)
	r.Patch("/api/agents/{agentId}/triggers/{triggerId}", h.update)
	r.Delete("/api/agents/{agentId}/triggers/{triggerId}", h.delete)
	r.Get("/api/agents/{agentId}/triggers/{triggerId}/runs", h.listRuns)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	agentID, err := uuid.Parse(chi.URLParam(r, "agentId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agent id")
		return
	}
	var req CreateTriggerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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
	page := pagination.ParsePageRequest(r)
	result, err := h.svc.List(r.Context(), agentID, page)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to list triggers")
		return
	}
	respond.JSON(w, http.StatusOK, result)
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "triggerId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid trigger id")
		return
	}
	t, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "trigger not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to get trigger")
		return
	}
	respond.JSON(w, http.StatusOK, t)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "triggerId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid trigger id")
		return
	}
	var req UpdateTriggerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	t, err := h.svc.Update(r.Context(), id, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "trigger not found")
			return
		}
		respond.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, t)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "triggerId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid trigger id")
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
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
	triggerID, err := uuid.Parse(chi.URLParam(r, "triggerId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid trigger id")
		return
	}
	page := pagination.ParsePageRequest(r)
	result, err := h.svc.ListRuns(r.Context(), triggerID, page)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to list runs")
		return
	}
	respond.JSON(w, http.StatusOK, result)
}
