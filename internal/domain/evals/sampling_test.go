package evals_test

import (
	"encoding/json"
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

func TestEvalConfigAcceptsSnakeCaseSampleRate(t *testing.T) {
	var cfg evals.EvalConfig
	if err := json.Unmarshal([]byte(`{"scorers":["exact_match"],"sample_rate":1}`), &cfg); err != nil {
		t.Fatalf("unmarshal eval config: %v", err)
	}
	if cfg.SampleRate != 1 {
		t.Fatalf("sample_rate not decoded: got %v", cfg.SampleRate)
	}
	if len(cfg.Scorers) != 1 || cfg.Scorers[0] != "exact_match" {
		t.Fatalf("scorers not decoded: %#v", cfg.Scorers)
	}
}

func TestEvalConfigSampleRateAliasesMustAgree(t *testing.T) {
	for _, payload := range []string{
		`{"sampleRate":0.25}`,
		`{"sample_rate":0.25}`,
		`{"sampleRate":0.25,"sample_rate":0.25}`,
	} {
		var cfg evals.EvalConfig
		if err := json.Unmarshal([]byte(payload), &cfg); err != nil {
			t.Fatalf("expected accepted sample-rate aliases for %s: %v", payload, err)
		}
		if cfg.SampleRate != 0.25 {
			t.Fatalf("expected sample rate 0.25 for %s, got %v", payload, cfg.SampleRate)
		}
	}

	var conflicting evals.EvalConfig
	if err := json.Unmarshal([]byte(`{"sampleRate":0,"sample_rate":1}`), &conflicting); err == nil {
		t.Fatal("expected conflicting sample-rate aliases to be rejected")
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
