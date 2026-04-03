package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// chatService defines the methods used by Handler.
type chatService interface {
	ListSessions(ctx context.Context, req pagination.PageRequest) (pagination.Page[ChatSessionResponse], error)
	CreateSession(ctx context.Context, req CreateSessionRequest) (ChatSessionResponse, error)
	GetSession(ctx context.Context, id uuid.UUID) (ChatSessionResponse, error)
	DeleteSession(ctx context.Context, id uuid.UUID) error
	ArchiveSession(ctx context.Context, id uuid.UUID) (ChatSessionResponse, error)
	ListMessages(ctx context.Context, sessionID uuid.UUID, req pagination.PageRequest) (pagination.Page[ChatMessageResponse], error)
	AddMessage(ctx context.Context, sessionID uuid.UUID, req CreateMessageRequest) (ChatMessageResponse, error)
	// RunSession starts an agentic run and returns a channel of events for SSE streaming.
	RunSession(ctx context.Context, sessionID uuid.UUID, userMessage, tenantID string) (<-chan RunEvent, error)
}

// Handler handles HTTP requests for chat sessions and messages.
type Handler struct {
	svc        chatService
	bgRegistry *BackgroundRunRegistry
}

// NewHandler creates a new Handler.
func NewHandler(svc chatService) *Handler {
	return &Handler{
		svc:        svc,
		bgRegistry: NewBackgroundRunRegistry(0),
	}
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
	r.Post("/api/chat/sessions/{id}/run", h.runSession)
	r.Get("/api/chat/sessions/{id}/run/{runId}/status", h.runStatus)
	r.Post("/api/chat/sessions/{id}/run/{runId}/cancel", h.cancelRun)
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

// runSessionRequest is the body for POST /api/chat/sessions/{id}/run.
type runSessionRequest struct {
	Message string `json:"message"`
}

// runSession handles POST /api/chat/sessions/{id}/run.
// It starts the agentic loop and streams RunEvents as SSE to the client.
// The run continues in the background even if the SSE client disconnects.
func (h *Handler) runSession(w http.ResponseWriter, r *http.Request) {
	sessionID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid session id")
		return
	}

	var req runSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Message == "" {
		respond.Error(w, http.StatusBadRequest, "message is required")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		respond.Error(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	tenantID := tenant.FromContext(r.Context())

	// Register a background run with a context decoupled from the HTTP request.
	// This ensures the Runner continues even if the SSE client disconnects.
	runID, runCtx := h.bgRegistry.Register(sessionID)

	ch, err := h.svc.RunSession(runCtx, sessionID, req.Message, tenantID)
	if err != nil {
		h.bgRegistry.Cancel(runID)
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "session not found")
			return
		}
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	h.bgRegistry.AttachEvents(runID, ch)

	// Set SSE headers.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("X-Run-ID", runID)
	w.WriteHeader(http.StatusOK)

	// Stream events to client. The goroutine is the sole consumer of the Runner's
	// channel. It forwards events to sseCh for the HTTP handler. When the client
	// disconnects, events are discarded (but the Runner continues in background).
	sseCh := make(chan RunEvent, 64)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer close(sseCh)
		for ev := range ch {
			select {
			case sseCh <- ev:
			default:
				// SSE writer can't keep up or disconnected — discard event.
				// The Runner persists everything, so no data is lost.
			}
		}
		h.bgRegistry.MarkCompleted(runID)
	}()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			// Client disconnected — run continues in background.
			return
		case <-done:
			// Run completed while we were connected.
			return
		case ev, ok := <-sseCh:
			if !ok {
				return
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, ev.Data)
			flusher.Flush()
		}
	}
}

// runStatusResponse is returned by the run status endpoint.
type runStatusResponse struct {
	RunID     string    `json:"runId"`
	SessionID uuid.UUID `json:"sessionId"`
	Status    RunStatus `json:"status"`
	StartedAt string    `json:"startedAt"`
}

// runStatus handles GET /api/chat/sessions/{id}/run/{runId}/status.
func (h *Handler) runStatus(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runId")
	if runID == "" {
		respond.Error(w, http.StatusBadRequest, "runId is required")
		return
	}

	run := h.bgRegistry.Get(runID)
	if run == nil {
		respond.Error(w, http.StatusNotFound, "run not found")
		return
	}

	respond.JSON(w, http.StatusOK, runStatusResponse{
		RunID:     run.RunID,
		SessionID: run.SessionID,
		Status:    run.Status,
		StartedAt: run.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// cancelRun handles POST /api/chat/sessions/{id}/run/{runId}/cancel.
func (h *Handler) cancelRun(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runId")
	if runID == "" {
		respond.Error(w, http.StatusBadRequest, "runId is required")
		return
	}

	run := h.bgRegistry.Get(runID)
	if run == nil {
		respond.Error(w, http.StatusNotFound, "run not found")
		return
	}

	h.bgRegistry.Cancel(runID)
	respond.JSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}
