package tool

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

// toolService defines the methods used by Handler.
type toolService interface {
	List(ctx context.Context, req pagination.PageRequest, toolType string) (pagination.Page[Response], error)
	ListLabels(ctx context.Context) ([]string, error)
	Create(ctx context.Context, req CreateRequest) (Response, error)
	GetByID(ctx context.Context, id uuid.UUID) (Response, error)
	Update(ctx context.Context, id uuid.UUID, req UpdateRequest) (Response, error)
	Delete(ctx context.Context, id uuid.UUID) error
	BindToSkill(ctx context.Context, skillID uuid.UUID, req BindRequest) (SkillToolResponse, error)
	UnbindFromSkill(ctx context.Context, skillID, toolID uuid.UUID) error
	ListBySkill(ctx context.Context, skillID uuid.UUID) ([]SkillToolResponse, error)
	TestTool(ctx context.Context, id uuid.UUID, inputs map[string]any) (string, error)
	GenerateCode(ctx context.Context, prompt, language string) (string, error)
	GenerateBlockly(ctx context.Context, prompt string) (any, error)
	GetDatabaseSchema(ctx context.Context, dataSourceID string) (DatabaseSchema, error)
}

// Handler exposes tool HTTP endpoints.
type Handler struct {
	svc toolService
}

// NewHandler creates a new Handler.
func NewHandler(svc toolService) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts tool routes on the given router.
// NOTE: fixed-path routes must be registered before parameterized ones so chi
// does not match "labels", "generate", etc. as {id}.
func (h *Handler) RegisterRoutes(r chi.Router) {
	// Fixed-path tool routes — must come before /{id}.
	r.Get("/api/tools/labels", h.listLabels)
	r.Get("/api/tools/database-schema", h.getDatabaseSchema)
	r.Post("/api/tools/generate/code", h.generateCode)
	r.Post("/api/tools/generate/blockly", h.generateBlockly)

	// CRUD routes.
	r.Get("/api/tools", h.list)
	r.Post("/api/tools", h.create)
	r.Get("/api/tools/{id}", h.getByID)
	r.Put("/api/tools/{id}", h.update)
	r.Patch("/api/tools/{id}", h.update) // PATCH delegates to the same handler — all fields are optional
	r.Delete("/api/tools/{id}", h.delete)
	r.Post("/api/tools/{id}/test", h.testTool)

	// Skill-tool binding routes.
	r.Post("/api/skills/{skillId}/tools", h.bindToSkill)
	r.Delete("/api/skills/{skillId}/tools/{toolId}", h.unbindFromSkill)
	r.Get("/api/skills/{skillId}/tools", h.listBySkill)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	req := pagination.ParsePageRequest(r)
	toolType := r.URL.Query().Get("type")
	page, err := h.svc.List(r.Context(), req, toolType)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, page)
}

func (h *Handler) listLabels(w http.ResponseWriter, r *http.Request) {
	labels, err := h.svc.ListLabels(r.Context())
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, labels)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.Type == "" {
		respond.Error(w, http.StatusUnprocessableEntity, "name and type are required")
		return
	}
	resp, err := h.svc.Create(r.Context(), req)
	if err != nil {
		if errors.Is(err, ErrDuplicateName) {
			// P-C338-1 (ACT-F3-15): return 422 with friendly message for duplicate names.
			respond.Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		if errors.Is(err, ErrValidation) {
			respond.Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		respond.Error(w, http.StatusInternalServerError, err.Error())
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
			respond.Error(w, http.StatusNotFound, "tool not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, err.Error())
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
			respond.Error(w, http.StatusNotFound, "tool not found")
			return
		}
		if errors.Is(err, ErrDuplicateName) {
			respond.Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		if errors.Is(err, ErrValidation) {
			respond.Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		respond.Error(w, http.StatusInternalServerError, err.Error())
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
			respond.Error(w, http.StatusNotFound, "tool not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.NoContent(w)
}

func (h *Handler) testTool(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	var inputs map[string]any
	if err := json.NewDecoder(r.Body).Decode(&inputs); err != nil {
		inputs = map[string]any{}
	}
	result, err := h.svc.TestTool(r.Context(), id, inputs)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "tool not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(result))
}

func (h *Handler) generateCode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Prompt   string `json:"prompt"`
		Language string `json:"language"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Prompt == "" {
		respond.Error(w, http.StatusUnprocessableEntity, "prompt is required")
		return
	}
	code, err := h.svc.GenerateCode(r.Context(), req.Prompt, req.Language)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(code))
}

func (h *Handler) generateBlockly(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Prompt string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Prompt == "" {
		respond.Error(w, http.StatusUnprocessableEntity, "prompt is required")
		return
	}
	result, err := h.svc.GenerateBlockly(r.Context(), req.Prompt)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, result)
}

func (h *Handler) getDatabaseSchema(w http.ResponseWriter, r *http.Request) {
	dataSourceID := r.URL.Query().Get("dataSourceId")
	if dataSourceID == "" {
		respond.Error(w, http.StatusBadRequest, "dataSourceId query parameter is required")
		return
	}
	schema, err := h.svc.GetDatabaseSchema(r.Context(), dataSourceID)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, schema)
}

func (h *Handler) bindToSkill(w http.ResponseWriter, r *http.Request) {
	skillID, err := uuid.Parse(chi.URLParam(r, "skillId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid skillId")
		return
	}
	var req BindRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.svc.BindToSkill(r.Context(), skillID, req)
	if err != nil {
		if errors.Is(err, ErrAlreadyBound) {
			respond.Error(w, http.StatusConflict, "tool already bound to this skill")
			return
		}
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "tool not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.JSON(w, http.StatusCreated, resp)
}

func (h *Handler) unbindFromSkill(w http.ResponseWriter, r *http.Request) {
	skillID, err := uuid.Parse(chi.URLParam(r, "skillId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid skillId")
		return
	}
	toolID, err := uuid.Parse(chi.URLParam(r, "toolId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid toolId")
		return
	}
	if err := h.svc.UnbindFromSkill(r.Context(), skillID, toolID); err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "binding not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.NoContent(w)
}

func (h *Handler) listBySkill(w http.ResponseWriter, r *http.Request) {
	skillID, err := uuid.Parse(chi.URLParam(r, "skillId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid skillId")
		return
	}
	resp, err := h.svc.ListBySkill(r.Context(), skillID)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, resp)
}
