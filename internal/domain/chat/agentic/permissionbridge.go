package agentic

import (
	"context"
	"sync"
	"time"
)

// Hierarchical permission delegation across isolation boundaries.
//
// Inspired by Claude Code's inProcessRunner + leaderPermissionBridge —
// worker agents delegate permission decisions to a leader through a
// bidirectional request/response queue. The leader resolves via UI or
// policy; the worker blocks until resolved or aborted. Supports
// permission wait-time tracking for subtracting from displayed elapsed.

// BridgePermBehavior is the resolved decision from the leader.
type BridgePermBehavior string

const (
	BridgePermAllow BridgePermBehavior = "allow"
	BridgePermDeny  BridgePermBehavior = "deny"
	BridgePermAsk   BridgePermBehavior = "ask"
)

// PermissionBridgeRequest describes a worker's permission request.
type PermissionBridgeRequest struct {
	ID          string
	WorkerName  string
	ToolName    string
	Input       interface{}
	Description string
	CreatedAt   time.Time
}

// PermissionBridgeDecision is the leader's decision on a request.
type PermissionBridgeDecision struct {
	Behavior     BridgePermBehavior
	UpdatedInput interface{}
	Feedback     string
	Reason       string
}

type pendingPermission struct {
	request  PermissionBridgeRequest
	resultCh chan PermissionBridgeDecision
}

// PermissionBridge handles permission delegation from workers to a leader.
type PermissionBridge struct {
	mu      sync.Mutex
	pending map[string]*pendingPermission
	onRequest func(PermissionBridgeRequest) // notification when new request arrives
}

// NewPermissionBridge creates a permission bridge.
func NewPermissionBridge() *PermissionBridge {
	return &PermissionBridge{
		pending: make(map[string]*pendingPermission),
	}
}

// OnRequest sets a callback invoked when a new permission request is queued.
func (b *PermissionBridge) OnRequest(fn func(PermissionBridgeRequest)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onRequest = fn
}

// Request submits a permission request and blocks until resolved or
// the context is cancelled. Returns the decision. Tracks wait time
// via the optional onWaitMs callback.
func (b *PermissionBridge) Request(ctx context.Context, req PermissionBridgeRequest, onWaitMs func(int64)) PermissionBridgeDecision {
	startMs := time.Now()
	pp := &pendingPermission{
		request:  req,
		resultCh: make(chan PermissionBridgeDecision, 1),
	}

	b.mu.Lock()
	b.pending[req.ID] = pp
	notify := b.onRequest
	b.mu.Unlock()

	if notify != nil {
		notify(req)
	}

	defer func() {
		if onWaitMs != nil {
			onWaitMs(time.Since(startMs).Milliseconds())
		}
		b.mu.Lock()
		delete(b.pending, req.ID)
		b.mu.Unlock()
	}()

	select {
	case decision := <-pp.resultCh:
		return decision
	case <-ctx.Done():
		return PermissionBridgeDecision{
			Behavior: BridgePermDeny,
			Reason:   "context cancelled",
		}
	}
}

// Resolve provides the leader's decision for a pending request.
// Returns true if the request was found.
func (b *PermissionBridge) Resolve(requestID string, decision PermissionBridgeDecision) bool {
	b.mu.Lock()
	pp, ok := b.pending[requestID]
	if ok {
		delete(b.pending, requestID)
	}
	b.mu.Unlock()

	if !ok {
		return false
	}
	pp.resultCh <- decision
	return true
}

// PendingRequests returns a snapshot of all pending requests.
func (b *PermissionBridge) PendingRequests() []PermissionBridgeRequest {
	b.mu.Lock()
	defer b.mu.Unlock()

	result := make([]PermissionBridgeRequest, 0, len(b.pending))
	for _, pp := range b.pending {
		result = append(result, pp.request)
	}
	return result
}

// PendingCount returns the number of unresolved requests.
func (b *PermissionBridge) PendingCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.pending)
}

// ResolveAll resolves all pending requests with the same decision.
// Returns the number resolved.
func (b *PermissionBridge) ResolveAll(decision PermissionBridgeDecision) int {
	b.mu.Lock()
	pending := make(map[string]*pendingPermission, len(b.pending))
	for k, v := range b.pending {
		pending[k] = v
	}
	b.pending = make(map[string]*pendingPermission)
	b.mu.Unlock()

	for _, pp := range pending {
		pp.resultCh <- decision
	}
	return len(pending)
}
