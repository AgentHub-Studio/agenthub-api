package agentic

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// CopilotKit-style frontend actions.
//
// The Flutter app declares a set of "frontend actions" — local functions that
// the agent may invoke during a run. Declaration flows through the
// `POST /api/chat/sessions/{id}/client-state` endpoint (handler in chat
// package) which delegates to a [ClientStateStore]. The runner then:
//
//   1. Appends declared actions to the LLM tool catalog (alongside skill/MCP
//      tools) when the next turn starts.
//   2. On a tool call whose name matches a declared action, blocks via
//      [FrontendActionHandler.Submit] until the Flutter client posts the
//      result back through the same `client-state` endpoint.
//
// This design intentionally mirrors [ElicitationHandler] — the existing
// request/response queue that powers `ask_user` — so the runner only needs to
// learn one new dependency interface.

// FrontendActionParameters mirrors the JSON Schema fragment declared by the
// client for a frontend action.
type FrontendActionParameters = json.RawMessage

// FrontendAction is a single client-declared action exposed to the agent.
type FrontendAction struct {
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Parameters  FrontendActionParameters `json:"parameters,omitempty"`
}

// FrontendActionResult is the response a client returns after executing an
// action that was triggered by an `EventFrontendActionCall`.
type FrontendActionResult struct {
	// ID is the tool-call ID echoed back from the originating event.
	ID     string          `json:"id"`
	Status string          `json:"status"` // "ok" | "error"
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// FrontendActionSubmitter is implemented by anything that can route a frontend
// action call to a client and block until the client posts the result back.
//
// The runner only needs the [Submit] method; the broader provider interface
// adds [GetActions] for catalog injection.
type FrontendActionSubmitter interface {
	// Submit dispatches a frontend action call and blocks until the client
	// returns a result or [ctx] is cancelled. A cancellation produces a result
	// with status="error" and Error="cancelled".
	Submit(ctx context.Context, sessionID uuid.UUID, callID, name string, args json.RawMessage) FrontendActionResult
}

// FrontendActionsProvider is the full surface the runner sees: catalog +
// submission + readables. Implementations are session-scoped via the registry
// (see [SessionRunnerAdapter.frontendActions]).
type FrontendActionsProvider interface {
	FrontendActionSubmitter

	// GetActions returns the actions currently declared by the client for
	// this session. Returns an empty slice when nothing has been declared.
	GetActions(sessionID uuid.UUID) []FrontendAction

	// GetReadables returns the readables currently declared by the client
	// for this session. Returns an empty slice when nothing has been declared.
	// Used by the runner to inject an <app_state> block into the system prompt.
	GetReadables(sessionID uuid.UUID) []Readable
}

// --- request queue -----------------------------------------------------------

// FrontendActionRequest is a pending request in the queue.
type FrontendActionRequest struct {
	SessionID uuid.UUID
	CallID    string
	Name      string
	Arguments json.RawMessage
	Ctx       context.Context
	respondCh chan FrontendActionResult
	responded bool
	CreatedAt time.Time
}

// FrontendActionHandler manages a queue of pending frontend action calls for a
// single session. Mirrors [ElicitationHandler] (request/response over channel)
// so the runner can block via Submit() and the HTTP handler unblocks via
// Respond() when the client posts an actionResult.
type FrontendActionHandler struct {
	mu        sync.Mutex
	queue     []*FrontendActionRequest
	onEnqueue func(*FrontendActionRequest)
}

// NewFrontendActionHandler creates a handler.
func NewFrontendActionHandler() *FrontendActionHandler {
	return &FrontendActionHandler{}
}

// OnEnqueue sets a callback invoked when a new request is queued. The adapter
// uses it to forward an `EventFrontendActionCall` to the SSE stream.
func (h *FrontendActionHandler) OnEnqueue(fn func(*FrontendActionRequest)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onEnqueue = fn
}

// Submit enqueues a request and blocks until [Respond] is called for the same
// callID, or until [ctx] is cancelled.
func (h *FrontendActionHandler) Submit(ctx context.Context, sessionID uuid.UUID, callID, name string, args json.RawMessage) FrontendActionResult {
	req := &FrontendActionRequest{
		SessionID: sessionID,
		CallID:    callID,
		Name:      name,
		Arguments: args,
		Ctx:       ctx,
		respondCh: make(chan FrontendActionResult, 1),
		CreatedAt: time.Now(),
	}

	h.mu.Lock()
	h.queue = append(h.queue, req)
	notify := h.onEnqueue
	h.mu.Unlock()

	if notify != nil {
		notify(req)
	}

	select {
	case result := <-req.respondCh:
		return result
	case <-ctx.Done():
		h.removeFromQueue(callID)
		return FrontendActionResult{ID: callID, Status: "error", Error: "cancelled"}
	}
}

// Respond resolves a pending request by its callID. Returns true when the
// request existed and was unresolved at call time.
func (h *FrontendActionHandler) Respond(callID string, result FrontendActionResult) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, req := range h.queue {
		if req.CallID == callID && !req.responded {
			req.responded = true
			result.ID = callID
			req.respondCh <- result
			return true
		}
	}
	return false
}

