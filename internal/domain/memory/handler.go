package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

// memoryService defines the methods used by Handler.
type memoryService interface {
	List(ctx context.Context, agentID uuid.UUID, userID *string) ([]AgentMemory, error)
	ListByType(ctx context.Context, agentID uuid.UUID, userID *string, memoryType MemoryType) ([]AgentMemory, error)
	Upsert(ctx context.Context, agentID uuid.UUID, key string, req UpsertMemoryRequest) (AgentMemory, error)
	GetByKey(ctx context.Context, agentID uuid.UUID, userID *string, key string) (AgentMemory, error)
	DeleteByKey(ctx context.Context, agentID uuid.UUID, userID *string, key string) error
	ClearByAgent(ctx context.Context, agentID uuid.UUID) error
	Recall(ctx context.Context, agentID uuid.UUID, req RecallRequest) ([]MemoryRecallResult, error)
	SearchByText(ctx context.Context, agentID uuid.UUID, query string, limit int) ([]AgentMemory, error)
	Stats(ctx context.Context, agentID uuid.UUID) (MemoryStats, error)
	BulkUpsert(ctx context.Context, agentID uuid.UUID, entries []BulkMemoryEntry) (int, error)
}

// agentExister verifies parent agent existence (bug 224 batch).
type agentExister interface {
	GetByID(ctx context.Context, id uuid.UUID) error
}

// Handler exposes the HTTP interface for agent memory.
type Handler struct {
	svc   memoryService
	agent agentExister
}

// NewHandler creates a new memory Handler.
func NewHandler(svc memoryService) *Handler {
	return &Handler{svc: svc}
}

// WithAgentExister wires the parent-agent existence checker (bug 224).
func (h *Handler) WithAgentExister(a agentExister) *Handler {
	h.agent = a
	return h
}

// RegisterRoutes mounts memory endpoints on the router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/agents/{agentId}/memory", h.list)
	r.Put("/api/agents/{agentId}/memory/{key}", h.upsert)
	r.Get("/api/agents/{agentId}/memory/{key}", h.getByKey)
	r.Delete("/api/agents/{agentId}/memory/{key}", h.deleteByKey)
	r.Delete("/api/agents/{agentId}/memory", h.clear)
	r.Post("/api/agents/{agentId}/memory/recall", h.recall)
	r.Get("/api/agents/{agentId}/memory/search", h.search)
	r.Get("/api/agents/{agentId}/memory/stats", h.stats)
	r.Post("/api/agents/{agentId}/memory/bulk", h.bulkUpsert)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	agentID, err := uuid.Parse(chi.URLParam(r, "agentId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agent id")
		return
	}
	if h.agent != nil {
		if err := h.agent.GetByID(r.Context(), agentID); err != nil {
			respond.Error(w, http.StatusNotFound, "agent not found")
			return
		}
	}
	var userID *string
	if v := r.URL.Query().Get("userId"); v != "" {
		userID = &v
	}

	// Optional filter by memory type.
	var items []AgentMemory
	if mt := r.URL.Query().Get("memoryType"); mt != "" {
		items, err = h.svc.ListByType(r.Context(), agentID, userID, MemoryType(mt))
	} else {
		items, err = h.svc.List(r.Context(), agentID, userID)
	}
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to list memory")
		return
	}
	// BUG-MEMORY-EMBEDDING-LEAKED: strip embedding vectors before responding.
	// Each 1024-float embedding is ~5 KB; returning it via the management API
	// leaks internal infrastructure and bloats responses unnecessarily.
	responses := make([]MemoryResponse, len(items))
	for i, m := range items {
		responses[i] = MemoryResponseFrom(m)
	}
	respond.JSON(w, http.StatusOK, responses)
}

func (h *Handler) upsert(w http.ResponseWriter, r *http.Request) {
	agentID, err := uuid.Parse(chi.URLParam(r, "agentId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agent id")
		return
	}
	key := chi.URLParam(r, "key")
	var req UpsertMemoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	m, err := h.svc.Upsert(r.Context(), agentID, key, req)
	if err != nil {
		// Bug 248: erros de validação (key pattern, value JSON) viram 422,
		// não 400. 400 é reservado para parse failures.
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, MemoryResponseFrom(m))
}

