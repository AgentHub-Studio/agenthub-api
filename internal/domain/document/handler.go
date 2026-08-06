package document

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/httputil"
	"github.com/AgentHub-Studio/agenthub-api/internal/metadata"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
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

// RegisterRoutes mounts administrator-only document routes onto the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireRole("admin"))
		r.Get("/api/knowledge-bases/{kbId}/documents", h.list)
		r.Post("/api/knowledge-bases/{kbId}/documents", h.upload)
		r.Get("/api/knowledge-bases/{kbId}/documents/{id}", h.getByID)
		r.Delete("/api/knowledge-bases/{kbId}/documents/{id}", h.delete)
		r.Post("/api/knowledge-bases/{kbId}/documents/{id}/reprocess", h.reprocess)
	})
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

	// Bug 212: valida KB antes de upload (evita 422 com leak de FK
	// constraint do PostgreSQL — "document_knowledge_base_id_fkey",
	// SQLSTATE 23503).
	if h.kb != nil {
		if err := h.kb.GetByID(r.Context(), kbID); err != nil {
			respond.Error(w, http.StatusNotFound, "knowledge base not found")
			return
		}
	}

	const (
		maxUploadSize      = 100 << 20 // 100 MB
		maxUploadBodyBytes = maxUploadSize + (1 << 20)
		maxUploadFieldSize = 16 << 10
	)
	file, fields, err := httputil.ReadLimitedMultipartFile(w, r, httputil.LimitedMultipartOptions{
		FileFields:    []string{"file"},
		MaxFileBytes:  maxUploadSize,
		MaxBodyBytes:  maxUploadBodyBytes,
		MaxFieldBytes: maxUploadFieldSize,
	})
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "multipart form required")
		return
	}

	contentType := file.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	metadata, err := parseUploadMetadata(fields["metadata"])
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid metadata")
		return
	}

	req := UploadRequest{
		KnowledgeBaseID: kbID,
		FileName:        file.Filename,
		ContentType:     contentType,
		FileSize:        file.Size,
		Content:         file.Reader(),
		Metadata:        metadata,
	}

	resp, err := h.svc.Upload(r.Context(), req)
	if err != nil {
		// Bug 212: nunca expor mensagens de erro raw do PostgreSQL.
		// Falha de validação semântica genérica → 422 sem .Error().
		slog.Warn("document: upload failed", "err", err, "kbId", kbID)
		respond.Error(w, http.StatusUnprocessableEntity, "upload failed")
		return
	}

	respond.JSON(w, http.StatusCreated, resp)
}

func parseUploadMetadata(raw string) (json.RawMessage, error) {
	return metadata.ParseDocument([]byte(raw))
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
