package tool

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// ErrNotFound is returned when a tool is not found.
var ErrNotFound = errors.New("tool: not found")

// ErrAlreadyBound is returned when a tool is already bound to a skill.
var ErrAlreadyBound = errors.New("tool: already bound to skill")

// ToolRepository defines the persistence interface for Tool.
type ToolRepository interface {
	List(ctx context.Context, req pagination.PageRequest, toolType string) ([]Tool, int64, error)
	ListLabels(ctx context.Context) ([]string, error)
	Create(ctx context.Context, t Tool) (Tool, error)
	GetByID(ctx context.Context, id uuid.UUID) (Tool, error)
	// Update replaces a tool's mutable fields. The caller is responsible for
	// merging the existing tool with the patch before calling Update so that
	// unset fields are preserved. P-C196-1.
	Update(ctx context.Context, id uuid.UUID, t Tool) (Tool, error)
	Delete(ctx context.Context, id uuid.UUID) error
	BindToSkill(ctx context.Context, skillID uuid.UUID, req BindRequest) (SkillTool, error)
	UnbindFromSkill(ctx context.Context, skillID, toolID uuid.UUID) error
	ListBySkill(ctx context.Context, skillID uuid.UUID) ([]SkillTool, []Tool, error)
}

// Repository handles persistence for tools and skill-tool bindings.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new Repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// List returns a paginated list of tools, optionally filtered by type.
func (r *Repository) List(ctx context.Context, req pagination.PageRequest, toolType string) ([]Tool, int64, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return nil, 0, err
	}
	defer release()

	var (
		total int64
		rows  pgx.Rows
	)
	if toolType != "" {
		if err := conn.QueryRow(ctx, `SELECT COUNT(*) FROM tool WHERE type=$1`, toolType).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("tool: count: %w", err)
		}
		rows, err = conn.Query(ctx,
			`SELECT id, name, type, config, input_schema, description, labels, read_only,
			        should_defer, is_destructive, search_hint, always_load, concurrency_safe,
			        max_result_chars, interrupt_behavior, is_search_or_read,
			        created_at, updated_at
			 FROM tool WHERE type=$1 ORDER BY name LIMIT $2 OFFSET $3`,
			toolType, req.Size, req.Offset(),
		)
	} else {
		if err := conn.QueryRow(ctx, `SELECT COUNT(*) FROM tool`).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("tool: count: %w", err)
		}
		rows, err = conn.Query(ctx,
			`SELECT id, name, type, config, input_schema, description, labels, read_only,
			        should_defer, is_destructive, search_hint, always_load, concurrency_safe,
			        max_result_chars, interrupt_behavior, is_search_or_read,
			        created_at, updated_at
			 FROM tool ORDER BY name LIMIT $1 OFFSET $2`,
			req.Size, req.Offset(),
		)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("tool: list: %w", err)
	}
	defer rows.Close()

	var tools []Tool
	for rows.Next() {
		t, err := scanToolFromRows(rows)
		if err != nil {
			return nil, 0, err
		}
		tools = append(tools, t)
	}
	return tools, total, rows.Err()
}

// Create inserts a new tool.
func (r *Repository) Create(ctx context.Context, t Tool) (Tool, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return Tool{}, err
	}
	defer release()

	cfg := t.Config
	if len(cfg) == 0 {
		cfg = []byte("{}")
	}

	labels := t.Labels
	if labels == nil {
		labels = []string{}
	}

	var inputSchema interface{}
	if len(t.InputSchema) > 0 {
		inputSchema = []byte(t.InputSchema)
	}

	row := conn.QueryRow(ctx,
		`INSERT INTO tool (name, type, config, input_schema, description, labels, read_only,
		                   should_defer, is_destructive, search_hint, always_load,
		                   concurrency_safe, max_result_chars, interrupt_behavior, is_search_or_read)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		 RETURNING id, name, type, config, input_schema, description, labels, read_only,
		           should_defer, is_destructive, search_hint, always_load, concurrency_safe,
		           max_result_chars, interrupt_behavior, is_search_or_read,
		           created_at, updated_at`,
		t.Name, t.Type, cfg, inputSchema, t.Description, labels, t.ReadOnly,
		t.ShouldDefer, t.IsDestructive, t.SearchHint, t.AlwaysLoad,
		t.ConcurrencySafe, t.MaxResultChars, t.InterruptBehavior, t.IsSearchOrRead,
	)
	return scanTool(row)
}

