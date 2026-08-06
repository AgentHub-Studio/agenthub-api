package agent

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/httputil"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

// TemplateGetter is the minimal interface the agent handler needs to
// resolve a prompt template by ID. Implemented by prompttemplate.Service.
type TemplateGetter interface {
	Get(ctx context.Context, id uuid.UUID) (TemplateContent, error)
}

// TemplateContent carries the fields from a prompt template that
// apply-template needs. Avoids a hard import of prompttemplate package.
type TemplateContent struct {
	Content       string
	ModelOverride *string
	IsBuiltin     bool // true when AgentID == nil (global/builtin template)
}

// Handler exposes agent HTTP endpoints.
type Handler struct {
	svc             Service
	templateSvc     TemplateGetter
	readAccess      ReadAccessChecker
	extractIdentity RequestIdentityExtractor
}

// NewHandler creates a new Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RequestIdentity is the authenticated subject used for resource-level ACL.
type RequestIdentity struct {
	SubjectID string
	Roles     []string
}

// RequestIdentityExtractor reads the authenticated subject from a request.
type RequestIdentityExtractor func(*http.Request) RequestIdentity

// ReadAccessChecker checks per-resource read grants.
type ReadAccessChecker interface {
	CanAccess(ctx context.Context, subjectID, resourceType, resourceID, action string) (bool, error)
}

// WithReadAccess enables resource-level ACL checks for GET /api/agents/{id}.
func (h *Handler) WithReadAccess(checker ReadAccessChecker, extract RequestIdentityExtractor) *Handler {
	h.readAccess = checker
	h.extractIdentity = extract
	return h
}

// WithTemplateGetter attaches a prompt-template resolver used by the
// POST /api/agents/{id}/apply-template endpoint.
func (h *Handler) WithTemplateGetter(g TemplateGetter) *Handler {
	h.templateSvc = g
	return h
}

// RegisterRoutes mounts agent routes on the given router. Read routes remain
// available to the chat and are protected by the resource-level ACL on get.
// Agent administration routes require the administrator role.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/agents", h.list)
	r.Get("/api/agents/{id}", h.get)

	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireRole("admin"))
		r.Post("/api/agents", h.create)
		// ACT-F3-19: bulk delete via DELETE /api/agents with a JSON body {"ids":[...]}.
		r.Delete("/api/agents", h.bulkDelete)
		r.Put("/api/agents/{id}", h.update)
		r.Patch("/api/agents/{id}", h.update) // PATCH delegates to the same handler — all fields are optional
		r.Delete("/api/agents/{id}", h.delete)
		r.Post("/api/agents/{id}/publish", h.publish)
		r.Post("/api/agents/{id}/archive", h.archive)
		r.Post("/api/agents/{id}/restore", h.restore)
		r.Post("/api/agents/{id}/clone", h.clone)
		r.Post("/api/agents/{id}/apply-template", h.applyTemplate)
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	req := pagination.ParsePageRequest(r)
	status := AgentStatus(r.URL.Query().Get("status"))
	q := r.URL.Query().Get("q") // P-C210-1: filter by name/description
	page, err := h.svc.List(r.Context(), status, q, req)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusOK, page)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateAgentRequest
	if err := httputil.DecodeSingleJSON(r.Body, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.svc.Create(r.Context(), req)
	if err != nil {
		// ErrSlugConflict deve virar 409 (não 422). 422 sugere
		// "corrija seu input" enquanto 409 sugere "estado do servidor
		// rejeita o pedido — tente outro slug". Sem essa branch o
		// cliente não distingue validação de payload de conflito de
		// recurso e retentaria criação inutilmente. Update já trata
		// isso corretamente; create estava simétrico errado.
		if errors.Is(err, ErrSlugConflict) {
			respond.Error(w, http.StatusConflict, err.Error())
			return
		}
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	respond.JSON(w, http.StatusCreated, resp)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	allowed, err := h.canReadAgent(r, id)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !allowed {
		respond.Error(w, http.StatusForbidden, "forbidden")
		return
	}
	resp, err := h.svc.GetWithReadiness(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "agent not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusOK, resp)
}

func (h *Handler) canReadAgent(r *http.Request, id uuid.UUID) (bool, error) {
	if h.readAccess == nil {
		return true, nil
	}
	if h.extractIdentity == nil {
		return false, nil
	}
	identity := h.extractIdentity(r)
	for _, role := range identity.Roles {
		if role == "admin" {
			return true, nil
		}
	}
	if identity.SubjectID == "" {
		return false, nil
	}
	return h.readAccess.CanAccess(r.Context(), identity.SubjectID, "agents", id.String(), "read")
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req UpdateAgentRequest
	if err := httputil.DecodeSingleJSON(r.Body, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.svc.Update(r.Context(), id, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "agent not found")
			return
		}
		// ErrSlugConflict precede a checagem genérica de validation
		// para retornar 409 (consistente com create após L39 fix).
		// 422 sugeriria "corrija seu input" enquanto 409 sugere
		// "estado do servidor rejeita o pedido — tente outro slug".
		if errors.Is(err, ErrSlugConflict) {
			respond.Error(w, http.StatusConflict, err.Error())
			return
		}
		if isValidationError(err) {
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
			respond.Error(w, http.StatusNotFound, "agent not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.NoContent(w)
}

// bulkDelete handles DELETE /api/agents with a JSON body {"ids": ["uuid1", "uuid2"]}.
// Returns 200 with {"deleted": N} where N is the count of agents successfully deleted.
// Agents not found are silently skipped (idempotent). P-C341-1 (ACT-F3-19).
func (h *Handler) bulkDelete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []uuid.UUID `json:"ids"`
	}
	if err := httputil.DecodeSingleJSON(r.Body, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.IDs) == 0 {
		respond.Error(w, http.StatusBadRequest, "ids must not be empty")
		return
	}
	count, err := h.svc.BulkDelete(r.Context(), req.IDs)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusOK, map[string]int{"deleted": count})
}

func (h *Handler) publish(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	resp, err := h.svc.Publish(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "agent not found")
			return
		}
		if isValidationError(err) {
			respond.Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusOK, resp)
}

