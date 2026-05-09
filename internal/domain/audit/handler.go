package audit

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// auditService is the interface required by Handler.
type auditService interface {
	ListAll(ctx context.Context, tenantID string, f ListFilter, pr pagination.PageRequest) ([]AuditLog, int, error)
	GetByID(ctx context.Context, tenantID string, id uuid.UUID) (AuditLog, error)
	Record(ctx context.Context, tenantID string, req RecordRequest) (AuditLog, error)
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
// GET endpoints (list, getByID) require the "admin" role.
// POST (record) is accessible to any authenticated caller so other services can log events.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Post("/", h.record)
	r.With(middleware.RequireRole("admin")).Get("/", h.list)
	r.With(middleware.RequireRole("admin")).Get("/{id}", h.getByID)
	return r
}

// ExtractIP returns the client IP from X-Forwarded-For or RemoteAddr.
func ExtractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first (leftmost) address, which is the original client.
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	pr := pagination.ParsePageRequest(r)

	f := ListFilter{
		EntityType: r.URL.Query().Get("entityType"),
		EntityID:   r.URL.Query().Get("entityId"),
		Action:     r.URL.Query().Get("action"),
		ActorID:    r.URL.Query().Get("actorId"),
	}

	if v := r.URL.Query().Get("dateFrom"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.DateFrom = &t
		}
	}
	if v := r.URL.Query().Get("dateTo"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.DateTo = &t
		}
	}

	items, total, err := h.svc.ListAll(r.Context(), tenantID, f, pr)
	if err != nil {
		// Bug 194: nunca expor err.Error() em fallback 500.
		slog.Error("audit: list failed", "tenantID", tenantID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "list failed")
		return
	}

	dtos := make([]AuditLogResponse, len(items))
	for i, l := range items {
		dtos[i] = ResponseFrom(l)
	}
	respond.JSON(w, http.StatusOK, pagination.NewPage(dtos, int64(total), pr))
}

func (h *Handler) record(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())

	var req RecordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Automatically extract IP if not provided by the caller.
	if req.IPAddress == "" {
		req.IPAddress = ExtractIP(r)
	}

	l, err := h.svc.Record(r.Context(), tenantID, req)
	if err != nil {
		if errors.Is(err, ErrValidation) {
			respond.Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		slog.Error("audit: record failed", "tenantID", tenantID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "record failed")
		return
	}
	respond.JSON(w, http.StatusCreated, ResponseFrom(l))
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
			respond.Error(w, http.StatusNotFound, "audit log not found")
			return
		}
		slog.Error("audit: getByID failed", "tenantID", tenantID, "id", id, "err", err)
		respond.Error(w, http.StatusInternalServerError, "get failed")
		return
	}
	respond.JSON(w, http.StatusOK, ResponseFrom(l))
}
