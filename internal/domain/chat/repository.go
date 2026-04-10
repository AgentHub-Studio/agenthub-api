package chat

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// Repository defines the persistence interface for chat sessions and messages.
type Repository interface {
	FindSessions(ctx context.Context, req pagination.PageRequest) ([]ChatSession, int64, error)
	GetSessionListStamp(ctx context.Context) (ChatSessionListStamp, error)
	GetSessionByID(ctx context.Context, id uuid.UUID) (ChatSession, error)
	CreateSession(ctx context.Context, s ChatSession) (ChatSession, error)
	UpdateSessionStatus(ctx context.Context, id uuid.UUID, status ChatStatus) (ChatSession, error)
	UpdateSessionTitle(ctx context.Context, id uuid.UUID, title string) (ChatSession, error)
	// UpdateSessionAgent binds an agent to a session that has none.
	UpdateSessionAgent(ctx context.Context, sessionID uuid.UUID, agentID uuid.UUID) error
	// UpdateSessionConfigHash stores the SHA-256 hash of the current modelConfig.
	// P-C173-1: used to detect config changes between turns.
	UpdateSessionConfigHash(ctx context.Context, sessionID uuid.UUID, hash string) error
	// FindDefaultAgentID returns the ID of the default agent for the tenant
	// (slug='agenthub-assistant', PUBLISHED). Falls back to any PUBLISHED agent.
	// Returns nil, nil when no published agent exists.
	FindDefaultAgentID(ctx context.Context) (*uuid.UUID, error)
	DeleteSession(ctx context.Context, id uuid.UUID) error
	FindMessages(ctx context.Context, sessionID uuid.UUID, req pagination.PageRequest) ([]ChatMessage, int64, error)
	CreateMessage(ctx context.Context, m ChatMessage) (ChatMessage, error)
	// GetLatestAssistantMessage returns the most recent assistant message created after
	// the given time, or (ChatMessage{}, false, nil) if none has arrived yet.
	GetLatestAssistantMessage(ctx context.Context, sessionID uuid.UUID, after time.Time) (ChatMessage, bool, error)
	// FindAllMessages returns all messages for a session ordered by created_at ASC.
	// Used by the agentic Runner to load conversation history.
	FindAllMessages(ctx context.Context, sessionID uuid.UUID) ([]ChatMessage, error)
	// GetLatestCompactSummary returns the most recent compact_summary message for the session,
	// or (ChatMessage{}, false, nil) if none exists.
	GetLatestCompactSummary(ctx context.Context, sessionID uuid.UUID) (ChatMessage, bool, error)

	// ChatRun persistence
	CreateRun(ctx context.Context, r ChatRun) (ChatRun, error)
	GetRunByID(ctx context.Context, id uuid.UUID) (ChatRun, error)
	GetActiveRunBySession(ctx context.Context, sessionID uuid.UUID) (ChatRun, bool, error)
	UpdateRunStatus(ctx context.Context, id uuid.UUID, status ChatRunStatus, lastEventID string) error
	MarkRunCompleted(ctx context.Context, id uuid.UUID) error
}

type postgresRepository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new postgres-backed Repository.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &postgresRepository{pool: pool}
}

func (r *postgresRepository) FindSessions(ctx context.Context, req pagination.PageRequest) ([]ChatSession, int64, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return nil, 0, err
	}
	defer release()

	var total int64
	if err := conn.QueryRow(ctx, `SELECT COUNT(*) FROM chat_session`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("chat: count sessions: %w", err)
	}

	rows, err := conn.Query(ctx,
		`SELECT id, agent_id, title, status, created_at, updated_at
		 FROM chat_session
		 ORDER BY created_at DESC
		 LIMIT $1 OFFSET $2`,
		req.Size, req.Offset(),
	)
	if err != nil {
		return nil, 0, fmt.Errorf("chat: find sessions: %w", err)
	}
	defer rows.Close()

	var items []ChatSession
	for rows.Next() {
		var s ChatSession
		if err := rows.Scan(&s.ID, &s.AgentID, &s.Title, &s.Status, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("chat: scan session: %w", err)
		}
		items = append(items, s)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("chat: rows: %w", err)
	}

	return items, total, nil
}

