package copilot

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

// Handler exposes the CopilotKit-aware HTTP endpoints.
type Handler struct {
	svc *Service
}

// NewHandler creates a new Handler backed by [svc].
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts the copilot routes on [r].
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/api/copilot/completions", h.completions)
}

func (h *Handler) completions(w http.ResponseWriter, r *http.Request) {
	var req CompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.svc.Suggest(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrEmptyText):
			respond.Error(w, http.StatusUnprocessableEntity, "text cannot be empty")
		case errors.Is(err, ErrLLMUnavailable):
			// 503: tenant has no provider configured. Clients should silently
			// disable autocompletion in this case.
			respond.Error(w, http.StatusServiceUnavailable, "no LLM provider configured")
		default:
			respond.Error(w, http.StatusInternalServerError, "completion failed")
		}
		return
	}

	respond.JSON(w, http.StatusOK, resp)
}
