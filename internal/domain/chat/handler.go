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
	RenameSession(ctx context.Context, id uuid.UUID, title string) (ChatSessionResponse, error)
	ListMessages(ctx context.Context, sessionID uuid.UUID, req pagination.PageRequest) (pagination.Page[ChatMessageResponse], error)
	AddMessage(ctx context.Context, sessionID uuid.UUID, req CreateMessageRequest) (ChatMessageResponse, error)
	// RunSession starts an agentic run and returns a channel of events for SSE streaming.
	RunSession(ctx context.Context, sessionID uuid.UUID, userMessage, tenantID string) (<-chan RunEvent, error)
}

// Handler handles HTTP requests for chat sessions and messages.
type Handler struct {
	svc            chatService
	bgRegistry     *BackgroundRunRegistry
	bufferRegistry *RunEventBufferRegistry
}

// NewHandler creates a new Handler.
func NewHandler(svc chatService) *Handler {
	return &Handler{
		svc:            svc,
		bgRegistry:     NewBackgroundRunRegistry(0),
		bufferRegistry: NewRunEventBufferRegistry(),
	}
}

// RegisterRoutes mounts chat routes onto the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/chat/sessions", h.listSessions)
	r.Post("/api/chat/sessions", h.createSession)
	r.Get("/api/chat/sessions/{id}", h.getSession)
	r.Patch("/api/chat/sessions/{id}", h.renameSession)
	r.Delete("/api/chat/sessions/{id}", h.deleteSession)
	r.Post("/api/chat/sessions/{id}/archive", h.archiveSession)
	r.Get("/api/chat/sessions/{id}/messages", h.listMessages)
	r.Post("/api/chat/sessions/{id}/messages", h.addMessage)
	r.Post("/api/chat/sessions/{id}/run", h.runSession)
	r.Get("/api/chat/sessions/{id}/run/{runId}/status", h.runStatus)
	r.Post("/api/chat/sessions/{id}/run/{runId}/cancel", h.cancelRun)
	r.Get("/api/chat/sessions/{id}/run/{runId}/resume", h.resumeSession)
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

func (h *Handler) renameSession(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Title == "" {
		respond.Error(w, http.StatusBadRequest, "title is required")
		return
	}

	resp, err := h.svc.RenameSession(r.Context(), id, req.Title)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "chat session not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to rename chat session")
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
// Each SSE event includes an id field for reconnection via Last-Event-ID.
// The response header X-Run-ID contains the run identifier for resume/status/cancel requests.
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
	buf := h.bufferRegistry.GetOrCreate(runID, DefaultEventBufferSize)

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
				// SSE writer can't keep up or disconnected -- discard event.
				// The Runner persists everything, so no data is lost.
			}
		}
		h.bgRegistry.MarkCompleted(runID)
	}()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			// Client disconnected -- run continues in background.
			return
		case <-done:
			// Run completed while we were connected.
			return
		case ev, ok := <-sseCh:
			if !ok {
				// Channel closed -- run complete.
				buf.MarkDone()
				return
			}
			seq := buf.Append(ev)
			fmt.Fprintf(w, "id: %s:%d\nevent: %s\ndata: %s\n\n", runID, seq, ev.Type, ev.Data)
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

// ReconnectOverflowData is the payload for the reconnect_overflow event.
type ReconnectOverflowData struct {
	Message     string `json:"message"`
	LastEventID string `json:"lastEventId"`
}

// resumeSession handles GET /api/chat/sessions/{id}/run/{runId}/resume.
// It replays buffered events since Last-Event-ID and continues streaming
// if the run is still in progress. Used for SSE reconnection.
func (h *Handler) resumeSession(w http.ResponseWriter, r *http.Request) {
	_, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid session id")
		return
	}

	runID := chi.URLParam(r, "runId")
	if runID == "" {
		respond.Error(w, http.StatusBadRequest, "run id is required")
		return
	}

	buf := h.bufferRegistry.Get(runID)
	if buf == nil {
		respond.Error(w, http.StatusNotFound, "run not found or expired")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		respond.Error(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// Parse Last-Event-ID from header or query param.
	var afterSeq uint64
	lastEventID := r.Header.Get("Last-Event-ID")
	if lastEventID == "" {
		lastEventID = r.URL.Query().Get("lastEventId")
	}
	if lastEventID != "" {
		_, seq, ok := ParseSSEID(lastEventID)
		if ok {
			afterSeq = seq
		}
	}

	// Set SSE headers.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("X-Run-ID", runID)
	w.WriteHeader(http.StatusOK)

	// Replay buffered events.
	events, ok := buf.EventsSince(afterSeq)
	if !ok {
		// Overflow -- client missed events that were evicted from the buffer.
		overflowData, _ := json.Marshal(ReconnectOverflowData{
			Message:     "events lost: buffer overflow since last event id",
			LastEventID: lastEventID,
		})
		fmt.Fprintf(w, "event: reconnect_overflow\ndata: %s\n\n", overflowData)
		flusher.Flush()
	}

	// Send replayed events.
	for _, ev := range events {
		fmt.Fprintf(w, "id: %s:%d\nevent: %s\ndata: %s\n\n", runID, ev.ID, ev.Event.Type, ev.Event.Data)
		flusher.Flush()
	}

	// If the run is done, no need to wait for more events.
	if buf.IsDone() {
		return
	}

	// Continue streaming new events by polling the buffer.
	// We poll at a short interval to avoid busy-waiting.
	ctx := r.Context()
	lastSent := buf.NewestSeq()
	ticker := newTicker(50 * millisecondsUnit)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C():
			newEvents, _ := buf.EventsSince(lastSent)
			for _, ev := range newEvents {
				fmt.Fprintf(w, "id: %s:%d\nevent: %s\ndata: %s\n\n", runID, ev.ID, ev.Event.Type, ev.Event.Data)
				flusher.Flush()
				lastSent = ev.ID
			}
			if buf.IsDone() && len(newEvents) == 0 {
				return
			}
		}
	}
}