// Pending returns a snapshot of all pending (unresponded) requests.
func (h *FrontendActionHandler) Pending() []*FrontendActionRequest {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]*FrontendActionRequest, 0, len(h.queue))
	for _, req := range h.queue {
		if !req.responded {
			out = append(out, req)
		}
	}
	return out
}

func (h *FrontendActionHandler) removeFromQueue(callID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i, req := range h.queue {
		if req.CallID == callID {
			h.queue = append(h.queue[:i], h.queue[i+1:]...)
			return
		}
	}
}

// --- client-state store ------------------------------------------------------

// SessionClientState holds the per-session declarations posted via
// `POST /api/chat/sessions/{id}/client-state`. Readables become a `<app_state>`
// block in the system prompt (Phase 2); actions become LLM tools (Phase 1).
type SessionClientState struct {
	Actions   []FrontendAction `json:"actions,omitempty"`
	Readables []Readable       `json:"readables,omitempty"`
	UpdatedAt time.Time        `json:"-"`
}

// Readable mirrors the client-declared readable payload.
type Readable struct {
	ID          string          `json:"id"`
	Description string          `json:"description"`
	Value       json.RawMessage `json:"value"`
	ParentID    string          `json:"parentId,omitempty"`
}

// ClientStatePatch is the incoming body of `POST /client-state`.
type ClientStatePatch struct {
	FrontendActions []FrontendAction       `json:"frontendActions,omitempty"`
	Readables       []Readable             `json:"readables,omitempty"`
	ActionResults   []FrontendActionResult `json:"actionResults,omitempty"`
}

// ErrUnknownAction is returned by [ClientStateStore.RespondAction] when the
// callID does not match any pending request.
var ErrUnknownAction = errors.New("no pending frontend action for callID")

// ClientStateStore is the per-session container for actions/readables and the
// dispatcher for action results. In Phase 1 we use a simple in-memory map with
// a TTL — persistence across restarts is not a hard requirement and the data
// is naturally rebuilt when the client reopens a session.
type ClientStateStore struct {
	mu       sync.RWMutex
	sessions map[uuid.UUID]*sessionEntry
	ttl      time.Duration
}

type sessionEntry struct {
	state   SessionClientState
	handler *FrontendActionHandler
}

// NewClientStateStore creates a store with the given TTL. Pass 0 to disable
// expiration (useful in tests).
func NewClientStateStore(ttl time.Duration) *ClientStateStore {
	return &ClientStateStore{
		sessions: make(map[uuid.UUID]*sessionEntry),
		ttl:      ttl,
	}
}

// Apply merges a patch into the session state.
//
// FrontendActions and Readables overwrite the existing slice when non-nil
// (clients send the full snapshot, not deltas). ActionResults are dispatched
// to the handler — when no handler is registered or no pending call matches,
// the result is silently dropped (the run has likely already completed).
func (s *ClientStateStore) Apply(sessionID uuid.UUID, patch ClientStatePatch) {
	s.mu.Lock()
	entry, ok := s.sessions[sessionID]
	if !ok {
		entry = &sessionEntry{}
		s.sessions[sessionID] = entry
	}
	if patch.FrontendActions != nil {
		entry.state.Actions = patch.FrontendActions
	}
	if patch.Readables != nil {
		entry.state.Readables = patch.Readables
	}
	entry.state.UpdatedAt = time.Now()
	handler := entry.handler
	s.mu.Unlock()

	if handler != nil {
		for _, res := range patch.ActionResults {
			handler.Respond(res.ID, res)
		}
	}
}

// GetActions returns the declared frontend actions for the session.
func (s *ClientStateStore) GetActions(sessionID uuid.UUID) []FrontendAction {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.sessions[sessionID]
	if !ok {
		return nil
	}
	out := make([]FrontendAction, len(entry.state.Actions))
	copy(out, entry.state.Actions)
	return out
}

