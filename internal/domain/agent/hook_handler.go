package agent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// agentHook is the domain entity persisted in agent_hook table.
type agentHook struct {
	ID             uuid.UUID       `json:"id"`
	AgentID        uuid.UUID       `json:"agentId"`
	Event          string          `json:"event"`
	Matcher        string          `json:"matcher,omitempty"`
	HookType       string          `json:"hookType"`
	Config         json.RawMessage `json:"config"`
	Enabled        bool            `json:"enabled"`
	Priority       int             `json:"priority"`
	TimeoutSeconds *int            `json:"timeoutSeconds,omitempty"`
	IsAsync        bool            `json:"isAsync"`
	RunOnce        bool            `json:"runOnce"`
	StatusMessage  string          `json:"statusMessage,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

type createHookRequest struct {
	Event          string          `json:"event"`
	Matcher        string          `json:"matcher,omitempty"`
	HookType       string          `json:"hookType"`
	Config         json.RawMessage `json:"config"`
	Enabled        *bool           `json:"enabled,omitempty"`
	Priority       int             `json:"priority,omitempty"`
	TimeoutSeconds *int            `json:"timeoutSeconds,omitempty"`
	IsAsync        bool            `json:"isAsync,omitempty"`
	RunOnce        bool            `json:"runOnce,omitempty"`
	StatusMessage  string          `json:"statusMessage,omitempty"`
}

type updateHookRequest struct {
	Event          *string         `json:"event,omitempty"`
	Matcher        *string         `json:"matcher,omitempty"`
	HookType       *string         `json:"hookType,omitempty"`
	Config         json.RawMessage `json:"config,omitempty"`
	Enabled        *bool           `json:"enabled,omitempty"`
	Priority       *int            `json:"priority,omitempty"`
	TimeoutSeconds *int            `json:"timeoutSeconds,omitempty"`
	IsAsync        *bool           `json:"isAsync,omitempty"`
	RunOnce        *bool           `json:"runOnce,omitempty"`
	StatusMessage  *string         `json:"statusMessage,omitempty"`
}

// HookHandler manages REST endpoints for agent hooks.
type HookHandler struct {
	pool *pgxpool.Pool
	svc  Service // optional: parent-agent existence checker (bug 209 batch)
}

// NewHookHandler creates a HookHandler backed by a connection pool.
func NewHookHandler(pool *pgxpool.Pool) *HookHandler {
	return &HookHandler{pool: pool}
}

// WithAgentService wires the agent service so list can validate parent existence.
func (h *HookHandler) WithAgentService(s Service) *HookHandler {
	h.svc = s
	return h
}

// RegisterRoutes mounts hook routes under /api/agents/{id}/hooks.
func (h *HookHandler) RegisterRoutes(r chi.Router) {
	r.Get("/api/agents/{id}/hooks", h.list)
	r.Post("/api/agents/{id}/hooks", h.create)
	r.Get("/api/agents/{id}/hooks/{hookId}", h.get)
	r.Put("/api/agents/{id}/hooks/{hookId}", h.update)
	r.Delete("/api/agents/{id}/hooks/{hookId}", h.delete)
}

func (h *HookHandler) acquire(ctx context.Context) (*pgxpool.Conn, func(), error) {
	tenantID := tenant.FromContext(ctx)
	return database.AcquireWithTenant(ctx, h.pool, tenantID)
}

func (h *HookHandler) list(w http.ResponseWriter, r *http.Request) {
	agentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agent id")
		return
	}
	if h.svc != nil {
		if _, err := h.svc.Get(r.Context(), agentID); err != nil {
			if errors.Is(err, ErrNotFound) {
				respond.Error(w, http.StatusNotFound, "agent not found")
				return
			}
			respond.Error(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
	conn, release, err := h.acquire(r.Context())
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer release()

	hooks, err := findHooksByAgent(r.Context(), conn, agentID)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusOK, hooks)
}

func (h *HookHandler) create(w http.ResponseWriter, r *http.Request) {
	agentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agent id")
		return
	}
	// Bug 213: valida agente antes de tentar inserir hook (FK violation
	// retornava 500 — UX confuso e potencial info-disclosure no log).
	if h.svc != nil {
		if _, err := h.svc.Get(r.Context(), agentID); err != nil {
			if errors.Is(err, ErrNotFound) {
				respond.Error(w, http.StatusNotFound, "agent not found")
				return
			}
			respond.Error(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
	var req createHookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Event == "" {
		respond.Error(w, http.StatusUnprocessableEntity, "field 'event' is required")
		return
	}
	if req.HookType == "" {
		respond.Error(w, http.StatusUnprocessableEntity, "field 'hookType' is required")
		return
	}
	if len(req.Config) == 0 {
		req.Config = json.RawMessage("{}")
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	conn, release, err := h.acquire(r.Context())
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer release()

	hook, err := insertHook(r.Context(), conn, agentID, req, enabled)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusCreated, hook)
}

func (h *HookHandler) get(w http.ResponseWriter, r *http.Request) {
	agentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agent id")
		return
	}
	hookID, err := uuid.Parse(chi.URLParam(r, "hookId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid hook id")
		return
	}
	conn, release, err := h.acquire(r.Context())
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer release()

	hook, err := findHookByID(r.Context(), conn, agentID, hookID)
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "hook not found")
		return
	}
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusOK, hook)
}

func (h *HookHandler) update(w http.ResponseWriter, r *http.Request) {
	agentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agent id")
		return
	}
	hookID, err := uuid.Parse(chi.URLParam(r, "hookId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid hook id")
		return
	}
	var req updateHookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	conn, release, err := h.acquire(r.Context())
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer release()

	hook, err := updateHook(r.Context(), conn, agentID, hookID, req)
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "hook not found")
		return
	}
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusOK, hook)
}

func (h *HookHandler) delete(w http.ResponseWriter, r *http.Request) {
	agentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agent id")
		return
	}
	hookID, err := uuid.Parse(chi.URLParam(r, "hookId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid hook id")
		return
	}

	conn, release, err := h.acquire(r.Context())
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer release()

	tag, err := conn.Exec(r.Context(),
		`DELETE FROM agent_hook WHERE id = $1 AND agent_id = $2`,
		hookID, agentID,
	)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	if tag.RowsAffected() == 0 {
		respond.Error(w, http.StatusNotFound, "hook not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- internal DB helpers ---

const hookSelectCols = `id, agent_id, event, matcher, hook_type, config, enabled, priority,
	timeout_seconds, is_async, run_once, status_message, created_at, updated_at`

func findHooksByAgent(ctx context.Context, conn *pgxpool.Conn, agentID uuid.UUID) ([]agentHook, error) {
	rows, err := conn.Query(ctx,
		`SELECT `+hookSelectCols+`
		 FROM agent_hook
		 WHERE agent_id = $1
		 ORDER BY priority ASC, created_at ASC`,
		agentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hooks []agentHook
	for rows.Next() {
		h, err := scanAgentHookRow(rows)
		if err != nil {
			return nil, err
		}
		hooks = append(hooks, h)
	}
	if hooks == nil {
		hooks = []agentHook{}
	}
	return hooks, rows.Err()
}

func findHookByID(ctx context.Context, conn *pgxpool.Conn, agentID, hookID uuid.UUID) (agentHook, error) {
	row := conn.QueryRow(ctx,
		`SELECT `+hookSelectCols+`
		 FROM agent_hook
		 WHERE id = $1 AND agent_id = $2`,
		hookID, agentID,
	)
	return scanAgentHookRow(row)
}

func insertHook(ctx context.Context, conn *pgxpool.Conn, agentID uuid.UUID, req createHookRequest, enabled bool) (agentHook, error) {
	row := conn.QueryRow(ctx,
		`INSERT INTO agent_hook
		   (agent_id, event, matcher, hook_type, config, enabled, priority,
		    timeout_seconds, is_async, run_once, status_message)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		 RETURNING `+hookSelectCols,
		agentID, req.Event, req.Matcher, req.HookType, req.Config, enabled, req.Priority,
		req.TimeoutSeconds, req.IsAsync, req.RunOnce, nullStr(req.StatusMessage),
	)
	return scanAgentHookRow(row)
}

func updateHook(ctx context.Context, conn *pgxpool.Conn, agentID, hookID uuid.UUID, req updateHookRequest) (agentHook, error) {
	existing, err := findHookByID(ctx, conn, agentID, hookID)
	if err != nil {
		return agentHook{}, err
	}
	if req.Event != nil {
		existing.Event = *req.Event
	}
	if req.Matcher != nil {
		existing.Matcher = *req.Matcher
	}
	if req.HookType != nil {
		existing.HookType = *req.HookType
	}
	if len(req.Config) > 0 {
		existing.Config = req.Config
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if req.Priority != nil {
		existing.Priority = *req.Priority
	}
	if req.TimeoutSeconds != nil {
		existing.TimeoutSeconds = req.TimeoutSeconds
	}
	if req.IsAsync != nil {
		existing.IsAsync = *req.IsAsync
	}
	if req.RunOnce != nil {
		existing.RunOnce = *req.RunOnce
	}
	if req.StatusMessage != nil {
		existing.StatusMessage = *req.StatusMessage
	}

	row := conn.QueryRow(ctx,
		`UPDATE agent_hook SET
		   event=$1, matcher=$2, hook_type=$3, config=$4, enabled=$5, priority=$6,
		   timeout_seconds=$7, is_async=$8, run_once=$9, status_message=$10, updated_at=NOW()
		 WHERE id=$11 AND agent_id=$12
		 RETURNING `+hookSelectCols,
		existing.Event, existing.Matcher, existing.HookType, existing.Config,
		existing.Enabled, existing.Priority,
		existing.TimeoutSeconds, existing.IsAsync, existing.RunOnce, nullStr(existing.StatusMessage),
		hookID, agentID,
	)
	return scanAgentHookRow(row)
}

func scanAgentHookRow(row pgx.Row) (agentHook, error) {
	var h agentHook
	var statusMsg *string
	err := row.Scan(
		&h.ID, &h.AgentID, &h.Event, &h.Matcher, &h.HookType, &h.Config,
		&h.Enabled, &h.Priority,
		&h.TimeoutSeconds, &h.IsAsync, &h.RunOnce, &statusMsg,
		&h.CreatedAt, &h.UpdatedAt,
	)
	if err != nil {
		return agentHook{}, err
	}
	if statusMsg != nil {
		h.StatusMessage = *statusMsg
	}
	return h, nil
}

func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
