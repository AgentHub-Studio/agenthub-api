package search

import (
	"context"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// searchService is the interface required by Handler.
type searchService interface {
	Search(ctx context.Context, tenantID, query, entityType string, limit int) (GlobalSearchResponse, error)
}

// Handler handles HTTP requests for global search.
type Handler struct {
	svc searchService
}

// NewHandler creates a new Handler.
func NewHandler(svc searchService) *Handler {
	return &Handler{svc: svc}
}

// Routes mounts the handler routes.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/", h.search)
	return r
}

func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	q := r.URL.Query().Get("q")
	if q == "" {
		respond.Error(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}

	limit := 5
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}

	entityType := r.URL.Query().Get("type") // optional: "agent", "skill", "tool", "knowledge_base"

	res, err := h.svc.Search(r.Context(), tenantID, q, entityType, limit)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusOK, res)
}