func (h *Handler) archive(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	resp, err := h.svc.Archive(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "agent not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusOK, resp)
}

func (h *Handler) restore(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	resp, err := h.svc.Restore(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "agent not found")
			return
		}
		if isValidationError(err) {
			respond.Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusOK, resp)
}

func (h *Handler) clone(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req CloneAgentRequest
	if err := httputil.DecodeSingleJSON(r.Body, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.svc.Clone(r.Context(), id, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "agent not found")
			return
		}
		if errors.Is(err, ErrSlugConflict) {
			respond.Error(w, http.StatusConflict, err.Error())
			return
		}
		// Bug 128: Clone com name >255 retorna ErrInvalidRequest (em vez
		// de 500 SQL leak). Outros validation errors também caem aqui.
		if errors.Is(err, ErrInvalidRequest) {
			respond.Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusCreated, resp)
}

// applyTemplate applies a prompt template's content to an agent's system_prompt.
// POST /api/agents/{id}/apply-template
// Body: {"template_id": "<uuid>", "merge": false}
// When merge=false (default), the template content replaces the system_prompt.
// When merge=true, the template content is appended to the existing system_prompt.
func (h *Handler) applyTemplate(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agent id")
		return
	}

	if h.templateSvc == nil {
		respond.Error(w, http.StatusNotImplemented, "prompt template service not configured")
		return
	}

	var body struct {
		TemplateID string `json:"template_id"`
		Merge      bool   `json:"merge"`
	}
	if err := httputil.DecodeSingleJSON(r.Body, &body); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	templateID, err := uuid.Parse(body.TemplateID)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid template_id: must be a UUID")
		return
	}

	tpl, err := h.templateSvc.Get(r.Context(), templateID)
	if err != nil {
		respond.Error(w, http.StatusNotFound, "prompt template not found")
		return
	}

	// Resolve the new system prompt content.
	newContent := tpl.Content
	if body.Merge {
		// Fetch existing system prompt and append.
		existing, getErr := h.svc.Get(r.Context(), id)
		if getErr != nil {
			if errors.Is(getErr, ErrNotFound) {
				respond.Error(w, http.StatusNotFound, "agent not found")
				return
			}
			respond.Error(w, http.StatusInternalServerError, getErr.Error())
			return
		}
		if existing.SystemPrompt != nil && *existing.SystemPrompt != "" {
			newContent = *existing.SystemPrompt + "\n\n" + tpl.Content
		}
	}

	updateReq := UpdateAgentRequest{SystemPrompt: &newContent}
	resp, err := h.svc.Update(r.Context(), id, updateReq)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "agent not found")
			return
		}
		if isValidationError(err) {
			respond.Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusOK, resp)
}

