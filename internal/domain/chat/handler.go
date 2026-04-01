package chat

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

// Handler handles HTTP requests for chat sessions and messages.
type Handler struct {
	svc *Service
}

// NewHandler creates a new Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts chat routes onto the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/chat/sessions", h.listSessions)
	r.Post("/api/chat/sessions", h.createSession)
	r.Get("/api/chat/sessions/{id}", h.getSession)
	r.Delete("/api/chat/sessions/{id}", h.deleteSession)
	r.Post("/api/chat/sessions/{id}/archive", h.archiveSession)
	r.Get("/api/chat/sessions/{id}/messages", h.listMessages)
	r.Post("/api/chat/sessions/{id}/messages", h.addMessage)
}

func (h *Handler) listSessions(w http.ResponseWriter, r *http.Request) {
	req := pagination.ParsePageRequest(r)
	page, err := h.svc.ListSessions(r.Context(), req)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to list chat sessions")
		return
	}
	respond.JSON(w, http.StatusOK, page)
}

func (h *Handler) createSession(w http.ResponseWriter, r *http.Request) {
	var req CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.svc.CreateSession(r.Context(), req)
	if err != nil {
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	respond.JSON(w, http.StatusCreated, resp)
}

func (h *Handler) getSession(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	resp, err := h.svc.GetSession(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "chat session not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to get chat session")
		return
	}

	respond.JSON(w, http.StatusOK, resp)
}

func (h *Handler) deleteSession(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := h.svc.DeleteSession(r.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "chat session not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to delete chat session")
		return
	}

	respond.NoContent(w)
}

func (h *Handler) archiveSession(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	resp, err := h.svc.ArchiveSession(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "chat session not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to archive chat session")
		return
	}

	respond.JSON(w, http.StatusOK, resp)
}

func (h *Handler) listMessages(w http.ResponseWriter, r *http.Request) {
	sessionID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid session id")
		return
	}

	req := pagination.ParsePageRequest(r)
	page, err := h.svc.ListMessages(r.Context(), sessionID, req)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to list messages")
		return
	}

	respond.JSON(w, http.StatusOK, page)
}

func (h *Handler) addMessage(w http.ResponseWriter, r *http.Request) {
	sessionID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid session id")
		return
	}

	var req CreateMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.svc.AddMessage(r.Context(), sessionID, req)
	if err != nil {
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	respond.JSON(w, http.StatusCreated, resp)
}
