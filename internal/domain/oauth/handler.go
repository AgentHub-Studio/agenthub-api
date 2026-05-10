package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// safeRedirectTarget returns the value to redirect to or empty string if rejected.
// Bug 203: o callback OAuth aceitava qualquer URL em ?redirect_ui — vetor de
// open redirect (atacante envia link legítimo do callback que redirige para
// site de phishing após auth). Aceita apenas:
//   - paths relativos ("/agents/123") sem schema
//   - URLs absolute apontando para o frontend known (cezar.dev domains)
func safeRedirectTarget(raw string) string {
	if raw == "" {
		return ""
	}
	// Reject control chars and protocol relative ("//evil.com").
	if strings.ContainsAny(raw, "\r\n\t") || strings.HasPrefix(raw, "//") {
		return ""
	}
	// Relative path: safe.
	if strings.HasPrefix(raw, "/") {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	if host == "app.cezar.dev" || host == "test.cezar.dev" || strings.HasSuffix(host, ".cezar.dev") {
		return raw
	}
	return ""
}

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
		respond.Error(w, http.StatusInternalServerError, "internal error")
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
		// ErrValidation veio do Service quando o request omite name
		// ou usa authType fora do enum. Sem essa branch, body {} caía
		// em 500 (silencioso na UI) ou em 201 com lixo persistido.
		if errors.Is(err, ErrValidation) {
			respond.Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		if errors.Is(err, ErrDuplicateName) {
			respond.Error(w, http.StatusConflict, err.Error())
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
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
		respond.Error(w, http.StatusInternalServerError, "internal error")
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
		// Service.Update agora valida (consistente com Create);
		// retornar 422 em vez de 500 silencioso quando o operador
		// envia body parcial sem name/authType.
		if errors.Is(err, ErrValidation) {
			respond.Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
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
		respond.Error(w, http.StatusInternalServerError, "internal error")
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
		respond.Error(w, http.StatusInternalServerError, "internal error")
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
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Redirect back to the UI (e.g. to a success page or back to MCP list).
	// Bug 203: validate redirect_ui is same-origin or whitelisted to prevent
	// open redirect / phishing via legit callback URL.
	redirectUI := safeRedirectTarget(r.URL.Query().Get("redirect_ui"))
	if redirectUI == "" {
		// Default fallback (also reached when raw URL is rejected).
		// Bug 278: CSP restritiva no único endpoint que retorna HTML. Sem
		// scripts, styles inline ou iframes — só texto estático seguro.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
		w.Write([]byte("<h1>Authentication Successful!</h1><p>You can close this window now.</p>"))
		return
	}

	http.Redirect(w, r, redirectUI, http.StatusTemporaryRedirect)
}
