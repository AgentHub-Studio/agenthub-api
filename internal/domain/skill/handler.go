package skill

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

// Handler exposes skill HTTP endpoints.
type Handler struct {
	svc *Service
}

// NewHandler creates a new Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts skill routes on the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/skills", h.list)
	r.Post("/api/skills", h.create)
	r.Get("/api/skills/{id}", h.getByID)
	r.Put("/api/skills/{id}", h.update)
	r.Delete("/api/skills/{id}", h.delete)
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

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		respond.Error(w, http.StatusUnprocessableEntity, "name is required")
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
			respond.Error(w, http.StatusNotFound, "skill not found")
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
			respond.Error(w, http.StatusNotFound, "skill not found")
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
			respond.Error(w, http.StatusNotFound, "skill not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.NoContent(w)
}
