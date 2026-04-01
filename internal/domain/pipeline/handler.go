package pipeline

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

// Handler exposes pipeline HTTP endpoints.
type Handler struct {
	svc *Service
}

// NewHandler creates a new Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts pipeline routes on the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/pipelines", h.list)
	r.Post("/api/pipelines", h.create)
	r.Get("/api/pipelines/{id}", h.getByID)
	r.Put("/api/pipelines/{id}", h.update)
	r.Delete("/api/pipelines/{id}", h.delete)
	r.Put("/api/pipelines/{id}/nodes", h.replaceNodes)
	r.Put("/api/pipelines/{id}/edges", h.replaceEdges)
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
			respond.Error(w, http.StatusNotFound, "pipeline not found")
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
			respond.Error(w, http.StatusNotFound, "pipeline not found")
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
			respond.Error(w, http.StatusNotFound, "pipeline not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.NoContent(w)
}

func (h *Handler) replaceNodes(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	var nodes []NodeRequest
	if err := json.NewDecoder(r.Body).Decode(&nodes); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.svc.ReplaceNodes(r.Context(), id, nodes)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			respond.Error(w, http.StatusNotFound, "pipeline not found")
		case errors.Is(err, ErrDuplicateNodeName):
			respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		default:
			respond.Error(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	respond.JSON(w, http.StatusOK, resp)
}

func (h *Handler) replaceEdges(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	var edges []EdgeRequest
	if err := json.NewDecoder(r.Body).Decode(&edges); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.svc.ReplaceEdges(r.Context(), id, edges)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			respond.Error(w, http.StatusNotFound, "pipeline not found")
		case errors.Is(err, ErrCyclicGraph):
			respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		default:
			respond.Error(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	respond.JSON(w, http.StatusOK, resp)
}
