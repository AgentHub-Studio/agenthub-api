package knowledgebase

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

// Handler handles HTTP requests for knowledge bases.
type Handler struct {
	svc *Service
}

// NewHandler creates a new Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts knowledge base routes onto the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/knowledge-bases", h.list)
	r.Post("/api/knowledge-bases", h.create)
	r.Get("/api/knowledge-bases/{id}", h.getByID)
	r.Put("/api/knowledge-bases/{id}", h.update)
	r.Delete("/api/knowledge-bases/{id}", h.delete)
	r.Post("/api/knowledge-bases/{id}/activate", h.activate)
	r.Post("/api/knowledge-bases/{id}/pause", h.pause)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	req := pagination.ParsePageRequest(r)
	page, err := h.svc.List(r.Context(), req)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to list knowledge bases")
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

	resp, err := h.svc.Create(r.Context(), req)
	if err != nil {
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
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
			respond.Error(w, http.StatusNotFound, "knowledge base not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to get knowledge base")
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
			respond.Error(w, http.StatusNotFound, "knowledge base not found")
			return
		}
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
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
			respond.Error(w, http.StatusNotFound, "knowledge base not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to delete knowledge base")
		return
	}

	respond.NoContent(w)
}

func (h *Handler) activate(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	resp, err := h.svc.Activate(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "knowledge base not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to activate knowledge base")
		return
	}

	respond.JSON(w, http.StatusOK, resp)
}

func (h *Handler) pause(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	resp, err := h.svc.Pause(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "knowledge base not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to pause knowledge base")
		return
	}

	respond.JSON(w, http.StatusOK, resp)
}