// GetByID returns a tool by ID.
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (Tool, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return Tool{}, err
	}
	defer release()

	row := conn.QueryRow(ctx,
		`SELECT id, name, type, config, input_schema, description, labels, read_only,
		        should_defer, is_destructive, search_hint, always_load, concurrency_safe,
		        max_result_chars, interrupt_behavior, is_search_or_read,
		        created_at, updated_at
		 FROM tool WHERE id=$1`, id,
	)
	t, err := scanTool(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Tool{}, ErrNotFound
		}
		return Tool{}, err
	}
	return t, nil
}

// Update replaces a tool's mutable fields. The caller must first merge the
// existing tool with the patch (via service.Update) so that unset fields are
// preserved. P-C196-1.
func (r *Repository) Update(ctx context.Context, id uuid.UUID, t Tool) (Tool, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return Tool{}, err
	}
	defer release()

	cfg := []byte("{}")
	if len(t.Config) > 0 {
		cfg = t.Config
	}
	labels := t.Labels
	if labels == nil {
		labels = []string{}
	}

	var inputSchema interface{}
	if len(t.InputSchema) > 0 {
		inputSchema = []byte(t.InputSchema)
	}

	row := conn.QueryRow(ctx,
		`UPDATE tool SET name=$1, type=$2, config=$3, input_schema=$4, description=$5, labels=$6, read_only=$7, updated_at=NOW()
		 WHERE id=$8
		 RETURNING id, name, type, config, input_schema, description, labels, read_only,
		           should_defer, is_destructive, search_hint, always_load, concurrency_safe,
		           max_result_chars, interrupt_behavior, is_search_or_read,
		           created_at, updated_at`,
		t.Name, t.Type, cfg, inputSchema, t.Description, labels, t.ReadOnly, id,
	)
	updated, err := scanTool(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Tool{}, ErrNotFound
		}
		return Tool{}, err
	}
	return updated, nil
}

