package agentic

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubagentSummary_IsValidOutcome(t *testing.T) {
	for _, o := range allSubagentReturnOutcomes {
		assert.True(t, IsValidSubagentReturnOutcome(o))
	}
	assert.False(t, IsValidSubagentReturnOutcome(SubagentReturnOutcome("nope")))
}

func TestSubagentSummary_ValidateEmptyID(t *testing.T) {
	s := SubagentReturnSummary{Task: "x", Outcome: SubagentOutcomeSuccess}
	assert.ErrorIs(t, s.Validate(), ErrSubagentSummaryEmptyID)
}

func TestSubagentSummary_ValidateEmptyTask(t *testing.T) {
	s := SubagentReturnSummary{SubagentID: "x", Outcome: SubagentOutcomeSuccess}
	assert.ErrorIs(t, s.Validate(), ErrSubagentSummaryEmptyTask)
}

func TestSubagentSummary_ValidateBadOutcome(t *testing.T) {
	s := SubagentReturnSummary{SubagentID: "x", Task: "y", Outcome: SubagentReturnOutcome("nope")}
	assert.ErrorIs(t, s.Validate(), ErrSubagentSummaryBadOutcome)
}

func TestSubagentSummary_ValidateNegativeLength(t *testing.T) {
	s := SubagentReturnSummary{
		SubagentID: "x", Task: "y", Outcome: SubagentOutcomeSuccess,
		TranscriptByteLength: -1,
	}
	assert.ErrorIs(t, s.Validate(), ErrSubagentSummaryNegativeLength)
}

func TestSubagentSummary_FormatForParentMinimal(t *testing.T) {
	s := SubagentReturnSummary{
		SubagentID: "sub-1", Task: "research X",
		Outcome: SubagentOutcomeSuccess, TranscriptByteLength: 1234,
	}
	got := s.FormatForParent()
	assert.Contains(t, got, "id=sub-1")
	assert.Contains(t, got, "outcome=success")
	assert.Contains(t, got, "task=research X")
	assert.Contains(t, got, "transcript_bytes=1234")
}

func TestSubagentSummary_FormatForParentFull(t *testing.T) {
	s := SubagentReturnSummary{
		SubagentID: "sub-1", Task: "research X",
		Outcome: SubagentOutcomeSuccess,
		KeyFindings:          []string{"finding A", "finding B"},
		ArtifactsProduced:    []string{"/tmp/report.md"},
		NextStepsRecommended: []string{"review with team"},
		TruncatedTranscriptHash: "abc123",
		TranscriptByteLength: 1234,
	}
	got := s.FormatForParent()
	assert.Contains(t, got, "findings:")
	assert.Contains(t, got, "- finding A")
	assert.Contains(t, got, "- finding B")
	assert.Contains(t, got, "artifacts:")
	assert.Contains(t, got, "- /tmp/report.md")
	assert.Contains(t, got, "next_steps:")
	assert.Contains(t, got, "transcript_hash=abc123")
}

func TestSubagentSummary_FormatOmitsEmptySections(t *testing.T) {
	s := SubagentReturnSummary{
		SubagentID: "sub-1", Task: "x", Outcome: SubagentOutcomeSuccess,
	}
	got := s.FormatForParent()
	assert.NotContains(t, got, "findings:")
	assert.NotContains(t, got, "artifacts:")
	assert.NotContains(t, got, "next_steps:")
}

func TestSubagentSummary_BuilderAddFindingCapped(t *testing.T) {
	b := NewSubagentSummaryBuilder("sub-1", "task")
	for i := 0; i < SubagentSummaryMaxFindings; i++ {
		assert.True(t, b.AddFinding("finding"))
	}
	assert.False(t, b.AddFinding("over cap"))
}

func TestSubagentSummary_BuilderAddArtifactCapped(t *testing.T) {
	b := NewSubagentSummaryBuilder("sub-1", "task")
	for i := 0; i < SubagentSummaryMaxArtifacts; i++ {
		assert.True(t, b.AddArtifact("artifact"))
	}
	assert.False(t, b.AddArtifact("over cap"))
}

func TestSubagentSummary_BuilderAddNextStepCapped(t *testing.T) {
	b := NewSubagentSummaryBuilder("sub-1", "task")
	for i := 0; i < SubagentSummaryMaxNextSteps; i++ {
		assert.True(t, b.AddNextStep("step"))
	}
	assert.False(t, b.AddNextStep("over cap"))
}

