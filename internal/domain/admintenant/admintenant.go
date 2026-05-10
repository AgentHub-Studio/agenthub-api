// Package admintenant exposes the authenticated admin endpoint used by the
// `core` tenant to provision new tenants. Unlike public signup, the caller
// supplies the admin user's username and password directly; the endpoint is
// intended for the platform operator UI shipped with the core tenant.
package admintenant

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/mail"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tenant"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/user"
	"github.com/AgentHub-Studio/agenthub-api/internal/httputil"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// CreateRequest is the JSON body for POST /api/admin/tenants.
type CreateRequest struct {
	TenantID      string `json:"tenantId"`
	TenantName    string `json:"tenantName"`
	AdminUsername string `json:"adminUsername"`
	AdminEmail    string `json:"adminEmail"`
	AdminPassword string `json:"adminPassword"`
	AdminFirstName string `json:"adminFirstName,omitempty"`
	AdminLastName  string `json:"adminLastName,omitempty"`
}

// CreateResponse is returned after a successful provisioning.
type CreateResponse struct {
	TenantID string `json:"tenantId"`
	Username string `json:"username"`
}

type tenantAPI interface {
	Create(ctx context.Context, req tenant.CreateTenantRequest) (tenant.TenantResponse, error)
	List(ctx context.Context, req pagination.PageRequest) (pagination.Page[tenant.TenantResponse], error)
}

type tenantRepoAPI interface {
	UpdateName(ctx context.Context, id string, name string) error
	Delete(ctx context.Context, id string) error
}

type userAPI interface {
	CreateUser(ctx context.Context, tenantID string, req user.CreateUserRequest) (user.User, error)
}

type realmDeleter interface {
	DeleteRealm(ctx context.Context, tenantID string) error
}

// Service orchestrates tenant + admin-user creation in a single call.
type Service struct {
	tenants  tenantAPI
	repo     tenantRepoAPI
	users    userAPI
	realmDel realmDeleter
}

// NewService wires the admin tenant service.
func NewService(tenants tenantAPI, repo tenantRepoAPI, users userAPI, realmDel realmDeleter) *Service {
	return &Service{tenants: tenants, repo: repo, users: users, realmDel: realmDel}
}

// UpdateRequest is the JSON body for PUT /api/admin/tenants/{id}.
type UpdateRequest struct {
	TenantName string `json:"tenantName"`
}

// Create provisions the tenant (Keycloak realm + schema + presets) and then
// creates the admin user with the caller-supplied credentials.
func (s *Service) Create(ctx context.Context, req CreateRequest) (CreateResponse, error) {
	tenantID := strings.ToLower(strings.TrimSpace(req.TenantID))
	tenantName := strings.TrimSpace(req.TenantName)
	username := strings.TrimSpace(req.AdminUsername)
	password := req.AdminPassword
	email := strings.TrimSpace(req.AdminEmail)

	if tenantID == "" {
		return CreateResponse{}, errors.New("tenantId is required")
	}
	if tenantName == "" {
		return CreateResponse{}, errors.New("tenantName is required")
	}
	if username == "" {
		return CreateResponse{}, errors.New("adminUsername is required")
	}
	if len(password) < 8 {
		return CreateResponse{}, errors.New("adminPassword must be at least 8 characters")
	}
	if email == "" {
		email = username + "@" + tenantID + ".local"
	} else if _, err := mail.ParseAddress(email); err != nil {
		return CreateResponse{}, errors.New("adminEmail is invalid")
	}
	firstName := req.AdminFirstName
	if firstName == "" {
		firstName = username
	}

	if _, err := s.tenants.Create(ctx, tenant.CreateTenantRequest{
		ID:   tenantID,
		Name: tenantName,
	}); err != nil {
		return CreateResponse{}, err
	}

	if _, err := s.users.CreateUser(ctx, tenantID, user.CreateUserRequest{
		Username:  username,
		Email:     email,
		FirstName: firstName,
		LastName:  req.AdminLastName,
		Password:  password,
		Roles:     []string{"admin"},
	}); err != nil {
		return CreateResponse{}, err
	}

	return CreateResponse{TenantID: tenantID, Username: username}, nil
}

