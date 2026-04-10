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
	GetActiveRun(ctx context.Context, sessionID uuid.UUID) (ChatRunResponse, bool, error)
	// RunSession starts an agentic run and returns a channel of events for SSE streaming.
	RunSession(ctx context.Context, sessionID uuid.UUID, userMessage, tenantID string) (<-chan RunEvent, error)
	// RespondElicitation routes a user response to an active elicitation request.
	RespondElicitation(sessionID, requestID string, result ElicitationResult) bool
}

// Handler handles HTTP requests for chat sessions and messages.
type Handler struct {
	svc            chatService
	executor       *AsyncExecutor
	bgRegistry     *BackgroundRunRegistry
	bufferRegistry *RunEventBufferRegistry
}

// NewHandler creates a new Handler.
func NewHandler(svc chatService, executor *AsyncExecutor) *Handler {
	return &Handler{
		svc:            svc,
		executor:       executor,
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
	r.Post("/api/chat/sessions/{id}/elicitation/{requestId}/respond", h.respondElicitation)
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

	// Check for active runs
	if run, found, _ := h.svc.GetActiveRun(r.Context(), id); found {
		// Include active run in session metadata or just as a separate field if we update DTO.
		// For now, we can add it to a map if we want to extend the response without breaking DTO.
		data, _ := json.Marshal(resp)
		var m map[string]interface{}
		json.Unmarshal(data, &m)
		m["activeRun"] = run
		respond.JSON(w, http.StatusOK, m)
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

// elicitationRespondRequest is the body for POST /api/chat/sessions/{id}/elicitation/{requestId}/respond.
type elicitationRespondRequest struct {
	Action  string                 `json:"action"`  // "accept" | "decline" | "cancel"
	Content map[string]interface{} `json:"content"` // form field values (for accept)
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

	// If AsyncExecutor is available, use it to start the run in background.
	if h.executor != nil {
		runID, err := h.executor.EnqueueRun(r.Context(), sessionID, tenantID, req.Message)
		if err != nil {
			respond.Error(w, http.StatusInternalServerError, fmt.Sprintf("failed to enqueue run: %v", err))
			return
		}
		respond.JSON(w, http.StatusAccepted, map[string]interface{}{
			"runId":     runID,
			"status":    "accepted",
			"sessionId": sessionID,
		})
		return
	}

	// Legacy synchronous-background flow (for environments without RabbitMQ)
	// Register a background run with a context decoupled from the HTTP request.
	// This ensures the Runner continues even if the SSE client disconnects.
	runID, runCtx := h.bgRegistry.Register(sessionID)

	// Inject tenant and raw token into the background context so that
	// repository calls (which use tenant.FromContext) work correctly.
	rawToken := tenant.TokenFromContext(r.Context())
	runCtx = tenant.NewContextWithToken(runCtx, tenantID, rawToken)

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
		case ev, ok := <-sseCh:
			if !ok {
				// Channel closed -- run complete.
				buf.MarkDone()
				return
			}
			seq := buf.Append(ev)
			fmt.Fprintf(w, "id: %s:%d\nevent: %s\ndata: %s\n\n", runID, seq, ev.Type, ev.Data)
			flusher.Flush()
		case <-done:
			// Run goroutine is done; drain any remaining buffered events.
			for {
				select {
				case ev, ok := <-sseCh:
					if !ok {
						buf.MarkDone()
						return
					}
					seq := buf.Append(ev)
					fmt.Fprintf(w, "id: %s:%d\nevent: %s\ndata: %s\n\n", runID, seq, ev.Type, ev.Data)
					flusher.Flush()
				default:
					// No more events pending; done signal may have raced with close(sseCh).
					// Wait for sseCh to be closed to ensure MarkDone is called.
					for ev := range sseCh {
						seq := buf.Append(ev)
						fmt.Fprintf(w, "id: %s:%d\nevent: %s\ndata: %s\n\n", runID, seq, ev.Type, ev.Data)
						flusher.Flush()
					}
					buf.MarkDone()
					return
				}
			}
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
	events = filterReplayableEvents(events)

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

// respondElicitation handles POST /api/chat/sessions/{id}/elicitation/{requestId}/respond.
// It routes the user's form response to the active agentic run so the loop can continue.
func (h *Handler) respondElicitation(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	if _, err := uuid.Parse(sessionID); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid session id")
		return
	}

	requestID := chi.URLParam(r, "requestId")
	if requestID == "" {
		respond.Error(w, http.StatusBadRequest, "requestId is required")
		return
	}

	var req elicitationRespondRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Action == "" {
		req.Action = "accept"
	}

	result := ElicitationResult{
		Action:  req.Action,
		Content: req.Content,
	}

	if ok := h.svc.RespondElicitation(sessionID, requestID, result); !ok {
		respond.Error(w, http.StatusNotFound, "elicitation request not found or already resolved")
		return
	}

	respond.NoContent(w)
}

// filterReplayableEvents drops stale input_request events that have already been
// resolved later in the same buffered event window. This prevents a resumed SSE
// connection from re-opening an already answered form.
func filterReplayableEvents(events []BufferedEvent) []BufferedEvent {
	if len(events) == 0 {
		return events
	}

	resolvedAt := make(map[string]uint64)
	for _, ev := range events {
		switch ev.Event.Type {
		case "tool_result":
			var data struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(ev.Event.Data, &data); err == nil && data.ID != "" {
				if _, exists := resolvedAt[data.ID]; !exists {
					resolvedAt[data.ID] = ev.ID
				}
			}
		case "tool_progress":
			var data struct {
				ID    string `json:"id"`
				State string `json:"state"`
			}
			if err := json.Unmarshal(ev.Event.Data, &data); err == nil && data.ID != "" && data.State == "completed" {
				if _, exists := resolvedAt[data.ID]; !exists {
					resolvedAt[data.ID] = ev.ID
				}
			}
		}
	}

	if len(resolvedAt) == 0 {
		return events
	}

	filtered := make([]BufferedEvent, 0, len(events))
	for _, ev := range events {
		if ev.Event.Type != "input_request" {
			filtered = append(filtered, ev)
			continue
		}

		var data struct {
			RequestID string `json:"requestId"`
		}
		if err := json.Unmarshal(ev.Event.Data, &data); err != nil || data.RequestID == "" {
			filtered = append(filtered, ev)
			continue
		}

		if resolvedSeq, resolved := resolvedAt[data.RequestID]; resolved && ev.ID < resolvedSeq {
			continue
		}

		filtered = append(filtered, ev)
	}

	return filtered
}
