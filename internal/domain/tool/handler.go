package tool

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

// Handler exposes tool HTTP endpoints.
type Handler struct {
	svc *Service
}

// NewHandler creates a new Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts tool routes on the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/tools", h.list)
	r.Post("/api/tools", h.create)
	r.Get("/api/tools/{id}", h.getByID)
	r.Put("/api/tools/{id}", h.update)
	r.Delete("/api/tools/{id}", h.delete)

	r.Post("/api/skills/{skillId}/tools", h.bindToSkill)
	r.Delete("/api/skills/{skillId}/tools/{toolId}", h.unbindFromSkill)
	r.Get("/api/skills/{skillId}/tools", h.listBySkill)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	req := pagination.ParsePageRequest(r)
	toolType := r.URL.Query().Get("type")
	page, err := h.svc.List(r.Context(), req, toolType)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, page)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.Type == "" {
		respond.Error(w, http.StatusUnprocessableEntity, "name and type are required")
		return
	}
	resp, err := h.svc.Create(r.Context(), req)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
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
			respond.Error(w, http.StatusNotFound, "tool not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, err.Error())
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
			respond.Error(w, http.StatusNotFound, "tool not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, err.Error())
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
			respond.Error(w, http.StatusNotFound, "tool not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.NoContent(w)
}

func (h *Handler) bindToSkill(w http.ResponseWriter, r *http.Request) {
	skillID, err := uuid.Parse(chi.URLParam(r, "skillId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid skillId")
		return
	}
	var req BindRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.svc.BindToSkill(r.Context(), skillID, req)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.JSON(w, http.StatusCreated, resp)
}

func (h *Handler) unbindFromSkill(w http.ResponseWriter, r *http.Request) {
	skillID, err := uuid.Parse(chi.URLParam(r, "skillId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid skillId")
		return
	}
	toolID, err := uuid.Parse(chi.URLParam(r, "toolId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid toolId")
		return
	}
	if err := h.svc.UnbindFromSkill(r.Context(), skillID, toolID); err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "binding not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.NoContent(w)
}

func (h *Handler) listBySkill(w http.ResponseWriter, r *http.Request) {
	skillID, err := uuid.Parse(chi.URLParam(r, "skillId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid skillId")
		return
	}
	resp, err := h.svc.ListBySkill(r.Context(), skillID)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, resp)
}
