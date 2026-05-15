package agentic

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBDD_SubagentSummaryReturn(t *testing.T) {
	t.Run("Scenario_ParentReceivesStructuredSummaryNotRawTranscript", func(t *testing.T) {
		// Given a subagent ran for 30 minutes producing a 200KB transcript,
		// When it completes,
		// Then the parent receives a small structured summary, not the
		// full transcript — preserves parent context budget.
		bigTranscript := strings.Repeat("a", 200_000)
		b := NewSubagentSummaryBuilder("doc-gen", "Generate API docs")
		b.AddFinding("Found 12 undocumented endpoints")
		b.AddFinding("Found 3 deprecated examples")
		b.AddArtifact("/tmp/api-docs.md")
		b.AddNextStep("Review with API team")
		s, err := b.Build(SubagentOutcomeSuccess, bigTranscript)
		require.NoError(t, err)
		got := s.FormatForParent()
		// The summary is much smaller than the transcript.
		assert.Less(t, len(got), 2000)
		assert.Equal(t, 200_000, s.TranscriptByteLength)
		assert.NotEmpty(t, s.TruncatedTranscriptHash)
	})

	t.Run("Scenario_OutcomeClassificationGuidesParentRecovery", func(t *testing.T) {
		// Given a subagent partially completed (hit max turns),
		// When parent reads outcome=partial,
		// Then it knows to follow up vs treat as done.
		b := NewSubagentSummaryBuilder("sub-1", "task")
		s, _ := b.Build(SubagentOutcomePartial, "")
		got := s.FormatForParent()
		assert.Contains(t, got, "outcome=partial")
	})

	t.Run("Scenario_FailedOutcomeSurfacedFirstInBatchSort", func(t *testing.T) {
		// Given a parent collected 4 subagent summaries,
		// When SortedSummaries runs,
		// Then failed/aborted surface first — operator sees problems
		// without scrolling.
		summaries := []SubagentReturnSummary{
			{SubagentID: "ok", Outcome: SubagentOutcomeSuccess, Task: "t"},
			{SubagentID: "broken", Outcome: SubagentOutcomeFailed, Task: "t"},
		}
		out := SortedSummaries(summaries)
		assert.Equal(t, SubagentOutcomeFailed, out[0].Outcome)
	})

	t.Run("Scenario_BoundedLengthsProtectParentContext", func(t *testing.T) {
		// Given a chatty subagent emits 50 findings each 1000 chars long,
		// When it builds the summary,
		// Then both the count cap (10 findings) and char cap (240/finding)
		// keep the output bounded.
		b := NewSubagentSummaryBuilder("chatty", "task")
		long := strings.Repeat("x", 1000)
		added := 0
		for i := 0; i < 50; i++ {
			if b.AddFinding(long) {
				added++
			}
		}
		assert.Equal(t, SubagentSummaryMaxFindings, added)
		s, _ := b.Build(SubagentOutcomeSuccess, "")
		for _, f := range s.KeyFindings {
			assert.LessOrEqual(t, len(f), SubagentSummaryMaxFindingLen)
		}
	})

	t.Run("Scenario_TranscriptHashCrossReferencesSidechainAudit", func(t *testing.T) {
		// Given SUB-009 sidechain log persists the full transcript,
		// And the parent only sees the summary,
		// When audit later needs to cross-reference,
		// Then TruncatedTranscriptHash matches the sidechain entry's
		// hash — round-trip from summary to full transcript is possible.
		transcript := "the full transcript with sensitive details"
		b := NewSubagentSummaryBuilder("sub-1", "task")
		s, _ := b.Build(SubagentOutcomeSuccess, transcript)
		// Two builders with the same transcript produce the same hash.
		b2 := NewSubagentSummaryBuilder("sub-2", "task")
		s2, _ := b2.Build(SubagentOutcomeSuccess, transcript)
		assert.Equal(t, s.TruncatedTranscriptHash, s2.TruncatedTranscriptHash)
	})

	t.Run("Scenario_RedactionEnablesCrossTenantHandoff", func(t *testing.T) {
		// Given a subagent's summary travels from tenant A to tenant B
		// (e.g., marketplace shared agent reports back),
		// When tenant A's runtime applies redaction before emission,
		// Then findings/artifacts are scrubbed but the bounded
		// structure remains — receiver still parses cleanly.
		s := SubagentReturnSummary{
			SubagentID: "sub-1", Task: "internal task",
			Outcome:           SubagentOutcomeSuccess,
			KeyFindings:       []string{"customer xyz pii", "tenant secret"},
			ArtifactsProduced: []string{"/internal/path/data.csv"},
		}
		policy := SubagentSummaryRedactionPolicy{
			RedactKeyFindings: true, RedactArtifacts: true,
		}
		out := policy.Redact(s)
		for _, f := range out.KeyFindings {
			assert.Equal(t, subagentSummaryRedactedMarker, f)
		}
		for _, a := range out.ArtifactsProduced {
			assert.Equal(t, subagentSummaryRedactedMarker, a)
		}
		// Structure remains.
		assert.Equal(t, SubagentOutcomeSuccess, out.Outcome)
	})

	t.Run("Scenario_OriginalSummaryNeverMutatedByRedaction", func(t *testing.T) {
		// Given the parent emits the same summary to two destinations
		// with different redaction policies,
		// When both Redact calls run,
		// Then the original summary is intact for the next emission.
		s := SubagentReturnSummary{
			KeyFindings: []string{"a"}, ArtifactsProduced: []string{"b"},
		}
		strict := SubagentSummaryRedactionPolicy{RedactKeyFindings: true, RedactArtifacts: true}
		lite := SubagentSummaryRedactionPolicy{RedactArtifacts: true}
		_ = strict.Redact(s)
		_ = lite.Redact(s)
		assert.Equal(t, "a", s.KeyFindings[0])
		assert.Equal(t, "b", s.ArtifactsProduced[0])
	})

	t.Run("Scenario_NextStepsLetParentChainAction", func(t *testing.T) {
		// Given a subagent identifies follow-up work,
		// When it records recommended next steps,
		// Then the parent can choose to spawn a new subagent or
		// continue itself.
		b := NewSubagentSummaryBuilder("research", "investigate bug 42")
		b.AddNextStep("Run the failing test in isolation")
		b.AddNextStep("Check related issues in tracker")
		s, _ := b.Build(SubagentOutcomeSuccess, "")
		got := s.FormatForParent()
		assert.Contains(t, got, "next_steps:")
		assert.Contains(t, got, "Run the failing test")
	})

	t.Run("Scenario_AbortedSubagentReportsPartialFindings", func(t *testing.T) {
		// Given the parent aborted the subagent after gathering some
		// initial data,
		// When the subagent emits a summary anyway,
		// Then outcome=aborted preserves whatever findings landed.
		b := NewSubagentSummaryBuilder("aborted-sub", "task")
		b.AddFinding("initial signal: high error rate")
		s, _ := b.Build(SubagentOutcomeAborted, "partial transcript")
		assert.Equal(t, SubagentOutcomeAborted, s.Outcome)
		assert.Equal(t, 1, len(s.KeyFindings))
	})

	t.Run("Scenario_BuilderErrorsPropagateForRuntimeHandling", func(t *testing.T) {
		// Given a runtime bug attempts to build a summary without an
		// id,
		// When Build runs,
		// Then validation error propagates so the runtime can fix
		// upstream rather than emit garbage to the parent.
		b := NewSubagentSummaryBuilder("", "task")
		_, err := b.Build(SubagentOutcomeSuccess, "")
		assert.ErrorIs(t, err, ErrSubagentSummaryEmptyID)
	})
}
