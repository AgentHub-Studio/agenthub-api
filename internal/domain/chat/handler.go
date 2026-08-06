package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"reflect"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/task"
	"github.com/AgentHub-Studio/agenthub-api/internal/httputil"
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
	CloneSession(ctx context.Context, id uuid.UUID, req CloneSessionRequest) (ChatSessionResponse, error)
	ListMessages(ctx context.Context, sessionID uuid.UUID, req pagination.PageRequest) (pagination.Page[ChatMessageResponse], error)
	AddMessage(ctx context.Context, sessionID uuid.UUID, req CreateMessageRequest) (ChatMessageResponse, error)
	GetActiveRun(ctx context.Context, sessionID uuid.UUID) (ChatRunResponse, bool, error)
	// RunSession starts an agentic run and returns a channel of events for SSE streaming.
	RunSession(ctx context.Context, sessionID uuid.UUID, userMessage, tenantID string, opts ...RunSessionOptions) (<-chan RunEvent, error)
	// RespondElicitation routes a user response to an active elicitation request.
	RespondElicitation(ctx context.Context, sessionID, requestID string, result ElicitationResult) bool
	// ApplyClientState merges a CopilotKit client-state patch (frontend actions,
	// readables, action results) into the per-session runner state.
	ApplyClientState(sessionID uuid.UUID, patch ClientStatePatch)
}

// RunLookup is a narrow interface for looking up a persisted run by ID.
// P-C325-1: decoupled from AsyncExecutor to allow unit-test injection.
type RunLookup interface {
	GetRunByID(ctx context.Context, id uuid.UUID) (ChatRun, error)
}

// PermissionAuditReader reads permission audit entries for a session.
type PermissionAuditReader interface {
	ListBySession(ctx context.Context, sessionID uuid.UUID, limit int) ([]PermissionAuditEntryResponse, error)
}

// EffectivePromptInspector renders the effective prompt for a session without
// starting an LLM run.
type EffectivePromptInspector interface {
	EffectivePrompt(ctx context.Context, sessionID uuid.UUID, identity PromptIdentity) (EffectivePromptResponse, error)
}

// EffectiveToolsInspector renders the tool schema for a session and request
// identity without starting an LLM run.
type EffectiveToolsInspector interface {
	EffectiveTools(ctx context.Context, sessionID uuid.UUID, identity PromptIdentity) (EffectiveToolsResponse, error)
}

// PromptIdentityExtractor extracts the authenticated request identity used by
// dynamic prompt placeholders.
type PromptIdentityExtractor func(r *http.Request) PromptIdentity

// PermissionAuditEntryResponse is the HTTP response shape for one audit entry.
type PermissionAuditEntryResponse struct {
	SessionID    string  `json:"sessionId"`
	RunID        *string `json:"runId,omitempty"`
	ToolName     string  `json:"toolName"`
	Decision     string  `json:"decision"`
	MatchedRule  string  `json:"matchedRule,omitempty"`
	InputSnippet string  `json:"inputSnippet,omitempty"`
	CreatedAt    string  `json:"createdAt"`
}

// Handler handles HTTP requests for chat sessions and messages.
type Handler struct {
	svc             chatService
	executor        *AsyncExecutor
	runLookup       RunLookup // nil when executor is nil (tests without DB)
	bgRegistry      *BackgroundRunRegistry
	bufferRegistry  *RunEventBufferRegistry
	taskRepo        task.Repository       // nil means task endpoints return 501
	permAuditReader PermissionAuditReader // nil means endpoint returns 501
	voiceSvc        VoiceService          // nil means voice endpoint returns 503
	promptInspector EffectivePromptInspector
	toolInspector   EffectiveToolsInspector
	identity        PromptIdentityExtractor
}

// NewHandler creates a new Handler.
func NewHandler(svc chatService, executor *AsyncExecutor) *Handler {
	h := &Handler{
		svc:            svc,
		executor:       executor,
		bgRegistry:     NewBackgroundRunRegistry(0),
		bufferRegistry: NewRunEventBufferRegistry(),
	}
	if executor != nil {
		executor.WithEventBufferRegistry(h.bufferRegistry)
	}
	if executor != nil {
		h.runLookup = executor
	}
	return h
}

// WithTaskRepository injects a task.Repository for the task listing endpoints.
func (h *Handler) WithTaskRepository(repo task.Repository) *Handler {
	h.taskRepo = repo
	return h
}

// WithPermissionAuditReader injects a reader for the permission audit log endpoint.
func (h *Handler) WithPermissionAuditReader(reader PermissionAuditReader) *Handler {
	h.permAuditReader = reader
	return h
}

// WithVoiceService injects speech-to-text / text-to-speech support.
func (h *Handler) WithVoiceService(svc VoiceService) *Handler {
	h.voiceSvc = svc
	return h
}

// WithEffectivePromptInspector injects the inspector used by
// GET /api/chat/sessions/{id}/effective-prompt.
func (h *Handler) WithEffectivePromptInspector(inspector EffectivePromptInspector, identity PromptIdentityExtractor) *Handler {
	h.promptInspector = inspector
	h.identity = identity
	return h
}

// WithEffectiveToolsInspector injects the inspector used by
// GET /api/chat/sessions/{id}/effective-tools.
func (h *Handler) WithEffectiveToolsInspector(inspector EffectiveToolsInspector, identity PromptIdentityExtractor) *Handler {
	h.toolInspector = inspector
	h.identity = identity
	return h
}

// WithRunLookup overrides the run lookup used by GET /api/chat/runs/{id}.
// Intended for use in unit tests where a real AsyncExecutor is not available.
func (h *Handler) WithRunLookup(rl RunLookup) *Handler {
	h.runLookup = rl
	return h
}

