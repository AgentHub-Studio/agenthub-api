package agent

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

// syncIDsRequest is the JSON body for syncing skill or KB bindings.
type syncIDsRequest struct {
	IDs []uuid.UUID `json:"ids"`
}

// BindingHandler exposes agent-skill and agent-knowledge-base binding endpoints.
type BindingHandler struct {
	agentRepo   Repository
	bindingRepo BindingRepository
}

// NewBindingHandler creates a new BindingHandler.
func NewBindingHandler(agentRepo Repository, bindingRepo BindingRepository) *BindingHandler {
	return &BindingHandler{agentRepo: agentRepo, bindingRepo: bindingRepo}
}

// RegisterBindingRoutes mounts binding routes under /api/agents/{id}/skills,
// /api/agents/{id}/knowledge-bases, and /api/agents/{id}/mcp-servers.
func (h *BindingHandler) RegisterBindingRoutes(r chi.Router) {
	r.Get("/api/agents/{id}/skills", h.listSkills)
	r.Put("/api/agents/{id}/skills", h.syncSkills)
	r.Get("/api/agents/{id}/knowledge-bases", h.listKnowledgeBases)
	r.Put("/api/agents/{id}/knowledge-bases", h.syncKnowledgeBases)
	// P-C253-1: per-agent MCP server bindings
	r.Get("/api/agents/{id}/mcp-servers", h.listMCPServers)
	r.Put("/api/agents/{id}/mcp-servers", h.syncMCPServers)
}

func (h *BindingHandler) parseAndValidateAgentID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agent id")
		return uuid.UUID{}, false
	}
	// Ensure the agent exists.
	if _, err := h.agentRepo.FindByID(r.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "agent not found")
			return uuid.UUID{}, false
		}
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return uuid.UUID{}, false
	}
	return id, true
}

func (h *BindingHandler) listSkills(w http.ResponseWriter, r *http.Request) {
	agentID, ok := h.parseAndValidateAgentID(w, r)
	if !ok {
		return
	}
	ids, err := h.bindingRepo.ListSkillIDs(r.Context(), agentID)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, ids)
}

func (h *BindingHandler) syncSkills(w http.ResponseWriter, r *http.Request) {
	agentID, ok := h.parseAndValidateAgentID(w, r)
	if !ok {
		return
	}
	var req syncIDsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.IDs == nil {
		req.IDs = []uuid.UUID{}
	}
	if err := h.bindingRepo.SyncSkills(r.Context(), agentID, req.IDs); err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Return the new state.
	ids, err := h.bindingRepo.ListSkillIDs(r.Context(), agentID)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, ids)
}

func (h *BindingHandler) listKnowledgeBases(w http.ResponseWriter, r *http.Request) {
	agentID, ok := h.parseAndValidateAgentID(w, r)
	if !ok {
		return
	}
	ids, err := h.bindingRepo.ListKnowledgeBaseIDs(r.Context(), agentID)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, ids)
}

func (h *BindingHandler) syncKnowledgeBases(w http.ResponseWriter, r *http.Request) {
	agentID, ok := h.parseAndValidateAgentID(w, r)
	if !ok {
		return
	}
	var req syncIDsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.IDs == nil {
		req.IDs = []uuid.UUID{}
	}
	if err := h.bindingRepo.SyncKnowledgeBases(r.Context(), agentID, req.IDs); err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	ids, err := h.bindingRepo.ListKnowledgeBaseIDs(r.Context(), agentID)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, ids)
}

func (h *BindingHandler) listMCPServers(w http.ResponseWriter, r *http.Request) {
	agentID, ok := h.parseAndValidateAgentID(w, r)
	if !ok {
		return
	}
	ids, err := h.bindingRepo.ListMCPServerIDs(r.Context(), agentID)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, ids)
}

func (h *BindingHandler) syncMCPServers(w http.ResponseWriter, r *http.Request) {
	agentID, ok := h.parseAndValidateAgentID(w, r)
	if !ok {
		return
	}
	var req syncIDsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.IDs == nil {
		req.IDs = []uuid.UUID{}
	}
	if err := h.bindingRepo.SyncMCPServers(r.Context(), agentID, req.IDs); err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	ids, err := h.bindingRepo.ListMCPServerIDs(r.Context(), agentID)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, ids)
}
