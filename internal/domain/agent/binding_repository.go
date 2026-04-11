package agent

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// BindingRepository manages agent_skill, agent_knowledge_base, and agent_mcp_server join tables.
type BindingRepository interface {
	ListSkillIDs(ctx context.Context, agentID uuid.UUID) ([]uuid.UUID, error)
	SyncSkills(ctx context.Context, agentID uuid.UUID, skillIDs []uuid.UUID) error
	ListKnowledgeBaseIDs(ctx context.Context, agentID uuid.UUID) ([]uuid.UUID, error)
	SyncKnowledgeBases(ctx context.Context, agentID uuid.UUID, kbIDs []uuid.UUID) error
	// P-C253-1: per-agent MCP server bindings
	ListMCPServerIDs(ctx context.Context, agentID uuid.UUID) ([]uuid.UUID, error)
	SyncMCPServers(ctx context.Context, agentID uuid.UUID, mcpServerIDs []uuid.UUID) error
	// GetSkillTokenBudgets returns the token_budget overrides from agent_skill for each
	// skill bound to the given agent. Nil values indicate no budget limit.
	GetSkillTokenBudgets(ctx context.Context, agentID uuid.UUID) (map[uuid.UUID]*int, error)
}

type pgBindingRepository struct {
	pool *pgxpool.Pool
}

// NewBindingRepository creates a new BindingRepository backed by PostgreSQL.
func NewBindingRepository(pool *pgxpool.Pool) BindingRepository {
	return &pgBindingRepository{pool: pool}
}

func (r *pgBindingRepository) acquire(ctx context.Context) (*pgxpool.Conn, func(), error) {
	return database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
}

// ListSkillIDs returns the IDs of skills bound to the given agent.
func (r *pgBindingRepository) ListSkillIDs(ctx context.Context, agentID uuid.UUID) ([]uuid.UUID, error) {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("agent.ListSkillIDs: acquire: %w", err)
	}
	defer release()

	rows, err := conn.Query(ctx,
		// P-C289-1: order by priority ASC so higher-priority skills appear first in
		// the LLM system prompt, then by created_at for stable tie-breaking.
		`SELECT skill_id FROM agent_skill WHERE agent_id = $1 ORDER BY priority ASC, created_at ASC`, agentID)
	if err != nil {
		return nil, fmt.Errorf("agent.ListSkillIDs: %w", err)
	}
	defer rows.Close()

	ids := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("agent.ListSkillIDs scan: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// SyncSkills replaces all skill bindings for the given agent with the provided IDs.
func (r *pgBindingRepository) SyncSkills(ctx context.Context, agentID uuid.UUID, skillIDs []uuid.UUID) error {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return fmt.Errorf("agent.SyncSkills: acquire: %w", err)
	}
	defer release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("agent.SyncSkills: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM agent_skill WHERE agent_id = $1`, agentID); err != nil {
		return fmt.Errorf("agent.SyncSkills: delete: %w", err)
	}

	// P-C289-1: use list index as priority so the caller's ordering is preserved.
	for i, sid := range skillIDs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO agent_skill (agent_id, skill_id, priority) VALUES ($1, $2, $3)
			 ON CONFLICT (agent_id, skill_id) DO UPDATE SET priority = EXCLUDED.priority`,
			agentID, sid, i,
		); err != nil {
			if database.IsPgError(err, database.PgErrForeignKeyViolation) {
				return fmt.Errorf("skill not found: %s", sid)
			}
			return fmt.Errorf("agent.SyncSkills: insert %s: %w", sid, err)
		}
	}

	return tx.Commit(ctx)
}

// ListKnowledgeBaseIDs returns the IDs of knowledge bases bound to the given agent.
func (r *pgBindingRepository) ListKnowledgeBaseIDs(ctx context.Context, agentID uuid.UUID) ([]uuid.UUID, error) {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("agent.ListKnowledgeBaseIDs: acquire: %w", err)
	}
	defer release()

	rows, err := conn.Query(ctx,
		`SELECT knowledge_base_id FROM agent_knowledge_base WHERE agent_id = $1 ORDER BY created_at`, agentID)
	if err != nil {
		return nil, fmt.Errorf("agent.ListKnowledgeBaseIDs: %w", err)
	}
	defer rows.Close()

	ids := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("agent.ListKnowledgeBaseIDs scan: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// SyncKnowledgeBases replaces all knowledge base bindings for the given agent with the provided IDs.
func (r *pgBindingRepository) SyncKnowledgeBases(ctx context.Context, agentID uuid.UUID, kbIDs []uuid.UUID) error {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return fmt.Errorf("agent.SyncKnowledgeBases: acquire: %w", err)
	}
	defer release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("agent.SyncKnowledgeBases: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM agent_knowledge_base WHERE agent_id = $1`, agentID); err != nil {
		return fmt.Errorf("agent.SyncKnowledgeBases: delete: %w", err)
	}

	for _, kbID := range kbIDs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO agent_knowledge_base (agent_id, knowledge_base_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			agentID, kbID,
		); err != nil {
			return fmt.Errorf("agent.SyncKnowledgeBases: insert %s: %w", kbID, err)
		}
	}

	return tx.Commit(ctx)
}