// RegisterRoutes mounts chat routes onto the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/chat/sessions", h.listSessions)
	r.Post("/api/chat/sessions", h.createSession)
	r.Get("/api/chat/sessions/{id}", h.getSession)
	r.Get("/api/chat/sessions/{id}/effective-prompt", h.effectivePrompt)
	r.Get("/api/chat/sessions/{id}/effective-tools", h.effectiveTools)
	r.Patch("/api/chat/sessions/{id}", h.renameSession)
	r.Delete("/api/chat/sessions/{id}", h.deleteSession)
	r.Post("/api/chat/sessions/{id}/archive", h.archiveSession)
	r.Post("/api/chat/sessions/{id}/clone", h.cloneSession)
	r.Get("/api/chat/sessions/{id}/messages", h.listMessages)
	r.Post("/api/chat/sessions/{id}/messages", h.addMessage)
	r.Post("/api/chat/sessions/{id}/run", h.runSession)
	r.Post("/api/chat/sessions/{id}/resume", h.resumeElicitation)
	r.Post("/api/chat/sessions/{id}/voice/input", h.voiceInput)
	r.Post("/api/chat/sessions/{id}/audio", h.voiceInput)
	r.Get("/api/chat/sessions/{id}/run/{runId}/status", h.runStatus)
	r.Post("/api/chat/sessions/{id}/run/{runId}/cancel", h.cancelRun)
	r.Get("/api/chat/sessions/{id}/run/{runId}/resume", h.resumeSession)
	r.Post("/api/chat/sessions/{id}/elicitation/{requestId}/respond", h.respondElicitation)
	r.Post("/api/chat/sessions/{id}/client-state", h.clientState)
	r.Get("/api/chat/runs/{id}", h.getRun)
	r.Get("/api/chat/sessions/{id}/tasks", h.listTasks)
	r.Get("/api/chat/sessions/{id}/tasks/{taskId}/notifications", h.listTaskNotifications)
	r.Get("/api/chat/sessions/{id}/permission-audit", h.listPermissionAudit)
}

func (h *Handler) listSessions(w http.ResponseWriter, r *http.Request) {
	if requestedTenant := strings.TrimSpace(r.URL.Query().Get("tenant")); requestedTenant != "" && requestedTenant != tenant.FromContext(r.Context()) {
		respond.Error(w, http.StatusForbidden, "cross-tenant chat listing is not allowed")
		return
	}

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
	if err := decodeJSONRequest(r, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.svc.CreateSession(r.Context(), req)
	if err != nil {
		if errors.Is(err, ErrAgentNotFound) {
			respond.Error(w, http.StatusNotFound, "agent not found")
			return
		}
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	respond.JSON(w, http.StatusCreated, resp)
}

// decodeJSONRequest accepts exactly one JSON value before a chat operation.
func decodeJSONRequest(r *http.Request, target any) error {
	return httputil.DecodeSingleJSON(r.Body, target)
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

	// Check for active legacy SSE runs first. Local/dev stacks without RabbitMQ
	// use bgRegistry, and Flutter reload recovery depends on activeRun being
	// visible via GET /api/chat/sessions/{id}.
	if run := h.bgRegistry.GetBySession(id); run != nil {
		h.respondSessionWithActiveRun(w, resp, backgroundRunResponse(run))
		return
	}

	// Check for active async runs persisted in chat_run.
	if run, found, _ := h.svc.GetActiveRun(r.Context(), id); found {
		h.respondSessionWithActiveRun(w, resp, run)
		return
	}

	respond.JSON(w, http.StatusOK, resp)
}

func (h *Handler) respondSessionWithActiveRun(w http.ResponseWriter, resp ChatSessionResponse, run ChatRunResponse) {
	data, _ := json.Marshal(resp)
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to encode chat session")
		return
	}
	m["activeRun"] = run
	respond.JSON(w, http.StatusOK, m)
}

func backgroundRunResponse(run *BackgroundRun) ChatRunResponse {
	startedAt := run.StartedAt
	resp := ChatRunResponse{
		SessionID: run.SessionID,
		Status:    ChatRunStatus(run.Status),
		StartedAt: &startedAt,
	}
	if id, err := uuid.Parse(run.RunID); err == nil {
		resp.ID = id
	}
	return resp
}

func (h *Handler) effectivePrompt(w http.ResponseWriter, r *http.Request) {
	if h.promptInspector == nil {
		respond.Error(w, http.StatusNotImplemented, "effective prompt inspector is not configured")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	identity := PromptIdentity{TenantID: tenant.FromContext(r.Context())}
	if h.identity != nil {
		identity = h.identity(r)
	}
	if identity.TenantID == "" {
		identity.TenantID = tenant.FromContext(r.Context())
	}
	if identity.TenantName == "" {
		identity.TenantName = identity.TenantID
	}

	resp, err := h.promptInspector.EffectivePrompt(r.Context(), id, identity)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "chat session not found")
			return
		}
		if errors.Is(err, ErrAgentNotFound) {
			respond.Error(w, http.StatusGone, "agent has been deleted; cannot resolve effective prompt")
			return
		}
		if errors.Is(err, ErrNoAgentAvailable) {
			respond.Error(w, http.StatusConflict, "chat session has no agent")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to resolve effective prompt")
		return
	}
	respond.JSON(w, http.StatusOK, resp)
}

