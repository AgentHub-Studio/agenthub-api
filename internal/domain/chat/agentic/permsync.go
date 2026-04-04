package agentic

import (
	"fmt"
	"sync"
	"time"
)

// Swarm permission synchronization.
//
// Inspired by Claude Code's permissionSync.ts — coordinates permission
// requests between worker agents and a team leader. Workers request
// permission for tool execution; the leader approves or rejects.
// Uses an in-memory model (no file I/O) suitable for server-side usage.

// PermissionStatus represents the state of a permission request.
type PermissionStatus string

const (
	PermissionPending  PermissionStatus = "pending"
	PermissionApproved PermissionStatus = "approved"
	PermissionRejected PermissionStatus = "rejected"
)

// PermissionResolvedBy indicates who resolved the request.
type PermissionResolvedBy string

const (
	ResolvedByWorker PermissionResolvedBy = "worker"
	ResolvedByLeader PermissionResolvedBy = "leader"
)

// DefaultPermissionMaxAge is the default TTL for resolved permissions (1 hour).
const DefaultPermissionMaxAge = time.Hour

// PermissionRequest represents a tool permission request from a worker.
type PermissionRequest struct {
	// ID uniquely identifies this request.
	ID string `json:"id"`
	// WorkerID identifies the requesting worker agent.
	WorkerID string `json:"workerId"`
	// WorkerName is the human-readable worker name.
	WorkerName string `json:"workerName"`
	// TeamName is the team for routing.
	TeamName string `json:"teamName,omitempty"`
	// ToolName is the tool requiring permission (e.g., "Bash").
	ToolName string `json:"toolName"`
	// Description is a human-readable explanation.
	Description string `json:"description"`
	// Input holds the serialized tool input.
	Input map[string]interface{} `json:"input,omitempty"`
	// Status is the current request state.
	Status PermissionStatus `json:"status"`
	// ResolvedBy indicates who resolved it.
	ResolvedBy PermissionResolvedBy `json:"resolvedBy,omitempty"`
	// ResolvedAt is when the request was resolved.
	ResolvedAt *time.Time `json:"resolvedAt,omitempty"`
	// Feedback is optional rejection feedback.
	Feedback string `json:"feedback,omitempty"`
	// UpdatedInput is the input modified by the resolver.
	UpdatedInput map[string]interface{} `json:"updatedInput,omitempty"`
	// CreatedAt is when the request was created.
	CreatedAt time.Time `json:"createdAt"`
}

// PermissionResolution holds the leader's decision.
type PermissionResolution struct {
	// Decision is approved or rejected.
	Decision PermissionStatus `json:"decision"`
	// ResolvedBy indicates the resolver role.
	ResolvedBy PermissionResolvedBy `json:"resolvedBy"`
	// Feedback is optional rejection feedback.
	Feedback string `json:"feedback,omitempty"`
	// UpdatedInput is the modified input (if any).
	UpdatedInput map[string]interface{} `json:"updatedInput,omitempty"`
}

// PermissionRegistry manages permission requests in memory.
type PermissionRegistry struct {
	mu       sync.Mutex
	pending  map[string]*PermissionRequest
	resolved map[string]*PermissionRequest
	counter  int
	maxAge   time.Duration
}

// NewPermissionRegistry creates a permission registry with default settings.
func NewPermissionRegistry() *PermissionRegistry {
	return &PermissionRegistry{
		pending:  make(map[string]*PermissionRequest),
		resolved: make(map[string]*PermissionRequest),
		maxAge:   DefaultPermissionMaxAge,
	}
}

// NewPermissionRegistryWithMaxAge creates a registry with a custom max age.
func NewPermissionRegistryWithMaxAge(maxAge time.Duration) *PermissionRegistry {
	r := NewPermissionRegistry()
	r.maxAge = maxAge
	return r
}

// GenerateRequestID creates a unique permission request ID.
func (r *PermissionRegistry) GenerateRequestID() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.counter++
	return fmt.Sprintf("perm-%d-%d", time.Now().UnixMilli(), r.counter)
}

// Submit adds a new permission request. Returns the request with ID and timestamp filled.
func (r *PermissionRegistry) Submit(req PermissionRequest) PermissionRequest {
	r.mu.Lock()
	defer r.mu.Unlock()

	if req.ID == "" {
		r.counter++
		req.ID = fmt.Sprintf("perm-%d-%d", time.Now().UnixMilli(), r.counter)
	}
	if req.CreatedAt.IsZero() {
		req.CreatedAt = time.Now()
	}
	req.Status = PermissionPending

	r.pending[req.ID] = &req
	return req
}

// Resolve moves a pending request to resolved with the given resolution.
// Returns true if the request was found and resolved.
func (r *PermissionRegistry) Resolve(requestID string, resolution PermissionResolution) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	req, ok := r.pending[requestID]
	if !ok {
		return false
	}

	now := time.Now()
	req.Status = resolution.Decision
	req.ResolvedBy = resolution.ResolvedBy
	req.ResolvedAt = &now
	req.Feedback = resolution.Feedback
	req.UpdatedInput = resolution.UpdatedInput

	delete(r.pending, requestID)
	r.resolved[requestID] = req
	return true
}

// GetPending returns all pending requests sorted by creation time (oldest first).
func (r *PermissionRegistry) GetPending() []PermissionRequest {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := make([]PermissionRequest, 0, len(r.pending))
	for _, req := range r.pending {
		result = append(result, *req)
	}

	// Sort by CreatedAt ascending.
	for i := 1; i < len(result); i++ {
		for j := i; j > 0 && result[j].CreatedAt.Before(result[j-1].CreatedAt); j-- {
			result[j], result[j-1] = result[j-1], result[j]
		}
	}
	return result
}

// GetPendingByTeam returns pending requests for a specific team.
func (r *PermissionRegistry) GetPendingByTeam(teamName string) []PermissionRequest {
	all := r.GetPending()
	var result []PermissionRequest
	for _, req := range all {
		if req.TeamName == teamName {
			result = append(result, req)
		}
	}
	return result
}

// GetResolved returns a resolved request by ID, or nil if not found.
func (r *PermissionRegistry) GetResolved(requestID string) *PermissionRequest {
	r.mu.Lock()
	defer r.mu.Unlock()

	req, ok := r.resolved[requestID]
	if !ok {
		return nil
	}
	copy := *req
	return &copy
}

// DeleteResolved removes a resolved request. Returns true if found.
func (r *PermissionRegistry) DeleteResolved(requestID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, ok := r.resolved[requestID]
	if ok {
		delete(r.resolved, requestID)
	}
	return ok
}

// PendingCount returns the number of pending requests.
func (r *PermissionRegistry) PendingCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.pending)
}

// ResolvedCount returns the number of resolved requests.
func (r *PermissionRegistry) ResolvedCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.resolved)
}

// CleanupOldResolutions removes resolved requests older than maxAge.
// Returns the number of cleaned entries.
func (r *PermissionRegistry) CleanupOldResolutions() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	cleaned := 0
	for id, req := range r.resolved {
		age := now.Sub(req.CreatedAt)
		if req.ResolvedAt != nil {
			age = now.Sub(*req.ResolvedAt)
		}
		if age > r.maxAge {
			delete(r.resolved, id)
			cleaned++
		}
	}
	return cleaned
}

// Clear removes all pending and resolved requests.
func (r *PermissionRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.pending = make(map[string]*PermissionRequest)
	r.resolved = make(map[string]*PermissionRequest)
}