func (r *postgresRepository) GetSessionListStamp(ctx context.Context) (ChatSessionListStamp, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return ChatSessionListStamp{}, err
	}
	defer release()

	var stamp ChatSessionListStamp
	err = conn.QueryRow(ctx,
		`SELECT COUNT(*), COALESCE(MAX(updated_at), 'epoch'::timestamptz)
		 FROM chat_session`,
	).Scan(&stamp.Count, &stamp.LatestUpdatedAt)
	if err != nil {
		return ChatSessionListStamp{}, fmt.Errorf("chat: session list stamp: %w", err)
	}

	return stamp, nil
}

func (r *postgresRepository) GetSessionByID(ctx context.Context, id uuid.UUID) (ChatSession, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return ChatSession{}, err
	}
	defer release()

	var s ChatSession
	err = conn.QueryRow(ctx,
		`SELECT id, agent_id, title, status,
		        system_prompt_snapshot, model_config_snapshot, skill_bindings_snapshot,
		        created_at, updated_at
		 FROM chat_session WHERE id = $1`,
		id,
	).Scan(&s.ID, &s.AgentID, &s.Title, &s.Status,
		&s.SystemPromptSnapshot, &s.ModelConfigSnapshot, &s.SkillBindingsSnapshot,
		&s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ChatSession{}, ErrNotFound
		}
		return ChatSession{}, fmt.Errorf("chat: get session by id: %w", err)
	}

	return s, nil
}

func (r *postgresRepository) CreateSession(ctx context.Context, s ChatSession) (ChatSession, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return ChatSession{}, err
	}
	defer release()

	now := time.Now().UTC()
	s.ID = uuid.New()
	s.CreatedAt = now
	s.UpdatedAt = now

	_, err = conn.Exec(ctx,
		`INSERT INTO chat_session
		 (id, agent_id, title, status,
		  system_prompt_snapshot, model_config_snapshot, skill_bindings_snapshot,
		  created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		s.ID, s.AgentID, s.Title, s.Status,
		s.SystemPromptSnapshot, s.ModelConfigSnapshot, s.SkillBindingsSnapshot,
		s.CreatedAt, s.UpdatedAt,
	)
	if err != nil {
		return ChatSession{}, fmt.Errorf("chat: create session: %w", err)
	}

	return s, nil
}

func (r *postgresRepository) UpdateSessionStatus(ctx context.Context, id uuid.UUID, status ChatStatus) (ChatSession, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return ChatSession{}, err
	}
	defer release()

	now := time.Now().UTC()
	var s ChatSession
	err = conn.QueryRow(ctx,
		`UPDATE chat_session
		 SET status = $1, updated_at = $2
		 WHERE id = $3
		 RETURNING id, agent_id, title, status, created_at, updated_at`,
		status, now, id,
	).Scan(&s.ID, &s.AgentID, &s.Title, &s.Status, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ChatSession{}, ErrNotFound
		}
		return ChatSession{}, fmt.Errorf("chat: update session status: %w", err)
	}

	return s, nil
}

func (r *postgresRepository) UpdateSessionTitle(ctx context.Context, id uuid.UUID, title string) (ChatSession, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return ChatSession{}, err
	}
	defer release()

	now := time.Now().UTC()
	var s ChatSession
	err = conn.QueryRow(ctx,
		`UPDATE chat_session
		 SET title = $1, updated_at = $2
		 WHERE id = $3
		 RETURNING id, agent_id, title, status, created_at, updated_at`,
		title, now, id,
	).Scan(&s.ID, &s.AgentID, &s.Title, &s.Status, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ChatSession{}, ErrNotFound
		}
		return ChatSession{}, fmt.Errorf("chat: update session title: %w", err)
	}

	return s, nil
}

func (r *postgresRepository) DeleteSession(ctx context.Context, id uuid.UUID) error {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return err
	}
	defer release()

	tag, err := conn.Exec(ctx, `DELETE FROM chat_session WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("chat: delete session: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *postgresRepository) UpdateSessionAgent(ctx context.Context, sessionID uuid.UUID, agentID uuid.UUID) error {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return err
	}
	defer release()

	tag, err := conn.Exec(ctx,
		`UPDATE chat_session SET agent_id = $1, updated_at = $2 WHERE id = $3`,
		agentID, time.Now().UTC(), sessionID,
	)
	if err != nil {
		return fmt.Errorf("chat: update session agent: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *postgresRepository) FindDefaultAgentID(ctx context.Context) (*uuid.UUID, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return nil, err
	}
	defer release()

	var id uuid.UUID
	// Prefer the well-known AgentHub Assistant; fall back to any published agent.
	err = conn.QueryRow(ctx,
		`SELECT id FROM agent WHERE status = 'PUBLISHED'
		 ORDER BY (slug = 'agenthub-assistant') DESC, created_at ASC
		 LIMIT 1`,
	).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("chat: find default agent: %w", err)
	}
	return &id, nil
}