func (h *Handler) effectiveTools(w http.ResponseWriter, r *http.Request) {
	if h.toolInspector == nil {
		respond.Error(w, http.StatusNotImplemented, "effective tools inspector is not configured")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	identity := PromptIdentity{TenantID: tenant.FromContext(r.Context())}
	if h.identity != nil {
		identity = h.identity(r)
	}
	if identity.TenantID == "" {
		identity.TenantID = tenant.FromContext(r.Context())
	}
	if identity.TenantName == "" {
		identity.TenantName = identity.TenantID
	}

	resp, err := h.toolInspector.EffectiveTools(r.Context(), id, identity)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "chat session not found")
			return
		}
		if errors.Is(err, ErrAgentNotFound) {
			respond.Error(w, http.StatusGone, "agent has been deleted; cannot resolve effective tools")
			return
		}
		if errors.Is(err, ErrNoAgentAvailable) {
			respond.Error(w, http.StatusConflict, "chat session has no agent")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to resolve effective tools")
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
	if err := decodeJSONRequest(r, &req); err != nil {
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

func (h *Handler) cloneSession(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req CloneSessionRequest
	if err := httputil.DecodeOptionalSingleJSON(r.Body, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.svc.CloneSession(r.Context(), id, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "chat session not found")
			return
		}
		if errors.Is(err, ErrMessageNotFound) {
			respond.Error(w, http.StatusNotFound, "clone boundary message not found")
			return
		}
		msg := err.Error()
		if strings.Contains(msg, "SQLSTATE") || strings.Contains(msg, "ERROR:") || strings.Contains(msg, "clone session:") || strings.Contains(msg, "clone message:") {
			slog.Error("chat: cloneSession failed", "sessionID", id, "err", err)
			respond.Error(w, http.StatusInternalServerError, "failed to clone chat session")
			return
		}
		respond.Error(w, http.StatusUnprocessableEntity, msg)
		return
	}

	respond.JSON(w, http.StatusCreated, resp)
}

func (h *Handler) listMessages(w http.ResponseWriter, r *http.Request) {
	sessionID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid session id")
		return
	}

	// Bug 204: validate session existence before listing messages.
	// Antes retornava 200+empty para session inexistente, vetor de
	// probing (ataque tentava UUIDs até bater num que retornasse
	// dados, indicando session existente em outro tenant — embora
	// schema isolation já bloquearia, o endpoint UX ficava confuso).
	if _, err := h.svc.GetSession(r.Context(), sessionID); err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "chat session not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to list messages")
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
	if err := decodeJSONRequest(r, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Bug 230: validar session existence antes de tentar INSERT.
	// Antes: 422 com FK constraint name + SQLSTATE 23503 leaked.
	if _, err := h.svc.GetSession(r.Context(), sessionID); err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "chat session not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to add message")
		return
	}

	resp, err := h.svc.AddMessage(r.Context(), sessionID, req)
	if err != nil {
		// Bug 247: session arquivada não aceita novas mensagens.
		if errors.Is(err, ErrSessionArchived) {
			respond.Error(w, http.StatusConflict, err.Error())
			return
		}
		// Bug 230: erros wrapped do repositório (FK/constraint/SQLSTATE)
		// não podem vazar — o caminho de validação do service usa
		// fmt.Errorf sem %w. Os com "%w" são wrapped errors do repo.
		msg := err.Error()
		if strings.Contains(msg, "SQLSTATE") || strings.Contains(msg, "ERROR:") || strings.Contains(msg, "create message:") {
			slog.Error("chat: addMessage failed", "sessionID", sessionID, "err", err)
			respond.Error(w, http.StatusInternalServerError, "failed to add message")
			return
		}
		respond.Error(w, http.StatusUnprocessableEntity, msg)
		return
	}

	respond.JSON(w, http.StatusCreated, resp)
}

// elicitationRespondRequest is the body for POST /api/chat/sessions/{id}/elicitation/{requestId}/respond.
type elicitationRespondRequest struct {
	Action  string                 `json:"action"`  // "accept" | "decline" | "cancel"
	Content map[string]interface{} `json:"content"` // form field values (for accept)
	Values  map[string]interface{} `json:"values"`  // alias for Content (some clients use "values")
}

