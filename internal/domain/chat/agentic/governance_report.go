package agentic

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// GOV-005 — Governance report.
//
// PDF arXiv:2604.14228v1 Section 11 (regulator-facing export — auditors
// need CSV/PDF/JSON dumps of governance state); Section 7 (audit-driven
// reporting feeds compliance reviews); CLAUDE.md (audit retention policy
// configurable per tenant).
//
// This file aggregates the existing governance signals into ONE
// REPORT that compliance tooling can export:
//   - Permission decisions (PERM-001 / GOV-004 explanations)
//   - Quality reports (OBS-008 / OBS-009 stored verdicts)
//   - Checkpoint outcomes (GOV-003 — approve/reject/timeout/cancel)
//   - Policy denials (GOV-002 — denied tool calls with reason)
//
// Three render targets are bounded into the surface: JSON (audit log
// pipeline), CSV (regulator dump), and HumanSummary (operator
// dashboard). All renderers are deterministic on the same input —
// integration tests can byte-compare.

// ReportPeriod is the time window the report covers.
type ReportPeriod struct {
	// Start is the inclusive start (UTC).
	Start time.Time `json:"start"`
	// End is the exclusive end (UTC).
	End time.Time `json:"end"`
}

// IsValid returns true when start < end and both are non-zero.
func (p ReportPeriod) IsValid() bool {
	return !p.Start.IsZero() && !p.End.IsZero() && p.Start.Before(p.End)
}

// PermissionDecisionSummary aggregates per-decision counts.
type PermissionDecisionSummary struct {
	Allow   int `json:"allow"`
	Deny    int `json:"deny"`
	Confirm int `json:"confirm"`
	Total   int `json:"total"`
}

// QualityReportSummary aggregates per-decision counts plus average score.
type QualityReportSummary struct {
	Pass         int     `json:"pass"`
	Warn         int     `json:"warn"`
	Fail         int     `json:"fail"`
	Total        int     `json:"total"`
	AverageScore float64 `json:"averageScore"`
}

// CheckpointSummary aggregates checkpoint resolution counts.
type CheckpointSummary struct {
	Approved  int `json:"approved"`
	Rejected  int `json:"rejected"`
	Cancelled int `json:"cancelled"`
	TimedOut  int `json:"timedOut"`
	Total     int `json:"total"`
}

// PolicyDenialSummary aggregates per-engine denial counts.
type PolicyDenialSummary struct {
	// ByEngine maps engine name → number of deny decisions in window.
	ByEngine map[string]int `json:"byEngine"`
	// Total is the sum across engines.
	Total int `json:"total"`
}

// GovernanceReport is the unified compliance envelope.
type GovernanceReport struct {
	// TenantID scopes the report. Required.
	TenantID string `json:"tenantId"`
	// Period is the time window covered.
	Period ReportPeriod `json:"period"`
	// GeneratedAt is the wall-clock timestamp when the report was built.
	GeneratedAt time.Time `json:"generatedAt"`
	// Permissions aggregates allow/deny/confirm decisions.
	Permissions PermissionDecisionSummary `json:"permissions"`
	// Quality aggregates evaluator outcomes.
	Quality QualityReportSummary `json:"quality"`
	// Checkpoints aggregates human-control checkpoint outcomes.
	Checkpoints CheckpointSummary `json:"checkpoints"`
	// PolicyDenials aggregates policy-engine deny outcomes by engine name.
	PolicyDenials PolicyDenialSummary `json:"policyDenials"`
	// FlaggedRunIDs lists run IDs that warrant human review.
	// A run is flagged when ANY of: quality=fail, checkpoint=rejected,
	// policy=deny. Sorted ascending for stable output.
	FlaggedRunIDs []string `json:"flaggedRunIds,omitempty"`
}

// ErrInvalidReportPeriod is returned when the builder receives an
// invalid period (zero start/end or start >= end).
var ErrInvalidReportPeriod = errors.New("governance report: invalid period")

// ErrEmptyTenantID is returned when the builder receives an empty tenant.
var ErrEmptyTenantID = errors.New("governance report: tenantId required")

// --- Inputs the builder consumes ---

// ReportPermissionEntry is one permission decision the report includes.
// Pre-filtered by caller (tenant + period).
type ReportPermissionEntry struct {
	RunID    string             `json:"runId"`
	ToolName string             `json:"toolName"`
	Decision PermissionDecision `json:"decision"`
	At       time.Time          `json:"at"`
}

