package document

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

// documentService defines the methods used by Handler.
type documentService interface {
	ListByKnowledgeBase(ctx context.Context, kbID uuid.UUID, req pagination.PageRequest) (pagination.Page[DocumentResponse], error)
	Upload(ctx context.Context, req UploadRequest) (DocumentResponse, error)
	GetByID(ctx context.Context, id uuid.UUID) (DocumentResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Reprocess(ctx context.Context, id uuid.UUID) (DocumentResponse, error)
}

// kbExister verifies parent knowledge-base existence (bug 209 batch).
type kbExister interface {
	GetByID(ctx context.Context, id uuid.UUID) error
}

// Handler handles HTTP requests for documents.
type Handler struct {
	svc documentService
	kb  kbExister
}

// NewHandler creates a new Handler.
func NewHandler(svc documentService) *Handler {
	return &Handler{svc: svc}
}

// WithKBExister wires KB existence checker for parent-resource validation.
func (h *Handler) WithKBExister(kb kbExister) *Handler {
	h.kb = kb
	return h
}

// RegisterRoutes mounts document routes onto the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/knowledge-bases/{kbId}/documents", h.list)
	r.Post("/api/knowledge-bases/{kbId}/documents", h.upload)
	r.Get("/api/knowledge-bases/{kbId}/documents/{id}", h.getByID)
	r.Delete("/api/knowledge-bases/{kbId}/documents/{id}", h.delete)
	r.Post("/api/knowledge-bases/{kbId}/documents/{id}/reprocess", h.reprocess)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	kbID, err := uuid.Parse(chi.URLParam(r, "kbId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid knowledge base id")
		return
	}

	if h.kb != nil {
		if err := h.kb.GetByID(r.Context(), kbID); err != nil {
			respond.Error(w, http.StatusNotFound, "knowledge base not found")
			return
		}
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

	// Accept multipart/form-data with a "file" field.
	const maxUploadSize = 100 << 20 // 100 MB
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		respond.Error(w, http.StatusBadRequest, "multipart form required")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "file field is required")
		return
	}
	defer file.Close() //nolint:errcheck

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	req := UploadRequest{
		KnowledgeBaseID: kbID,
		FileName:        header.Filename,
		ContentType:     contentType,
		FileSize:        header.Size,
		Content:         file,
	}

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

func (h *Handler) reprocess(w http.ResponseWriter, r *http.Request) {
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

	resp, err := h.svc.Reprocess(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "document not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to reprocess document")
		return
	}

	respond.JSON(w, http.StatusOK, resp)
}
