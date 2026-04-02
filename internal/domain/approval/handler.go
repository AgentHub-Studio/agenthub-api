package approval

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

// approvalService defines the methods used by Handler.
type approvalService interface {
	Create(ctx context.Context, req CreateApprovalRequest) (PendingApproval, error)
	GetByID(ctx context.Context, id uuid.UUID) (PendingApproval, error)
	List(ctx context.Context, req pagination.PageRequest) (pagination.Page[PendingApproval], error)
	PendingCount(ctx context.Context) (PendingCountResponse, error)
	Respond(ctx context.Context, id uuid.UUID, respondedBy string, req RespondRequest) (PendingApproval, error)
}

// Handler exposes the HTTP interface for approval management.
type Handler struct {
	svc approvalService
}

// NewHandler creates a new approval Handler.
func NewHandler(svc approvalService) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts approval endpoints on the router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/approvals", h.list)
	r.Post("/api/approvals", h.create)
	r.Get("/api/approvals/pending-count", h.pendingCount)
	r.Get("/api/approvals/stream", h.stream)
	r.Get("/api/approvals/{id}", h.getByID)
	r.Post("/api/approvals/{id}/respond", h.respond)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	req := pagination.ParsePageRequest(r)
	page, err := h.svc.List(r.Context(), req)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to list approvals")
		return
	}
	respond.JSON(w, http.StatusOK, page)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateApprovalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	a, err := h.svc.Create(r.Context(), req)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	respond.JSON(w, http.StatusCreated, a)
}

func (h *Handler) pendingCount(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.PendingCount(r.Context())
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to count pending approvals")
		return
	}
	respond.JSON(w, http.StatusOK, resp)
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid approval id")
		return
	}
	a, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "approval not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to get approval")
		return
	}
	respond.JSON(w, http.StatusOK, a)
}

func (h *Handler) respond(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid approval id")
		return
	}

	var req RespondRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	respondedBy := subjectFromRequest(r)
	a, err := h.svc.Respond(r.Context(), id, respondedBy, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "approval not found")
			return
		}
		if errors.Is(err, ErrAlreadyResolved) {
			respond.Error(w, http.StatusConflict, "approval already resolved")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to respond to approval")
		return
	}
	respond.JSON(w, http.StatusOK, a)
}

// stream sends Server-Sent Events when the pending count changes.
// Clients (e.g. the Angular approval-list) use this to refresh the list in real time.
// The endpoint polls every 5 seconds and emits an event whenever a new PENDING approval exists.
func (h *Handler) stream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		respond.Error(w, http.StatusInternalServerError, "streaming not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var lastCount int64
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			resp, err := h.svc.PendingCount(r.Context())
			if err != nil {
				return
			}
			if resp.Count != lastCount {
				lastCount = resp.Count
				data, _ := json.Marshal(map[string]int64{"count": resp.Count})
				_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			}
		}
	}
}

// subjectFromRequest extracts the JWT `sub` claim from the Authorization header.
// Returns "unknown" if the claim cannot be parsed.
func subjectFromRequest(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	tokenStr, ok := strings.CutPrefix(authHeader, "Bearer ")
	if !ok {
		return "unknown"
	}
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return "unknown"
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "unknown"
	}
	var claims struct {
		Subject string `json:"sub"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Subject == "" {
		return "unknown"
	}
	return claims.Subject
}
