package auth

import (
	"encoding/json"
	"net/http"
	"sort"

	"github.com/go-chi/chi/v5"
)

// FeatureFlags signals which capability-gated features are enabled in this deployment.
// The Angular frontend uses this to show/hide navigation entries.
type FeatureFlags struct {
	RBAC bool `json:"rbac"`
	ACL  bool `json:"acl"`
}

// CapabilitiesResponse is the body of GET /api/auth/capabilities.
type CapabilitiesResponse struct {
	User        string       `json:"user"`
	TenantID    string       `json:"tenantId"`
	Roles       []string     `json:"roles"`
	Permissions []string     `json:"permissions"`
	Features    FeatureFlags `json:"features"`
}

// RoleExtractor pulls the authenticated subject and roles out of a request.
// Implementations read from the JWT claims placed in ctx by the auth middleware.
// Returns zero values in dev mode (placeholder auth) — capabilities then returns
// an empty envelope rather than 401 so the frontend can still bootstrap.
type RoleExtractor func(r *http.Request) (user, tenantID string, roles []string)

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
}

func (h *Handler) capabilities(w http.ResponseWriter, r *http.Request) {
	user, tenantID, roles := "", "", []string(nil)
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
