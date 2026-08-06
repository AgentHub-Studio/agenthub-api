package user

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/httputil"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
)

// Handler holds HTTP handlers for the user domain.
type Handler struct {
	svc Service
}

// NewHandler creates a new user Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

func isKeycloakUpstreamError(err error) bool {
	if err == nil {
		return false
	}

	msg := err.Error()
	return strings.Contains(msg, "context deadline exceeded") ||
		strings.Contains(msg, "keycloak") ||
		strings.Contains(msg, ".svc.cluster.local") ||
		strings.Contains(msg, "Post \"http") ||
		strings.Contains(msg, "Get \"http")
}

func respondKeycloakUpstreamError(w http.ResponseWriter, operation string, err error) bool {
	if !isKeycloakUpstreamError(err) {
		return false
	}

	slog.Error("user: keycloak upstream error", "operation", operation, "err", err)
	httputil.JSON(w, http.StatusBadGateway, map[string]string{"error": "user provisioning service unavailable"})
	return true
}

// RegisterProtectedRoutes mounts authenticated user routes onto r.
func (h *Handler) RegisterProtectedRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireRole("admin"))
		r.Get("/api/users", h.list)
		r.Post("/api/users", h.create)
		r.Get("/api/users/roles", h.listRoles)
		r.Get("/api/users/{id}", h.get)
		r.Patch("/api/users/{id}", h.update)
		r.Delete("/api/users/{id}", h.delete)
		r.Post("/api/users/{id}/roles/{role}", h.assignRole)
		r.Delete("/api/users/{id}/roles/{role}", h.removeRole)
		r.Post("/api/users/{id}/reset-password", h.resetPassword)
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	users, err := h.svc.List(r.Context())
	if err != nil {
		if respondKeycloakUpstreamError(w, "list users", err) {
			return
		}
		// Bug 268: erro era engolido sem log — qualquer falha do
		// Keycloak Admin API caía em 500 sem visibilidade. Loga
		// server-side para diagnóstico.
		slog.Error("user: list failed", "err", err)
		httputil.InternalServerError(w, "internal error")
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
		if respondKeycloakUpstreamError(w, "get user", err) {
			return
		}
		// Bug 270: log err para diagnóstico (mesma classe do bug 268).
		slog.Error("user: get failed", "id", id, "err", err)
		httputil.InternalServerError(w, "internal error")
		return
	}
	httputil.JSON(w, http.StatusOK, u)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := httputil.DecodeSingleJSON(r.Body, &req); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	u, err := h.svc.Create(r.Context(), req)
	if errors.Is(err, ErrAlreadyExists) {
		httputil.Conflict(w, "user already exists")
		return
	}
	if errors.Is(err, ErrValidation) {
		httputil.UnprocessableEntity(w, err.Error())
		return
	}
	if err != nil {
		// Bug 254: erros do Keycloak admin API vazavam URL interna do
		// cluster (`http://keycloak.agenthub.svc.cluster.local:8080/...`)
		// e mensagens técnicas (`context deadline exceeded`). Mapeia
		// para 502 Bad Gateway sem vazar topology.
		if respondKeycloakUpstreamError(w, "create user", err) {
			return
		}
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
	if err := httputil.DecodeSingleJSON(r.Body, &req); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	u, err := h.svc.Update(r.Context(), id, req)
	if errors.Is(err, ErrNotFound) {
		httputil.NotFound(w, "user not found")
		return
	}
	if err != nil {
		if respondKeycloakUpstreamError(w, "update user", err) {
			return
		}
		slog.Error("user: update failed", "id", id, "err", err)
		httputil.InternalServerError(w, "internal error")
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
		if respondKeycloakUpstreamError(w, "delete user", err) {
			return
		}
		slog.Error("user: delete failed", "id", id, "err", err)
		httputil.InternalServerError(w, "internal error")
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
		if respondKeycloakUpstreamError(w, "assign role", err) {
			return
		}
		slog.Error("user: assign role failed", "id", id, "role", role, "err", err)
		httputil.InternalServerError(w, "internal error")
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
		if respondKeycloakUpstreamError(w, "remove role", err) {
			return
		}
		slog.Error("user: remove role failed", "id", id, "role", role, "err", err)
		httputil.InternalServerError(w, "internal error")
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
		if respondKeycloakUpstreamError(w, "reset password", err) {
			return
		}
		slog.Error("user: reset password failed", "id", id, "err", err)
		httputil.InternalServerError(w, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := h.svc.ListRoles(r.Context())
	if err != nil {
		if respondKeycloakUpstreamError(w, "list roles", err) {
			return
		}
		slog.Error("user: list roles failed", "err", err)
		httputil.InternalServerError(w, "internal error")
		return
	}
	httputil.JSON(w, http.StatusOK, roles)
}
