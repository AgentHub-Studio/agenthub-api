package channel

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// Handler exposes channel management and inbound relay endpoints.
type Handler struct {
	svc Service
}

// NewHandler creates a new Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts protected CRUD endpoints under /api/channels.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/channels", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.create)
		r.Get("/{id}", h.getByID)
		r.Put("/{id}", h.update)
		r.Patch("/{id}", h.update)
		r.Delete("/{id}", h.delete)
	})
}

// RegisterPublicRoutes mounts the unauthenticated inbound relay endpoint.
// Placed under /api/channels/inbound/{token} so it is accessible without JWT.
func (h *Handler) RegisterPublicRoutes(r chi.Router) {
	r.Post("/api/channels/inbound/{token}", h.inbound)
}

// list handles GET /api/channels.
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	req := pagination.ParsePageRequest(r)
	page, err := h.svc.List(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, page)
}

// create handles POST /api/channels.
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.svc.Create(r.Context(), req)
	if err != nil {
		if errors.Is(err, ErrSlugConflict) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		// ErrValidation: name/type vazios → 422 (não 500). Padroniza
		// com agent/oauth/datasource/skill/vpn/llmpreset.
		if errors.Is(err, ErrValidation) {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

// getByID handles GET /api/channels/{id}.
func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid channel id")
		return
	}
	resp, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "channel not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// update handles PUT /api/channels/{id}.
func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid channel id")
		return
	}
	var req UpdateChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.svc.Update(r.Context(), id, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "channel not found")
			return
		}
		// Bug 112: ErrValidation no Update precisa do mesmo mapeamento
		// 422 que Create já fazia em linha 69.
		if errors.Is(err, ErrValidation) {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// delete handles DELETE /api/channels/{id}.
func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid channel id")
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "channel not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// inbound handles POST /api/channels/inbound/{token}.
// This is the public webhook receiver — no JWT required.
// The token in the URL path is the channel's inbound authentication secret.
func (h *Handler) inbound(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	// Collect request headers for adapter signature verification.
	headers := make(map[string]string, len(r.Header))
	for k, vs := range r.Header {
		if len(vs) > 0 {
			headers[k] = vs[0]
		}
	}

	msg, err := h.svc.HandleInbound(r.Context(), token, headers, body)
	if err != nil {
		if errors.Is(err, ErrTokenNotFound) {
			writeError(w, http.StatusNotFound, "channel not found")
			return
		}
		// Signature / verification failure → 401.
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	// Platform-specific handshake responses.
	if msg.Challenge != "" {
		// Discord PING (type 1) — must respond with {"type":1}.
		if msg.Challenge == "pong" {
			writeJSON(w, http.StatusOK, map[string]int{"type": 1})
			return
		}
		// Slack URL-verification — echo the challenge token back.
		writeJSON(w, http.StatusOK, map[string]string{"challenge": msg.Challenge})
		return
	}

	w.WriteHeader(http.StatusOK)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
