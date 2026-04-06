package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// service is the interface required by the Handler.
type service interface {
	ListAll(ctx context.Context, tenantID string, pr pagination.PageRequest) ([]OAuthCredential, int, error)
	GetByID(ctx context.Context, tenantID string, id uuid.UUID) (OAuthCredential, error)
	Create(ctx context.Context, tenantID string, req CreateRequest) (OAuthCredential, error)
	Update(ctx context.Context, tenantID string, id uuid.UUID, req CreateRequest) (OAuthCredential, error)
	Delete(ctx context.Context, tenantID string, id uuid.UUID) error
	ResolveAuthHeader(ctx context.Context, tenantID string, id uuid.UUID) (ResolveResponse, error)
	ExchangeCode(ctx context.Context, tenantID string, id uuid.UUID, code string) error
	RefreshToken(ctx context.Context, tenantID string, id uuid.UUID) (OAuthCredential, error)
}

// Handler handles HTTP requests for OAuth credentials.
type Handler struct {
	svc service
}

// NewHandler creates a new Handler.
func NewHandler(svc service) *Handler {
	return &Handler{svc: svc}
}

// Routes mounts the handler routes and returns the router.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Get("/{id}", h.getByID)
	r.Put("/{id}", h.update)
	r.Patch("/{id}", h.update)
	r.Delete("/{id}", h.delete)
	r.Get("/{id}/resolve", h.resolve)
	r.Get("/callback", h.callback)
	return r
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	pr := pagination.ParsePageRequest(r)

	items, total, err := h.svc.ListAll(r.Context(), tenantID, pr)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	dtos := make([]OAuthCredentialResponse, len(items))
	for i, c := range items {
		dtos[i] = ResponseFrom(c)
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
		respond.Error(w, http.StatusInternalServerError, err.Error())
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

	c, err := h.svc.GetByID(r.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, err.Error())
			return
		}
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, ResponseFrom(c))
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
			respond.Error(w, http.StatusNotFound, err.Error())
			return
		}
		respond.Error(w, http.StatusInternalServerError, err.Error())
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
			respond.Error(w, http.StatusNotFound, err.Error())
			return
		}
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.NoContent(w)
}

func (h *Handler) resolve(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	res, err := h.svc.ResolveAuthHeader(r.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, err.Error())
			return
		}
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, res)
}

func (h *Handler) callback(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenantId")
	if tenantID == "" {
		respond.Error(w, http.StatusBadRequest, "tenantId is required")
		return
	}

	state := r.URL.Query().Get("state")
	if state == "" {
		respond.Error(w, http.StatusBadRequest, "state (credential id) is required")
		return
	}

	id, err := uuid.Parse(state)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid state")
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		respond.Error(w, http.StatusBadRequest, "code is required")
		return
	}

	if err := h.svc.ExchangeCode(r.Context(), tenantID, id, code); err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Redirect back to the UI (e.g. to a success page or back to MCP list)
	redirectUI := r.URL.Query().Get("redirect_ui")
	if redirectUI == "" {
		// Default fallback
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<h1>Authentication Successful!</h1><p>You can close this window now.</p>"))
		return
	}

	http.Redirect(w, r, redirectUI, http.StatusTemporaryRedirect)
}
