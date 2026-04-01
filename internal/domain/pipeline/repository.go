package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// ErrNotFound is returned when a pipeline is not found.
var ErrNotFound = errors.New("pipeline: not found")

// Repository handles persistence for pipelines, nodes and edges.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new Repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// List returns a paginated list of pipelines for the current tenant.
func (r *Repository) List(ctx context.Context, req pagination.PageRequest) ([]Pipeline, int64, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return nil, 0, err
	}
	defer release()

	var total int64
	if err := conn.QueryRow(ctx, `SELECT COUNT(*) FROM pipeline`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("pipeline: count: %w", err)
	}

	rows, err := conn.Query(ctx,
		`SELECT id, name, description, agent_id, status, config, created_at, updated_at
		 FROM pipeline
		 ORDER BY created_at DESC
		 LIMIT $1 OFFSET $2`,
		req.Size, req.Offset(),
	)
	if err != nil {
		return nil, 0, fmt.Errorf("pipeline: list: %w", err)
	}
	defer rows.Close()

	pipelines, err := scanPipelines(rows)
	if err != nil {
		return nil, 0, err
	}
	return pipelines, total, nil
}

// Create inserts a new pipeline.
func (r *Repository) Create(ctx context.Context, p Pipeline) (Pipeline, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return Pipeline{}, err
	}
	defer release()

	config := p.Config
	if len(config) == 0 {
		config = []byte("{}")
	}

	row := conn.QueryRow(ctx,
		`INSERT INTO pipeline (name, description, agent_id, status, config)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, name, description, agent_id, status, config, created_at, updated_at`,
		p.Name, p.Description, p.AgentID, p.Status, config,
	)
	return scanPipeline(row)
}

// GetByID returns a pipeline with its nodes and edges.
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (Pipeline, []Node, []Edge, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return Pipeline{}, nil, nil, err
	}
	defer release()

	row := conn.QueryRow(ctx,
		`SELECT id, name, description, agent_id, status, config, created_at, updated_at
		 FROM pipeline WHERE id = $1`, id,
	)
	p, err := scanPipeline(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Pipeline{}, nil, nil, ErrNotFound
		}
		return Pipeline{}, nil, nil, err
	}

	nodes, err := r.listNodes(ctx, conn, id)
	if err != nil {
		return Pipeline{}, nil, nil, err
	}
	edges, err := r.listEdges(ctx, conn, id)
	if err != nil {
		return Pipeline{}, nil, nil, err
	}
	return p, nodes, edges, nil
}

// Update modifies an existing pipeline.
func (r *Repository) Update(ctx context.Context, id uuid.UUID, req UpdateRequest) (Pipeline, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return Pipeline{}, err
	}
	defer release()

	config := []byte("{}")
	if len(req.Config) > 0 {
		config = req.Config
	}
	status := req.Status
	if status == "" {
		status = "DRAFT"
	}

	row := conn.QueryRow(ctx,
		`UPDATE pipeline SET name=$1, description=$2, status=$3, config=$4, updated_at=NOW()
		 WHERE id=$5
		 RETURNING id, name, description, agent_id, status, config, created_at, updated_at`,
		req.Name, req.Description, status, config, id,
	)
	p, err := scanPipeline(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Pipeline{}, ErrNotFound
		}
		return Pipeline{}, err
	}
	return p, nil
}