// UnmarshalJSON accepts content aliases only when they describe the same
// elicitation response.
func (r *elicitationRespondRequest) UnmarshalJSON(data []byte) error {
	var wire struct {
		Action  string                  `json:"action"`
		Content *map[string]interface{} `json:"content"`
		Values  *map[string]interface{} `json:"values"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	if err := validateElicitationContentAliases(wire.Content, wire.Values); err != nil {
		return err
	}

	*r = elicitationRespondRequest{Action: wire.Action}
	if wire.Content != nil {
		r.Content = *wire.Content
	}
	if wire.Values != nil {
		r.Values = *wire.Values
	}
	return nil
}

// resumeElicitationRequest is the canonical body for
// POST /api/chat/sessions/{id}/resume.
type resumeElicitationRequest struct {
	RequestID       string                 `json:"requestId"`
	ResumeData      map[string]interface{} `json:"resume_data"`
	ResumeDataCamel map[string]interface{} `json:"resumeData"`
	Action          string                 `json:"action"`
	Content         map[string]interface{} `json:"content"`
	Values          map[string]interface{} `json:"values"`
}

// UnmarshalJSON preserves the historical content aliases only when all
// non-empty aliases describe the same elicitation response.
func (r *resumeElicitationRequest) UnmarshalJSON(data []byte) error {
	var wire struct {
		RequestID       string                  `json:"requestId"`
		ResumeData      *map[string]interface{} `json:"resume_data"`
		ResumeDataCamel *map[string]interface{} `json:"resumeData"`
		Action          string                  `json:"action"`
		Content         *map[string]interface{} `json:"content"`
		Values          *map[string]interface{} `json:"values"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	if err := validateElicitationContentAliases(wire.ResumeData, wire.ResumeDataCamel, wire.Content, wire.Values); err != nil {
		return err
	}

	*r = resumeElicitationRequest{
		RequestID: wire.RequestID,
		Action:    wire.Action,
	}
	if wire.ResumeData != nil {
		r.ResumeData = *wire.ResumeData
	}
	if wire.ResumeDataCamel != nil {
		r.ResumeDataCamel = *wire.ResumeDataCamel
	}
	if wire.Content != nil {
		r.Content = *wire.Content
	}
	if wire.Values != nil {
		r.Values = *wire.Values
	}
	return nil
}

func validateElicitationContentAliases(aliases ...*map[string]interface{}) error {
	var canonical map[string]interface{}
	for _, alias := range aliases {
		if alias == nil || len(*alias) == 0 {
			continue
		}
		if canonical == nil {
			canonical = *alias
			continue
		}
		if !reflect.DeepEqual(canonical, *alias) {
			return fmt.Errorf("chat: conflicting resume content aliases")
		}
	}
	return nil
}

// runSessionRequest is the body for POST /api/chat/sessions/{id}/run.
type runSessionRequest struct {
	Message   string              `json:"message"`
	Overrides runSessionOverrides `json:"overrides,omitempty"`
}

type runSessionOverrides struct {
	SystemPrompt *string `json:"systemPrompt,omitempty"`
}

const (
	maxVoiceUploadBytes       = 25 << 20
	maxVoiceMultipartOverhead = 64 << 10
	maxVoiceMultipartField    = 8 << 10
)

// voiceInput handles POST /api/chat/sessions/{id}/voice/input.
func (h *Handler) voiceInput(w http.ResponseWriter, r *http.Request) {
	if h.voiceSvc == nil {
		respond.Error(w, http.StatusServiceUnavailable, "voice provider is not configured")
		return
	}
	sessionID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid session id")
		return
	}
	if _, err := h.svc.GetSession(r.Context(), sessionID); err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "session not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to load session")
		return
	}

	input, err := readVoiceInput(w, r)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	transcription, err := h.voiceSvc.Transcribe(r.Context(), input)
	if err != nil {
		slog.Error("chat: voice transcription failed", "sessionID", sessionID, "err", err)
		respond.Error(w, http.StatusBadGateway, "voice transcription is temporarily unavailable")
		return
	}
	if strings.TrimSpace(transcription.Text) == "" {
		respond.Error(w, http.StatusUnprocessableEntity, "audio transcription is empty")
		return
	}

	resp := VoiceInputResponse{
		SessionID:     sessionID.String(),
		Status:        "transcribed",
		Transcription: transcription,
	}
	status := http.StatusOK
	if r.URL.Query().Get("run") != "false" && h.executor == nil {
		tenantID := tenant.FromContext(r.Context())
		runID, audio, err := h.runVoiceSessionSynchronously(r.Context(), sessionID, tenantID, transcription)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				respond.Error(w, http.StatusNotFound, "session not found")
				return
			}
			if errors.Is(err, ErrNoAgentAvailable) {
				respond.Error(w, http.StatusServiceUnavailable, friendlyStartupError(err.Error()))
				return
			}
			if errors.Is(err, ErrSessionArchived) {
				respond.Error(w, http.StatusConflict, err.Error())
				return
			}
			slog.Error("chat: synchronous voice run failed", "sessionID", sessionID, "err", err)
			respond.Error(w, http.StatusBadGateway, "failed to process voice run")
			return
		}
		resp.RunID = &runID
		resp.Status = "completed"
		resp.Audio = audio
	}
	if r.URL.Query().Get("run") != "false" && h.executor != nil {
		tenantID := tenant.FromContext(r.Context())
		runID, err := h.executor.EnqueueRunWithOptions(r.Context(), sessionID, tenantID, transcription.Text, EnqueueRunOptions{
			VoiceOutput: true,
		})
		if err != nil {
			if errors.Is(err, ErrRunAlreadyActive) {
				respond.Error(w, http.StatusConflict, "a run is already in progress for this session")
				return
			}
			if errors.Is(err, ErrQueueUnavailable) {
				respond.Error(w, http.StatusServiceUnavailable, "chat queue is temporarily unavailable")
				return
			}
			if errors.Is(err, ErrAgentNotFound) {
				respond.Error(w, http.StatusGone, "agent has been deleted; cannot run on orphaned session")
				return
			}
			if errors.Is(err, ErrSessionArchived) {
				respond.Error(w, http.StatusConflict, err.Error())
				return
			}
			respond.Error(w, http.StatusInternalServerError, "failed to enqueue voice run")
			return
		}
		runIDText := runID.String()
		resp.RunID = &runIDText
		resp.Status = "accepted"
		h.appendVoiceTranscriptionEvent(runIDText, sessionID, transcription)
		status = http.StatusAccepted
	}
	respond.JSON(w, status, resp)
}

func (h *Handler) runVoiceSessionSynchronously(ctx context.Context, sessionID uuid.UUID, tenantID string, transcription VoiceTranscription) (string, *VoiceAudio, error) {
	runID, runCtx := h.bgRegistry.Register(sessionID)
	rawToken := tenant.TokenFromContext(ctx)
	runCtx = tenant.NewContextWithToken(runCtx, tenantID, rawToken)

	ch, err := h.svc.RunSession(runCtx, sessionID, transcription.Text, tenantID)
	if err != nil {
		h.bgRegistry.Cancel(runID)
		return "", nil, err
	}

	h.bgRegistry.AttachEvents(runID, ch)
	buf := h.bufferRegistry.GetOrCreateForSession(runID, sessionID, DefaultEventBufferSize)
	var assistantText strings.Builder
	for ev := range ch {
		buf.Append(ev)
		if ev.Type == "text_delta" {
			assistantText.WriteString(extractTextDelta(ev.Data))
		}
	}

	h.bgRegistry.MarkCompleted(runID)
	var audio *VoiceAudio
	if text := strings.TrimSpace(assistantText.String()); text != "" {
		synthesized, err := h.voiceSvc.Synthesize(ctx, VoiceSynthesisInput{Text: text})
		if err != nil {
			buf.MarkDone()
			return runID, nil, fmt.Errorf("voice: synthesize assistant answer: %w", err)
		}
		audio = &synthesized
		if data, err := json.Marshal(VoiceAudioDelta{Chunk: synthesized.Base64, Format: synthesized.Format}); err == nil {
			buf.Append(RunEvent{Type: EventAudioDelta, Data: data})
		}
	}
	buf.MarkDone()
	return runID, audio, nil
}

