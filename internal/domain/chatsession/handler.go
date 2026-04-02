// Package chatsession provides a stateless session relay used by the Flutter
// chat widget embedded in the Angular frontend.
//
// Instead of storing state server-side, the session payload is JSON-encoded
// and base64url-encoded into the key itself. This keeps the design
// pod-count agnostic — any replica can serve any request.
//
// Flow:
//  1. Angular (test.cezar.dev) POSTs to /api/session with the Keycloak JWT,
//     tenant ID, API base URL, and optional conversation ID.
//  2. The handler base64url-encodes the payload and returns it as {key}.
//  3. The Flutter iframe loads with ?key=<encoded> and GETs /api/session/<key>
//     to retrieve the init data.
//  4. Every 60 s Angular PUTs /api/session/<key>/token with a refreshed token;
//     the handler decodes the key, swaps the token, re-encodes and returns
//     the new key so the caller can update the iframe URL if needed.
package chatsession

import (
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// sessionPayload holds the data that Angular passes to the Flutter widget.
type sessionPayload struct {
	Token          string  `json:"token"`
	TenantID       string  `json:"tenantId"`
	APIBaseURL     string  `json:"apiBaseUrl"`
	ConversationID *string `json:"conversationId,omitempty"`
}

// Handler handles /api/session endpoints.
type Handler struct{}

// NewHandler creates a new Handler.
func NewHandler() *Handler { return &Handler{} }

// RegisterRoutes mounts the session endpoints under r.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/api/session", h.create)
	r.Get("/api/session/{key}", h.get)
	r.Put("/api/session/{key}/token", h.refreshToken)
}

// create handles POST /api/session.
// Encodes the payload as a base64url key and returns it.
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var payload sessionPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	key, err := encode(payload)
	if err != nil {
		http.Error(w, "failed to encode session", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"key": key})
}

// get handles GET /api/session/{key}.
// Decodes the base64url key and returns the payload.
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	payload, err := decode(key)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

// refreshToken handles PUT /api/session/{key}/token.
// Decodes the key, swaps the token, re-encodes and returns the new key.
func (h *Handler) refreshToken(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	payload, err := decode(key)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	payload.Token = req.Token
	newKey, err := encode(payload)
	if err != nil {
		http.Error(w, "failed to encode session", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"key": newKey})
}

func encode(p sessionPayload) (string, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func decode(key string) (sessionPayload, error) {
	data, err := base64.RawURLEncoding.DecodeString(key)
	if err != nil {
		return sessionPayload{}, err
	}
	var p sessionPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return sessionPayload{}, err
	}
	return p, nil
}
