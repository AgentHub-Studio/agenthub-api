package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreAgent represents a platform-managed specialist agent from ah_core.agent.
type CoreAgent struct {
	ID               uuid.UUID
	Name             string
	Slug             string
	Description      string
	AgentType        string
	SystemPrompt     string
	ModelConfig      json.RawMessage
	EnableManagement bool
	IsActive         bool
}

// CoreAgentLoader loads platform-managed specialist agents from the ah_core schema.
type CoreAgentLoader struct {
	pool *pgxpool.Pool
}

// NewCoreAgentLoader creates a CoreAgentLoader.
func NewCoreAgentLoader(pool *pgxpool.Pool) *CoreAgentLoader {
	return &CoreAgentLoader{pool: pool}
}

// LoadAll returns all active agents from ah_core.agent.
// Returns an empty slice (not an error) if the ah_core schema does not exist.
func (l *CoreAgentLoader) LoadAll(ctx context.Context) ([]CoreAgent, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, name, slug, description, agent_type, system_prompt, model_config, enable_management, is_active
		  FROM ah_core.agent
		 WHERE is_active = true
		 ORDER BY name`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		slog.WarnContext(ctx, "core: ah_core.agent not accessible, core agents unavailable", "err", err)
		return nil, nil
	}
	defer rows.Close()

	var agents []CoreAgent
	for rows.Next() {
		var a CoreAgent
		var sp *string
		if err := rows.Scan(&a.ID, &a.Name, &a.Slug, &a.Description, &a.AgentType, &sp, &a.ModelConfig, &a.EnableManagement, &a.IsActive); err != nil {
			return nil, fmt.Errorf("core: scan agent: %w", err)
		}
		if sp != nil {
			a.SystemPrompt = *sp
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

// ListSpecialists returns all active specialist agents (agent_type = 'SPECIALIST').
func (l *CoreAgentLoader) ListSpecialists(ctx context.Context) ([]CoreAgent, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var specialists []CoreAgent
	for _, a := range all {
		if a.AgentType == "SPECIALIST" {
			specialists = append(specialists, a)
		}
	}
	return specialists, nil
}

// CoreAgentResponse is the DTO returned by GET /api/core/agents.
type CoreAgentResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	AgentType   string    `json:"agentType"`
}

// AgentResponseFrom converts a CoreAgent to CoreAgentResponse.
func AgentResponseFrom(a CoreAgent) CoreAgentResponse {
	return CoreAgentResponse{
		ID:          a.ID,
		Name:        a.Name,
		Slug:        a.Slug,
		Description: a.Description,
		AgentType:   a.AgentType,
	}
}
