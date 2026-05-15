package agentic

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// SUB-010 — Summary-only return.
//
// PDF arXiv:2604.14228v1 §8 (Subagents) — when a subagent completes
// its task, the parent agent should receive only a STRUCTURED SUMMARY
// of what was accomplished, NOT the raw transcript. This protects the
// parent's context budget (subagent runs can be long) and gives the
// parent a clean, parseable interface to integrate the result.
//
// Distinct from existing AgentHub plumbing:
//   - SUB-001 (Agent tool) returns whatever the subagent's last LLM
//     turn emitted — raw output, no structure.
//   - SUB-009 (Sidechain transcripts) persists the full transcript for
//     replay/audit but is NOT the return value the parent sees.
//   - SUB-010 (this file) is the SHAPED return value the parent agent
//     consumes — bounded length, structured outcome, artifact list,
//     truncated raw transcript hash for audit cross-reference.

// SubagentReturnOutcome bounded enum classifies how the subagent
// finished.
type SubagentReturnOutcome string

const (
	// SubagentOutcomeSuccess — subagent completed the task fully.
	SubagentOutcomeSuccess SubagentReturnOutcome = "success"
	// SubagentOutcomePartial — subagent completed part of the task but
	// hit a constraint (depth, timeout, permission deny, max turns).
	SubagentOutcomePartial SubagentReturnOutcome = "partial"
	// SubagentOutcomeFailed — subagent could not complete the task
	// (error, exhausted strategies, gave up).
	SubagentOutcomeFailed SubagentReturnOutcome = "failed"
	// SubagentOutcomeAborted — caller aborted before completion.
	SubagentOutcomeAborted SubagentReturnOutcome = "aborted"
)

var allSubagentReturnOutcomes = []SubagentReturnOutcome{
	SubagentOutcomeSuccess, SubagentOutcomePartial,
	SubagentOutcomeFailed, SubagentOutcomeAborted,
}

// IsValidSubagentReturnOutcome returns true for the bounded set.
func IsValidSubagentReturnOutcome(o SubagentReturnOutcome) bool {
	for _, v := range allSubagentReturnOutcomes {
		if o == v {
			return true
		}
	}
	return false
}

// Field length limits to keep summaries bounded.
const (
	SubagentSummaryMaxTaskLen      = 300
	SubagentSummaryMaxFindingLen   = 240
	SubagentSummaryMaxArtifactLen  = 200
	SubagentSummaryMaxNextStepLen  = 240
	SubagentSummaryMaxFindings     = 10
	SubagentSummaryMaxArtifacts    = 20
	SubagentSummaryMaxNextSteps    = 10
)

// SubagentReturnSummary is the structured payload the parent consumes.
type SubagentReturnSummary struct {
	SubagentID              string
	Task                    string
	Outcome                 SubagentReturnOutcome
	KeyFindings             []string
	ArtifactsProduced       []string
	NextStepsRecommended    []string
	TruncatedTranscriptHash string // sha256 hex of full transcript
	TranscriptByteLength    int
	CompletedAt             time.Time
}

// Validate enforces invariants.
func (s SubagentReturnSummary) Validate() error {
	if strings.TrimSpace(s.SubagentID) == "" {
		return ErrSubagentSummaryEmptyID
	}
	if strings.TrimSpace(s.Task) == "" {
		return ErrSubagentSummaryEmptyTask
	}
	if !IsValidSubagentReturnOutcome(s.Outcome) {
		return fmt.Errorf("%w: %q", ErrSubagentSummaryBadOutcome, s.Outcome)
	}
	if s.TranscriptByteLength < 0 {
		return ErrSubagentSummaryNegativeLength
	}
	return nil
}

