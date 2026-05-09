package datasource

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// datasourceService is the interface required by Handler.
type datasourceService interface {
	ListAll(ctx context.Context, tenantID string, pr pagination.PageRequest) ([]DataSource, int, error)
	GetByID(ctx context.Context, tenantID string, id uuid.UUID) (DataSource, error)
	Create(ctx context.Context, tenantID string, req CreateRequest) (DataSource, error)
	Update(ctx context.Context, tenantID string, id uuid.UUID, req CreateRequest) (DataSource, error)
	Delete(ctx context.Context, tenantID string, id uuid.UUID) error
	GetCredentials(ctx context.Context, tenantID string, id uuid.UUID) (DataSourceCredentials, error)
}

// Handler handles HTTP requests for datasources.
type Handler struct {
	svc datasourceService
}

// NewHandler creates a new Handler.
func NewHandler(svc datasourceService) *Handler {
	return &Handler{svc: svc}
}

// Routes mounts the public datasource routes.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Post("/", h.create)
	r.Get("/", h.list)
	r.Get("/{id}", h.getByID)
	r.Put("/{id}", h.update)
	r.Patch("/{id}", h.update)
	r.Delete("/{id}", h.delete)
	return r
}

// ProxyRoutes mounts the internal proxy routes (includes credentials endpoint).
func (h *Handler) ProxyRoutes() http.Handler {
	r := chi.NewRouter()
	r.Get("/{id}", h.getCredentials)
	return r
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	pr := pagination.ParsePageRequest(r)

	items, total, err := h.svc.ListAll(r.Context(), tenantID, pr)
	if err != nil {
		// Bug 192: nunca expor err.Error() em endpoint de datasource —
		// repo wrapper pode conter SQLSTATE/host/db internals.
		slog.Error("datasource: list failed", "tenantID", tenantID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "list failed")
		return
	}

	dtos := make([]DataSourceResponse, len(items))
	for i, d := range items {
		dtos[i] = ResponseFrom(d)
	}
	respond.JSON(w, http.StatusOK, pagination.NewPage(dtos, int64(total), pr))
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())

	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	created, err := h.svc.Create(r.Context(), tenantID, req)
	if err != nil {
		// ErrValidation veio do service quando o request omite name/host
		// ou usa type fora do enum {POSTGRESQL,MYSQL,SQL_SERVER}. Sem
		// essa branch, o usuário recebia 500 silencioso na UI quando
		// esquecia preencher um campo — feedback errado.
		if errors.Is(err, ErrValidation) {
			respond.Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		if errors.Is(err, ErrDuplicateName) {
			respond.Error(w, http.StatusConflict, err.Error())
			return
		}
		slog.Error("datasource: create failed", "tenantID", tenantID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "create failed")
		return
	}
	respond.JSON(w, http.StatusCreated, ResponseFrom(created))
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	d, err := h.svc.GetByID(r.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "datasource not found")
			return
		}
		slog.Error("datasource: getByID failed", "tenantID", tenantID, "id", id, "err", err)
		respond.Error(w, http.StatusInternalServerError, "get failed")
		return
	}
	respond.JSON(w, http.StatusOK, ResponseFrom(d))
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	updated, err := h.svc.Update(r.Context(), tenantID, id, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "datasource not found")
			return
		}
		// Update também passa por validateRequest — sem essa branch
		// o operador recebia 500 quando enviava update parcial com
		// host/type vazio.
		if errors.Is(err, ErrValidation) {
			respond.Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		slog.Error("datasource: update failed", "tenantID", tenantID, "id", id, "err", err)
		respond.Error(w, http.StatusInternalServerError, "update failed")
		return
	}
	respond.JSON(w, http.StatusOK, ResponseFrom(updated))
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := h.svc.Delete(r.Context(), tenantID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "datasource not found")
			return
		}
		slog.Error("datasource: delete failed", "tenantID", tenantID, "id", id, "err", err)
		respond.Error(w, http.StatusInternalServerError, "delete failed")
		return
	}
	respond.NoContent(w)
}

// getCredentials returns datasource credentials including password.
// For internal/proxy use only — must be protected via auth/network policies in production.
func (h *Handler) getCredentials(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	creds, err := h.svc.GetCredentials(r.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "datasource not found")
			return
		}
		// Endpoint interno (proxy service token) — sanitizar mesmo aqui.
		slog.Error("datasource: getCredentials failed", "tenantID", tenantID, "id", id, "err", err)
		respond.Error(w, http.StatusInternalServerError, "credentials lookup failed")
		return
	}
	respond.JSON(w, http.StatusOK, creds)
}