// VersionHandler exposes agent version HTTP endpoints.
type VersionHandler struct {
	svc      VersionService
	agentSvc Service // optional: bug 208 — used by listVersions to verify agent existence
}

// NewVersionHandler creates a new VersionHandler.
func NewVersionHandler(svc VersionService) *VersionHandler {
	return &VersionHandler{svc: svc}
}

// WithAgentService wires the agent Service for parent-resource validation
// (bug 208 listVersions returns 404 for bogus agentId).
func (h *VersionHandler) WithAgentService(s Service) *VersionHandler {
	h.agentSvc = s
	return h
}

// RegisterVersionRoutes mounts administrator-only version routes under
// /api/agents/{agentId}/versions.
func (h *VersionHandler) RegisterVersionRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireRole("admin"))
		r.Get("/api/agents/{agentId}/versions", h.listVersions)
		r.Post("/api/agents/{agentId}/versions", h.createDraft)
		r.Get("/api/agents/{agentId}/versions/draft", h.getDraft)
		r.Get("/api/agents/{agentId}/versions/latest-published", h.getLatestPublished)
		r.Get("/api/agents/{agentId}/versions/by-id/{versionId}", h.getVersionByID)
		r.Put("/api/agents/{agentId}/versions/by-id/{versionId}", h.updateDraft)
		r.Patch("/api/agents/{agentId}/versions/by-id/{versionId}", h.updateDraft)
		r.Post("/api/agents/{agentId}/versions/{versionId}/publish", h.publishVersion)
		r.Post("/api/agents/{agentId}/versions/{versionId}/rollback", h.rollbackVersion)
	})
}

func parseAgentID(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, "agentId"))
}

func parseVersionID(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, "versionId"))
}

func (h *VersionHandler) requireVersionForAgent(ctx context.Context, agentID, versionID uuid.UUID) (AgentVersionResponse, error) {
	resp, err := h.svc.GetVersionByID(ctx, versionID)
	if err != nil {
		return AgentVersionResponse{}, err
	}
	if resp.AgentID != agentID {
		return AgentVersionResponse{}, ErrVersionNotFound
	}
	return resp, nil
}

