package document

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

// Handler handles HTTP requests for documents.
type Handler struct {
	svc *Service
}

// NewHandler creates a new Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts document routes onto the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/knowledge-bases/{kbId}/documents", h.list)
	r.Post("/api/knowledge-bases/{kbId}/documents", h.upload)
	r.Get("/api/knowledge-bases/{kbId}/documents/{id}", h.getByID)
	r.Delete("/api/knowledge-bases/{kbId}/documents/{id}", h.delete)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	kbID, err := uuid.Parse(chi.URLParam(r, "kbId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid knowledge base id")
		return
	}

	req := pagination.ParsePageRequest(r)
	page, err := h.svc.ListByKnowledgeBase(r.Context(), kbID, req)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to list documents")
		return
	}

	respond.JSON(w, http.StatusOK, page)
}

func (h *Handler) upload(w http.ResponseWriter, r *http.Request) {
	kbID, err := uuid.Parse(chi.URLParam(r, "kbId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid knowledge base id")
		return
	}

	var req UploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.KnowledgeBaseID = kbID

	resp, err := h.svc.Upload(r.Context(), req)
	if err != nil {
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	respond.JSON(w, http.StatusCreated, resp)
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	_, err := uuid.Parse(chi.URLParam(r, "kbId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid knowledge base id")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	resp, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "document not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to get document")
		return
	}

	respond.JSON(w, http.StatusOK, resp)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	_, err := uuid.Parse(chi.URLParam(r, "kbId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid knowledge base id")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "document not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to delete document")
		return
	}

	respond.NoContent(w)
}
