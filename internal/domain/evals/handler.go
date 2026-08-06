package evals

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

type Handler struct {
	repo Repository
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.With(middleware.RequireRole("admin")).Get("/api/evals/runs", h.listRuns)
}

func (h *Handler) listRuns(w http.ResponseWriter, r *http.Request) {
	req := pagination.ParsePageRequest(r)
	var agentID *uuid.UUID
	if raw := r.URL.Query().Get("agent_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			respond.Error(w, http.StatusBadRequest, "invalid agent_id")
			return
		}
		agentID = &id
	}
	runs, total, err := h.repo.ListRuns(r.Context(), agentID, req)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	responses := make([]EvalRunResponse, len(runs))
	for i, run := range runs {
		responses[i] = ResponseFrom(run)
	}
	respond.JSON(w, http.StatusOK, pagination.NewPage(responses, total, req))
}
