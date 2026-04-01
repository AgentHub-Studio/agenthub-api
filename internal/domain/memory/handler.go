package memory

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

// Handler exposes the HTTP interface for agent memory.
type Handler struct {
	svc *Service
}

// NewHandler creates a new memory Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts memory endpoints on the router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/agents/{agentId}/memory", h.list)
	r.Put("/api/agents/{agentId}/memory/{key}", h.upsert)
	r.Get("/api/agents/{agentId}/memory/{key}", h.getByKey)
	r.Delete("/api/agents/{agentId}/memory/{key}", h.deleteByKey)
	r.Delete("/api/agents/{agentId}/memory", h.clear)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	agentID, err := uuid.Parse(chi.URLParam(r, "agentId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agent id")
		return
	}
	var userID *string
	if v := r.URL.Query().Get("userId"); v != "" {
		userID = &v
	}
	items, err := h.svc.List(r.Context(), agentID, userID)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to list memory")
		return
	}
	respond.JSON(w, http.StatusOK, items)
}

func (h *Handler) upsert(w http.ResponseWriter, r *http.Request) {
	agentID, err := uuid.Parse(chi.URLParam(r, "agentId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agent id")
		return
	}
	key := chi.URLParam(r, "key")
	var req UpsertMemoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	m, err := h.svc.Upsert(r.Context(), agentID, key, req)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, m)
}

func (h *Handler) getByKey(w http.ResponseWriter, r *http.Request) {
	agentID, err := uuid.Parse(chi.URLParam(r, "agentId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agent id")
		return
	}
	key := chi.URLParam(r, "key")
	var userID *string
	if v := r.URL.Query().Get("userId"); v != "" {
		userID = &v
	}
	m, err := h.svc.GetByKey(r.Context(), agentID, userID, key)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "memory entry not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to get memory")
		return
	}
	respond.JSON(w, http.StatusOK, m)
}

func (h *Handler) deleteByKey(w http.ResponseWriter, r *http.Request) {
	agentID, err := uuid.Parse(chi.URLParam(r, "agentId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agent id")
		return
	}
	key := chi.URLParam(r, "key")
	var userID *string
	if v := r.URL.Query().Get("userId"); v != "" {
		userID = &v
	}
	if err := h.svc.DeleteByKey(r.Context(), agentID, userID, key); err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "memory entry not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to delete memory")
		return
	}
	respond.NoContent(w)
}

func (h *Handler) clear(w http.ResponseWriter, r *http.Request) {
	agentID, err := uuid.Parse(chi.URLParam(r, "agentId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agent id")
		return
	}
	if err := h.svc.ClearByAgent(r.Context(), agentID); err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to clear memory")
		return
	}
	respond.NoContent(w)
}
