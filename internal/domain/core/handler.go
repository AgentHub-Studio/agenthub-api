package core

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

// Handler exposes core-platform HTTP endpoints.
type Handler struct {
	agentLoader *CoreAgentLoader
}

// NewHandler creates a new Handler.
func NewHandler(agentLoader *CoreAgentLoader) *Handler {
	return &Handler{agentLoader: agentLoader}
}

// RegisterRoutes mounts core routes on the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/core/agents", h.listAgents)
}

// listAgents returns all active platform specialist agents.
// GET /api/core/agents
func (h *Handler) listAgents(w http.ResponseWriter, r *http.Request) {
	agents, err := h.agentLoader.LoadAll(r.Context())
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := make([]CoreAgentResponse, len(agents))
	for i, a := range agents {
		resp[i] = AgentResponseFrom(a)
	}
	respond.JSON(w, http.StatusOK, resp)
}