func (r *postgresRepository) FindMessages(ctx context.Context, sessionID uuid.UUID, req pagination.PageRequest) ([]ChatMessage, int64, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return nil, 0, err
	}
	defer release()

	var total int64
	if err := conn.QueryRow(ctx, `SELECT COUNT(*) FROM chat_message WHERE session_id = $1`, sessionID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("chat: count messages: %w", err)
	}

	rows, err := conn.Query(ctx,
		`SELECT id, session_id, role, content,
		        message_type, tool_calls, tool_call_id,
		        metadata, token_usage, finish_reason, turn_index, run_id,
		        created_at
		 FROM chat_message
		 WHERE session_id = $1
		 ORDER BY created_at ASC
		 LIMIT $2 OFFSET $3`,
		sessionID, req.Size, req.Offset(),
	)
	if err != nil {
		return nil, 0, fmt.Errorf("chat: find messages: %w", err)
	}
	defer rows.Close()

	var items []ChatMessage
	for rows.Next() {
		var m ChatMessage
		if err := rows.Scan(
			&m.ID, &m.SessionID, &m.Role, &m.Content,
			&m.MessageType, &m.ToolCalls, &m.ToolCallID,
			&m.Metadata, &m.TokenUsage, &m.FinishReason, &m.TurnIndex, &m.RunID,
			&m.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("chat: scan message: %w", err)
		}
		items = append(items, m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("chat: rows: %w", err)
	}

	return items, total, nil
}

