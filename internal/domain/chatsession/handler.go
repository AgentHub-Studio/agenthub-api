// Package chatsession provides an in-memory session relay used by the Flutter
// chat widget embedded in the Angular frontend.
//
// Flow:
//  1. Angular (test.cezar.dev) POSTs to /api/session with the Keycloak JWT,
//     tenant ID, API base URL, and optional conversation ID.
//  2. The handler stores the payload keyed by a random UUID and returns {key}.
//  3. The Flutter iframe loads with ?key=<uuid> and GETs /api/session/<key>
//     to retrieve the init data.
//  4. Every 60 s Angular PUTs /api/session/<key>/token to refresh the token
//     stored in the session.
//
// Sessions expire after 15 minutes of inactivity and are cleaned up lazily.
package chatsession

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const sessionTTL = 15 * time.Minute

// sessionEntry holds the payload for a single chat-widget session.
type sessionEntry struct {
	Token          string  `json:"token"`
	TenantID       string  `json:"tenantId"`
	APIBaseURL     string  `json:"apiBaseUrl"`
	ConversationID *string `json:"conversationId,omitempty"`
	expiresAt      time.Time
}

// Handler handles /api/session endpoints.
type Handler struct {
	mu       sync.Mutex
	sessions map[string]*sessionEntry
}

// NewHandler creates a new Handler with background cleanup.
func NewHandler() *Handler {
	h := &Handler{sessions: make(map[string]*sessionEntry)}
	go h.cleanup()
	return h
}

// RegisterRoutes mounts the session endpoints under r.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/api/session", h.create)
	r.Get("/api/session/{key}", h.get)
	r.Put("/api/session/{key}/token", h.refreshToken)
}

// create handles POST /api/session.
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token          string  `json:"token"`
		TenantID       string  `json:"tenantId"`
		APIBaseURL     string  `json:"apiBaseUrl"`
		ConversationID *string `json:"conversationId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	key := uuid.New().String()
	entry := &sessionEntry{
		Token:          req.Token,
		TenantID:       req.TenantID,
		APIBaseURL:     req.APIBaseURL,
		ConversationID: req.ConversationID,
		expiresAt:      time.Now().Add(sessionTTL),
	}

	h.mu.Lock()
	h.sessions[key] = entry
	h.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"key": key})
}

// get handles GET /api/session/{key}.
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")

	h.mu.Lock()
	entry, ok := h.sessions[key]
	if ok {
		entry.expiresAt = time.Now().Add(sessionTTL) // refresh TTL on access
	}
	h.mu.Unlock()

	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(entry)
}

// refreshToken handles PUT /api/session/{key}/token.
func (h *Handler) refreshToken(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")

	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	h.mu.Lock()
	entry, ok := h.sessions[key]
	if ok {
		entry.Token = req.Token
		entry.expiresAt = time.Now().Add(sessionTTL)
	}
	h.mu.Unlock()

	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// cleanup removes expired sessions every minute.
func (h *Handler) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		h.mu.Lock()
		for key, entry := range h.sessions {
			if now.After(entry.expiresAt) {
				delete(h.sessions, key)
			}
		}
		h.mu.Unlock()
	}
}
