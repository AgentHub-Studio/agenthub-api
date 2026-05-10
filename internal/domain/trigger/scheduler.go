package trigger

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	tenantctx "github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// Scheduler runs a goroutine that periodically scans all tenants for
// due cron triggers and fires them.
//
// Bug 237 (fase 1): este skeleton itera tenants e chama ListDue por
// tenant, mas NÃO faz fire real (criar chat run) ainda — apenas log
// "would fire N triggers" para validar a estrutura. Fase 2 (próximo
// iter) adiciona fire real + MarkRun.
type Scheduler struct {
	pool         *pgxpool.Pool
	tenantLister TenantLister
	repo         TriggerRepository
	cron         CronParser
	interval     time.Duration
}

// TenantLister é o subset de tenant.Repository que o scheduler precisa.
// Mantém o coupling mínimo (sem dep no service inteiro).
type TenantLister interface {
	ListAllIDs(ctx context.Context) ([]string, error)
}

// NewScheduler creates a new Scheduler.
func NewScheduler(pool *pgxpool.Pool, tenantLister TenantLister, repo TriggerRepository, cron CronParser) *Scheduler {
	return &Scheduler{
		pool:         pool,
		tenantLister: tenantLister,
		repo:         repo,
		cron:         cron,
		interval:     30 * time.Second,
	}
}

// Start launches the goroutine that ticks every `interval`. Cancel via ctx.
func (s *Scheduler) Start(ctx context.Context) {
	go s.loop(ctx)
}

func (s *Scheduler) loop(ctx context.Context) {
	slog.Info("trigger.scheduler: starting", "interval", s.interval)
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

// tick scans all tenants for due triggers.
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

// tickTenant queries due triggers for one tenant.
// Fase 1: only logs counts. Fase 2 (#490) firará chat run + MarkRun.
func (s *Scheduler) tickTenant(parentCtx context.Context, tenantID string) {
	// Build per-tenant context for repo (search_path resolution).
	ctx := tenantctx.NewContext(parentCtx, tenantID)
	conn, release, err := database.AcquireWithTenant(ctx, s.pool, tenantID)
	if err != nil {
		slog.Warn("trigger.scheduler: acquire tenant conn failed", "tenantID", tenantID, "err", err)
		return
	}
	defer release()
	_ = conn // reserved for fase 2 (direct exec)

	due, err := s.repo.ListDue(ctx, time.Now().UTC(), 100)
	if err != nil {
		slog.Warn("trigger.scheduler: list due failed", "tenantID", tenantID, "err", err)
		return
	}
	if len(due) == 0 {
		return
	}
	slog.Info("trigger.scheduler: due triggers (fase 1, no fire)", "tenantID", tenantID, "count", len(due))
	// Fase 2 TODO:
	//   for _, t := range due {
	//       create chat session + EnqueueRun com inputTemplate
	//       repo.MarkRun(ctx, t.ID, now, nextRun)
	//   }
}