// ListMCPServerIDs returns the IDs of MCP servers bound to the given agent.
// P-C253-1: agent-level MCP binding.
func (r *pgBindingRepository) ListMCPServerIDs(ctx context.Context, agentID uuid.UUID) ([]uuid.UUID, error) {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("agent.ListMCPServerIDs: acquire: %w", err)
	}
	defer release()

	rows, err := conn.Query(ctx,
		`SELECT mcp_server_id FROM agent_mcp_server WHERE agent_id = $1 ORDER BY created_at`, agentID)
	if err != nil {
		return nil, fmt.Errorf("agent.ListMCPServerIDs: %w", err)
	}
	defer rows.Close()

	ids := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("agent.ListMCPServerIDs scan: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetSkillTokenBudgets returns the token_budget column from agent_skill for each
// skill bound to the given agent. Nil values indicate no budget limit for that skill.
func (r *pgBindingRepository) GetSkillTokenBudgets(ctx context.Context, agentID uuid.UUID) (map[uuid.UUID]*int, error) {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("agent.GetSkillTokenBudgets: acquire: %w", err)
	}
	defer release()

	rows, err := conn.Query(ctx,
		`SELECT skill_id, token_budget FROM agent_skill WHERE agent_id = $1`, agentID)
	if err != nil {
		return nil, fmt.Errorf("agent.GetSkillTokenBudgets: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID]*int)
	for rows.Next() {
		var skillID uuid.UUID
		var budget *int
		if err := rows.Scan(&skillID, &budget); err != nil {
			return nil, fmt.Errorf("agent.GetSkillTokenBudgets scan: %w", err)
		}
		result[skillID] = budget
	}
	return result, rows.Err()
}

// SyncMCPServers replaces all MCP server bindings for the given agent.
// P-C253-1: agent-level MCP binding.
func (r *pgBindingRepository) SyncMCPServers(ctx context.Context, agentID uuid.UUID, mcpServerIDs []uuid.UUID) error {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return fmt.Errorf("agent.SyncMCPServers: acquire: %w", err)
	}
	defer release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("agent.SyncMCPServers: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM agent_mcp_server WHERE agent_id = $1`, agentID); err != nil {
		return fmt.Errorf("agent.SyncMCPServers: delete: %w", err)
	}

	for _, mcpID := range mcpServerIDs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO agent_mcp_server (agent_id, mcp_server_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			agentID, mcpID,
		); err != nil {
			return fmt.Errorf("agent.SyncMCPServers: insert %s: %w", mcpID, err)
		}
	}

	return tx.Commit(ctx)
}