// GetReadables returns the declared readables for the session.
func (s *ClientStateStore) GetReadables(sessionID uuid.UUID) []Readable {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.sessions[sessionID]
	if !ok {
		return nil
	}
	out := make([]Readable, len(entry.state.Readables))
	copy(out, entry.state.Readables)
	return out
}

// AttachHandler binds a per-session [FrontendActionHandler] so that subsequent
// [Apply] calls with ActionResults can be routed to its pending Submit() calls.
// The handler stays attached until [DetachHandler] is called (typically at the
// end of a run).
func (s *ClientStateStore) AttachHandler(sessionID uuid.UUID, h *FrontendActionHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.sessions[sessionID]
	if !ok {
		entry = &sessionEntry{}
		s.sessions[sessionID] = entry
	}
	entry.handler = h
	entry.state.UpdatedAt = time.Now()
}

// DetachHandler removes the handler binding. Subsequent action results for the
// session become no-ops.
func (s *ClientStateStore) DetachHandler(sessionID uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if entry, ok := s.sessions[sessionID]; ok {
		entry.handler = nil
	}
}

// Submit fulfils [FrontendActionSubmitter] by routing the call through the
// session's attached handler. Returns an error result immediately when no
// handler is bound (the run isn't actively listening), matching the elicitation
// pattern where a disabled handler equals "decline".
func (s *ClientStateStore) Submit(ctx context.Context, sessionID uuid.UUID, callID, name string, args json.RawMessage) FrontendActionResult {
	s.mu.RLock()
	entry, ok := s.sessions[sessionID]
	var handler *FrontendActionHandler
	if ok {
		handler = entry.handler
	}
	s.mu.RUnlock()

	if handler == nil {
		return FrontendActionResult{ID: callID, Status: "error", Error: "no client connected"}
	}
	return handler.Submit(ctx, sessionID, callID, name, args)
}

// FormatAppStateBlock renders the readable snapshot as an XML-tagged block
// suitable for appending to a system prompt. Returns an empty string when no
// readables are present so callers can `prompt += FormatAppStateBlock(...)`
// without conditionals.
//
// Format:
//
//	<app_state>
//	  <readable id="route" description="current route">
//	    "/standalone/abc"
//	  </readable>
//	  <readable id="user" description="logged-in user" parent="auth">
//	    {"name":"Cezar"}
//	  </readable>
//	</app_state>
//
// JSON-encoded values are inlined verbatim (Readable.Value is json.RawMessage).
// Long values are NOT truncated here — the caller controls budget.
func FormatAppStateBlock(readables []Readable) string {
	if len(readables) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n<app_state>\n")
	for _, r := range readables {
		b.WriteString(`  <readable id="`)
		b.WriteString(xmlEscape(r.ID))
		b.WriteString(`" description="`)
		b.WriteString(xmlEscape(r.Description))
		b.WriteString(`"`)
		if r.ParentID != "" {
			b.WriteString(` parent="`)
			b.WriteString(xmlEscape(r.ParentID))
			b.WriteString(`"`)
		}
		b.WriteString(">\n    ")
		if len(r.Value) > 0 {
			b.Write(r.Value)
		} else {
			b.WriteString("null")
		}
		b.WriteString("\n  </readable>\n")
	}
	b.WriteString("</app_state>")
	return b.String()
}

// xmlEscape escapes the five XML metacharacters in attribute values so a
// description like `it's "high"` doesn't break the rendered block.
func xmlEscape(s string) string {
	r := strings.NewReplacer(
		`&`, `&amp;`,
		`<`, `&lt;`,
		`>`, `&gt;`,
		`"`, `&quot;`,
		`'`, `&apos;`,
	)
	return r.Replace(s)
}

// Cleanup evicts sessions whose state hasn't been touched within the TTL.
// Callers should invoke this periodically (e.g. every 30 minutes). When TTL
// is 0 the call is a no-op.
func (s *ClientStateStore) Cleanup() {
	if s.ttl <= 0 {
		return
	}
	cutoff := time.Now().Add(-s.ttl)
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, entry := range s.sessions {
		// Never evict a session with an attached handler — a run is in flight.
		if entry.handler != nil {
			continue
		}
		if entry.state.UpdatedAt.Before(cutoff) {
			delete(s.sessions, id)
		}
	}
}
