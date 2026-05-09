package agent

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

// PortableBindingLookup is the minimal subset of BindingRepository needed to
// export an agent's skill/KB links. Kept as its own interface so the portable
// handler doesn't require a full BindingRepository for read-only scenarios.
type PortableBindingLookup interface {
	ListSkillIDs(ctx context.Context, agentID uuid.UUID) ([]uuid.UUID, error)
	ListKnowledgeBaseIDs(ctx context.Context, agentID uuid.UUID) ([]uuid.UUID, error)
}

// WithPortableBindings wires the binding lookup used by the YAML export endpoint.
func (h *Handler) WithPortableBindings(b PortableBindingLookup) *Handler {
	h.portableBindings = b
	return h
}

func (h *Handler) exportPortable(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	resp, err := h.svc.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "agent not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	var skillIDs, kbIDs []uuid.UUID
	if h.portableBindings != nil {
		skillIDs, _ = h.portableBindings.ListSkillIDs(r.Context(), id)
		kbIDs, _ = h.portableBindings.ListKnowledgeBaseIDs(r.Context(), id)
	}
	portable := ToPortable(responseToAgent(resp), skillIDs, kbIDs)
	data, err := MarshalPortableYAML(portable)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.Header().Set("Content-Type", "application/yaml")
	w.Header().Set("Content-Disposition", `attachment; filename="`+portable.Metadata.Slug+`.yaml"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *Handler) importPortable(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "failed to read body")
		return
	}
	portable, err := UnmarshalPortableYAML(body)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := h.svc.Create(r.Context(), portable.ToCreateRequest())
	if err != nil {
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	respond.JSON(w, http.StatusCreated, resp)
}

func responseToAgent(r AgentResponse) Agent {
	return Agent{
		ID:               r.ID,
		Name:             r.Name,
		Slug:             r.Slug,
		Description:      r.Description,
		Status:           AgentStatus(r.Status),
		CurrentVersion:   r.CurrentVersion,
		SystemPrompt:     r.SystemPrompt,
		ModelConfig:      r.ModelConfig,
		PermissionRules:  r.PermissionRules,
		Config:           r.Config,
		EnableManagement: r.EnableManagement,
		CreatedAt:        r.CreatedAt,
		UpdatedAt:        r.UpdatedAt,
	}
}