func TestSubagentSummary_BuilderFindingTruncated(t *testing.T) {
	b := NewSubagentSummaryBuilder("sub-1", "task")
	long := strings.Repeat("a", SubagentSummaryMaxFindingLen+50)
	b.AddFinding(long)
	s, _ := b.Build(SubagentOutcomeSuccess, "")
	assert.Equal(t, SubagentSummaryMaxFindingLen, len(s.KeyFindings[0]))
}

func TestSubagentSummary_BuilderArtifactTruncated(t *testing.T) {
	b := NewSubagentSummaryBuilder("sub-1", "task")
	long := strings.Repeat("a", SubagentSummaryMaxArtifactLen+50)
	b.AddArtifact(long)
	s, _ := b.Build(SubagentOutcomeSuccess, "")
	assert.Equal(t, SubagentSummaryMaxArtifactLen, len(s.ArtifactsProduced[0]))
}

func TestSubagentSummary_BuilderBuildComputesHashAndLength(t *testing.T) {
	b := NewSubagentSummaryBuilder("sub-1", "task")
	transcript := "this is the full transcript text"
	s, err := b.Build(SubagentOutcomeSuccess, transcript)
	require.NoError(t, err)
	assert.Equal(t, len(transcript), s.TranscriptByteLength)
	assert.NotEmpty(t, s.TruncatedTranscriptHash)
	assert.Len(t, s.TruncatedTranscriptHash, 64) // sha256 hex
}

func TestSubagentSummary_BuilderBuildEmptyTranscriptNoHash(t *testing.T) {
	b := NewSubagentSummaryBuilder("sub-1", "task")
	s, _ := b.Build(SubagentOutcomeSuccess, "")
	assert.Empty(t, s.TruncatedTranscriptHash)
	assert.Equal(t, 0, s.TranscriptByteLength)
}

func TestSubagentSummary_BuilderHashDeterministic(t *testing.T) {
	b1 := NewSubagentSummaryBuilder("sub-1", "task")
	s1, _ := b1.Build(SubagentOutcomeSuccess, "same transcript")
	b2 := NewSubagentSummaryBuilder("sub-2", "task")
	s2, _ := b2.Build(SubagentOutcomeSuccess, "same transcript")
	assert.Equal(t, s1.TruncatedTranscriptHash, s2.TruncatedTranscriptHash)
}

func TestSubagentSummary_BuilderBuildPropagatesValidationError(t *testing.T) {
	b := NewSubagentSummaryBuilder("", "task")
	_, err := b.Build(SubagentOutcomeSuccess, "")
	assert.ErrorIs(t, err, ErrSubagentSummaryEmptyID)
}

func TestSubagentSummary_BuilderClockInjectable(t *testing.T) {
	stamp := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	b := NewSubagentSummaryBuilder("sub-1", "task")
	b.SetClock(func() time.Time { return stamp })
	s, _ := b.Build(SubagentOutcomeSuccess, "")
	assert.Equal(t, stamp, s.CompletedAt)
}

func TestSubagentSummary_SetClockNilIsNoop(t *testing.T) {
	stamp := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	b := NewSubagentSummaryBuilder("sub-1", "task")
	b.SetClock(func() time.Time { return stamp })
	b.SetClock(nil)
	s, _ := b.Build(SubagentOutcomeSuccess, "")
	assert.Equal(t, stamp, s.CompletedAt)
}

func TestSubagentSummary_BuilderTaskTruncated(t *testing.T) {
	long := strings.Repeat("x", SubagentSummaryMaxTaskLen+50)
	b := NewSubagentSummaryBuilder("sub-1", long)
	s, _ := b.Build(SubagentOutcomeSuccess, "")
	assert.Equal(t, SubagentSummaryMaxTaskLen, len(s.Task))
}

func TestSubagentSummary_RedactionRedactsTask(t *testing.T) {
	s := SubagentReturnSummary{Task: "secret task"}
	p := SubagentSummaryRedactionPolicy{RedactTask: true}
	out := p.Redact(s)
	assert.Equal(t, subagentSummaryRedactedMarker, out.Task)
	assert.Equal(t, "secret task", s.Task) // input unchanged
}

