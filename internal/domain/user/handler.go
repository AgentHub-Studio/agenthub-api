package user

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/httputil"
)

// Handler holds HTTP handlers for the user domain.
type Handler struct {
	svc Service
}

// NewHandler creates a new user Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterProtectedRoutes mounts authenticated user routes onto r.
func (h *Handler) RegisterProtectedRoutes(r chi.Router) {
	r.Get("/api/users", h.list)
	r.Post("/api/users", h.create)
	r.Get("/api/users/roles", h.listRoles)
	r.Get("/api/users/{id}", h.get)
	r.Patch("/api/users/{id}", h.update)
	r.Delete("/api/users/{id}", h.delete)
	r.Post("/api/users/{id}/roles/{role}", h.assignRole)
	r.Delete("/api/users/{id}/roles/{role}", h.removeRole)
	r.Post("/api/users/{id}/reset-password", h.resetPassword)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	users, err := h.svc.List(r.Context())
	if err != nil {
		httputil.InternalServerError(w, err.Error())
		return
	}
	httputil.JSON(w, http.StatusOK, users)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httputil.BadRequest(w, "invalid user id")
		return
	}
	u, err := h.svc.Get(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		httputil.NotFound(w, "user not found")
		return
	}
	if err != nil {
		httputil.InternalServerError(w, err.Error())
		return
	}
	httputil.JSON(w, http.StatusOK, u)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	u, err := h.svc.Create(r.Context(), req)
	if errors.Is(err, ErrAlreadyExists) {
		httputil.Conflict(w, "user already exists")
		return
	}
	if err != nil {
		httputil.BadRequest(w, err.Error())
		return
	}
	httputil.JSON(w, http.StatusCreated, u)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httputil.BadRequest(w, "invalid user id")
		return
	}
	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	u, err := h.svc.Update(r.Context(), id, req)
	if errors.Is(err, ErrNotFound) {
		httputil.NotFound(w, "user not found")
		return
	}
	if err != nil {
		httputil.InternalServerError(w, err.Error())
		return
	}
	httputil.JSON(w, http.StatusOK, u)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httputil.BadRequest(w, "invalid user id")
		return
	}
	err := h.svc.Delete(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		httputil.NotFound(w, "user not found")
		return
	}
	if err != nil {
		httputil.InternalServerError(w, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) assignRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	role := chi.URLParam(r, "role")
	if _, err := uuid.Parse(id); err != nil {
		httputil.BadRequest(w, "invalid user id")
		return
	}
	if err := h.svc.AssignRole(r.Context(), id, role); err != nil {
		if errors.Is(err, ErrNotFound) {
			httputil.NotFound(w, "user not found")
			return
		}
		httputil.InternalServerError(w, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) removeRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	role := chi.URLParam(r, "role")
	if _, err := uuid.Parse(id); err != nil {
		httputil.BadRequest(w, "invalid user id")
		return
	}
	if err := h.svc.RemoveRole(r.Context(), id, role); err != nil {
		if errors.Is(err, ErrNotFound) {
			httputil.NotFound(w, "user not found")
			return
		}
		httputil.InternalServerError(w, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) resetPassword(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httputil.BadRequest(w, "invalid user id")
		return
	}
	if err := h.svc.ResetPassword(r.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			httputil.NotFound(w, "user not found")
			return
		}
		httputil.InternalServerError(w, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := h.svc.ListRoles(r.Context())
	if err != nil {
		httputil.InternalServerError(w, err.Error())
		return
	}
	httputil.JSON(w, http.StatusOK, roles)
}
