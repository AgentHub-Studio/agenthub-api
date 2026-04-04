package agentic

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

// Request-response queue for structured user interactions.
//
// Inspired by Claude Code's elicitationHandler — manages asynchronous
// user-facing prompts (confirmations, form inputs, URL approvals) over
// MCP or any channel. Supports hook-based pre/post processing,
// context-based cancellation, and completion notifications.

// ElicitationMode is the kind of interaction requested.
type ElicitationMode string

const (
	ElicitationModeForm ElicitationMode = "form"
	ElicitationModeURL  ElicitationMode = "url"
)

// ElicitationParams describes what the server is asking the user.
type ElicitationParams struct {
	Mode            ElicitationMode `json:"mode"`
	Message         string          `json:"message"`
	RequestedSchema json.RawMessage `json:"requestedSchema,omitempty"`
	URL             string          `json:"url,omitempty"`
	ElicitationID   string          `json:"elicitationId,omitempty"`
}

// ElicitationAction is the user's decision.
type ElicitationAction string

const (
	ElicitationAccept  ElicitationAction = "accept"
	ElicitationDecline ElicitationAction = "decline"
	ElicitationCancel  ElicitationAction = "cancel"
)

// ElicitationResult is the user's response.
type ElicitationResult struct {
	Action  ElicitationAction      `json:"action"`
	Content map[string]interface{} `json:"content,omitempty"`
}

// ElicitationRequest is a pending request in the queue.
type ElicitationRequest struct {
	ServerName    string
	RequestID     string
	Params        ElicitationParams
	Ctx           context.Context
	respondCh     chan ElicitationResult
	responded     bool
	Completed     bool // set by completion notification
	CreatedAt     time.Time
}

// ElicitationHookFunc can intercept a request before it reaches the user.
// Return a non-nil result to resolve programmatically, or nil to pass through.
type ElicitationHookFunc func(serverName string, params ElicitationParams) *ElicitationResult

// ElicitationResultHookFunc can modify the user's response before it's sent back.
type ElicitationResultHookFunc func(serverName string, result ElicitationResult) ElicitationResult

// ElicitationHandler manages a queue of pending elicitation requests.
type ElicitationHandler struct {
	mu          sync.Mutex
	queue       []*ElicitationRequest
	preHooks    []ElicitationHookFunc
	postHooks   []ElicitationResultHookFunc
	onEnqueue   func(*ElicitationRequest) // notification callback
}

// NewElicitationHandler creates a handler.
func NewElicitationHandler() *ElicitationHandler {
	return &ElicitationHandler{}
}

// OnEnqueue sets a callback invoked when a new request is queued.
func (h *ElicitationHandler) OnEnqueue(fn func(*ElicitationRequest)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onEnqueue = fn
}

// AddPreHook registers a hook that can resolve requests programmatically.
func (h *ElicitationHandler) AddPreHook(fn ElicitationHookFunc) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.preHooks = append(h.preHooks, fn)
}

// AddPostHook registers a hook that can modify responses.
func (h *ElicitationHandler) AddPostHook(fn ElicitationResultHookFunc) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.postHooks = append(h.postHooks, fn)
}

// Submit creates a new elicitation request and blocks until it is
// responded to or the context is cancelled. Returns the result.
func (h *ElicitationHandler) Submit(ctx context.Context, serverName, requestID string, params ElicitationParams) ElicitationResult {
	// Run pre-hooks — may resolve without queuing
	h.mu.Lock()
	hooks := make([]ElicitationHookFunc, len(h.preHooks))
	copy(hooks, h.preHooks)
	h.mu.Unlock()

	for _, hook := range hooks {
		if result := hook(serverName, params); result != nil {
			return h.applyPostHooks(serverName, *result)
		}
	}

	req := &ElicitationRequest{
		ServerName: serverName,
		RequestID:  requestID,
		Params:     params,
		Ctx:        ctx,
		respondCh:  make(chan ElicitationResult, 1),
		CreatedAt:  time.Now(),
	}

	h.mu.Lock()
	h.queue = append(h.queue, req)
	notify := h.onEnqueue
	h.mu.Unlock()

	if notify != nil {
		notify(req)
	}

	// Wait for response or cancellation
	select {
	case result := <-req.respondCh:
		return h.applyPostHooks(serverName, result)
	case <-ctx.Done():
		h.removeFromQueue(requestID)
		return ElicitationResult{Action: ElicitationCancel}
	}
}

// Respond resolves a pending request by its request ID.
// Returns true if the request was found and resolved.
func (h *ElicitationHandler) Respond(requestID string, result ElicitationResult) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, req := range h.queue {
		if req.RequestID == requestID && !req.responded {
			req.responded = true
			req.respondCh <- result
			return true
		}
	}
	return false
}

// MarkCompleted sets the Completed flag on a request (e.g., URL confirmation).
// Returns true if the request was found.
func (h *ElicitationHandler) MarkCompleted(elicitationID string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, req := range h.queue {
		if req.Params.ElicitationID == elicitationID {
			req.Completed = true
			return true
		}
	}
	return false
}

// Pending returns a snapshot of all pending (unresponded) requests.
func (h *ElicitationHandler) Pending() []*ElicitationRequest {
	h.mu.Lock()
	defer h.mu.Unlock()

	var result []*ElicitationRequest
	for _, req := range h.queue {
		if !req.responded {
			result = append(result, req)
		}
	}
	return result
}

// PendingCount returns the number of unresponded requests.
func (h *ElicitationHandler) PendingCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	count := 0
	for _, req := range h.queue {
		if !req.responded {
			count++
		}
	}
	return count
}

// Clear removes all responded requests from the queue.
func (h *ElicitationHandler) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()
	var kept []*ElicitationRequest
	for _, req := range h.queue {
		if !req.responded {
			kept = append(kept, req)
		}
	}
	h.queue = kept
}

func (h *ElicitationHandler) removeFromQueue(requestID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i, req := range h.queue {
		if req.RequestID == requestID {
			h.queue = append(h.queue[:i], h.queue[i+1:]...)
			return
		}
	}
}

func (h *ElicitationHandler) applyPostHooks(serverName string, result ElicitationResult) ElicitationResult {
	h.mu.Lock()
	hooks := make([]ElicitationResultHookFunc, len(h.postHooks))
	copy(hooks, h.postHooks)
	h.mu.Unlock()

	for _, hook := range hooks {
		result = hook(serverName, result)
	}
	return result
}