// FormatForParent renders the summary as a compact tagged string the
// parent LLM can parse without ambiguity.
//
// Format:
//   [subagent_return id=X outcome=Y task=... transcript_bytes=N]
//   findings:
//   - ...
//   artifacts:
//   - ...
//   next_steps:
//   - ...
func (s SubagentReturnSummary) FormatForParent() string {
	headerParts := []string{
		"id=" + s.SubagentID,
		"outcome=" + string(s.Outcome),
		"task=" + truncateBytes(s.Task, SubagentSummaryMaxTaskLen),
		fmt.Sprintf("transcript_bytes=%d", s.TranscriptByteLength),
	}
	if s.TruncatedTranscriptHash != "" {
		headerParts = append(headerParts, "transcript_hash="+s.TruncatedTranscriptHash)
	}
	lines := []string{"[subagent_return " + strings.Join(headerParts, " ") + "]"}
	if len(s.KeyFindings) > 0 {
		lines = append(lines, "findings:")
		for _, f := range s.KeyFindings {
			lines = append(lines, "- "+truncateBytes(f, SubagentSummaryMaxFindingLen))
		}
	}
	if len(s.ArtifactsProduced) > 0 {
		lines = append(lines, "artifacts:")
		for _, a := range s.ArtifactsProduced {
			lines = append(lines, "- "+truncateBytes(a, SubagentSummaryMaxArtifactLen))
		}
	}
	if len(s.NextStepsRecommended) > 0 {
		lines = append(lines, "next_steps:")
		for _, n := range s.NextStepsRecommended {
			lines = append(lines, "- "+truncateBytes(n, SubagentSummaryMaxNextStepLen))
		}
	}
	return strings.Join(lines, "\n")
}

// SubagentSummaryRedactionPolicy declares which fields to scrub for
// downstream emission (e.g., compliance export or cross-tenant agent
// hand-off).
type SubagentSummaryRedactionPolicy struct {
	RedactKeyFindings       bool
	RedactArtifacts         bool
	RedactNextSteps         bool
	RedactTask              bool
	RedactTranscriptHash    bool
}

// Redact applies the policy to a copy. Original is untouched.
func (p SubagentSummaryRedactionPolicy) Redact(s SubagentReturnSummary) SubagentReturnSummary {
	out := s
	out.KeyFindings = append([]string(nil), s.KeyFindings...)
	out.ArtifactsProduced = append([]string(nil), s.ArtifactsProduced...)
	out.NextStepsRecommended = append([]string(nil), s.NextStepsRecommended...)

	if p.RedactTask && out.Task != "" {
		out.Task = subagentSummaryRedactedMarker
	}
	if p.RedactKeyFindings {
		for i := range out.KeyFindings {
			out.KeyFindings[i] = subagentSummaryRedactedMarker
		}
	}
	if p.RedactArtifacts {
		for i := range out.ArtifactsProduced {
			out.ArtifactsProduced[i] = subagentSummaryRedactedMarker
		}
	}
	if p.RedactNextSteps {
		for i := range out.NextStepsRecommended {
			out.NextStepsRecommended[i] = subagentSummaryRedactedMarker
		}
	}
	if p.RedactTranscriptHash {
		out.TruncatedTranscriptHash = ""
	}
	return out
}

const subagentSummaryRedactedMarker = "[REDACTED]"

// Sentinel errors.
var (
	ErrSubagentSummaryEmptyID        = errors.New("subagent summary: subagent id required")
	ErrSubagentSummaryEmptyTask      = errors.New("subagent summary: task required")
	ErrSubagentSummaryBadOutcome     = errors.New("subagent summary: invalid outcome")
	ErrSubagentSummaryNegativeLength = errors.New("subagent summary: transcript length must be >= 0")
)

// SubagentSummaryBuilder accumulates findings/artifacts/next-steps with
// length-and-count caps so the final summary is bounded regardless of
// subagent verbosity. Thread-safe.
type SubagentSummaryBuilder struct {
	subagentID string
	task       string
	findings   []string
	artifacts  []string
	nextSteps  []string
	now        func() time.Time
}

// NewSubagentSummaryBuilder creates a builder.
func NewSubagentSummaryBuilder(subagentID, task string) *SubagentSummaryBuilder {
	return &SubagentSummaryBuilder{
		subagentID: subagentID,
		task:       task,
		now:        time.Now,
	}
}

