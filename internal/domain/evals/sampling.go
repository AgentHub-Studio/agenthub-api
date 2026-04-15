// Package evals provides sampling primitives so a fraction of agent runs can
// be sent to automated scorers in production without incurring the cost of
// evaluating every run.
//
// Inspired by Mastra's scorer samplingConfig { sampleRate }. Each agent
// declares its own EvalConfig; ShouldSample is called once per run_complete.
package evals

import (
	"math/rand/v2"

	"github.com/google/uuid"
)

// EvalConfig is the per-agent configuration persisted on the agent row.
type EvalConfig struct {
	// Scorers is the ordered list of scorer IDs to invoke asynchronously.
	Scorers []string `json:"scorers,omitempty"`
	// SampleRate is the probability (0.0 – 1.0) that a completed run is eval'd.
	SampleRate float64 `json:"sampleRate,omitempty"`
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
	return rand.Float64() < cfg.SampleRate
}

// EvalJob is enqueued for async processing when a run is sampled.
type EvalJob struct {
	RunID    uuid.UUID
	AgentID  uuid.UUID
	Scorers  []string
	TenantID string
}
