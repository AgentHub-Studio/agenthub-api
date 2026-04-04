package agentic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

// SessionIngressConfig configures the session log persistence behavior.
//
// Inspired by Claude Code's sessionIngress.ts constants.
type SessionIngressConfig struct {
	// MaxRetries is the maximum number of retry attempts per log entry.
	MaxRetries int `json:"maxRetries"`
	// BaseDelayMs is the initial backoff delay in milliseconds.
	BaseDelayMs int `json:"baseDelayMs"`
	// MaxDelayMs caps the exponential backoff.
	MaxDelayMs int `json:"maxDelayMs"`
	// FlushIntervalMs controls how often buffered entries are flushed.
	FlushIntervalMs int `json:"flushIntervalMs"`
}

// DefaultSessionIngressConfig returns production defaults.
func DefaultSessionIngressConfig() SessionIngressConfig {
	return SessionIngressConfig{
		MaxRetries:      10,
		BaseDelayMs:     500,
		MaxDelayMs:      8000,
		FlushIntervalMs: 100,
	}
}

// RemoteSessionIngressConfig returns config optimized for remote/CCR sessions.
func RemoteSessionIngressConfig() SessionIngressConfig {
	cfg := DefaultSessionIngressConfig()
	cfg.FlushIntervalMs = 10
	return cfg
}

// TranscriptEntry is a single log entry in the session transcript.
//
// Inspired by Claude Code's TranscriptMessage type.
type TranscriptEntry struct {
	UUID             string          `json:"uuid"`
	ParentUUID       *string         `json:"parentUuid"`
	Type             string          `json:"type"` // "user", "assistant", "tool_use", "tool_result", "system"
	Message          json.RawMessage `json:"message"`
	IsSidechain      bool            `json:"isSidechain,omitempty"`
	AgentID          string          `json:"agentId,omitempty"`
	AgentName        string          `json:"agentName,omitempty"`
	Timestamp        int64           `json:"timestamp"`
}

// NewTranscriptEntry creates a new entry with a fresh UUID and current timestamp.
func NewTranscriptEntry(entryType string, message json.RawMessage, parentUUID *string) TranscriptEntry {
	return TranscriptEntry{
		UUID:      uuid.New().String(),
		ParentUUID: parentUUID,
		Type:      entryType,
		Message:   message,
		Timestamp: time.Now().UnixMilli(),
	}
}

// --- Sequential write coordinator ---

// sequentialQueue serializes async operations for a single session.
// Each session gets its own queue to prevent concurrent writes.
//
// Inspired by Claude Code's sequential() wrapper in utils/sequential.ts.
type sequentialQueue struct {
	mu      sync.Mutex
	pending []sequentialItem
	running bool
}

type sequentialItem struct {
	fn   func() error
	done chan error
}

func newSequentialQueue() *sequentialQueue {
	return &sequentialQueue{}
}

func (q *sequentialQueue) enqueue(fn func() error) error {
	item := sequentialItem{
		fn:   fn,
		done: make(chan error, 1),
	}

	q.mu.Lock()
	q.pending = append(q.pending, item)
	shouldStart := !q.running
	if shouldStart {
		q.running = true
	}
	q.mu.Unlock()

	if shouldStart {
		go q.process()
	}

	return <-item.done
}

func (q *sequentialQueue) process() {
	for {
		q.mu.Lock()
		if len(q.pending) == 0 {
			q.running = false
			q.mu.Unlock()
			return
		}
		item := q.pending[0]
		q.pending = q.pending[1:]
		q.mu.Unlock()

		err := item.fn()
		item.done <- err
	}
}

// --- Session Ingress ---

// SessionIngress manages per-session sequential log persistence with
// UUID-based deduplication, 409 conflict resolution, and retry with backoff.
//
// Inspired by Claude Code's sessionIngress.ts.
type SessionIngress struct {
	config  SessionIngressConfig
	client  *http.Client
	mu      sync.Mutex
	queues  map[string]*sequentialQueue
	lastUUID map[string]string // sessionID → last successful UUID
}

// NewSessionIngress creates a new ingress manager.
func NewSessionIngress(config SessionIngressConfig) *SessionIngress {
	return &SessionIngress{
		config:   config,
		client:   &http.Client{Timeout: 30 * time.Second},
		queues:   make(map[string]*sequentialQueue),
		lastUUID: make(map[string]string),
	}
}

// getQueue returns the sequential queue for a session, creating if needed.
func (si *SessionIngress) getQueue(sessionID string) *sequentialQueue {
	si.mu.Lock()
	defer si.mu.Unlock()
	q, ok := si.queues[sessionID]
	if !ok {
		q = newSequentialQueue()
		si.queues[sessionID] = q
	}
	return q
}

// GetLastUUID returns the last successful UUID for a session (for testing/debugging).
func (si *SessionIngress) GetLastUUID(sessionID string) string {
	si.mu.Lock()
	defer si.mu.Unlock()
	return si.lastUUID[sessionID]
}

// setLastUUID updates the last successful UUID for a session.
func (si *SessionIngress) setLastUUID(sessionID, entryUUID string) {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.lastUUID[sessionID] = entryUUID
}

// getLastUUIDInternal reads without exposing lock semantics.
func (si *SessionIngress) getLastUUIDInternal(sessionID string) string {
	si.mu.Lock()
	defer si.mu.Unlock()
	return si.lastUUID[sessionID]
}