func (h *Handler) getByKey(w http.ResponseWriter, r *http.Request) {
	agentID, err := uuid.Parse(chi.URLParam(r, "agentId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agent id")
		return
	}
	key := chi.URLParam(r, "key")
	var userID *string
	if v := r.URL.Query().Get("userId"); v != "" {
		userID = &v
	}
	m, err := h.svc.GetByKey(r.Context(), agentID, userID, key)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "memory entry not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to get memory")
		return
	}
	respond.JSON(w, http.StatusOK, MemoryResponseFrom(m))
}

func (h *Handler) deleteByKey(w http.ResponseWriter, r *http.Request) {
	agentID, err := uuid.Parse(chi.URLParam(r, "agentId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agent id")
		return
	}
	key := chi.URLParam(r, "key")
	var userID *string
	if v := r.URL.Query().Get("userId"); v != "" {
		userID = &v
	}
	if err := h.svc.DeleteByKey(r.Context(), agentID, userID, key); err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "memory entry not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to delete memory")
		return
	}
	respond.NoContent(w)
}

func (h *Handler) clear(w http.ResponseWriter, r *http.Request) {
	agentID, err := uuid.Parse(chi.URLParam(r, "agentId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agent id")
		return
	}
	if err := h.svc.ClearByAgent(r.Context(), agentID); err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to clear memory")
		return
	}
	respond.NoContent(w)
}

// recall handles POST /api/agents/{agentId}/memory/recall.
// It returns the top-N semantically similar memories with decay-adjusted relevance scores.
func (h *Handler) recall(w http.ResponseWriter, r *http.Request) {
	agentID, err := uuid.Parse(chi.URLParam(r, "agentId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agent id")
		return
	}
	var req RecallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	results, err := h.svc.Recall(r.Context(), agentID, req)
	if errors.Is(err, ErrValidation) {
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err != nil {
		// Bug 260: fallback may include repo SQL errors.
		slog.Error("memory: recall failed", "agentID", agentID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "recall failed")
		return
	}
	// Strip embedding vectors from recall results too.
	safeResults := make([]MemoryRecallResponse, len(results))
	for i, res := range results {
		safeResults[i] = MemoryRecallResponse{
			MemoryResponse: MemoryResponseFrom(res.AgentMemory),
			Relevance:      res.Relevance,
		}
	}
	respond.JSON(w, http.StatusOK, safeResults)
}

// search handles GET /api/agents/{agentId}/memory/search?q=text&limit=20.
func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	agentID, err := uuid.Parse(chi.URLParam(r, "agentId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agent id")
		return
	}
	q := r.URL.Query().Get("q")
	if q == "" {
		respond.Error(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}

	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if _, err := fmt.Sscanf(v, "%d", &limit); err != nil || limit <= 0 {
			limit = 20
		}
	}

	items, err := h.svc.SearchByText(r.Context(), agentID, q, limit)
	if errors.Is(err, ErrValidation) {
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err != nil {
		// Bug 260: fallback may include repo SQL errors.
		slog.Error("memory: search failed", "agentID", agentID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "search failed")
		return
	}
	// Strip embedding vectors.
	responses := make([]MemoryResponse, len(items))
	for i, m := range items {
		responses[i] = MemoryResponseFrom(m)
	}
	respond.JSON(w, http.StatusOK, responses)
}

// stats handles GET /api/agents/{agentId}/memory/stats.
func (h *Handler) stats(w http.ResponseWriter, r *http.Request) {
	agentID, err := uuid.Parse(chi.URLParam(r, "agentId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agent id")
		return
	}
	if h.agent != nil {
		if err := h.agent.GetByID(r.Context(), agentID); err != nil {
			respond.Error(w, http.StatusNotFound, "agent not found")
			return
		}
	}
	s, err := h.svc.Stats(r.Context(), agentID)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to get memory stats")
		return
	}
	respond.JSON(w, http.StatusOK, s)
}

// bulkUpsert handles POST /api/agents/{agentId}/memory/bulk.
func (h *Handler) bulkUpsert(w http.ResponseWriter, r *http.Request) {
	agentID, err := uuid.Parse(chi.URLParam(r, "agentId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agent id")
		return
	}
	var entries []BulkMemoryEntry
	if err := json.NewDecoder(r.Body).Decode(&entries); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(entries) == 0 {
		respond.Error(w, http.StatusBadRequest, "at least one entry is required")
		return
	}
	if len(entries) > 100 {
		respond.Error(w, http.StatusBadRequest, "maximum 100 entries per bulk import")
		return
	}
	stored, err := h.svc.BulkUpsert(r.Context(), agentID, entries)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to bulk upsert")
		return
	}
	respond.JSON(w, http.StatusOK, map[string]int{"stored": stored, "total": len(entries)})
}