// ReportQualityEntry is one quality verdict.
type ReportQualityEntry struct {
	RunID        string             `json:"runId"`
	OverallScore float64            `json:"overallScore"`
	Decision     EvaluationDecision `json:"decision"`
	At           time.Time          `json:"at"`
}

// ReportCheckpointEntry is one checkpoint resolution.
type ReportCheckpointEntry struct {
	RunID    string             `json:"runId"`
	Kind     CheckpointKind     `json:"kind"`
	Decision CheckpointDecision `json:"decision"`
	At       time.Time          `json:"at"`
}

// ReportPolicyDenialEntry is one policy deny decision.
type ReportPolicyDenialEntry struct {
	RunID      string `json:"runId"`
	ToolName   string `json:"toolName"`
	EngineName string `json:"engineName"`
	Reason     string `json:"reason"`
	At         time.Time `json:"at"`
}

// --- Builder ---

// GovernanceReportInputs is the bundle of pre-filtered data the builder
// aggregates. Caller is responsible for slicing each list to (tenant +
// period) — the builder does the math, not the query.
type GovernanceReportInputs struct {
	Permissions    []ReportPermissionEntry
	Quality        []ReportQualityEntry
	Checkpoints    []ReportCheckpointEntry
	PolicyDenials  []ReportPolicyDenialEntry
}

// BuildGovernanceReport aggregates inputs into the unified report.
func BuildGovernanceReport(tenantID string, period ReportPeriod, in GovernanceReportInputs) (GovernanceReport, error) {
	if tenantID == "" {
		return GovernanceReport{}, ErrEmptyTenantID
	}
	if !period.IsValid() {
		return GovernanceReport{}, ErrInvalidReportPeriod
	}

	rep := GovernanceReport{
		TenantID:    tenantID,
		Period:      period,
		GeneratedAt: time.Now(),
		PolicyDenials: PolicyDenialSummary{
			ByEngine: map[string]int{},
		},
	}

	flaggedSet := map[string]struct{}{}

	for _, e := range in.Permissions {
		switch e.Decision {
		case PermissionAllow:
			rep.Permissions.Allow++
		case PermissionDeny:
			rep.Permissions.Deny++
		case PermissionConfirm:
			rep.Permissions.Confirm++
		}
		rep.Permissions.Total++
	}

	scoreSum := 0.0
	for _, e := range in.Quality {
		switch e.Decision {
		case EvalPass:
			rep.Quality.Pass++
		case EvalWarn:
			rep.Quality.Warn++
		case EvalFail:
			rep.Quality.Fail++
			flaggedSet[e.RunID] = struct{}{}
		}
		rep.Quality.Total++
		scoreSum += e.OverallScore
	}
	if rep.Quality.Total > 0 {
		rep.Quality.AverageScore = scoreSum / float64(rep.Quality.Total)
	}

	for _, e := range in.Checkpoints {
		switch e.Decision {
		case CheckpointApproved:
			rep.Checkpoints.Approved++
		case CheckpointRejected:
			rep.Checkpoints.Rejected++
			flaggedSet[e.RunID] = struct{}{}
		case CheckpointCancelled:
			rep.Checkpoints.Cancelled++
		case CheckpointTimedOut:
			rep.Checkpoints.TimedOut++
		}
		rep.Checkpoints.Total++
	}

	for _, e := range in.PolicyDenials {
		rep.PolicyDenials.ByEngine[e.EngineName]++
		rep.PolicyDenials.Total++
		flaggedSet[e.RunID] = struct{}{}
	}

	for runID := range flaggedSet {
		rep.FlaggedRunIDs = append(rep.FlaggedRunIDs, runID)
	}
	sort.Strings(rep.FlaggedRunIDs)

	return rep, nil
}

// --- Renderers ---

// JSON renders the report as wire-stable JSON.
func (r GovernanceReport) JSON() (string, error) {
	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", fmt.Errorf("governance report: marshal: %w", err)
	}
	return string(raw), nil
}