// Delete removes a tool by ID.
func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return err
	}
	defer release()

	tag, err := conn.Exec(ctx, `DELETE FROM tool WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("tool: delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// BindToSkill creates a skill_tool binding.
func (r *Repository) BindToSkill(ctx context.Context, skillID uuid.UUID, req BindRequest) (SkillTool, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return SkillTool{}, err
	}
	defer release()

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	var st SkillTool
	row := conn.QueryRow(ctx,
		`INSERT INTO skill_tool (skill_id, tool_id, priority, is_active)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, skill_id, tool_id, priority, is_active, created_at`,
		skillID, req.ToolID, req.Priority, isActive,
	)
	if err := row.Scan(&st.ID, &st.SkillID, &st.ToolID, &st.Priority, &st.IsActive, &st.CreatedAt); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return SkillTool{}, ErrAlreadyBound
		}
		return SkillTool{}, fmt.Errorf("tool: bind to skill: %w", err)
	}
	return st, nil
}

// UnbindFromSkill removes a skill_tool binding.
func (r *Repository) UnbindFromSkill(ctx context.Context, skillID, toolID uuid.UUID) error {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return err
	}
	defer release()

	tag, err := conn.Exec(ctx, `DELETE FROM skill_tool WHERE skill_id=$1 AND tool_id=$2`, skillID, toolID)
	if err != nil {
		return fmt.Errorf("tool: unbind from skill: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListBySkill returns all tools bound to a skill.
func (r *Repository) ListBySkill(ctx context.Context, skillID uuid.UUID) ([]SkillTool, []Tool, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return nil, nil, err
	}
	defer release()

	rows, err := conn.Query(ctx,
		`SELECT st.id, st.skill_id, st.tool_id, st.priority, st.is_active, st.created_at,
		        t.id, t.name, t.type, t.config, t.input_schema, t.description, t.labels, t.read_only,
		        t.should_defer, t.is_destructive, t.search_hint, t.always_load, t.concurrency_safe,
		        t.max_result_chars, t.interrupt_behavior, t.is_search_or_read,
		        t.created_at, t.updated_at
		 FROM skill_tool st
		 JOIN tool t ON t.id = st.tool_id
		 WHERE st.skill_id=$1
		 ORDER BY st.priority, st.created_at`,
		skillID,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("tool: list by skill: %w", err)
	}
	defer rows.Close()

	bindings := []SkillTool{}
	tools := []Tool{}
	for rows.Next() {
		var st SkillTool
		var t Tool
		var cfg []byte
		var inputSchema []byte
		if err := rows.Scan(
			&st.ID, &st.SkillID, &st.ToolID, &st.Priority, &st.IsActive, &st.CreatedAt,
			&t.ID, &t.Name, &t.Type, &cfg, &inputSchema, &t.Description, &t.Labels, &t.ReadOnly,
			&t.ShouldDefer, &t.IsDestructive, &t.SearchHint, &t.AlwaysLoad, &t.ConcurrencySafe,
			&t.MaxResultChars, &t.InterruptBehavior, &t.IsSearchOrRead,
			&t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, nil, fmt.Errorf("tool: scan skill_tool: %w", err)
		}
		t.Config = cfg
		if len(inputSchema) > 0 {
			t.InputSchema = inputSchema
		}
		bindings = append(bindings, st)
		tools = append(tools, t)
	}
	return bindings, tools, rows.Err()
}

// --- scan helpers ---

func scanTool(row pgx.Row) (Tool, error) {
	var t Tool
	var cfg []byte
	var inputSchema []byte
	if err := row.Scan(
		&t.ID, &t.Name, &t.Type, &cfg, &inputSchema, &t.Description, &t.Labels, &t.ReadOnly,
		&t.ShouldDefer, &t.IsDestructive, &t.SearchHint, &t.AlwaysLoad, &t.ConcurrencySafe,
		&t.MaxResultChars, &t.InterruptBehavior, &t.IsSearchOrRead,
		&t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		return Tool{}, fmt.Errorf("tool: scan: %w", err)
	}
	t.Config = cfg
	if len(inputSchema) > 0 {
		t.InputSchema = inputSchema
	}
	return t, nil
}

func scanToolFromRows(rows pgx.Rows) (Tool, error) {
	var t Tool
	var cfg []byte
	var inputSchema []byte
	if err := rows.Scan(
		&t.ID, &t.Name, &t.Type, &cfg, &inputSchema, &t.Description, &t.Labels, &t.ReadOnly,
		&t.ShouldDefer, &t.IsDestructive, &t.SearchHint, &t.AlwaysLoad, &t.ConcurrencySafe,
		&t.MaxResultChars, &t.InterruptBehavior, &t.IsSearchOrRead,
		&t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		return Tool{}, fmt.Errorf("tool: scan: %w", err)
	}
	t.Config = cfg
	if len(inputSchema) > 0 {
		t.InputSchema = inputSchema
	}
	return t, nil
}

// ListLabels returns all distinct labels used across tools.
func (r *Repository) ListLabels(ctx context.Context) ([]string, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return nil, err
	}
	defer release()

	rows, err := conn.Query(ctx,
		`SELECT DISTINCT unnest(labels) AS label FROM tool WHERE array_length(labels, 1) > 0 ORDER BY label`,
	)
	if err != nil {
		return nil, fmt.Errorf("tool: list labels: %w", err)
	}
	defer rows.Close()

	var labels []string
	for rows.Next() {
		var label string
		if err := rows.Scan(&label); err != nil {
			return nil, fmt.Errorf("tool: scan label: %w", err)
		}
		labels = append(labels, label)
	}
	if labels == nil {
		labels = []string{}
	}
	return labels, rows.Err()
}
