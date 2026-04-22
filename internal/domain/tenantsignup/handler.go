package tenantsignup

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

// Handler exposes the public signup endpoint.
type Handler struct {
	svc *Service
}

// NewHandler creates a new Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterPublicRoutes mounts the signup route on the given router.
// The endpoint lives under /api/tenants/signup (not /public/...) so that
// rate-limiting and captcha middleware — if added later — attach at the
// /api prefix, but it MUST be exempted from auth middleware at the edge.
func (h *Handler) RegisterPublicRoutes(r chi.Router) {
	r.Post("/api/tenants/signup", h.signup)
}

func (h *Handler) signup(w http.ResponseWriter, r *http.Request) {
	var req SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.svc.Signup(r.Context(), req)
	if err != nil {
		if errors.Is(err, ErrWeakTenantID) {
			respond.Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	respond.JSON(w, http.StatusCreated, resp)
}