// Delete removes a pipeline by ID.
func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return err
	}
	defer release()

	tag, err := conn.Exec(ctx, `DELETE FROM pipeline WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("pipeline: delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ReplaceNodes replaces all nodes for a pipeline.
func (r *Repository) ReplaceNodes(ctx context.Context, pipelineID uuid.UUID, nodes []NodeRequest) ([]Node, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return nil, err
	}
	defer release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("pipeline: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, `DELETE FROM pipeline_node WHERE pipeline_id=$1`, pipelineID); err != nil {
		return nil, fmt.Errorf("pipeline: delete nodes: %w", err)
	}

	result := make([]Node, 0, len(nodes))
	for _, n := range nodes {
		cfg := []byte("{}")
		if len(n.Config) > 0 {
			cfg = n.Config
		}
		row := tx.QueryRow(ctx,
			`INSERT INTO pipeline_node (pipeline_id, node_type, name, config, position_x, position_y)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 RETURNING id, pipeline_id, node_type, name, config, position_x, position_y, created_at`,
			pipelineID, n.NodeType, n.Name, cfg, n.PositionX, n.PositionY,
		)
		node, err := scanNode(row)
		if err != nil {
			return nil, fmt.Errorf("pipeline: insert node: %w", err)
		}
		result = append(result, node)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("pipeline: commit tx: %w", err)
	}
	return result, nil
}

// ReplaceEdges replaces all edges for a pipeline.
func (r *Repository) ReplaceEdges(ctx context.Context, pipelineID uuid.UUID, edges []EdgeRequest) ([]Edge, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return nil, err
	}
	defer release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("pipeline: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, `DELETE FROM pipeline_edge WHERE pipeline_id=$1`, pipelineID); err != nil {
		return nil, fmt.Errorf("pipeline: delete edges: %w", err)
	}

	result := make([]Edge, 0, len(edges))
	for _, e := range edges {
		row := tx.QueryRow(ctx,
			`INSERT INTO pipeline_edge (pipeline_id, source_node_id, target_node_id, label)
			 VALUES ($1, $2, $3, $4)
			 RETURNING id, pipeline_id, source_node_id, target_node_id, label, created_at`,
			pipelineID, e.SourceNodeID, e.TargetNodeID, e.Label,
		)
		edge, err := scanEdge(row)
		if err != nil {
			return nil, fmt.Errorf("pipeline: insert edge: %w", err)
		}
		result = append(result, edge)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("pipeline: commit tx: %w", err)
	}
	return result, nil
}

// --- internal scan helpers ---

func (r *Repository) listNodes(ctx context.Context, conn *pgxpool.Conn, pipelineID uuid.UUID) ([]Node, error) {
	rows, err := conn.Query(ctx,
		`SELECT id, pipeline_id, node_type, name, config, position_x, position_y, created_at
		 FROM pipeline_node WHERE pipeline_id=$1 ORDER BY created_at`, pipelineID,
	)
	if err != nil {
		return nil, fmt.Errorf("pipeline: list nodes: %w", err)
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var n Node
		var cfg []byte
		if err := rows.Scan(&n.ID, &n.PipelineID, &n.NodeType, &n.Name, &cfg, &n.PositionX, &n.PositionY, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("pipeline: scan node: %w", err)
		}
		n.Config = cfg
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

func (r *Repository) listEdges(ctx context.Context, conn *pgxpool.Conn, pipelineID uuid.UUID) ([]Edge, error) {
	rows, err := conn.Query(ctx,
		`SELECT id, pipeline_id, source_node_id, target_node_id, label, created_at
		 FROM pipeline_edge WHERE pipeline_id=$1 ORDER BY created_at`, pipelineID,
	)
	if err != nil {
		return nil, fmt.Errorf("pipeline: list edges: %w", err)
	}
	defer rows.Close()

	var edges []Edge
	for rows.Next() {
		e, err := scanEdgeFromRows(rows)
		if err != nil {
			return nil, err
		}
		edges = append(edges, e)
	}
	return edges, rows.Err()
}

func scanPipelines(rows pgx.Rows) ([]Pipeline, error) {
	var result []Pipeline
	for rows.Next() {
		var p Pipeline
		var cfg []byte
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.AgentID, &p.Status, &cfg, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("pipeline: scan: %w", err)
		}
		p.Config = cfg
		result = append(result, p)
	}
	return result, rows.Err()
}

func scanPipeline(row pgx.Row) (Pipeline, error) {
	var p Pipeline
	var cfg []byte
	if err := row.Scan(&p.ID, &p.Name, &p.Description, &p.AgentID, &p.Status, &cfg, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return Pipeline{}, fmt.Errorf("pipeline: scan: %w", err)
	}
	p.Config = cfg
	return p, nil
}

func scanNode(row pgx.Row) (Node, error) {
	var n Node
	var cfg []byte
	if err := row.Scan(&n.ID, &n.PipelineID, &n.NodeType, &n.Name, &cfg, &n.PositionX, &n.PositionY, &n.CreatedAt); err != nil {
		return Node{}, fmt.Errorf("pipeline: scan node: %w", err)
	}
	n.Config = cfg
	return n, nil
}

func scanEdge(row pgx.Row) (Edge, error) {
	var e Edge
	if err := row.Scan(&e.ID, &e.PipelineID, &e.SourceNodeID, &e.TargetNodeID, &e.Label, &e.CreatedAt); err != nil {
		return Edge{}, fmt.Errorf("pipeline: scan edge: %w", err)
	}
	return e, nil
}

func scanEdgeFromRows(rows pgx.Rows) (Edge, error) {
	var e Edge
	if err := rows.Scan(&e.ID, &e.PipelineID, &e.SourceNodeID, &e.TargetNodeID, &e.Label, &e.CreatedAt); err != nil {
		return Edge{}, fmt.Errorf("pipeline: scan edge: %w", err)
	}
	return e, nil
}

// ensure json import is used
var _ = json.Marshal