// CSV renders the report as a regulator-friendly CSV with one row per
// metric. Format: section,metric,value. Deterministic ordering.
func (r GovernanceReport) CSV() (string, error) {
	var b strings.Builder
	w := csv.NewWriter(&b)

	rows := [][]string{
		{"section", "metric", "value"},
		{"meta", "tenantId", r.TenantID},
		{"meta", "periodStart", r.Period.Start.UTC().Format(time.RFC3339)},
		{"meta", "periodEnd", r.Period.End.UTC().Format(time.RFC3339)},
		{"meta", "generatedAt", r.GeneratedAt.UTC().Format(time.RFC3339)},

		{"permissions", "allow", fmt.Sprintf("%d", r.Permissions.Allow)},
		{"permissions", "deny", fmt.Sprintf("%d", r.Permissions.Deny)},
		{"permissions", "confirm", fmt.Sprintf("%d", r.Permissions.Confirm)},
		{"permissions", "total", fmt.Sprintf("%d", r.Permissions.Total)},

		{"quality", "pass", fmt.Sprintf("%d", r.Quality.Pass)},
		{"quality", "warn", fmt.Sprintf("%d", r.Quality.Warn)},
		{"quality", "fail", fmt.Sprintf("%d", r.Quality.Fail)},
		{"quality", "total", fmt.Sprintf("%d", r.Quality.Total)},
		{"quality", "averageScore", fmt.Sprintf("%.4f", r.Quality.AverageScore)},

		{"checkpoints", "approved", fmt.Sprintf("%d", r.Checkpoints.Approved)},
		{"checkpoints", "rejected", fmt.Sprintf("%d", r.Checkpoints.Rejected)},
		{"checkpoints", "cancelled", fmt.Sprintf("%d", r.Checkpoints.Cancelled)},
		{"checkpoints", "timedOut", fmt.Sprintf("%d", r.Checkpoints.TimedOut)},
		{"checkpoints", "total", fmt.Sprintf("%d", r.Checkpoints.Total)},

		{"policyDenials", "total", fmt.Sprintf("%d", r.PolicyDenials.Total)},
	}

	// Per-engine policy denial rows — sorted for determinism.
	engineNames := make([]string, 0, len(r.PolicyDenials.ByEngine))
	for name := range r.PolicyDenials.ByEngine {
		engineNames = append(engineNames, name)
	}
	sort.Strings(engineNames)
	for _, name := range engineNames {
		rows = append(rows, []string{
			"policyDenials.byEngine", name,
			fmt.Sprintf("%d", r.PolicyDenials.ByEngine[name]),
		})
	}

	rows = append(rows, []string{
		"flaggedRunIds", "count", fmt.Sprintf("%d", len(r.FlaggedRunIDs)),
	})
	for _, id := range r.FlaggedRunIDs {
		rows = append(rows, []string{"flaggedRunIds", "id", id})
	}

	for _, row := range rows {
		if err := w.Write(row); err != nil {
			return "", fmt.Errorf("governance report: csv write: %w", err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return "", fmt.Errorf("governance report: csv flush: %w", err)
	}
	return b.String(), nil
}

// HumanSummary renders a concise multi-line summary suitable for an
// operator dashboard or email digest.
func (r GovernanceReport) HumanSummary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Governance report — tenant %s\n", r.TenantID)
	fmt.Fprintf(&b, "Period: %s → %s\n",
		r.Period.Start.UTC().Format(time.RFC3339),
		r.Period.End.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "\nPermissions: %d total (allow=%d / deny=%d / confirm=%d)\n",
		r.Permissions.Total, r.Permissions.Allow, r.Permissions.Deny, r.Permissions.Confirm)
	fmt.Fprintf(&b, "Quality: %d total (pass=%d / warn=%d / fail=%d), avg score %.2f\n",
		r.Quality.Total, r.Quality.Pass, r.Quality.Warn, r.Quality.Fail, r.Quality.AverageScore)
	fmt.Fprintf(&b, "Checkpoints: %d total (approved=%d / rejected=%d / cancelled=%d / timedOut=%d)\n",
		r.Checkpoints.Total, r.Checkpoints.Approved, r.Checkpoints.Rejected,
		r.Checkpoints.Cancelled, r.Checkpoints.TimedOut)
	fmt.Fprintf(&b, "Policy denials: %d total\n", r.PolicyDenials.Total)
	if len(r.FlaggedRunIDs) > 0 {
		fmt.Fprintf(&b, "\nFLAGGED FOR REVIEW: %d runs\n", len(r.FlaggedRunIDs))
		for _, id := range r.FlaggedRunIDs {
			fmt.Fprintf(&b, "  - %s\n", id)
		}
	} else {
		b.WriteString("\nNo runs flagged for review.\n")
	}
	return b.String()
}

// HasFlaggedRuns returns true when at least one run requires review.
// Used by alerting code to trigger compliance escalation.
func (r GovernanceReport) HasFlaggedRuns() bool {
	return len(r.FlaggedRunIDs) > 0
}
