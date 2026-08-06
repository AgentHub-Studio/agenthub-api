package audit

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

const DefaultAuditRetentionInterval = 24 * time.Hour

type RetentionTenantLister interface {
	ListAllIDs(ctx context.Context) ([]string, error)
}

type RetentionApplier interface {
	ApplyRetention(ctx context.Context, tenantID string, req AuditRetentionRequest, now time.Time) (AuditRetentionResponse, error)
}

type RetentionSchedulerConfig struct {
	Interval      time.Duration
	RetentionDays int
	Now           func() time.Time
}

type RetentionSchedulerResult struct {
	Tenants int
	Deleted int
	Failed  int
}

type RetentionScheduler struct {
	tenantLister  RetentionTenantLister
	applier       RetentionApplier
	interval      time.Duration
	retentionDays int
	now           func() time.Time
}

func DefaultRetentionSchedulerConfig() RetentionSchedulerConfig {
	return RetentionSchedulerConfig{
		Interval:      DefaultAuditRetentionInterval,
		RetentionDays: DefaultAuditRetentionDays,
		Now:           time.Now,
	}
}

func NewRetentionScheduler(tenantLister RetentionTenantLister, applier RetentionApplier, cfg RetentionSchedulerConfig) *RetentionScheduler {
	if cfg.Interval <= 0 {
		cfg.Interval = DefaultAuditRetentionInterval
	}
	if cfg.RetentionDays <= 0 {
		cfg.RetentionDays = DefaultAuditRetentionDays
	}
	if cfg.RetentionDays < MinAuditRetentionDays {
		cfg.RetentionDays = MinAuditRetentionDays
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &RetentionScheduler{
		tenantLister:  tenantLister,
		applier:       applier,
		interval:      cfg.Interval,
		retentionDays: cfg.RetentionDays,
		now:           cfg.Now,
	}
}

func (s *RetentionScheduler) Start(ctx context.Context) {
	go s.loop(ctx)
}

func (s *RetentionScheduler) loop(ctx context.Context) {
	slog.Info("audit.retention: starting",
		"interval", s.interval,
		"retentionDays", s.retentionDays,
	)
	s.runAndLog(ctx)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("audit.retention: stopping")
			return
		case <-ticker.C:
			s.runAndLog(ctx)
		}
	}
}

func (s *RetentionScheduler) runAndLog(ctx context.Context) {
	result, err := s.RunOnce(ctx)
	if err != nil {
		slog.Warn("audit.retention: cycle completed with failures",
			"tenants", result.Tenants,
			"deleted", result.Deleted,
			"failed", result.Failed,
			"err", err,
		)
		return
	}
	slog.Info("audit.retention: cycle completed",
		"tenants", result.Tenants,
		"deleted", result.Deleted,
	)
}

func (s *RetentionScheduler) RunOnce(ctx context.Context) (RetentionSchedulerResult, error) {
	if s.tenantLister == nil {
		return RetentionSchedulerResult{}, errors.New("audit.retention: tenant lister is nil")
	}
	if s.applier == nil {
		return RetentionSchedulerResult{}, errors.New("audit.retention: applier is nil")
	}

	tenants, err := s.tenantLister.ListAllIDs(ctx)
	if err != nil {
		return RetentionSchedulerResult{}, fmt.Errorf("audit.retention: list tenants: %w", err)
	}

	result := RetentionSchedulerResult{Tenants: len(tenants)}
	now := s.now().UTC()
	var errs []error
	for _, tenantID := range tenants {
		resp, err := s.applier.ApplyRetention(ctx, tenantID, AuditRetentionRequest{
			RetentionDays: s.retentionDays,
		}, now)
		if err != nil {
			result.Failed++
			errs = append(errs, fmt.Errorf("%s: %w", tenantID, err))
			continue
		}
		result.Deleted += resp.Deleted
		if resp.Deleted > 0 {
			slog.Info("audit.retention: expired audit logs removed",
				"tenantID", tenantID,
				"deleted", resp.Deleted,
				"cutoff", resp.Cutoff,
			)
		}
	}
	return result, errors.Join(errs...)
}