func (r *postgresRepository) CreateMessage(ctx context.Context, m ChatMessage) (ChatMessage, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return ChatMessage{}, err
	}
	defer release()

	m.ID = uuid.New()
	m.CreatedAt = time.Now().UTC()
	if m.MessageType == "" {
		m.MessageType = MessageTypeText
	}

	_, err = conn.Exec(ctx,
		`INSERT INTO chat_message
		 (id, session_id, role, content,
		  message_type, tool_calls, tool_call_id,
		  metadata, token_usage, finish_reason, turn_index, run_id,
		  created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		m.ID, m.SessionID, m.Role, m.Content,
		m.MessageType, m.ToolCalls, m.ToolCallID,
		m.Metadata, m.TokenUsage, m.FinishReason, m.TurnIndex, m.RunID,
		m.CreatedAt,
	)
	if err != nil {
		return ChatMessage{}, fmt.Errorf("chat: create message: %w", err)
	}

	return m, nil
}

func (r *postgresRepository) GetLatestAssistantMessage(ctx context.Context, sessionID uuid.UUID, after time.Time) (ChatMessage, bool, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return ChatMessage{}, false, err
	}
	defer release()

	var m ChatMessage
	err = conn.QueryRow(ctx,
		`SELECT id, session_id, role, content,
		        message_type, tool_calls, tool_call_id,
		        metadata, token_usage, finish_reason, turn_index,
		        created_at
		 FROM chat_message
		 WHERE session_id = $1 AND role = 'assistant' AND created_at > $2
		 ORDER BY created_at DESC
		 LIMIT 1`,
		sessionID, after,
	).Scan(
		&m.ID, &m.SessionID, &m.Role, &m.Content,
		&m.MessageType, &m.ToolCalls, &m.ToolCallID,
		&m.Metadata, &m.TokenUsage, &m.FinishReason, &m.TurnIndex, &m.RunID,
		&m.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ChatMessage{}, false, nil
		}
		return ChatMessage{}, false, fmt.Errorf("chat: get latest assistant message: %w", err)
	}

	return m, true, nil
}

func (r *postgresRepository) FindAllMessages(ctx context.Context, sessionID uuid.UUID) ([]ChatMessage, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return nil, err
	}
	defer release()

	rows, err := conn.Query(ctx,
		`SELECT id, session_id, role, content,
		        message_type, tool_calls, tool_call_id,
		        metadata, token_usage, finish_reason, turn_index, run_id,
		        created_at
		 FROM chat_message
		 WHERE session_id = $1
		 ORDER BY created_at ASC`,
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("chat: find all messages: %w", err)
	}
	defer rows.Close()

	var items []ChatMessage
	for rows.Next() {
		var m ChatMessage
		if err := rows.Scan(
			&m.ID, &m.SessionID, &m.Role, &m.Content,
			&m.MessageType, &m.ToolCalls, &m.ToolCallID,
			&m.Metadata, &m.TokenUsage, &m.FinishReason, &m.TurnIndex, &m.RunID,
			&m.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("chat: scan message: %w", err)
		}
		items = append(items, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chat: rows: %w", err)
	}

	return items, nil
}

func (r *postgresRepository) GetLatestCompactSummary(ctx context.Context, sessionID uuid.UUID) (ChatMessage, bool, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return ChatMessage{}, false, err
	}
	defer release()

	var m ChatMessage
	err = conn.QueryRow(ctx,
		`SELECT id, session_id, role, content,
		        message_type, tool_calls, tool_call_id,
		        metadata, token_usage, finish_reason, turn_index,
		        created_at
		 FROM chat_message
		 WHERE session_id = $1 AND message_type = 'compact_summary'
		 ORDER BY created_at DESC
		 LIMIT 1`,
		sessionID,
	).Scan(
		&m.ID, &m.SessionID, &m.Role, &m.Content,
		&m.MessageType, &m.ToolCalls, &m.ToolCallID,
		&m.Metadata, &m.TokenUsage, &m.FinishReason, &m.TurnIndex, &m.RunID,
		&m.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ChatMessage{}, false, nil
		}
		return ChatMessage{}, false, fmt.Errorf("chat: get latest compact summary: %w", err)
	}

	return m, true, nil
}

func (r *postgresRepository) CreateRun(ctx context.Context, run ChatRun) (ChatRun, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return ChatRun{}, err
	}
	defer release()

	run.ID = uuid.New()
	if run.Status == "" {
		run.Status = ChatRunStatusActive
	}
	// P-C299-1: only set StartedAt for runs that are immediately active.
	// Queued runs have no started_at until the worker transitions them to active.
	if run.Status != ChatRunStatusQueued {
		run.StartedAt = time.Now().UTC()
	}

	// P-C299-1: for queued runs, omit started_at so DB default (NULL) is used.
	// started_at is set by UpdateRunStatus when the worker transitions to active.
	var execErr error
	if run.Status == ChatRunStatusQueued {
		_, execErr = conn.Exec(ctx,
			`INSERT INTO chat_run (id, session_id, tenant_id, status, last_event_id, metadata, started_at)
			 VALUES ($1, $2, $3, $4, $5, $6, NULL)`,
			run.ID, run.SessionID, run.TenantID, run.Status, run.LastEventID, run.Metadata,
		)
	} else {
		_, execErr = conn.Exec(ctx,
			`INSERT INTO chat_run (id, session_id, tenant_id, status, last_event_id, metadata, started_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			run.ID, run.SessionID, run.TenantID, run.Status, run.LastEventID, run.Metadata, run.StartedAt,
		)
	}
	if execErr != nil {
		// P-C99-1: uq_chat_run_session_active prevents two concurrent active runs per session.
		if database.IsPgError(execErr, database.PgErrUniqueViolation) {
			return ChatRun{}, ErrRunAlreadyActive
		}
		return ChatRun{}, fmt.Errorf("chat: create run: %w", execErr)
	}

	return run, nil
}