// Update changes the tenant display name.
func (s *Service) Update(ctx context.Context, id string, req UpdateRequest) error {
	name := strings.TrimSpace(req.TenantName)
	if name == "" {
		return errors.New("tenantName is required")
	}
	return s.repo.UpdateName(ctx, id, name)
}

// Delete removes the Keycloak realm and the tenant schema. Refuses to delete
// the administrative `core` tenant.
func (s *Service) Delete(ctx context.Context, id string) error {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "core" {
		return errors.New("cannot delete the core tenant")
	}
	if id == "" {
		return errors.New("id is required")
	}
	if s.realmDel != nil {
		if err := s.realmDel.DeleteRealm(ctx, id); err != nil {
			return err
		}
	}
	return s.repo.Delete(ctx, id)
}

// Handler exposes the admin tenant routes.
type Handler struct {
	svc *Service
}

// NewHandler creates a new Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts the admin tenant routes. The caller is expected to
// chain auth + RequireRole("admin") + RequireCoreTenant before this group.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/admin/tenants", func(r chi.Router) {
		r.Post("/", h.create)
		r.Get("/", h.list)
		r.Get("/{id}", h.get)
		r.Put("/{id}", h.update)
		r.Delete("/{id}", h.delete)
	})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	page, err := h.svc.tenants.List(r.Context(), pagination.PageRequest{Page: 0, Size: 1000})
	if err != nil {
		httputil.InternalServerError(w, "internal error")
		return
	}
	for _, t := range page.Content {
		if t.ID == id {
			httputil.JSON(w, http.StatusOK, t)
			return
		}
	}
	httputil.NotFound(w, "tenant not found")
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if err := h.svc.Update(r.Context(), id, req); err != nil {
		if errors.Is(err, tenant.ErrNotFound) {
			httputil.NotFound(w, "tenant not found")
			return
		}
		writeAdminTenantErr(w, "admintenant.update", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.Delete(r.Context(), id); err != nil {
		if errors.Is(err, tenant.ErrNotFound) {
			httputil.NotFound(w, "tenant not found")
			return
		}
		writeAdminTenantErr(w, "admintenant.delete", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	resp, err := h.svc.Create(r.Context(), req)
	if errors.Is(err, tenant.ErrAlreadyExists) {
		httputil.Conflict(w, "tenant already exists")
		return
	}
	if err != nil {
		writeAdminTenantErr(w, "admintenant.create", err)
		return
	}
	httputil.JSON(w, http.StatusCreated, resp)
}

// writeAdminTenantErr distinguishes validation errors (raw `errors.New(...)`
// from the service, safe to surface as 400) from upstream/transport errors
// containing internal cluster URLs or technical detail (Keycloak admin API,
// SQL drivers). Bug 255 — same class of fix as 254 (`user.handler`).
func writeAdminTenantErr(w http.ResponseWriter, op string, err error) {
	msg := err.Error()
	if strings.Contains(msg, "context deadline exceeded") ||
		strings.Contains(msg, "keycloak") ||
		strings.Contains(msg, ".svc.cluster.local") ||
		strings.Contains(msg, "Post \"http") ||
		strings.Contains(msg, "Get \"http") ||
		strings.Contains(msg, "Delete \"http") ||
		strings.Contains(msg, "dial tcp") {
		slog.Error(op+": upstream error", "err", err)
		httputil.JSON(w, http.StatusBadGateway, map[string]string{"error": "tenant provisioning service unavailable"})
		return
	}
	httputil.BadRequest(w, msg)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	req := pagination.ParsePageRequest(r)
	page, err := h.svc.tenants.List(r.Context(), req)
	if err != nil {
		httputil.InternalServerError(w, "internal error")
		return
	}
	httputil.JSON(w, http.StatusOK, page)
}
