package acl

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/httputil"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

// Identity is the authenticated subject used by ACL HTTP handlers.
type Identity struct {
	SubjectID string
	Roles     []string
}

// IdentityExtractor reads the authenticated subject from a request.
type IdentityExtractor func(*http.Request) Identity

// Handler exposes resource-grant endpoints.
type Handler struct {
	provider Provider
	extract  IdentityExtractor
}

// NewHandler creates an ACL handler.
func NewHandler(provider Provider, extract IdentityExtractor) *Handler {
	return &Handler{provider: provider, extract: extract}
}

// RegisterRoutes mounts ACL routes.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/acl/grants", h.list)
	r.Post("/api/acl/grants", h.create)
	r.Delete("/api/acl/grants/{id}", h.delete)
}

type grantRef struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type createGrantRequest struct {
	Subject  grantRef `json:"subject"`
	Resource grantRef `json:"resource"`
	Actions  []string `json:"actions"`
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	if !h.isAdmin(r) {
		respond.Error(w, http.StatusForbidden, "forbidden")
		return
	}
	grants, err := h.provider.ListGrants(r.Context(), GrantFilter{
		SubjectID:    r.URL.Query().Get("subject_id"),
		ResourceType: r.URL.Query().Get("resource_type"),
		ResourceID:   r.URL.Query().Get("resource_id"),
	})
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	req := pagination.ParsePageRequest(r)
	total := int64(len(grants))
	start := req.Offset()
	if start > len(grants) {
		start = len(grants)
	}
	end := start + req.Size
	if end > len(grants) {
		end = len(grants)
	}
	respond.JSON(w, http.StatusOK, pagination.NewPage(grants[start:end], total, req))
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	if !h.isAdmin(r) {
		respond.Error(w, http.StatusForbidden, "forbidden")
		return
	}
	var req createGrantRequest
	if err := httputil.DecodeSingleJSON(r.Body, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Subject.ID == "" || req.Resource.Type == "" || req.Resource.ID == "" || len(req.Actions) == 0 {
		respond.Error(w, http.StatusUnprocessableEntity, "subject, resource and actions are required")
		return
	}
	subjectType := SubjectType(req.Subject.Type)
	if subjectType == "" {
		subjectType = SubjectUser
	}
	if subjectType != SubjectUser {
		respond.Error(w, http.StatusUnprocessableEntity, "only user subjects are supported")
		return
	}
	grant, err := h.provider.AddGrant(r.Context(), Grant{
		SubjectType:  subjectType,
		SubjectID:    req.Subject.ID,
		ResourceType: req.Resource.Type,
		ResourceID:   req.Resource.ID,
		Actions:      req.Actions,
	})
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusCreated, grant)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	if !h.isAdmin(r) {
		respond.Error(w, http.StatusForbidden, "forbidden")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.provider.RemoveGrant(r.Context(), id); err != nil {
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.NoContent(w)
}

func (h *Handler) isAdmin(r *http.Request) bool {
	if h.extract == nil {
		return false
	}
	for _, role := range h.extract(r).Roles {
		if role == "admin" {
			return true
		}
	}
	return false
}
