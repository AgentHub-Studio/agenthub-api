package auth

import (
	"encoding/json"
	"net/http"
	"sort"

	"github.com/go-chi/chi/v5"

	"github.com/AgentHub-Studio/agenthub-api/internal/httputil"
)

// FeatureFlags signals which capability-gated features are enabled in this deployment.
// The Angular frontend uses this to show/hide navigation entries.
type FeatureFlags struct {
	RBAC bool `json:"rbac"`
	ACL  bool `json:"acl"`
}

// AuthenticatedUser is the user envelope returned by the capabilities endpoint.
type AuthenticatedUser struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
}

// CapabilitiesResponse is the body of GET /api/auth/capabilities.
type CapabilitiesResponse struct {
	User        AuthenticatedUser `json:"user"`
	TenantID    string            `json:"tenantId"`
	Roles       []string          `json:"roles"`
	Permissions []string          `json:"permissions"`
	Features    FeatureFlags      `json:"features"`
}

// RoleExtractor pulls the authenticated subject and roles out of a request.
// Implementations read from the JWT claims placed in ctx by the auth middleware.
// Returns zero values in dev mode (placeholder auth) — capabilities then returns
// an empty envelope rather than 401 so the frontend can still bootstrap.
type RoleExtractor func(r *http.Request) (user AuthenticatedUser, tenantID string, roles []string)

// Handler serves the capabilities endpoint.
type Handler struct {
	extract  RoleExtractor
	features FeatureFlags
}

// NewHandler returns a capabilities handler with the given role extractor.
func NewHandler(extract RoleExtractor, features FeatureFlags) *Handler {
	return &Handler{extract: extract, features: features}
}

// RegisterRoutes mounts GET /api/auth/capabilities.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/auth/capabilities", h.capabilities)
	r.Post("/api/auth/permissions/match", h.matchPermission)
}

func (h *Handler) capabilities(w http.ResponseWriter, r *http.Request) {
	user, tenantID, roles := AuthenticatedUser{}, "", []string(nil)
	if h.extract != nil {
		user, tenantID, roles = h.extract(r)
	}
	perms := DefaultPermissionsForRoles(roles)
	sort.Strings(perms)
	resp := CapabilitiesResponse{
		User:        user,
		TenantID:    tenantID,
		Roles:       roles,
		Permissions: perms,
		Features:    h.features,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

type permissionMatchRequest struct {
	Granted  string `json:"granted"`
	Required string `json:"required"`
}

func (h *Handler) matchPermission(w http.ResponseWriter, r *http.Request) {
	var req permissionMatchRequest
	if err := httputil.DecodeSingleJSON(r.Body, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if req.Granted == "" || req.Required == "" {
		writeJSONError(w, http.StatusBadRequest, "granted and required are required")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{
		"matches": MatchesPermission(req.Granted, req.Required),
	})
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
