package core

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

// Handler exposes core-platform HTTP endpoints.
type Handler struct {
	agentLoader *CoreAgentLoader
	onboarding  *OnboardingService
}

// NewHandler creates a new Handler. onboarding may be nil, in which case the
// onboarding routes are not registered.
func NewHandler(agentLoader *CoreAgentLoader, onboarding *OnboardingService) *Handler {
	return &Handler{agentLoader: agentLoader, onboarding: onboarding}
}

// RegisterRoutes mounts core routes on the given router. These run inside the
// JWT-protected group, so the tenant is resolved from the request context.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/core/agents", h.listAgents)
	if h.onboarding != nil {
		r.Get("/api/core/onboarding/checklist", h.getOnboardingChecklist)
		r.Get("/api/core/onboarding/status", h.getOnboardingStatus)
	}
}

// listAgents returns all active platform specialist agents.
// GET /api/core/agents
func (h *Handler) listAgents(w http.ResponseWriter, r *http.Request) {
	agents, err := h.agentLoader.LoadAll(r.Context())
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}

	resp := make([]CoreAgentResponse, len(agents))
	for i, a := range agents {
		resp[i] = AgentResponseFrom(a)
	}
	respond.JSON(w, http.StatusOK, resp)
}

// getOnboardingChecklist returns the onboarding checklist definition (no
// per-tenant completion state).
// GET /api/core/onboarding/checklist
func (h *Handler) getOnboardingChecklist(w http.ResponseWriter, r *http.Request) {
	steps, err := h.onboarding.Checklist(r.Context())
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusOK, steps)
}

// getOnboardingStatus returns the onboarding checklist with each step's
// completion state for the calling tenant, plus an overall completed flag.
// GET /api/core/onboarding/status
func (h *Handler) getOnboardingStatus(w http.ResponseWriter, r *http.Request) {
	status, err := h.onboarding.Status(r.Context())
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusOK, status)
}
