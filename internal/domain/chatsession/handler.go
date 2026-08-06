// Package chatsession provides an opaque session relay used by the Flutter
// chat widget embedded in the Angular frontend.
//
// Flow:
//  1. Angular (test.cezar.dev) POSTs to /api/session with the Keycloak JWT,
//     tenant ID, API base URL, and optional conversation ID.
//  2. The handler stores the payload server-side and returns an opaque {key}.
//  3. The Flutter iframe loads with ?key=<encoded> and GETs /api/session/<key>
//     to retrieve the init data.
//  4. Token refresh can update the stored payload without putting secrets in URLs.
package chatsession

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/AgentHub-Studio/agenthub-api/internal/httputil"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

const defaultSessionTTL = 30 * time.Minute

// sessionPayload holds the data that Angular passes to the Flutter widget.
type sessionPayload struct {
	Token          string  `json:"token"`
	TenantID       string  `json:"tenantId"`
	APIBaseURL     string  `json:"apiBaseUrl"`
	ConversationID *string `json:"conversationId,omitempty"`
}

type storedSession struct {
	payload   sessionPayload
	expiresAt time.Time
}

// Handler handles /api/session endpoints.
type Handler struct {
	mu       sync.RWMutex
	sessions map[string]storedSession
	ttl      time.Duration
	now      func() time.Time
}

// NewHandler creates a new Handler.
func NewHandler() *Handler {
	return &Handler{
		sessions: make(map[string]storedSession),
		ttl:      defaultSessionTTL,
		now:      time.Now,
	}
}

// RegisterRoutes mounts the session endpoints under r.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/api/session", h.create)
	r.Get("/api/session/{key}", h.get)
	r.Put("/api/session/{key}/token", h.refreshToken)
}

// create handles POST /api/session.
// Stores the payload behind an opaque key and returns it.
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var payload sessionPayload
	if err := httputil.DecodeSingleJSON(r.Body, &payload); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if payload.Token == "" || payload.TenantID == "" || payload.APIBaseURL == "" {
		respond.Error(w, http.StatusBadRequest, "token, tenantId and apiBaseUrl are required")
		return
	}

	key, err := newSessionKey()
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to create session")
		return
	}
	h.store(key, payload)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"key": key})
}

// get handles GET /api/session/{key}.
// Returns the payload stored for an opaque key.
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	payload, ok := h.load(key)
	if !ok {
		respond.Error(w, http.StatusNotFound, "session not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

// refreshToken handles PUT /api/session/{key}/token.
// Swaps the token in the stored payload and returns the same opaque key.
func (h *Handler) refreshToken(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	payload, ok := h.load(key)
	if !ok {
		respond.Error(w, http.StatusNotFound, "session not found")
		return
	}

	var req struct {
		Token string `json:"token"`
	}
	if err := httputil.DecodeSingleJSON(r.Body, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Token == "" {
		respond.Error(w, http.StatusBadRequest, "token required")
		return
	}

	payload.Token = req.Token
	h.store(key, payload)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"key": key})
}

func (h *Handler) store(key string, payload sessionPayload) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sessions[key] = storedSession{
		payload:   payload,
		expiresAt: h.now().Add(h.ttl),
	}
}

func (h *Handler) load(key string) (sessionPayload, bool) {
	h.mu.RLock()
	session, ok := h.sessions[key]
	h.mu.RUnlock()
	if !ok {
		return sessionPayload{}, false
	}
	if !h.now().Before(session.expiresAt) {
		h.mu.Lock()
		delete(h.sessions, key)
		h.mu.Unlock()
		return sessionPayload{}, false
	}
	return session.payload, true
}

func newSessionKey() (string, error) {
	buf := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", errors.New("read random bytes")
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