func (h *Handler) appendVoiceTranscriptionEvent(runID string, sessionID uuid.UUID, transcription VoiceTranscription) {
	data, err := json.Marshal(transcription)
	if err != nil {
		return
	}
	h.bufferRegistry.GetOrCreateForSession(runID, sessionID, DefaultEventBufferSize).Append(RunEvent{
		Type: EventTranscription,
		Data: data,
	})
}

func readVoiceInput(w http.ResponseWriter, r *http.Request) (VoiceTranscriptionInput, error) {
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		file, fields, err := httputil.ReadLimitedMultipartFile(w, r, httputil.LimitedMultipartOptions{
			FileFields:    []string{"audio", "file"},
			MaxFileBytes:  maxVoiceUploadBytes,
			MaxBodyBytes:  maxVoiceUploadBytes + maxVoiceMultipartOverhead,
			MaxFieldBytes: maxVoiceMultipartField,
		})
		if err != nil {
			return VoiceTranscriptionInput{}, fmt.Errorf("invalid multipart audio upload")
		}
		return VoiceTranscriptionInput{
			Filename:    file.Filename,
			ContentType: file.ContentType,
			Audio:       file.Content,
			Language:    fields["language"],
		}, nil
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxVoiceUploadBytes)
	audio, err := io.ReadAll(io.LimitReader(r.Body, maxVoiceUploadBytes+1))
	if err != nil {
		return VoiceTranscriptionInput{}, fmt.Errorf("failed to read audio")
	}
	if len(audio) == 0 {
		return VoiceTranscriptionInput{}, fmt.Errorf("audio body is required")
	}
	if len(audio) > maxVoiceUploadBytes {
		return VoiceTranscriptionInput{}, fmt.Errorf("audio file exceeds 25MB")
	}
	return VoiceTranscriptionInput{
		Filename:    r.URL.Query().Get("filename"),
		ContentType: contentType,
		Audio:       audio,
		Language:    r.URL.Query().Get("language"),
	}, nil
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
	if err := decodeJSONRequest(r, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Message == "" {
		respond.Error(w, http.StatusBadRequest, "message is required")
		return
	}
	runOpts := RunSessionOptions{}
	if req.Overrides.SystemPrompt != nil {
		runOpts.SystemPromptOverride = req.Overrides.SystemPrompt
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		respond.Error(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	tenantID := tenant.FromContext(r.Context())

	// If AsyncExecutor is available, use it to start the run in background.
	if h.executor != nil {
		runID, err := h.executor.EnqueueRunWithOptions(r.Context(), sessionID, tenantID, req.Message, EnqueueRunOptions{
			SystemPromptOverride: runOpts.SystemPromptOverride,
		})
		if err != nil {
			if errors.Is(err, ErrRunAlreadyActive) {
				respond.Error(w, http.StatusConflict, "a run is already in progress for this session")
				return
			}
			if errors.Is(err, ErrNotFound) {
				respond.Error(w, http.StatusNotFound, "chat session not found")
				return
			}
			if errors.Is(err, ErrQueueUnavailable) {
				respond.Error(w, http.StatusServiceUnavailable, "chat queue is temporarily unavailable")
				return
			}
			// Bug 244: session existe mas agent foi deletado → 410 Gone.
			// Sem isso retornava 202 e o run falhava silenciosamente no worker.
			if errors.Is(err, ErrAgentNotFound) {
				respond.Error(w, http.StatusGone, "agent has been deleted; cannot run on orphaned session")
				return
			}
			// Bug 246: session arquivada não aceita novos runs.
			if errors.Is(err, ErrSessionArchived) {
				respond.Error(w, http.StatusConflict, err.Error())
				return
			}
			respond.Error(w, http.StatusInternalServerError, "failed to enqueue run")
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

	ch, err := h.svc.RunSession(runCtx, sessionID, req.Message, tenantID, runOpts)
	if err != nil {
		h.bgRegistry.Cancel(runID)
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "session not found")
			return
		}
		// OOB: a session with no agent and no published agent to route to. 503
		// (not 422) — it is a tenant-level configuration gap, retryable once an
		// admin creates/publishes an agent.
		if errors.Is(err, ErrNoAgentAvailable) {
			respond.Error(w, http.StatusServiceUnavailable, friendlyStartupError(err.Error()))
			return
		}
		slog.Error("chat: synchronous run failed", "sessionID", sessionID, "err", err)
		respond.Error(w, http.StatusUnprocessableEntity, "failed to start chat run")
		return
	}

	h.bgRegistry.AttachEvents(runID, ch)
	buf := h.bufferRegistry.GetOrCreateForSession(runID, sessionID, DefaultEventBufferSize)

	// Set SSE headers.
	w.Header().Set("Content-Type", "text/event-stream")
	// Bug 283: no-cache + no-store (preserva proteção do bug 280)
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("X-Run-ID", runID)
	w.WriteHeader(http.StatusOK)

	// Stream events to client. The goroutine is the sole consumer of the Runner's
	// channel and appends every event to the resume buffer before optional live
	// delivery. When the client disconnects, live delivery is discarded while the
	// buffer keeps receiving events for /resume.
	sseCh := make(chan BufferedEvent, 64)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer close(sseCh)
		for ev := range ch {
			seq := buf.Append(ev)
			buffered := BufferedEvent{ID: seq, Event: ev}
			select {
			case sseCh <- buffered:
			default:
				// SSE writer can't keep up or disconnected -- discard event.
				// The Runner persists everything, so no data is lost.
			}
		}
		h.bgRegistry.MarkCompleted(runID)
		buf.MarkDone()
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
				return
			}
			if err := writeSSEBufferedEvent(w, runID, ev); err != nil {
				return
			}
			flusher.Flush()
		case <-done:
			// Run goroutine is done; drain any remaining buffered events.
			for {
				select {
				case ev, ok := <-sseCh:
					if !ok {
						return
					}
					if err := writeSSEBufferedEvent(w, runID, ev); err != nil {
						return
					}
					flusher.Flush()
				default:
					// No more events pending; done signal may have raced with close(sseCh).
					// Wait for sseCh to be closed to ensure MarkDone is called.
					for ev := range sseCh {
						if err := writeSSEBufferedEvent(w, runID, ev); err != nil {
							return
						}
						flusher.Flush()
					}
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
//
// Runs are tracked in two places depending on the execution path:
//  1. bgRegistry (in-memory) — synchronous SSE runs held for the HTTP handler's lifetime
//  2. chat_run persistent store — async runs dispatched via AsyncExecutor/RabbitMQ
//
// The handler consults both so programmatic polling works for every run type.
// P-C102-2: async runs are not in bgRegistry and previously returned 404.
func (h *Handler) runStatus(w http.ResponseWriter, r *http.Request) {
	// Bug 226: validar session id da URL contra run.session_id.
	// Antes: GET /sessions/{bogus}/run/{realRun}/status retornava 200
	// com os dados do run mesmo que a session da URL não existisse ou
	// não fosse a dona do run — cross-resource leak entre tenants/sessions.
	urlSessionID := chi.URLParam(r, "id")
	runID := chi.URLParam(r, "runId")
	if runID == "" {
		respond.Error(w, http.StatusBadRequest, "runId is required")
		return
	}

	// Fast path: in-memory bgRegistry (sync SSE runs).
	if run := h.bgRegistry.Get(runID); run != nil {
		if urlSessionID != "" && run.SessionID.String() != urlSessionID {
			respond.Error(w, http.StatusNotFound, "run not found")
			return
		}
		respond.JSON(w, http.StatusOK, runStatusResponse{
			RunID:     run.RunID,
			SessionID: run.SessionID,
			Status:    run.Status,
			StartedAt: run.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
		return
	}

	// Fallback: persistent store for async runs.
	if h.executor != nil {
		runUUID, err := uuid.Parse(runID)
		if err == nil {
			if persisted, perr := h.executor.GetRunByID(r.Context(), runUUID); perr == nil {
				if urlSessionID != "" && persisted.SessionID.String() != urlSessionID {
					respond.Error(w, http.StatusNotFound, "run not found")
					return
				}
				startedAt := ""
				if persisted.StartedAt != nil {
					startedAt = persisted.StartedAt.Format("2006-01-02T15:04:05Z07:00")
				}
				respond.JSON(w, http.StatusOK, runStatusResponse{
					RunID:     persisted.ID.String(),
					SessionID: persisted.SessionID,
					Status:    RunStatus(persisted.Status),
					StartedAt: startedAt,
				})
				return
			}
		}
	}

	respond.Error(w, http.StatusNotFound, "run not found")
}

// cancelRun handles POST /api/chat/sessions/{id}/run/{runId}/cancel.
// Tries the in-process bgRegistry (legacy synchronous path) first, then
// falls back to AsyncExecutor.Cancel for runs dispatched via RabbitMQ.
func (h *Handler) cancelRun(w http.ResponseWriter, r *http.Request) {
	// Bug 227: validar session id da URL contra run.session_id antes
	// de aplicar o cancel. Antes era possível cancelar qualquer run
	// passando uma session id arbitrária.
	urlSessionID := chi.URLParam(r, "id")
	runID := chi.URLParam(r, "runId")
	if runID == "" {
		respond.Error(w, http.StatusBadRequest, "runId is required")
		return
	}

	if run := h.bgRegistry.Get(runID); run != nil {
		if urlSessionID != "" && run.SessionID.String() != urlSessionID {
			respond.Error(w, http.StatusNotFound, "run not found")
			return
		}
		h.bgRegistry.Cancel(runID)
		respond.JSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
		return
	}

	if h.executor != nil {
		runUUID, err := uuid.Parse(runID)
		if err != nil {
			respond.Error(w, http.StatusBadRequest, "invalid runId")
			return
		}
		// Bug 207 + 227: antes de qualquer UPDATE, validar a existência E
		// que o run pertence à session da URL.
		persisted, getErr := h.executor.repo.GetRunByID(r.Context(), runUUID)
		if getErr != nil {
			respond.Error(w, http.StatusNotFound, "run not found")
			return
		}
		if urlSessionID != "" && persisted.SessionID.String() != urlSessionID {
			respond.Error(w, http.StatusNotFound, "run not found")
			return
		}
		if h.executor.Cancel(runUUID) {
			respond.JSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
			return
		}
		// Run not in-flight on this pod — best-effort: persist the
		// cancellation in the DB so a poll on /status surfaces the right
		// state even if the worker already finished or runs on another pod.
		if err := h.executor.repo.UpdateRunStatus(r.Context(), runUUID, ChatRunStatusCancelled, ""); err == nil {
			respond.JSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
			return
		}
	}

	respond.Error(w, http.StatusNotFound, "run not found")
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
	// Bug 228: validar que o run pertence à session da URL antes de
	// começar o stream. Antes era possível resumir qualquer run com
	// uma session id arbitrária e receber o SSE inteiro.
	urlSessionID := chi.URLParam(r, "id")
	if _, err := uuid.Parse(urlSessionID); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid session id")
		return
	}

	runID := chi.URLParam(r, "runId")
	if runID == "" {
		respond.Error(w, http.StatusBadRequest, "run id is required")
		return
	}

	if ownerSessionID, ok := h.bufferRegistry.Owner(runID); ok && ownerSessionID.String() != urlSessionID {
		respond.Error(w, http.StatusNotFound, "run not found")
		return
	}

	if runUUID, err := uuid.Parse(runID); err == nil && h.executor != nil {
		if persisted, perr := h.executor.GetRunByID(r.Context(), runUUID); perr == nil {
			if persisted.SessionID.String() != urlSessionID {
				respond.Error(w, http.StatusNotFound, "run not found")
				return
			}
		}
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
		parsedRunID, seq, ok := ParseSSEID(lastEventID)
		if !ok || parsedRunID != runID {
			respond.Error(w, http.StatusBadRequest, "invalid last event id")
			return
		}
		afterSeq = seq
	}

	// Set SSE headers.
	w.Header().Set("Content-Type", "text/event-stream")
	// Bug 283: no-cache + no-store (preserva proteção do bug 280)
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("X-Run-ID", runID)
	w.WriteHeader(http.StatusOK)

	// Replay buffered events.
	lastSent := afterSeq
	events, ok := buf.EventsSince(afterSeq)
	if !ok {
		// Overflow -- client missed events that were evicted from the buffer.
		overflowData, _ := json.Marshal(ReconnectOverflowData{
			Message:     "events lost: buffer overflow since last event id",
			LastEventID: lastEventID,
		})
		if err := writeSSEFrame(w, "", "reconnect_overflow", overflowData); err != nil {
			return
		}
		flusher.Flush()
		lastSent = buf.NewestSeq()
	}
	events = filterReplayableEvents(events)

	// Send replayed events.
	for _, ev := range events {
		if err := writeSSEBufferedEvent(w, runID, ev); err != nil {
			return
		}
		flusher.Flush()
		lastSent = ev.ID
	}

	// If the run is done, no need to wait for more events.
	if buf.IsDone() {
		return
	}

	// Continue streaming new events by polling the buffer.
	// We poll at a short interval to avoid busy-waiting.
	ctx := r.Context()
	ticker := newTicker(50 * millisecondsUnit)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C():
			newEvents, _ := buf.EventsSince(lastSent)
			for _, ev := range newEvents {
				if err := writeSSEBufferedEvent(w, runID, ev); err != nil {
					return
				}
				flusher.Flush()
				lastSent = ev.ID
			}
			if buf.IsDone() && len(newEvents) == 0 {
				return
			}
		}
	}
}

func writeSSEBufferedEvent(w io.Writer, runID string, ev BufferedEvent) error {
	return writeSSEFrame(w, ev.FormatSSEID(runID), ev.Event.Type, ev.Event.Data)
}

func writeSSEFrame(w io.Writer, id, eventType string, data json.RawMessage) error {
	id = safeSSELineField(id, "")
	eventType = safeSSELineField(eventType, "message")
	payload := safeSSEPayload(data)

	if id != "" {
		// #nosec G705 -- SSE id is restricted to a single line before writing to a text/event-stream response.
		if _, err := fmt.Fprintf(w, "id: %s\n", id); err != nil {
			return err
		}
	}
	// #nosec G705 -- SSE event type is restricted to a single line before writing to a text/event-stream response.
	if _, err := fmt.Fprintf(w, "event: %s\n", eventType); err != nil {
		return err
	}
	for _, line := range strings.Split(payload, "\n") {
		// #nosec G705 -- SSE data is compacted or JSON-encoded, then each line is emitted with a data prefix.
		if _, err := fmt.Fprintf(w, "data: %s\n", line); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "\n")
	return err
}

func safeSSEPayload(data json.RawMessage) string {
	if len(data) == 0 {
		return "null"
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, data); err == nil {
		return compact.String()
	}
	encoded, err := json.Marshal(string(data))
	if err != nil {
		return "null"
	}
	return string(encoded)
}

func safeSSELineField(value, fallback string) string {
	if value == "" || strings.IndexFunc(value, func(r rune) bool {
		return r < 0x20 || r == 0x7f
	}) >= 0 {
		return fallback
	}
	return value
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
	if err := decodeJSONRequest(r, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Action == "" {
		req.Action = "accept"
	}
	// BUG-ASK_USER-1: Accept "values" as an alias for "content" so clients that
	// send {"values": {...}} instead of {"content": {...}} are not silently ignored.
	if len(req.Content) == 0 && len(req.Values) > 0 {
		req.Content = req.Values
	}

	result := ElicitationResult{
		Action:  req.Action,
		Content: req.Content,
	}

	if ok := h.svc.RespondElicitation(r.Context(), sessionID, requestID, result); !ok {
		respond.Error(w, http.StatusNotFound, "elicitation request not found or already resolved")
		return
	}

	respond.NoContent(w)
}

// resumeElicitation handles POST /api/chat/sessions/{id}/resume.
// It is the canonical suspend/resume endpoint; the legacy elicitation route
// remains accepted for older clients.
func (h *Handler) resumeElicitation(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	if _, err := uuid.Parse(sessionID); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid session id")
		return
	}

	var req resumeElicitationRequest
	if err := decodeJSONRequest(r, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.RequestID) == "" {
		respond.Error(w, http.StatusBadRequest, "requestId is required")
		return
	}
	if req.Action == "" {
		req.Action = "accept"
	}
	content := req.ResumeData
	if len(content) == 0 {
		content = req.ResumeDataCamel
	}
	if len(content) == 0 {
		content = req.Content
	}
	if len(content) == 0 {
		content = req.Values
	}

	result := ElicitationResult{
		Action:  req.Action,
		Content: content,
	}

	if ok := h.svc.RespondElicitation(r.Context(), sessionID, req.RequestID, result); !ok {
		respond.Error(w, http.StatusNotFound, "elicitation request not found or already resolved")
		return
	}

	respond.NoContent(w)
}

// clientState handles POST /api/chat/sessions/{id}/client-state.
//
// CopilotKit Phase 1+: the body may carry any combination of:
//   - frontendActions: actions the Flutter client is exposing for this session
//   - readables:       app state exposed to the agent as <app_state>
//   - actionResults:   results for a previously-emitted frontend_action_call
//
// Returns 204 on success. Sessions are not validated against the DB — the
// store is a best-effort in-memory cache that survives a missing or stale
// session UUID by silently dropping pending callbacks.
func (h *Handler) clientState(w http.ResponseWriter, r *http.Request) {
	sessionID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid session id")
		return
	}

	var patch ClientStatePatch
	if err := httputil.DecodeOptionalSingleJSON(r.Body, &patch); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	h.svc.ApplyClientState(sessionID, patch)
	respond.NoContent(w)
}

// listTasks handles GET /api/chat/sessions/{id}/tasks.
// Returns a paginated list of coordinator tasks for the session.
func (h *Handler) listTasks(w http.ResponseWriter, r *http.Request) {
	if h.taskRepo == nil {
		respond.Error(w, http.StatusNotImplemented, "task persistence not configured")
		return
	}
	sessionID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid session id")
		return
	}
	// Bug 205: validate session existence (mesmo padrão de listMessages bug 204).
	if _, err := h.svc.GetSession(r.Context(), sessionID); err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "chat session not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to list tasks")
		return
	}
	req := pagination.ParsePageRequest(r)
	tasks, total, err := h.taskRepo.ListBySession(r.Context(), sessionID, req)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to list tasks")
		return
	}
	respond.JSON(w, http.StatusOK, pagination.NewPage(tasks, total, req))
}

