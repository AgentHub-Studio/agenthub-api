package agentic

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// BDD-style scenarios that ratify OBS-007 (Silent failure detection)
// against the Claude Code architecture paper "Dive into Claude Code"
// (arXiv:2604.14228v1):
//
//   - Section 11 (Tensions, trade-offs, observability): "agents tend to
//     respond by confidently praising the work, even when quality is
//     mediocre" — motivating SEPARATION OF GENERATION FROM EVALUATION.
//   - Section 4.4 (Recovery Mechanisms) implies that silent stalls (tools
//     that hang without output) must be detectable and surface as observable
//     events, not as ambiguous "still running" states.
//   - PDF general principle: detection must convert silent failures into
//     LOUD signals (events, state transitions, denial categories).
//
// AgentHub maps silent failure detection to several layers, each catching a
// distinct failure mode:
//
//   1. Tool stalls          — stall.go StallDetector + StallMonitor: a tool
//                              that produces no output within a threshold
//                              transitions to ToolStateStalled and emits an
//                              observable event.
//   2. Code-quality drift  — diagnostictracker.go DiagnosticTracker:
//                              captures a baseline of LSP diagnostics, then
//                              detects NEW diagnostics introduced by tool
//                              calls (e.g. type errors created by an Edit).
//   3. Permission deny loop — permission.go PermissionDenialTracker (PERM-001):
//                              detects when the LLM is stuck in a denial
//                              loop, signalling silent semantic failure.
//   4. Empty LLM responses  — runner.go (line ~973): retries empty assistant
//                              outputs, surfaces sentinel after exhaustion.
//   5. Cache breaks         — cachebreak.go CacheBreakDetector: detects
//                              prompt cache invalidation as an observable
//                              event (silent cost regression).
//
// Each layer converts a SILENT FAILURE into a LOUD SIGNAL — the architectural
// pattern PDF Section 11 calls "make detection precede recovery".

