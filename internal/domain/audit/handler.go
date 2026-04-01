package audit

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// auditService is the interface required by Handler.
type auditService interface {
	ListAll(ctx context.Context, tenantID string, f ListFilter, pr pagination.PageRequest) ([]AuditLog, int, error)
	GetByID(ctx context.Context, tenantID string, id uuid.UUID) (AuditLog, error)
}

// Handler handles HTTP requests for audit logs.
type Handler struct {
	svc auditService
}

// NewHandler creates a new Handler.
func NewHandler(svc auditService) *Handler {
	return &Handler{svc: svc}
}

// Routes mounts the handler routes.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Get("/{id}", h.getByID)
	return r
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	pr := pagination.ParsePageRequest(r)

	f := ListFilter{
		EntityType: r.URL.Query().Get("entityType"),
		EntityID:   r.URL.Query().Get("entityId"),
		Action:     r.URL.Query().Get("action"),
	}

	items, total, err := h.svc.ListAll(r.Context(), tenantID, f, pr)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	dtos := make([]AuditLogResponse, len(items))
	for i, l := range items {
		dtos[i] = ResponseFrom(l)
	}
	respond.JSON(w, http.StatusOK, pagination.NewPage(dtos, int64(total), pr))
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	l, err := h.svc.GetByID(r.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, err.Error())
			return
		}
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, ResponseFrom(l))
}