// AppendLog queues a transcript entry for sequential persistence.
// Returns true if the entry was successfully persisted remotely.
func (si *SessionIngress) AppendLog(ctx context.Context, sessionID string, entry TranscriptEntry, url string, headers map[string]string) (bool, error) {
	q := si.getQueue(sessionID)
	var success bool
	err := q.enqueue(func() error {
		var appendErr error
		success, appendErr = si.appendImpl(ctx, sessionID, entry, url, headers)
		return appendErr
	})
	return success, err
}

// appendImpl is the actual append logic with retry and conflict resolution.
func (si *SessionIngress) appendImpl(ctx context.Context, sessionID string, entry TranscriptEntry, url string, headers map[string]string) (bool, error) {
	body, err := json.Marshal(entry)
	if err != nil {
		return false, fmt.Errorf("session ingress: marshal entry: %w", err)
	}

	for attempt := 1; attempt <= si.config.MaxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
		}

		reqHeaders := make(map[string]string, len(headers)+2)
		for k, v := range headers {
			reqHeaders[k] = v
		}
		reqHeaders["Content-Type"] = "application/json"

		lastUUID := si.getLastUUIDInternal(sessionID)
		if lastUUID != "" {
			reqHeaders["Last-Uuid"] = lastUUID
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
		if err != nil {
			return false, fmt.Errorf("session ingress: create request: %w", err)
		}
		for k, v := range reqHeaders {
			req.Header.Set(k, v)
		}

		resp, err := si.client.Do(req)
		if err != nil {
			// Network error — retryable.
			slog.Debug("session ingress: network error", "attempt", attempt, "error", err)
			si.backoff(ctx, attempt)
			continue
		}

		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 10_000))
		_ = resp.Body.Close()

		switch {
		case resp.StatusCode == 200 || resp.StatusCode == 201:
			si.setLastUUID(sessionID, entry.UUID)
			return true, nil

		case resp.StatusCode == 409:
			resolved := si.handle409(sessionID, entry.UUID, resp, url, headers)
			if resolved {
				return true, nil
			}
			// Conflict resolved by adopting server state — retry.
			continue

		case resp.StatusCode == 401:
			// Auth failure — non-retryable.
			return false, fmt.Errorf("session ingress: auth failed (401): %s", string(respBody))

		case resp.StatusCode == 429 || resp.StatusCode >= 500:
			// Retryable server error.
			slog.Debug("session ingress: server error", "status", resp.StatusCode, "attempt", attempt)
			si.backoff(ctx, attempt)
			continue

		default:
			slog.Warn("session ingress: unexpected status", "status", resp.StatusCode, "body", string(respBody))
			si.backoff(ctx, attempt)
			continue
		}
	}

	slog.Error("session ingress: retries exhausted", "sessionID", sessionID, "entryUUID", entry.UUID)
	return false, nil
}

// handle409 implements 409 conflict resolution.
// Returns true if the entry was already present on the server (idempotent).
func (si *SessionIngress) handle409(sessionID, entryUUID string, resp *http.Response, url string, headers map[string]string) bool {
	serverLastUUID := resp.Header.Get("X-Last-Uuid")

	// Idempotency check: our entry is already the server's last entry.
	if serverLastUUID == entryUUID {
		si.setLastUUID(sessionID, entryUUID)
		slog.Debug("session ingress: 409 idempotent recovery", "sessionID", sessionID, "uuid", entryUUID)
		return true
	}

	// Another writer advanced the chain — adopt server's UUID and retry.
	if serverLastUUID != "" {
		si.setLastUUID(sessionID, serverLastUUID)
		slog.Debug("session ingress: 409 adopting server UUID", "sessionID", sessionID, "serverUUID", serverLastUUID)
	}

	return false
}

// backoff sleeps with exponential backoff, respecting context cancellation.
func (si *SessionIngress) backoff(ctx context.Context, attempt int) {
	delay := float64(si.config.BaseDelayMs) * math.Pow(2, float64(attempt-1))
	if delay > float64(si.config.MaxDelayMs) {
		delay = float64(si.config.MaxDelayMs)
	}

	timer := time.NewTimer(time.Duration(delay) * time.Millisecond)
	defer timer.Stop()

	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

// HydrateSession fetches all log entries for a session and updates the lastUUID tracker.
//
// Inspired by Claude Code's getSessionLogs.
func (si *SessionIngress) HydrateSession(ctx context.Context, sessionID string, url string, headers map[string]string) ([]TranscriptEntry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("session ingress: hydrate request: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := si.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("session ingress: hydrate fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 10_000))
		return nil, fmt.Errorf("session ingress: hydrate status %d: %s", resp.StatusCode, string(body))
	}

	var entries []TranscriptEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("session ingress: hydrate decode: %w", err)
	}

	// Update lastUUID from the latest entry.
	if len(entries) > 0 {
		last := entries[len(entries)-1]
		if last.UUID != "" {
			si.setLastUUID(sessionID, last.UUID)
		}
	}

	return entries, nil
}

// CleanupSession removes tracking state for a session.
func (si *SessionIngress) CleanupSession(sessionID string) {
	si.mu.Lock()
	defer si.mu.Unlock()
	delete(si.queues, sessionID)
	delete(si.lastUUID, sessionID)
}

// ActiveSessions returns the number of sessions currently tracked.
func (si *SessionIngress) ActiveSessions() int {
	si.mu.Lock()
	defer si.mu.Unlock()
	return len(si.queues)
}
