package abtest

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/httputil"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// Handler exposes A/B test management under /api/agents/{agentId}/ab-tests.
type Handler struct {
	svc Service
}

// NewHandler creates a new Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts the A/B test endpoints under r.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireRole("admin"))
		// The canonical API path does not require a trailing slash. Keep the
		// legacy trailing-slash routes below for existing clients.
		r.Get("/api/agents/{agentId}/ab-tests", h.list)
		r.Post("/api/agents/{agentId}/ab-tests", h.create)
		r.Route("/api/agents/{agentId}/ab-tests", func(r chi.Router) {
			r.Get("/", h.list)
			r.Post("/", h.create)
			r.Get("/{id}", h.getByID)
			r.Put("/{id}", h.update)
			r.Patch("/{id}", h.update)
			r.Delete("/{id}", h.delete)
		})
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	agentID, err := uuid.Parse(chi.URLParam(r, "agentId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid agent id")
		return
	}
	req := pagination.ParsePageRequest(r)
	page, err := h.svc.List(r.Context(), agentID, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	agentID, ok := parseAgentID(w, r)
	if !ok {
		return
	}
	var req CreateABTestRequest
	if err := decodeSingleJSON(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.AgentID = agentID
	resp, err := h.svc.Create(r.Context(), req)
	if err != nil {
		if errors.Is(err, ErrNameConflict) || errors.Is(err, ErrActiveTestConflict) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, ErrValidation) {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		// Bug 263: fallback only sees repo SQL errors after sentinels.
		slog.Error("abtest: create failed", "agentID", agentID, "err", err)
		writeError(w, http.StatusInternalServerError, "create failed")
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	agentID, ok := parseAgentID(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid ab test id")
		return
	}
	resp, err := h.getByIDForAgent(r, agentID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "ab test not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	agentID, ok := parseAgentID(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid ab test id")
		return
	}
	var req UpdateABTestRequest
	if err := decodeSingleJSON(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if _, err := h.getByIDForAgent(r, agentID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "ab test not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	resp, err := h.svc.Update(r.Context(), id, req)
	if err != nil {
		if errors.Is(err, ErrActiveTestConflict) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "ab test not found")
			return
		}
		if errors.Is(err, ErrValidation) {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		// Bug 263: fallback only sees repo SQL errors after sentinels.
		slog.Error("abtest: update failed", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, "update failed")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	agentID, ok := parseAgentID(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid ab test id")
		return
	}
	if _, err := h.getByIDForAgent(r, agentID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "ab test not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "ab test not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseAgentID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	agentID, err := uuid.Parse(chi.URLParam(r, "agentId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid agent id")
		return uuid.UUID{}, false
	}
	return agentID, true
}

func (h *Handler) getByIDForAgent(r *http.Request, agentID, id uuid.UUID) (ABTestResponse, error) {
	resp, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		return ABTestResponse{}, err
	}
	if resp.AgentID != agentID {
		return ABTestResponse{}, ErrNotFound
	}
	return resp, nil
}

func decodeSingleJSON(body io.Reader, dst any) error {
	return httputil.DecodeSingleJSON(body, dst)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
