package device

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/httputil"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// Handler exposes Device Node Network endpoints.
type Handler struct {
	svc   Service
	agent agentExister
}

// agentExister verifies that an agent parent exists before nested device routes.
type agentExister interface {
	GetByID(ctx context.Context, id uuid.UUID) error
}

// NewHandler creates a Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// WithAgentExister wires the parent-agent existence checker for nested routes.
func (h *Handler) WithAgentExister(agent agentExister) *Handler {
	h.agent = agent
	return h
}

// RegisterRoutes mounts device endpoints under r.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/devices", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole("admin"))
			r.Get("/", h.list)
			r.Post("/", h.create)
			r.Get("/{id}", h.getByID)
			r.Put("/{id}", h.update)
			r.Patch("/{id}", h.update)
			r.Delete("/{id}", h.delete)
		})
		// Heartbeat endpoint — called by MCP client runtime.
		r.With(middleware.RequireRole("mcp-client-runtime")).Post("/{id}/heartbeat", h.heartbeat)
	})

	// Agent–device bindings nested under agents.
	r.Route("/api/agents/{agentId}/devices", func(r chi.Router) {
		r.Use(middleware.RequireRole("admin"))
		r.Get("/", h.listByAgent)
		r.Post("/{deviceId}", h.attach)
		r.Delete("/{deviceId}", h.detach)
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	req := pagination.ParsePageRequest(r)
	page, err := h.svc.List(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateDeviceRequest
	if err := httputil.DecodeSingleJSON(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.svc.Create(r.Context(), req)
	if err != nil {
		if errors.Is(err, ErrNameConflict) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, ErrValidation) {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid device id")
		return
	}
	resp, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "device not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid device id")
		return
	}
	var req UpdateDeviceRequest
	if err := httputil.DecodeSingleJSON(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.svc.Update(r.Context(), id, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "device not found")
			return
		}
		// Bug 113: ErrValidation no Update precisa do mesmo mapeamento
		// 422 que Create já fazia em linha 67.
		if errors.Is(err, ErrValidation) {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid device id")
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "device not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) heartbeat(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid device id")
		return
	}
	var req HeartbeatRequest
	if err := httputil.DecodeSingleJSON(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.Heartbeat(r.Context(), id, req); err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "device not found")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listByAgent(w http.ResponseWriter, r *http.Request) {
	agentID, err := uuid.Parse(chi.URLParam(r, "agentId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid agent id")
		return
	}
	if h.agent != nil {
		if err := h.agent.GetByID(r.Context(), agentID); err != nil {
			writeError(w, http.StatusNotFound, "agent not found")
			return
		}
	}
	devices, err := h.svc.ListByAgent(r.Context(), agentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if devices == nil {
		devices = []DeviceResponse{}
	}
	writeJSON(w, http.StatusOK, devices)
}

func (h *Handler) attach(w http.ResponseWriter, r *http.Request) {
	agentID, err := uuid.Parse(chi.URLParam(r, "agentId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid agent id")
		return
	}
	deviceID, err := uuid.Parse(chi.URLParam(r, "deviceId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid device id")
		return
	}
	if err := h.svc.AttachToAgent(r.Context(), agentID, deviceID); err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "device not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) detach(w http.ResponseWriter, r *http.Request) {
	agentID, err := uuid.Parse(chi.URLParam(r, "agentId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid agent id")
		return
	}
	deviceID, err := uuid.Parse(chi.URLParam(r, "deviceId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid device id")
		return
	}
	if err := h.svc.DetachFromAgent(r.Context(), agentID, deviceID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
