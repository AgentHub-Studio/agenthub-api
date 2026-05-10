package agenttemplate

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/agent"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

// Handler exposes agent template HTTP endpoints.
type Handler struct {
	svc *Service
}

// NewHandler creates a new Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts agent template routes on the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/agent-templates", h.list)
	r.Post("/api/agent-templates", h.create)
	r.Get("/api/agent-templates/{slug}", h.getBySlug)
	r.Post("/api/agent-templates/{slug}/instantiate", h.instantiate)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	templates, err := h.svc.ListAll(r.Context(), category)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusOK, templates)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		respond.Error(w, http.StatusUnprocessableEntity, "name is required")
		return
	}
	resp, err := h.svc.Create(r.Context(), req)
	if err != nil {
		if errors.Is(err, ErrSlugConflict) {
			respond.Error(w, http.StatusConflict, "slug already in use")
			return
		}
		// Validações server-side (slug pattern, name vazio) → 422.
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	respond.JSON(w, http.StatusCreated, resp)
}

func (h *Handler) getBySlug(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	resp, err := h.svc.GetBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "agent template not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusOK, resp)
}

func (h *Handler) instantiate(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	var req InstantiateRequest
	// Body is optional — ignore decode errors on empty body.
	_ = json.NewDecoder(r.Body).Decode(&req)

	resp, err := h.svc.Instantiate(r.Context(), slug, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "agent template not found")
			return
		}
		// Bug 239: 2º instantiate sem name custom dispara slug duplicado
		// no agent (default vem do template). Antes virava 500 silencioso;
		// agora 409 com mensagem acionável.
		if errors.Is(err, agent.ErrSlugConflict) {
			respond.Error(w, http.StatusConflict, "agent slug already exists; provide a custom name or slug to instantiate")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusCreated, resp)
}
