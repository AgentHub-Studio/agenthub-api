package agent

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

// Handler exposes agent HTTP endpoints.
type Handler struct {
	svc Service
}

// NewHandler creates a new Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts agent routes on the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/agents", h.list)
	r.Post("/api/agents", h.create)
	r.Get("/api/agents/{id}", h.get)
	r.Put("/api/agents/{id}", h.update)
	r.Patch("/api/agents/{id}", h.update) // PATCH delegates to the same handler — all fields are optional
	r.Delete("/api/agents/{id}", h.delete)
	r.Post("/api/agents/{id}/publish", h.publish)
	r.Post("/api/agents/{id}/archive", h.archive)
	r.Post("/api/agents/{id}/clone", h.clone)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	req := pagination.ParsePageRequest(r)
	status := AgentStatus(r.URL.Query().Get("status"))
	q := r.URL.Query().Get("q") // P-C210-1: filter by name/description
	page, err := h.svc.List(r.Context(), status, q, req)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, page)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.svc.Create(r.Context(), req)
	if err != nil {
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
	resp, err := h.svc.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "agent not found")
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
	var req UpdateAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.svc.Update(r.Context(), id, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "agent not found")
			return
		}
		if isValidationError(err) {
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
			respond.Error(w, http.StatusNotFound, "agent not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.NoContent(w)
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
		respond.Error(w, http.StatusInternalServerError, err.Error())
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
		respond.Error(w, http.StatusInternalServerError, err.Error())
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.svc.Clone(r.Context(), id, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "agent not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.JSON(w, http.StatusCreated, resp)
}

// VersionHandler exposes agent version HTTP endpoints.
type VersionHandler struct {
	svc VersionService
}

// NewVersionHandler creates a new VersionHandler.
func NewVersionHandler(svc VersionService) *VersionHandler {
	return &VersionHandler{svc: svc}
}

// RegisterVersionRoutes mounts version routes under /api/agents/{agentId}/versions.
func (h *VersionHandler) RegisterVersionRoutes(r chi.Router) {
	r.Get("/api/agents/{agentId}/versions", h.listVersions)
	r.Post("/api/agents/{agentId}/versions", h.createDraft)
	r.Get("/api/agents/{agentId}/versions/draft", h.getDraft)
	r.Get("/api/agents/{agentId}/versions/latest-published", h.getLatestPublished)
	r.Put("/api/agents/{agentId}/versions/by-id/{versionId}", h.updateDraft)
	r.Post("/api/agents/{agentId}/versions/{versionId}/publish", h.publishVersion)
}

func parseAgentID(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, "agentId"))
}

func parseVersionID(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, "versionId"))
}

func (h *VersionHandler) listVersions(w http.ResponseWriter, r *http.Request) {
	agentID, err := parseAgentID(r)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agentId")
		return
	}
	req := pagination.ParsePageRequest(r)
	page, err := h.svc.ListVersions(r.Context(), agentID, req)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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
			respond.Error(w, http.StatusInternalServerError, err.Error())
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
		respond.Error(w, http.StatusInternalServerError, err.Error())
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
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, resp)
}

func (h *VersionHandler) updateDraft(w http.ResponseWriter, r *http.Request) {
	versionID, err := parseVersionID(r)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid versionId")
		return
	}
	var req UpdateAgentVersionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
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
			respond.Error(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	respond.JSON(w, http.StatusOK, resp)
}

func (h *VersionHandler) publishVersion(w http.ResponseWriter, r *http.Request) {
	versionID, err := parseVersionID(r)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid versionId")
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
			respond.Error(w, http.StatusInternalServerError, err.Error())
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
		errors.Is(err, ErrSlugConflict) ||
		errors.Is(err, ErrUnsupportedProvider) ||
		errors.Is(err, ErrInvalidRequest)
}