func (h *VersionHandler) listVersions(w http.ResponseWriter, r *http.Request) {
	agentID, err := parseAgentID(r)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agentId")
		return
	}
	// Bug 208: validar agent existence; antes retornava 200+empty page
	// para qualquer UUID. Mesmo padrão de bugs 204/205.
	if h.agentSvc != nil {
		if _, err := h.agentSvc.Get(r.Context(), agentID); err != nil {
			if errors.Is(err, ErrNotFound) {
				respond.Error(w, http.StatusNotFound, "agent not found")
				return
			}
			respond.Error(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
	req := pagination.ParsePageRequest(r)
	page, err := h.svc.ListVersions(r.Context(), agentID, req)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusOK, page)
}

func (h *VersionHandler) createDraft(w http.ResponseWriter, r *http.Request) {
	agentID, err := parseAgentID(r)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agentId")
		return
	}
	var req CreateAgentVersionRequest
	if err := httputil.DecodeSingleJSON(r.Body, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.svc.CreateDraft(r.Context(), agentID, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			respond.Error(w, http.StatusNotFound, "agent not found")
		case errors.Is(err, ErrDraftAlreadyExists):
			respond.Error(w, http.StatusConflict, err.Error())
		default:
			respond.Error(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	respond.JSON(w, http.StatusCreated, resp)
}

func (h *VersionHandler) getDraft(w http.ResponseWriter, r *http.Request) {
	agentID, err := parseAgentID(r)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agentId")
		return
	}
	resp, err := h.svc.GetDraft(r.Context(), agentID)
	if err != nil {
		if errors.Is(err, ErrVersionNotFound) {
			respond.Error(w, http.StatusNotFound, "no draft version found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusOK, resp)
}

func (h *VersionHandler) getLatestPublished(w http.ResponseWriter, r *http.Request) {
	agentID, err := parseAgentID(r)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agentId")
		return
	}
	resp, err := h.svc.GetLatestPublished(r.Context(), agentID)
	if err != nil {
		if errors.Is(err, ErrVersionNotFound) {
			respond.Error(w, http.StatusNotFound, "no published version found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusOK, resp)
}

func (h *VersionHandler) getVersionByID(w http.ResponseWriter, r *http.Request) {
	agentID, err := parseAgentID(r)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agentId")
		return
	}
	versionID, err := parseVersionID(r)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid versionId")
		return
	}
	resp, err := h.requireVersionForAgent(r.Context(), agentID, versionID)
	if err != nil {
		if errors.Is(err, ErrVersionNotFound) {
			respond.Error(w, http.StatusNotFound, "version not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusOK, resp)
}

func (h *VersionHandler) updateDraft(w http.ResponseWriter, r *http.Request) {
	agentID, err := parseAgentID(r)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agentId")
		return
	}
	versionID, err := parseVersionID(r)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid versionId")
		return
	}
	var req UpdateAgentVersionRequest
	if err := httputil.DecodeSingleJSON(r.Body, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if _, err := h.requireVersionForAgent(r.Context(), agentID, versionID); err != nil {
		if errors.Is(err, ErrVersionNotFound) {
			respond.Error(w, http.StatusNotFound, "version not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	resp, err := h.svc.UpdateDraft(r.Context(), versionID, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrVersionNotFound):
			respond.Error(w, http.StatusNotFound, "version not found")
		case errors.Is(err, ErrVersionImmutable):
			respond.Error(w, http.StatusConflict, err.Error())
		default:
			respond.Error(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	respond.JSON(w, http.StatusOK, resp)
}

func (h *VersionHandler) publishVersion(w http.ResponseWriter, r *http.Request) {
	agentID, err := parseAgentID(r)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agentId")
		return
	}
	versionID, err := parseVersionID(r)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid versionId")
		return
	}
	if _, err := h.requireVersionForAgent(r.Context(), agentID, versionID); err != nil {
		if errors.Is(err, ErrVersionNotFound) {
			respond.Error(w, http.StatusNotFound, "version not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	resp, err := h.svc.Publish(r.Context(), versionID)
	if err != nil {
		switch {
		case errors.Is(err, ErrVersionNotFound):
			respond.Error(w, http.StatusNotFound, "version not found")
		case errors.Is(err, ErrVersionImmutable):
			respond.Error(w, http.StatusConflict, err.Error())
		default:
			respond.Error(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	respond.JSON(w, http.StatusOK, resp)
}

func (h *VersionHandler) rollbackVersion(w http.ResponseWriter, r *http.Request) {
	agentID, err := parseAgentID(r)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agentId")
		return
	}
	versionID, err := parseVersionID(r)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid versionId")
		return
	}
	resp, err := h.svc.Rollback(r.Context(), agentID, versionID)
	if err != nil {
		switch {
		case errors.Is(err, ErrVersionNotFound):
			respond.Error(w, http.StatusNotFound, "version not found")
		case errors.Is(err, ErrRollbackBlockedByDraft):
			respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		default:
			respond.Error(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	respond.JSON(w, http.StatusOK, resp)
}

// isValidationError returns true for errors that should map to HTTP 422.
// P-C249-3: invalid model config, unsupported provider, invalid skill IDs,
// invalid request body, and nested-modelConfig errors are user-input problems.
func isValidationError(err error) bool {
	return errors.Is(err, ErrInvalidModelConfig) ||
		errors.Is(err, ErrInvalidSkillIDs) ||
		errors.Is(err, ErrInvalidKnowledgeBaseIDs) ||
		errors.Is(err, ErrSlugConflict) ||
		errors.Is(err, ErrUnsupportedProvider) ||
		errors.Is(err, ErrInvalidRequest) ||
		errors.Is(err, ErrInvalidStatusTransition)
}
