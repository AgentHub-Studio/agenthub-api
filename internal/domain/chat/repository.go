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
	GetSessionByID(ctx context.Context, id uuid.UUID) (ChatSession, error)
	CreateSession(ctx context.Context, s ChatSession) (ChatSession, error)
	UpdateSessionStatus(ctx context.Context, id uuid.UUID, status ChatStatus) (ChatSession, error)
	DeleteSession(ctx context.Context, id uuid.UUID) error
	FindMessages(ctx context.Context, sessionID uuid.UUID, req pagination.PageRequest) ([]ChatMessage, int64, error)
	CreateMessage(ctx context.Context, m ChatMessage) (ChatMessage, error)
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

func (r *postgresRepository) GetSessionByID(ctx context.Context, id uuid.UUID) (ChatSession, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return ChatSession{}, err
	}
	defer release()

	var s ChatSession
	err = conn.QueryRow(ctx,
		`SELECT id, agent_id, title, status, created_at, updated_at
		 FROM chat_session WHERE id = $1`,
		id,
	).Scan(&s.ID, &s.AgentID, &s.Title, &s.Status, &s.CreatedAt, &s.UpdatedAt)
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
		`INSERT INTO chat_session (id, agent_id, title, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		s.ID, s.AgentID, s.Title, s.Status, s.CreatedAt, s.UpdatedAt,
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
		`SELECT id, session_id, role, content, created_at
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
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
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

	_, err = conn.Exec(ctx,
		`INSERT INTO chat_message (id, session_id, role, content, created_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		m.ID, m.SessionID, m.Role, m.Content, m.CreatedAt,
	)
	if err != nil {
		return ChatMessage{}, fmt.Errorf("chat: create message: %w", err)
	}

	return m, nil
}
