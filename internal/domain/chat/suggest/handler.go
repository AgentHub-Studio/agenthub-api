package suggest

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

// Handler exposes GET /api/chat/suggestions.
type Handler struct {
	svc *Service
}

// NewHandler creates a new Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts suggestion routes on the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/chat/suggestions", h.list)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit := defaultLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	suggestions, err := h.svc.Generate(r.Context(), limit)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusOK, suggestions)
}
