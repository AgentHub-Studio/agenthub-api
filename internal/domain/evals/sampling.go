// Package evals provides sampling primitives so a fraction of agent runs can
// be sent to automated scorers in production without incurring the cost of
// evaluating every run.
//
// Inspired by Mastra's scorer samplingConfig { sampleRate }. Each agent
// declares its own EvalConfig; ShouldSample is called once per run_complete.
package evals

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/randutil"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

const EvalRunStatusQueued = "queued"

// EvalConfig is the per-agent configuration persisted on the agent row.
type EvalConfig struct {
	// Scorers is the ordered list of scorer IDs to invoke asynchronously.
	Scorers []string `json:"scorers,omitempty"`
	// SampleRate is the probability (0.0 to 1.0) that a completed run is eval'd.
	SampleRate float64 `json:"sample_rate,omitempty"`
}

func (c EvalConfig) MarshalJSON() ([]byte, error) {
	type wire struct {
		Scorers    []string `json:"scorers,omitempty"`
		SampleRate float64  `json:"sample_rate,omitempty"`
	}
	return json.Marshal(wire(c))
}

func (c *EvalConfig) UnmarshalJSON(raw []byte) error {
	if string(raw) == "null" {
		*c = EvalConfig{}
		return nil
	}
	var wire struct {
		Scorers         []string `json:"scorers"`
		SampleRate      *float64 `json:"sampleRate"`
		SampleRateSnake *float64 `json:"sample_rate"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return err
	}
	if wire.SampleRate != nil && wire.SampleRateSnake != nil && *wire.SampleRate != *wire.SampleRateSnake {
		return fmt.Errorf("evals: conflicting sample rate aliases")
	}
	c.Scorers = wire.Scorers
	c.SampleRate = 0
	if wire.SampleRate != nil {
		c.SampleRate = *wire.SampleRate
	}
	if wire.SampleRateSnake != nil {
		c.SampleRate = *wire.SampleRateSnake
	}
	return nil
}

// Sampler decides whether a given run is eligible for automated evaluation.
type Sampler interface {
	ShouldSample(runID uuid.UUID, cfg EvalConfig) bool
}

// RandomSampler draws a uniform float in [0,1) and compares against SampleRate.
type RandomSampler struct{}

func (RandomSampler) ShouldSample(_ uuid.UUID, cfg EvalConfig) bool {
	if cfg.SampleRate <= 0 || len(cfg.Scorers) == 0 {
		return false
	}
	if cfg.SampleRate >= 1 {
		return true
	}
	return randutil.Float64() < cfg.SampleRate
}

// EvalJob is enqueued for async processing when a run is sampled.
type EvalJob struct {
	RunID    uuid.UUID
	AgentID  uuid.UUID
	Scorers  []string
	TenantID string
}

// EvalRun is the observable record created when a completed chat run is sampled.
type EvalRun struct {
	ID         uuid.UUID
	ChatRunID  *uuid.UUID
	SessionID  uuid.UUID
	AgentID    uuid.UUID
	Scorers    []string
	Status     string
	Score      float64
	SampleRate float64
	CreatedAt  time.Time
}

type EvalRunResponse struct {
	ID         uuid.UUID  `json:"id"`
	ChatRunID  *uuid.UUID `json:"chatRunId,omitempty"`
	SessionID  uuid.UUID  `json:"sessionId"`
	AgentID    uuid.UUID  `json:"agentId"`
	Scorers    []string   `json:"scorers"`
	Status     string     `json:"status"`
	Score      float64    `json:"score"`
	SampleRate float64    `json:"sampleRate"`
	CreatedAt  time.Time  `json:"createdAt"`
}

func ResponseFrom(run EvalRun) EvalRunResponse {
	scorers := run.Scorers
	if scorers == nil {
		scorers = []string{}
	}
	return EvalRunResponse{
		ID:         run.ID,
		ChatRunID:  run.ChatRunID,
		SessionID:  run.SessionID,
		AgentID:    run.AgentID,
		Scorers:    append([]string(nil), scorers...),
		Status:     run.Status,
		Score:      run.Score,
		SampleRate: run.SampleRate,
		CreatedAt:  run.CreatedAt,
	}
}

// RecordRequest contains the data needed when run_complete reaches the sampler.
type RecordRequest struct {
	ChatRunID *uuid.UUID
	SessionID uuid.UUID
	AgentID   uuid.UUID
	Config    EvalConfig
}

type Repository interface {
	CreateRun(ctx context.Context, run EvalRun) (EvalRun, error)
	ListRuns(ctx context.Context, agentID *uuid.UUID, req pagination.PageRequest) ([]EvalRun, int64, error)
}

type pgRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

func (r *pgRepository) acquire(ctx context.Context) (*pgxpool.Conn, func(), error) {
	return database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
}

func (r *pgRepository) CreateRun(ctx context.Context, run EvalRun) (EvalRun, error) {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return EvalRun{}, fmt.Errorf("evals.CreateRun: acquire: %w", err)
	}
	defer release()

	if run.ID == uuid.Nil {
		run.ID = uuid.New()
	}
	if run.Status == "" {
		run.Status = EvalRunStatusQueued
	}
	scorers, err := json.Marshal(run.Scorers)
	if err != nil {
		return EvalRun{}, fmt.Errorf("evals.CreateRun: marshal scorers: %w", err)
	}
	row := conn.QueryRow(ctx, `
		INSERT INTO eval_run (id, chat_run_id, session_id, agent_id, scorers, status, score, sample_rate)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, chat_run_id, session_id, agent_id, scorers, status, score, sample_rate, created_at`,
		run.ID, run.ChatRunID, run.SessionID, run.AgentID, scorers, run.Status, run.Score, run.SampleRate,
	)
	created, err := scanRun(row)
	if err != nil {
		return EvalRun{}, fmt.Errorf("evals.CreateRun: %w", err)
	}
	return created, nil
}

func (r *pgRepository) ListRuns(ctx context.Context, agentID *uuid.UUID, req pagination.PageRequest) ([]EvalRun, int64, error) {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("evals.ListRuns: acquire: %w", err)
	}
	defer release()

	var args []any
	where := ""
	if agentID != nil {
		args = append(args, *agentID)
		where = " WHERE agent_id = $1"
	}

	var total int64
	if err := conn.QueryRow(ctx, "SELECT COUNT(*) FROM eval_run"+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("evals.ListRuns count: %w", err)
	}

	offsetParam := len(args) + 1
	limitParam := len(args) + 2
	args = append(args, req.Offset(), req.Size)
	rows, err := conn.Query(ctx, fmt.Sprintf(`
		SELECT id, chat_run_id, session_id, agent_id, scorers, status, score, sample_rate, created_at
		FROM eval_run%s
		ORDER BY created_at DESC
		OFFSET $%d LIMIT $%d`, where, offsetParam, limitParam), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("evals.ListRuns: %w", err)
	}
	defer rows.Close()

	runs := []EvalRun{}
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, 0, err
		}
		runs = append(runs, run)
	}
	return runs, total, rows.Err()
}

type scannable interface {
	Scan(dest ...any) error
}

func scanRun(row scannable) (EvalRun, error) {
	var run EvalRun
	var scorersRaw []byte
	if err := row.Scan(
		&run.ID, &run.ChatRunID, &run.SessionID, &run.AgentID, &scorersRaw,
		&run.Status, &run.Score, &run.SampleRate, &run.CreatedAt,
	); err != nil {
		return EvalRun{}, err
	}
	if len(scorersRaw) > 0 {
		_ = json.Unmarshal(scorersRaw, &run.Scorers)
	}
	if run.Scorers == nil {
		run.Scorers = []string{}
	}
	return run, nil
}

// Recorder decides whether a run should be sampled and records the eval job.
type Recorder struct {
	repo    Repository
	sampler Sampler
}

func NewRecorder(repo Repository, sampler Sampler) *Recorder {
	return &Recorder{repo: repo, sampler: sampler}
}

func (r *Recorder) RecordRunComplete(ctx context.Context, req RecordRequest) (EvalRun, bool, error) {
	if r == nil || r.repo == nil || r.sampler == nil {
		return EvalRun{}, false, nil
	}
	sampleID := uuid.New()
	if !r.sampler.ShouldSample(sampleID, req.Config) {
		return EvalRun{}, false, nil
	}
	run := EvalRun{
		ID:         sampleID,
		ChatRunID:  req.ChatRunID,
		SessionID:  req.SessionID,
		AgentID:    req.AgentID,
		Scorers:    append([]string(nil), req.Config.Scorers...),
		Status:     EvalRunStatusQueued,
		Score:      1,
		SampleRate: req.Config.SampleRate,
	}
	created, err := r.repo.CreateRun(ctx, run)
	return created, err == nil, err
}
