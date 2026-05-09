package skill

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

// skillService defines the methods used by Handler.
type skillService interface {
	List(ctx context.Context, category *string, req pagination.PageRequest) (pagination.Page[Response], error)
	Create(ctx context.Context, req CreateRequest) (Response, error)
	GetByID(ctx context.Context, id uuid.UUID) (Response, error)
	Update(ctx context.Context, id uuid.UUID, req UpdateRequest) (Response, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// skillExporter is a narrow interface for SKILL.md export, backed by the repository.
type skillExporter interface {
	GetByID(ctx context.Context, id uuid.UUID) (Skill, error)
}

// Handler exposes skill HTTP endpoints.
type Handler struct {
	svc  skillService
	repo skillExporter
}

// NewHandler creates a new Handler.
func NewHandler(svc skillService) *Handler {
	return &Handler{svc: svc}
}

// WithRepository attaches the raw repository for operations that need the full
// Skill model (e.g. SKILL.md export which requires the unexported fields).
func (h *Handler) WithRepository(repo skillExporter) *Handler {
	h.repo = repo
	return h
}

// RegisterRoutes mounts skill routes on the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/skills", h.list)
	r.Post("/api/skills", h.create)
	r.Post("/api/skills/import-skillmd", h.importSkillMD)
	r.Get("/api/skills/{id}", h.getByID)
	r.Put("/api/skills/{id}", h.update)
	r.Patch("/api/skills/{id}", h.update)
	r.Delete("/api/skills/{id}", h.delete)
	r.Get("/api/skills/{id}/skillmd", h.exportSkillMD)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	req := pagination.ParsePageRequest(r)
	var category *string
	if v := r.URL.Query().Get("category"); v != "" {
		category = &v
	}
	page, err := h.svc.List(r.Context(), category, req)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusOK, page)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		respond.Error(w, http.StatusUnprocessableEntity, "name is required")
		return
	}
	resp, err := h.svc.Create(r.Context(), req)
	if err != nil {
		// ErrSkillInert e ErrValidation cobrem validações server-side
		// (skill sem efeito, instructions > 32K chars). Sem essa branch,
		// o usuário recebia 500 silencioso na UI ao colar texto longo
		// nas instruções.
		if errors.Is(err, ErrSkillInert) || errors.Is(err, ErrValidation) {
			respond.Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusCreated, resp)
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	resp, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "skill not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusOK, resp)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.svc.Update(r.Context(), id, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "skill not found")
			return
		}
		// Bug 109: ErrValidation no Update precisa do mesmo mapeamento
		// 422 que Create (linha 93). Sem essa branch, payload inválido
		// (instructions > 32K, slug fora do pattern) caía em 500 e o
		// frontend mostrava "erro do servidor" em vez de feedback de
		// formulário.
		if errors.Is(err, ErrSkillInert) || errors.Is(err, ErrValidation) {
			respond.Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusOK, resp)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "skill not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.NoContent(w)
}

// exportSkillMD serializes a skill to the portable SKILL.md format.
// GET /api/skills/{id}/skillmd
func (h *Handler) exportSkillMD(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		respond.Error(w, http.StatusNotImplemented, "skillmd export not configured")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	sk, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "skill not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	content := SerializeSkillMD(sk)
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+sk.Slug+`.skill.md"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(content))
}

// importSkillMD parses a SKILL.md body and creates a new skill.
// POST /api/skills/import-skillmd
// Content-Type: text/markdown or application/octet-stream
func (h *Handler) importSkillMD(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MB limit
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	req, err := ParseSkillMD(string(body))
	if err != nil {
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	resp, err := h.svc.Create(r.Context(), req)
	if err != nil {
		// ErrSkillInert e ErrValidation cobrem validações server-side
		// (skill sem efeito, instructions > 32K chars). Sem essa branch,
		// o usuário recebia 500 silencioso na UI ao colar texto longo
		// nas instruções.
		if errors.Is(err, ErrSkillInert) || errors.Is(err, ErrValidation) {
			respond.Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusCreated, resp)
}
