package memory

import (
	"context"
	"log/slog"
	"time"

	tenantctx "github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// Pruner runs a background goroutine that periodically cleans stale memory
// entries across all tenants. Two cleanup paths run on each tick:
//
//   1. DeleteExpired — drops rows whose ExpiresAt is in the past (admin-controlled TTL).
//   2. PruneStaleGeneral — drops "general" type memories older than StaleAfter
//      (default 90 days). Structured types (user/feedback/project/reference)
//      are preserved indefinitely.
//
// Mirrors the pattern of trigger.Scheduler so observability (logs/metrics)
// stays consistent.
type Pruner struct {
	tenantLister TenantLister
	repo         *Repository
	interval     time.Duration
	staleAfter   time.Duration
}

// TenantLister is the subset of tenant.Repository the pruner needs.
type TenantLister interface {
	ListAllIDs(ctx context.Context) ([]string, error)
}

// PrunerConfig captures the tunables for the pruner.
type PrunerConfig struct {
	Interval   time.Duration
	StaleAfter time.Duration
}

// DefaultPrunerConfig returns conservative defaults:
//   - tick every 6 hours
//   - prune general memories untouched for 90 days
func DefaultPrunerConfig() PrunerConfig {
	return PrunerConfig{
		Interval:   6 * time.Hour,
		StaleAfter: 90 * 24 * time.Hour,
	}
}

// NewPruner creates a Pruner. Use Start to launch the background loop.
func NewPruner(tenantLister TenantLister, repo *Repository, cfg PrunerConfig) *Pruner {
	if cfg.Interval <= 0 {
		cfg.Interval = 6 * time.Hour
	}
	if cfg.StaleAfter <= 0 {
		cfg.StaleAfter = 90 * 24 * time.Hour
	}
	return &Pruner{
		tenantLister: tenantLister,
		repo:         repo,
		interval:     cfg.Interval,
		staleAfter:   cfg.StaleAfter,
	}
}

// Start launches the goroutine that ticks every Interval. Cancel via ctx.
func (p *Pruner) Start(ctx context.Context) {
	go p.loop(ctx)
}

func (p *Pruner) loop(ctx context.Context) {
	slog.Info("memory.pruner: starting",
		"interval", p.interval,
		"staleAfter", p.staleAfter,
	)
	p.tick(ctx)
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("memory.pruner: stopping")
			return
		case <-ticker.C:
			p.tick(ctx)
		}
	}
}

// tick runs one prune cycle across every tenant in the cluster.
func (p *Pruner) tick(ctx context.Context) {
	tenants, err := p.tenantLister.ListAllIDs(ctx)
	if err != nil {
		slog.Error("memory.pruner: list tenants failed", "err", err)
		return
	}
	cutoff := time.Now().Add(-p.staleAfter)
	for _, tid := range tenants {
		p.tickTenant(ctx, tid, cutoff)
	}
}

func (p *Pruner) tickTenant(parentCtx context.Context, tenantID string, cutoff time.Time) {
	ctx := tenantctx.NewContext(parentCtx, tenantID)

	expired, err := p.repo.DeleteExpired(ctx)
	if err != nil {
		slog.Warn("memory.pruner: delete expired failed", "tenantId", tenantID, "err", err)
	} else if expired > 0 {
		slog.Info("memory.pruner: expired removed", "tenantId", tenantID, "count", expired)
	}

	stale, err := p.repo.PruneStaleGeneral(ctx, cutoff)
	if err != nil {
		slog.Warn("memory.pruner: prune stale general failed", "tenantId", tenantID, "err", err)
	} else if stale > 0 {
		slog.Info("memory.pruner: stale general removed", "tenantId", tenantID, "count", stale)
	}
}
