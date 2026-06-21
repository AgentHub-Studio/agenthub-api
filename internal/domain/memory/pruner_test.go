package memory_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/memory"
)

func TestDefaultPrunerConfig_ProvidesConservativeDefaults(t *testing.T) {
	cfg := memory.DefaultPrunerConfig()
	assert.Equal(t, 6*time.Hour, cfg.Interval)
	assert.Equal(t, 90*24*time.Hour, cfg.StaleAfter)
	assert.Equal(t, 0.05, cfg.MinRelevance)
}

func TestNewPruner_FillsZeroIntervalWithDefault(t *testing.T) {
	p := memory.NewPruner(&stubTenantLister{}, nil, memory.PrunerConfig{})
	assert.NotNil(t, p, "should construct with zero values without panicking")
}

func TestNewPruner_AcceptsCustomConfig(t *testing.T) {
	p := memory.NewPruner(&stubTenantLister{}, nil, memory.PrunerConfig{
		Interval:   30 * time.Minute,
		StaleAfter: 7 * 24 * time.Hour,
	})
	assert.NotNil(t, p)
}

type stubTenantLister struct {
	ids []string
	err error
}

func (s *stubTenantLister) ListAllIDs(_ context.Context) ([]string, error) {
	return s.ids, s.err
}