func (r *postgresRepository) GetRunByID(ctx context.Context, id uuid.UUID) (ChatRun, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return ChatRun{}, err
	}
	defer release()

	var run ChatRun
	err = conn.QueryRow(ctx,
		`SELECT id, session_id, tenant_id, status, last_event_id, metadata, started_at, completed_at, created_at
		 FROM chat_run WHERE id = $1`,
		id,
	).Scan(&run.ID, &run.SessionID, &run.TenantID, &run.Status, &run.LastEventID, &run.Metadata, &run.StartedAt, &run.CompletedAt, &run.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ChatRun{}, ErrNotFound
		}
		return ChatRun{}, fmt.Errorf("chat: get run by id: %w", err)
	}

	return run, nil
}

func (r *postgresRepository) GetActiveRunBySession(ctx context.Context, sessionID uuid.UUID) (ChatRun, bool, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return ChatRun{}, false, err
	}
	defer release()

	var run ChatRun
	err = conn.QueryRow(ctx,
		// P-C299-1: include 'queued' runs so that runs published to RabbitMQ but
		// not yet started by a worker also block concurrent run creation.
		`SELECT id, session_id, tenant_id, status, last_event_id, metadata, started_at, completed_at, created_at
		 FROM chat_run WHERE session_id = $1 AND status IN ('queued', 'active')
		 ORDER BY created_at DESC LIMIT 1`,
		sessionID,
	).Scan(&run.ID, &run.SessionID, &run.TenantID, &run.Status, &run.LastEventID, &run.Metadata, &run.StartedAt, &run.CompletedAt, &run.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ChatRun{}, false, nil
		}
		return ChatRun{}, false, fmt.Errorf("chat: get active run by session: %w", err)
	}

	return run, true, nil
}

func (r *postgresRepository) UpdateRunStatus(ctx context.Context, id uuid.UUID, status ChatRunStatus, lastEventID string) error {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return err
	}
	defer release()

	// P-C299-1: when transitioning queued → active, also set started_at.
	if status == ChatRunStatusActive {
		_, err = conn.Exec(ctx,
			`UPDATE chat_run SET status = $1, last_event_id = $2, started_at = COALESCE(started_at, NOW()) WHERE id = $3`,
			status, lastEventID, id,
		)
	} else {
		_, err = conn.Exec(ctx,
			`UPDATE chat_run SET status = $1, last_event_id = $2 WHERE id = $3`,
			status, lastEventID, id,
		)
	}
	if err != nil {
		return fmt.Errorf("chat: update run status: %w", err)
	}

	return nil
}

func (r *postgresRepository) MarkRunCompleted(ctx context.Context, id uuid.UUID) error {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return err
	}
	defer release()

	now := time.Now().UTC()
	_, err = conn.Exec(ctx,
		`UPDATE chat_run SET status = $1, completed_at = $2 WHERE id = $3`,
		ChatRunStatusCompleted, now, id,
	)
	if err != nil {
		return fmt.Errorf("chat: mark run completed: %w", err)
	}

	return nil
}

// UpdateSessionConfigHash persists the SHA-256 hash of the current modelConfig.
// P-C173-1: used to detect config changes between turns and inject a system notification.
func (r *postgresRepository) UpdateSessionConfigHash(ctx context.Context, sessionID uuid.UUID, hash string) error {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return err
	}
	defer release()

	tag, err := conn.Exec(ctx,
		`UPDATE chat_session SET config_hash = $1, updated_at = $2 WHERE id = $3`,
		hash, time.Now().UTC(), sessionID,
	)
	if err != nil {
		return fmt.Errorf("chat: update session config hash: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