func TestSubagentSummary_RedactionRedactsAllFindings(t *testing.T) {
	s := SubagentReturnSummary{KeyFindings: []string{"a", "b"}}
	p := SubagentSummaryRedactionPolicy{RedactKeyFindings: true}
	out := p.Redact(s)
	assert.Equal(t, subagentSummaryRedactedMarker, out.KeyFindings[0])
	assert.Equal(t, subagentSummaryRedactedMarker, out.KeyFindings[1])
	// Input untouched.
	assert.Equal(t, "a", s.KeyFindings[0])
}

func TestSubagentSummary_RedactionRedactsArtifactsAndSteps(t *testing.T) {
	s := SubagentReturnSummary{
		ArtifactsProduced:    []string{"x"},
		NextStepsRecommended: []string{"y"},
	}
	p := SubagentSummaryRedactionPolicy{
		RedactArtifacts: true, RedactNextSteps: true,
	}
	out := p.Redact(s)
	assert.Equal(t, subagentSummaryRedactedMarker, out.ArtifactsProduced[0])
	assert.Equal(t, subagentSummaryRedactedMarker, out.NextStepsRecommended[0])
}

func TestSubagentSummary_RedactionRedactsTranscriptHash(t *testing.T) {
	s := SubagentReturnSummary{TruncatedTranscriptHash: "abc"}
	p := SubagentSummaryRedactionPolicy{RedactTranscriptHash: true}
	out := p.Redact(s)
	assert.Empty(t, out.TruncatedTranscriptHash)
}

func TestSubagentSummary_RedactionDefensiveCopyForSlices(t *testing.T) {
	s := SubagentReturnSummary{KeyFindings: []string{"a"}}
	p := SubagentSummaryRedactionPolicy{RedactKeyFindings: true}
	out := p.Redact(s)
	out.KeyFindings[0] = "tampered"
	// Original untouched.
	assert.Equal(t, "a", s.KeyFindings[0])
	_ = out
}

func TestSubagentSummary_SortedByOutcomePriority(t *testing.T) {
	in := []SubagentReturnSummary{
		{SubagentID: "a", Outcome: SubagentOutcomeSuccess},
		{SubagentID: "b", Outcome: SubagentOutcomeFailed},
		{SubagentID: "c", Outcome: SubagentOutcomePartial},
		{SubagentID: "d", Outcome: SubagentOutcomeAborted},
	}
	out := SortedSummaries(in)
	// Worst-first: failed > aborted > partial > success.
	assert.Equal(t, SubagentOutcomeFailed, out[0].Outcome)
	assert.Equal(t, SubagentOutcomeAborted, out[1].Outcome)
	assert.Equal(t, SubagentOutcomePartial, out[2].Outcome)
	assert.Equal(t, SubagentOutcomeSuccess, out[3].Outcome)
}

func TestSubagentSummary_SortedDoesNotMutateInput(t *testing.T) {
	in := []SubagentReturnSummary{
		{SubagentID: "a", Outcome: SubagentOutcomeSuccess},
		{SubagentID: "b", Outcome: SubagentOutcomeFailed},
	}
	_ = SortedSummaries(in)
	assert.Equal(t, "a", in[0].SubagentID)
}

func TestSubagentSummary_TruncateBytesRespectsUTF8(t *testing.T) {
	// "🦊" is 4 bytes. Limit 3 should produce empty string (or full
	// prior rune) — never split mid-rune.
	s := "🦊x"
	out := truncateBytes(s, 3)
	// Either "" or just before any partial rune; never bytes [0,1,2]
	// since that would corrupt the 🦊 rune.
	assert.NotContains(t, out, "\xf0\x9f\xa6") // partial rune marker
}

func TestSubagentSummary_FormatTaskTruncatedInHeader(t *testing.T) {
	long := strings.Repeat("x", SubagentSummaryMaxTaskLen+50)
	s := SubagentReturnSummary{
		SubagentID: "sub-1", Task: long, Outcome: SubagentOutcomeSuccess,
	}
	got := s.FormatForParent()
	taskFieldLen := SubagentSummaryMaxTaskLen
	// Header should contain truncated task.
	assert.Contains(t, got, "task="+strings.Repeat("x", taskFieldLen))
}

func TestSubagentSummary_TruncateBytesShortStringUnchanged(t *testing.T) {
	s := "hello"
	assert.Equal(t, "hello", truncateBytes(s, 10))
}

func TestSubagentSummary_TruncateBytesExactLen(t *testing.T) {
	s := "hello"
	assert.Equal(t, "hello", truncateBytes(s, 5))
}