// listTaskNotifications handles GET /api/chat/sessions/{id}/tasks/{taskId}/notifications.
// Returns all worker notifications for the given task.
func (h *Handler) listTaskNotifications(w http.ResponseWriter, r *http.Request) {
	if h.taskRepo == nil {
		respond.Error(w, http.StatusNotImplemented, "task persistence not configured")
		return
	}
	sessionID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid session id")
		return
	}
	// Bug 229: validar a existência da session antes de listar
	// notifications (mesmo padrão de listMessages bug 204 / listTasks
	// bug 205). Antes retornávamos 200 [] mesmo para sessions bogus.
	if _, err := h.svc.GetSession(r.Context(), sessionID); err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "chat session not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to list notifications")
		return
	}
	taskID := chi.URLParam(r, "taskId")
	if taskID == "" {
		respond.Error(w, http.StatusBadRequest, "taskId is required")
		return
	}
	notifications, err := h.taskRepo.ListNotificationsByTask(r.Context(), taskID)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to list notifications")
		return
	}
	publicNotifications := make([]task.Notification, len(notifications))
	for index, notification := range notifications {
		publicNotifications[index] = publicTaskNotificationFrom(notification)
	}
	respond.JSON(w, http.StatusOK, publicNotifications)
}

func publicTaskNotificationFrom(notification task.Notification) task.Notification {
	public := notification
	if notification.Error != nil {
		redacted := redactAsyncExternalDiagnosticText(*notification.Error)
		public.Error = &redacted
	}
	if len(notification.Findings) > 0 {
		redacted := redactAsyncExternalDiagnosticText(string(notification.Findings))
		if json.Valid([]byte(redacted)) {
			public.Findings = json.RawMessage(redacted)
		} else {
			encoded, _ := json.Marshal(redacted)
			public.Findings = encoded
		}
	}
	return public
}