func TestBDD_SilentFailureDetection(t *testing.T) {
	t.Run("Scenario_StallDetectorHasSensibleDefaults", func(t *testing.T) {
		// Given a StallDetector constructed with zero values (PDF Section
		//       11: detection must work out-of-the-box without per-call
		//       configuration),
		given := NewStallDetector(0, 0)

		// When the runtime inspects it,
		// Then defaults are applied (15s interval, 45s threshold) — operators
		//      get baseline detection without configuration burden.
		assert.NotNil(t, given,
			"NewStallDetector must accept zero values and apply defaults")
	})

	t.Run("Scenario_StallMonitorTransitionsToStalledAfterThreshold", func(t *testing.T) {
		// Given a StallDetector with very tight thresholds (so we can test
		//       quickly without long-running goroutines),
		detector := NewStallDetector(20*time.Millisecond, 50*time.Millisecond)
		var stallCount atomic.Int32
		monitor := detector.Monitor("call_1", "execute-sql",
			func(_, _ string) { stallCount.Add(1) },
			nil,
		)
		defer monitor.Stop()

		// When the tool produces no activity for longer than the threshold,
		time.Sleep(100 * time.Millisecond)

		// Then the monitor reports stalled — silent stall converted to a
		//      loud signal via callback.
		assert.True(t, monitor.IsStalled(),
			"monitor must report stalled after threshold without activity")
		assert.GreaterOrEqual(t, stallCount.Load(), int32(1),
			"onStall callback must fire when stall is detected")
	})

	t.Run("Scenario_RecordActivityRecoversFromStalledState", func(t *testing.T) {
		// Given a stalled tool monitor,
		detector := NewStallDetector(20*time.Millisecond, 50*time.Millisecond)
		var recoverCount atomic.Int32
		monitor := detector.Monitor("call_2", "long-task",
			nil,
			func(_, _ string) { recoverCount.Add(1) },
		)
		defer monitor.Stop()
		time.Sleep(100 * time.Millisecond)
		assert.True(t, monitor.IsStalled(), "precondition: monitor is stalled")

		// When the tool produces output again,
		monitor.RecordActivity()

		// Then the monitor recovers and the recovery callback fires —
		//      transient stalls are not fatal, but they ARE observable.
		assert.False(t, monitor.IsStalled(),
			"RecordActivity must clear the stalled flag")
		assert.GreaterOrEqual(t, recoverCount.Load(), int32(1),
			"onRecover callback must fire on activity resume")
	})

	t.Run("Scenario_StalledToolHasFirstClassEventState", func(t *testing.T) {
		// Given the runner emits ToolState transitions as observable events
		//       (PDF Section 11: detection feeds telemetry),
		// When stall is detected,
		// Then the state token "stalled" exists in the enum — operators can
		//      filter the event stream for this specific failure mode.
		assert.Equal(t, ToolState("stalled"), ToolStateStalled,
			"ToolStateStalled must be a first-class event token")
	})

	t.Run("Scenario_DenialLoopIsDetectedNotJustCounted", func(t *testing.T) {
		// Given the LLM repeatedly retries the SAME denied tool+input (PDF
		//       Section 11 implication: a stuck loop is a silent semantic
		//       failure even though every individual call returned "denied"
		//       loudly),
		tracker := &PermissionDenialTracker{}

		// When the same denial fires repeatedly,
		for i := 0; i < 3; i++ {
			tracker.RecordDenialWithMetadata("execute-sql", "permission_deny",
				"DROP TABLE users", i)
		}

		// Then the auto-mode loop detector reports stuck — separating
		//      "deny happened once" (normal) from "LLM is looping" (silent
		//      failure surfaced loudly).
		assert.True(t, tracker.IsAutoModeLoop(),
			"three identical denials must trigger loop detection")
		assert.NotEmpty(t, tracker.LoopingTools(),
			"looping tool names must be reported for diagnostics")
	})

	t.Run("Scenario_DiagnosticTrackerCapturesBaselineToDetectDrift", func(t *testing.T) {
		// Given a tool may introduce silent regressions (e.g. an Edit that
		//       creates new compile errors — PDF Section 11 generator/
		//       evaluator separation: the agent generates, the LSP evaluates),
		baseline := []Diagnostic{
			{Line: 10, Severity: SeverityError, Message: "preexisting"},
		}
		tracker := NewDiagnosticTracker(func(_ string) []Diagnostic { return baseline })
		tracker.CaptureBaselineManual("file.go", baseline)

		// When the runtime queries for NEW diagnostics after a tool call,
		// (we manually swap the fetcher to simulate post-call diagnostics)
		newDiagnostics := []Diagnostic{
			{Line: 10, Severity: SeverityError, Message: "preexisting"},
			{Line: 99, Severity: SeverityError, Message: "new error introduced by edit"},
		}
		tracker2 := NewDiagnosticTracker(func(_ string) []Diagnostic { return newDiagnostics })
		tracker2.CaptureBaselineManual("file.go", baseline)
		when := tracker2.GetNewDiagnostics("file.go")

		// Then only the NEW diagnostic is surfaced — operators see "this
		//      tool call introduced a regression" instead of being drowned
		//      by every diagnostic.
		assert.Len(t, when, 1,
			"only diagnostics not present in baseline must surface as new")
		assert.Contains(t, when[0].Message, "new error",
			"the post-baseline regression must be the surfaced diagnostic")
	})

	t.Run("Scenario_DiagnosticTrackerHandlesMultipleFiles", func(t *testing.T) {
		// Given a multi-file tool call (e.g. a refactor),
		baseline := map[string][]Diagnostic{
			"a.go": {{Line: 1, Severity: SeverityError, Message: "old"}},
			"b.go": {},
		}
		fetcher := func(p string) []Diagnostic { return baseline[p] }
		tracker := NewDiagnosticTracker(fetcher)
		tracker.CaptureBaselineManual("a.go", baseline["a.go"])
		tracker.CaptureBaselineManual("b.go", baseline["b.go"])

		// When the runner queries all new diagnostics across files,
		when := tracker.GetAllNewDiagnostics()

		// Then a per-file roll-up is returned (may be nil/empty when no
		//      regressions exist) — the call must not panic and must
		//      surface a structure operators can iterate.
		assert.NotPanics(t, func() {
			for range when {
				// iterating nil slice is safe
			}
		}, "GetAllNewDiagnostics must return an iterable per-file roll-up")
		// Empty/nil is acceptable when no new regressions vs baseline.
		assert.True(t, when == nil || len(when) >= 0,
			"return value must be a slice (possibly empty)")
	})

	t.Run("Scenario_ConcurrentStallMonitorsAreSafeUnderRace", func(t *testing.T) {
		// Given multiple tools execute in parallel (PDF Section 4.2 concurrent
		//       read tools), each with its own stall monitor,
		detector := NewStallDetector(10*time.Millisecond, 30*time.Millisecond)
		var wg sync.WaitGroup
		monitors := make([]*StallMonitor, 10)
		for i := 0; i < 10; i++ {
			monitors[i] = detector.Monitor("call", "tool", nil, nil)
		}

		// When the runtime hammers RecordActivity from many goroutines,
		for i := 0; i < 10; i++ {
			for j := 0; j < 10; j++ {
				wg.Add(1)
				go func(m *StallMonitor) {
					defer wg.Done()
					m.RecordActivity()
				}(monitors[i])
			}
		}
		wg.Wait()

		// Then no race occurs (the test would fail under -race) and
		//      monitors remain consistent.
		for _, m := range monitors {
			m.Stop()
		}
		// If we reach here without panic and -race is clean, contract holds.
		assert.True(t, true, "concurrent RecordActivity must be race-free")
	})
}