// SetClock injects a clock for deterministic test timestamps.
func (b *SubagentSummaryBuilder) SetClock(fn func() time.Time) {
	if fn == nil {
		return
	}
	b.now = fn
}

// AddFinding appends a finding (capped + truncated). Returns true if
// the finding was added; false if the cap had been reached.
func (b *SubagentSummaryBuilder) AddFinding(finding string) bool {
	if len(b.findings) >= SubagentSummaryMaxFindings {
		return false
	}
	b.findings = append(b.findings, truncateBytes(finding, SubagentSummaryMaxFindingLen))
	return true
}

// AddArtifact appends an artifact reference (capped + truncated).
func (b *SubagentSummaryBuilder) AddArtifact(artifact string) bool {
	if len(b.artifacts) >= SubagentSummaryMaxArtifacts {
		return false
	}
	b.artifacts = append(b.artifacts, truncateBytes(artifact, SubagentSummaryMaxArtifactLen))
	return true
}

// AddNextStep appends a recommended next step (capped + truncated).
func (b *SubagentSummaryBuilder) AddNextStep(step string) bool {
	if len(b.nextSteps) >= SubagentSummaryMaxNextSteps {
		return false
	}
	b.nextSteps = append(b.nextSteps, truncateBytes(step, SubagentSummaryMaxNextStepLen))
	return true
}

// Build emits a SubagentReturnSummary with the specified outcome and
// transcript metadata. Pass the full transcript text so the builder
// computes the hash + byte length.
func (b *SubagentSummaryBuilder) Build(outcome SubagentReturnOutcome, fullTranscript string) (SubagentReturnSummary, error) {
	hash := ""
	if fullTranscript != "" {
		h := sha256.Sum256([]byte(fullTranscript))
		hash = hex.EncodeToString(h[:])
	}
	s := SubagentReturnSummary{
		SubagentID:              b.subagentID,
		Task:                    truncateBytes(b.task, SubagentSummaryMaxTaskLen),
		Outcome:                 outcome,
		KeyFindings:             append([]string(nil), b.findings...),
		ArtifactsProduced:       append([]string(nil), b.artifacts...),
		NextStepsRecommended:    append([]string(nil), b.nextSteps...),
		TruncatedTranscriptHash: hash,
		TranscriptByteLength:    len(fullTranscript),
		CompletedAt:             b.now(),
	}
	if err := s.Validate(); err != nil {
		return SubagentReturnSummary{}, err
	}
	return s, nil
}

// truncateBytes returns s clipped to n bytes. UTF-8 aware: if the cut
// would split a rune, it backs up to the previous rune boundary.
func truncateBytes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	// Find last rune boundary at or before n.
	for n > 0 && (s[n]&0xC0) == 0x80 {
		n--
	}
	return s[:n]
}

// SortedSummaries sorts a slice by (Outcome desc-priority, SubagentID,
// CompletedAt). Useful for audit emission. Outcome priority order:
// failed > aborted > partial > success (worst-first surfacing).
func SortedSummaries(in []SubagentReturnSummary) []SubagentReturnSummary {
	out := append([]SubagentReturnSummary(nil), in...)
	sort.SliceStable(out, func(i, j int) bool {
		ri, rj := outcomeRank(out[i].Outcome), outcomeRank(out[j].Outcome)
		if ri != rj {
			return ri < rj
		}
		if out[i].SubagentID != out[j].SubagentID {
			return out[i].SubagentID < out[j].SubagentID
		}
		return out[i].CompletedAt.Before(out[j].CompletedAt)
	})
	return out
}

func outcomeRank(o SubagentReturnOutcome) int {
	switch o {
	case SubagentOutcomeFailed:
		return 0
	case SubagentOutcomeAborted:
		return 1
	case SubagentOutcomePartial:
		return 2
	case SubagentOutcomeSuccess:
		return 3
	}
	return 99
}
