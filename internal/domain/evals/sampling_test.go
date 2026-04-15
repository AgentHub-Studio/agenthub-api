package evals_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/evals"
)

func TestSampleRateZeroSkips(t *testing.T) {
	s := evals.RandomSampler{}
	if s.ShouldSample(uuid.New(), evals.EvalConfig{Scorers: []string{"s"}, SampleRate: 0}) {
		t.Fatal("rate 0 must skip")
	}
}

func TestSampleRateOneAlways(t *testing.T) {
	s := evals.RandomSampler{}
	if !s.ShouldSample(uuid.New(), evals.EvalConfig{Scorers: []string{"s"}, SampleRate: 1}) {
		t.Fatal("rate 1 must sample")
	}
}

func TestSampleNoScorersSkips(t *testing.T) {
	s := evals.RandomSampler{}
	if s.ShouldSample(uuid.New(), evals.EvalConfig{SampleRate: 1}) {
		t.Fatal("empty scorers must skip")
	}
}

func TestSampleRateApproxHalf(t *testing.T) {
	cfg := evals.EvalConfig{Scorers: []string{"s"}, SampleRate: 0.5}
	s := evals.RandomSampler{}
	hits := 0
	for i := 0; i < 2000; i++ {
		if s.ShouldSample(uuid.New(), cfg) {
			hits++
		}
	}
	// loose bounds — 35%..65% is enough to detect gross regressions.
	if hits < 700 || hits > 1300 {
		t.Fatalf("hits=%d out of expected band", hits)
	}
}
