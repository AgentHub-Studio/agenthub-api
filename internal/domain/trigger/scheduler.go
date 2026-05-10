package trigger

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	tenantctx "github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// Scheduler runs a goroutine that periodically scans all tenants for
// due cron triggers and fires them.
//
// Bug 237 fase 2: além de detectar, agora cria chat session +
// EnqueueRun + MarkRun (avança next_run_at). Quando o Firer não está
// configurado o scheduler volta ao comportamento de fase 1 (só log).
type Scheduler struct {
	pool         *pgxpool.Pool
	tenantLister TenantLister
	repo         TriggerRepository
	cron         CronParser
	firer        Firer // optional; nil → fase 1 (no fire)
	interval     time.Duration
}

// TenantLister é o subset de tenant.Repository que o scheduler precisa.
type TenantLister interface {
	ListAllIDs(ctx context.Context) ([]string, error)
}

// Firer abstracts the chat session/run creation pipeline so the
// scheduler doesn't depend on the concrete chat.Service.
type Firer interface {
	CreateSessionForTrigger(ctx context.Context, tenantID string, agentID uuid.UUID, title string) (uuid.UUID, error)
	EnqueueRun(ctx context.Context, sessionID uuid.UUID, tenantID, message string) (uuid.UUID, error)
}

// NewScheduler creates a new Scheduler. firer pode ser nil (fase 1).
func NewScheduler(pool *pgxpool.Pool, tenantLister TenantLister, repo TriggerRepository, cron CronParser) *Scheduler {
	return &Scheduler{
		pool:         pool,
		tenantLister: tenantLister,
		repo:         repo,
		cron:         cron,
		interval:     30 * time.Second,
	}
}

// WithFirer injects the Firer dependency to enable fase 2 (real fire).
func (s *Scheduler) WithFirer(f Firer) *Scheduler {
	s.firer = f
	return s
}

// Start launches the goroutine that ticks every `interval`. Cancel via ctx.
func (s *Scheduler) Start(ctx context.Context) {
	go s.loop(ctx)
}

func (s *Scheduler) loop(ctx context.Context) {
	mode := "fase1-no-fire"
	if s.firer != nil {
		mode = "fase2-fire"
	}
	slog.Info("trigger.scheduler: starting", "interval", s.interval, "mode", mode)
	// Tick imediato para feedback no startup.
	s.tick(ctx)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("trigger.scheduler: stopping")
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	tenants, err := s.tenantLister.ListAllIDs(ctx)
	if err != nil {
		slog.Error("trigger.scheduler: list tenants failed", "err", err)
		return
	}
	for _, tid := range tenants {
		s.tickTenant(ctx, tid)
	}
}

func (s *Scheduler) tickTenant(parentCtx context.Context, tenantID string) {
	ctx := tenantctx.NewContext(parentCtx, tenantID)
	conn, release, err := database.AcquireWithTenant(ctx, s.pool, tenantID)
	if err != nil {
		slog.Warn("trigger.scheduler: acquire tenant conn failed", "tenantID", tenantID, "err", err)
		return
	}
	defer release()
	_ = conn

	due, err := s.repo.ListDue(ctx, time.Now().UTC(), 100)
	if err != nil {
		slog.Warn("trigger.scheduler: list due failed", "tenantID", tenantID, "err", err)
		return
	}
	if len(due) == 0 {
		return
	}
	if s.firer == nil {
		slog.Info("trigger.scheduler: due triggers (fase 1, no fire)", "tenantID", tenantID, "count", len(due))
		return
	}
	for _, t := range due {
		s.fireOne(ctx, tenantID, t)
	}
}

func (s *Scheduler) fireOne(ctx context.Context, tenantID string, t AgentTrigger) {
	now := time.Now().UTC()

	// Compute next run BEFORE firing — even if fire fails we want to
	// advance next_run_at so we don't loop on the same trigger forever.
	nextRun, nrErr := s.cron.NextRun(t.CronExpression, now)
	if nrErr != nil {
		slog.Warn("trigger.scheduler: cron NextRun failed; advancing 1h",
			"tenantID", tenantID, "triggerID", t.ID, "err", nrErr)
		nextRun = now.Add(time.Hour)
	}

	// Create chat session for this trigger fire.
	title := "trigger:" + t.Name
	sessionID, err := s.firer.CreateSessionForTrigger(ctx, tenantID, t.AgentID, title)
	if err != nil {
		slog.Error("trigger.scheduler: create session failed",
			"tenantID", tenantID, "triggerID", t.ID, "agentID", t.AgentID, "err", err)
		_ = s.repo.MarkRun(ctx, t.ID, now, nextRun)
		return
	}

	// inputTemplate é JSON. Para a fase 2 simplificada, serializamos como string.
	// Se for objeto JSON com {"message": "..."}, extraímos; senão, marshal direto.
	message := extractMessage(t.InputTemplate)

	runID, err := s.firer.EnqueueRun(ctx, sessionID, tenantID, message)
	if err != nil {
		slog.Error("trigger.scheduler: enqueue run failed",
			"tenantID", tenantID, "triggerID", t.ID, "sessionID", sessionID, "err", err)
		_ = s.repo.MarkRun(ctx, t.ID, now, nextRun)
		return
	}

	if err := s.repo.MarkRun(ctx, t.ID, now, nextRun); err != nil {
		slog.Warn("trigger.scheduler: MarkRun failed",
			"tenantID", tenantID, "triggerID", t.ID, "err", err)
	}
	slog.Info("trigger.scheduler: fired",
		"tenantID", tenantID, "triggerID", t.ID, "sessionID", sessionID, "runID", runID, "nextRunAt", nextRun)
}

// extractMessage reads the inputTemplate JSON and pulls a "message" field
// if present. Otherwise it returns the raw JSON serialized as a string.
// Empty payload → fallback to "trigger fired".
func extractMessage(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "trigger fired"
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err == nil {
		if msg, ok := obj["message"].(string); ok && msg != "" {
			return msg
		}
	}
	return string(raw)
}