// listPermissionAudit handles GET /api/chat/sessions/{id}/permission-audit.
// Returns the most recent permission decisions for the session.
func (h *Handler) listPermissionAudit(w http.ResponseWriter, r *http.Request) {
	if h.permAuditReader == nil {
		respond.Error(w, http.StatusNotImplemented, "permission audit not configured")
		return
	}
	sessionID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid session id")
		return
	}
	// Bug 211: valida session antes de listar permission-audit (evita 200+empty).
	if _, err := h.svc.GetSession(r.Context(), sessionID); err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "chat session not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to list permission audit")
		return
	}
	entries, err := h.permAuditReader.ListBySession(r.Context(), sessionID, 0)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to list permission audit")
		return
	}
	respond.JSON(w, http.StatusOK, entries)
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

// getRun handles GET /api/chat/runs/{id}. P-C325-1: allows clients to query
// run state and metrics by run ID without knowing the parent session ID.
func (h *Handler) getRun(w http.ResponseWriter, r *http.Request) {
	if h.runLookup == nil {
		respond.Error(w, http.StatusServiceUnavailable, "run lookup not available")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid run id")
		return
	}

	run, err := h.runLookup.GetRunByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "run not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to get run")
		return
	}

	respond.JSON(w, http.StatusOK, RunResponseFrom(run))
}
