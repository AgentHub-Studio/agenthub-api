package settings

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/AgentHub-Studio/agenthub-api/internal/httputil"
)

// Handler holds HTTP handlers for the settings domain.
type Handler struct {
	svc Service
}

// NewHandler creates a new settings Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterProtectedRoutes mounts authenticated settings routes onto r.
func (h *Handler) RegisterProtectedRoutes(r chi.Router) {
	r.Get("/api/settings", h.list)
	r.Get("/api/settings/{key}", h.get)
	r.Put("/api/settings/{key}", h.upsert)
	r.Delete("/api/settings/{key}", h.delete)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	settings, err := h.svc.List(r.Context())
	if err != nil {
		httputil.InternalServerError(w, err.Error())
		return
	}
	httputil.JSON(w, http.StatusOK, settings)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	s, err := h.svc.Get(r.Context(), key)
	if errors.Is(err, ErrNotFound) {
		httputil.NotFound(w, "setting not found")
		return
	}
	if err != nil {
		httputil.InternalServerError(w, err.Error())
		return
	}
	httputil.JSON(w, http.StatusOK, s)
}

func (h *Handler) upsert(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	var req UpdateSettingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	s, err := h.svc.Upsert(r.Context(), key, req)
	if err != nil {
		httputil.BadRequest(w, err.Error())
		return
	}
	httputil.JSON(w, http.StatusOK, s)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	err := h.svc.Delete(r.Context(), key)
	if errors.Is(err, ErrNotFound) {
		httputil.NotFound(w, "setting not found")
		return
	}
	if err != nil {
		httputil.InternalServerError(w, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
