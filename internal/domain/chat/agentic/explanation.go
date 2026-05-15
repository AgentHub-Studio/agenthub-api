package agentic

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// GOV-004 — Permission explainability.
//
// PDF arXiv:2604.14228v1 Section 5.3 (decisions must be observable by
// the audit layer + user-facing UI); Section 11 (silent failures and
// silent denials are the bug class to prevent — every decision needs a
// WHY with stable category + human + machine views).
//
// AgentHub already has decision-producing surfaces:
//   - PERM-001 PermissionDecision (allow/deny/confirm) from rule-matching.
//   - GOV-002 PolicyEvaluationResult.Reason from PolicyEngine.
//   - GOV-003 Checkpoint.Reason from CheckpointGate.
//   - permission_audit.go for after-the-fact audit trail.
//
// What was missing: a UNIFIED EXPLANATION envelope that aggregates
// rationale across all sources, exposes BOUNDED categories for
// analytics, and renders to multiple destinations (plain text for
// CLI, markdown for chat UI, JSON for SSE/audit log).
//
// PermissionExplanation is the carrier of that "WHY". The runner builds
// one alongside every PermissionDecision and feeds it to: the user
// (Markdown), the audit log (JSON), the agent transcript (PlainText).

// ExplanationSource bounded enum classifies which subsystem
// contributed to the decision. Stable strings — analytics aggregates.
type ExplanationSource string

const (
	// ExplanationSourceRuleMatch — PermissionRules allow/deny/confirm matched.
	ExplanationSourceRuleMatch ExplanationSource = "rule_match"
	// ExplanationSourcePolicyEngine — external PolicyEngine returned a decision.
	ExplanationSourcePolicyEngine ExplanationSource = "policy_engine"
	// ExplanationSourceCheckpoint — human-in-the-loop checkpoint outcome.
	ExplanationSourceCheckpoint ExplanationSource = "checkpoint"
	// ExplanationSourceHook — pre/post-tool hook decision.
	ExplanationSourceHook ExplanationSource = "hook"
	// ExplanationSourceDefaultMode — fell back to PermissionMode default.
	ExplanationSourceDefaultMode ExplanationSource = "default_mode"
	// ExplanationSourceSafetyImmune — bypassed by safety-immune check
	// (cannot be permissionMode.bypass'd).
	ExplanationSourceSafetyImmune ExplanationSource = "safety_immune"
)

// allExplanationSources is the closed bounded set.
var allExplanationSources = []ExplanationSource{
	ExplanationSourceRuleMatch,
	ExplanationSourcePolicyEngine,
	ExplanationSourceCheckpoint,
	ExplanationSourceHook,
	ExplanationSourceDefaultMode,
	ExplanationSourceSafetyImmune,
}

// IsValidExplanationSource returns true for the bounded set.
func IsValidExplanationSource(s ExplanationSource) bool {
	for _, v := range allExplanationSources {
		if s == v {
			return true
		}
	}
	return false
}

// AllExplanationSources returns a copy of the bounded set (for UI dropdowns).
func AllExplanationSources() []ExplanationSource {
	out := make([]ExplanationSource, len(allExplanationSources))
	copy(out, allExplanationSources)
	return out
}

// ExplanationSeverity bounded enum.
type ExplanationSeverity string

const (
	ExplanationSeverityInfo     ExplanationSeverity = "info"
	ExplanationSeverityWarn     ExplanationSeverity = "warn"
	ExplanationSeverityCritical ExplanationSeverity = "critical"
)

// IsValidExplanationSeverity returns true for the bounded set.
func IsValidExplanationSeverity(s ExplanationSeverity) bool {
	switch s {
	case ExplanationSeverityInfo, ExplanationSeverityWarn, ExplanationSeverityCritical:
		return true
	}
	return false
}

// ExplanationContribution is one piece of rationale from one source.
// Multiple contributions per explanation (e.g. policy denied + checkpoint
// timed out — both contributed).
type ExplanationContribution struct {
	// Source identifies the subsystem that produced this contribution.
	// Always in the bounded set.
	Source ExplanationSource `json:"source"`
	// SourceID is the concrete identifier within the source (e.g. rule
	// pattern, engine name, checkpoint ID, hook slug). Empty when not
	// applicable.
	SourceID string `json:"sourceId,omitempty"`
	// Severity classifies the contribution importance.
	Severity ExplanationSeverity `json:"severity"`
	// Message is the human-readable rationale (≤ 500 chars).
	Message string `json:"message"`
	// At is when this contribution was recorded.
	At time.Time `json:"at"`
}

// PermissionExplanation is the unified rationale envelope. One per
// decision. Aggregates contributions from all sources that voted.
type PermissionExplanation struct {
	// Decision is the final outcome (allow/deny/confirm). Mirror of the
	// PermissionDecision the runner records — keeps explanation paired
	// with the decision so audit cannot get out of sync.
	Decision PermissionDecision `json:"decision"`
	// PrimaryReason is the dominant rationale — what the user sees first
	// in a UI tooltip / single-line audit summary.
	PrimaryReason string `json:"primaryReason"`
	// Contributions is the ordered (by severity DESC then At ASC) list
	// of all rationales that influenced the decision.
	Contributions []ExplanationContribution `json:"contributions,omitempty"`
	// ToolName identifies the tool the decision applied to.
	ToolName string `json:"toolName"`
	// CreatedAt is the wall-clock timestamp.
	CreatedAt time.Time `json:"createdAt"`
}

// NewPermissionExplanation creates a builder seeded with decision +
// tool + primary reason. PrimaryReason is REQUIRED — the contract is
// "no decision without a stated reason" (audit guarantee).
func NewPermissionExplanation(decision PermissionDecision, toolName, primaryReason string) *PermissionExplanation {
	return &PermissionExplanation{
		Decision:      decision,
		ToolName:      toolName,
		PrimaryReason: primaryReason,
		CreatedAt:     time.Now(),
	}
}

// AddContribution appends a contribution. Builder returns *e for chaining.
// Invalid source/severity values are silently skipped to prevent
// log-spam from caller bugs (the caller-bug surfaces in tests via the
// IsValid* guards).
func (e *PermissionExplanation) AddContribution(c ExplanationContribution) *PermissionExplanation {
	if !IsValidExplanationSource(c.Source) {
		return e
	}
	if !IsValidExplanationSeverity(c.Severity) {
		return e
	}
	if c.At.IsZero() {
		c.At = time.Now()
	}
	if len(c.Message) > 500 {
		c.Message = c.Message[:497] + "..."
	}
	e.Contributions = append(e.Contributions, c)
	return e
}

// AddRuleMatch is a convenience: adds a rule-match contribution.
func (e *PermissionExplanation) AddRuleMatch(rulePattern, message string) *PermissionExplanation {
	return e.AddContribution(ExplanationContribution{
		Source:   ExplanationSourceRuleMatch,
		SourceID: rulePattern,
		Severity: ExplanationSeverityInfo,
		Message:  message,
	})
}

// AddPolicyDecision is a convenience: adds a PolicyEngine contribution.
// Severity is derived from the policy outcome — Deny = critical,
// RequireApproval = warn, others = info.
func (e *PermissionExplanation) AddPolicyDecision(engineName string, outcome PolicyOutcome, message string) *PermissionExplanation {
	sev := ExplanationSeverityInfo
	switch outcome {
	case PolicyDeny:
		sev = ExplanationSeverityCritical
	case PolicyRequireApproval:
		sev = ExplanationSeverityWarn
	}
	return e.AddContribution(ExplanationContribution{
		Source:   ExplanationSourcePolicyEngine,
		SourceID: engineName,
		Severity: sev,
		Message:  message,
	})
}

// AddCheckpointDecision is a convenience: adds a Checkpoint contribution.
func (e *PermissionExplanation) AddCheckpointDecision(checkpointID, kind string, decision CheckpointDecision, message string) *PermissionExplanation {
	sev := ExplanationSeverityInfo
	switch decision {
	case CheckpointRejected:
		sev = ExplanationSeverityCritical
	case CheckpointTimedOut, CheckpointCancelled:
		sev = ExplanationSeverityWarn
	}
	return e.AddContribution(ExplanationContribution{
		Source:   ExplanationSourceCheckpoint,
		SourceID: kind + ":" + checkpointID,
		Severity: sev,
		Message:  message,
	})
}

// AddHookDecision is a convenience: adds a hook-decision contribution.
func (e *PermissionExplanation) AddHookDecision(hookSlug, message string, blocking bool) *PermissionExplanation {
	sev := ExplanationSeverityInfo
	if blocking {
		sev = ExplanationSeverityCritical
	}
	return e.AddContribution(ExplanationContribution{
		Source:   ExplanationSourceHook,
		SourceID: hookSlug,
		Severity: sev,
		Message:  message,
	})
}

// AddDefaultModeFallback is a convenience: documents that no specific
// rule/policy/checkpoint matched and the PermissionMode default kicked in.
func (e *PermissionExplanation) AddDefaultModeFallback(mode PermissionMode, message string) *PermissionExplanation {
	return e.AddContribution(ExplanationContribution{
		Source:   ExplanationSourceDefaultMode,
		SourceID: string(mode),
		Severity: ExplanationSeverityInfo,
		Message:  message,
	})
}

// AddSafetyImmune is a convenience: documents that a safety-immune
// check overrode bypass mode (e.g. dangerous file deletion under
// PermissionModeBypass still requires confirm).
func (e *PermissionExplanation) AddSafetyImmune(immuneCheck, message string) *PermissionExplanation {
	return e.AddContribution(ExplanationContribution{
		Source:   ExplanationSourceSafetyImmune,
		SourceID: immuneCheck,
		Severity: ExplanationSeverityCritical,
		Message:  message,
	})
}

// SortContributionsBySeverity orders contributions by severity DESC
// (critical first, then warn, then info) then by At ASC. Used by
// renderers so the most important rationale leads.
func (e *PermissionExplanation) SortContributionsBySeverity() {
	sevOrder := map[ExplanationSeverity]int{
		ExplanationSeverityCritical: 0,
		ExplanationSeverityWarn:     1,
		ExplanationSeverityInfo:     2,
	}
	sort.SliceStable(e.Contributions, func(i, j int) bool {
		ai, aj := sevOrder[e.Contributions[i].Severity], sevOrder[e.Contributions[j].Severity]
		if ai != aj {
			return ai < aj
		}
		return e.Contributions[i].At.Before(e.Contributions[j].At)
	})
}

// PlainText renders the explanation as a single-line summary suitable
// for terminals / log lines / agent transcripts.
//
// Format: "[<DECISION>] <toolName>: <primaryReason> (N contributions)"
func (e *PermissionExplanation) PlainText() string {
	return fmt.Sprintf("[%s] %s: %s (%d contributions)",
		strings.ToUpper(string(e.Decision)),
		e.ToolName,
		e.PrimaryReason,
		len(e.Contributions),
	)
}

// Markdown renders the explanation as a multi-line markdown block
// suitable for chat UI panels.
//
// Format:
//   **<DECISION>** — `<toolName>`
//
//   <primaryReason>
//
//   - **[critical] [source]** sourceId: message
//   - **[warn] [source]** sourceId: message
//   - **[info] [source]** sourceId: message
func (e *PermissionExplanation) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "**%s** — `%s`\n\n",
		strings.ToUpper(string(e.Decision)), e.ToolName)
	fmt.Fprintf(&b, "%s\n", e.PrimaryReason)
	if len(e.Contributions) == 0 {
		return b.String()
	}
	b.WriteString("\n")
	for _, c := range e.Contributions {
		idPart := ""
		if c.SourceID != "" {
			idPart = " " + c.SourceID
		}
		fmt.Fprintf(&b, "- **[%s] [%s]**%s: %s\n",
			c.Severity, c.Source, idPart, c.Message)
	}
	return b.String()
}

// JSON renders as wire-stable JSON for SSE / audit log.
func (e *PermissionExplanation) JSON() (string, error) {
	raw, err := json.Marshal(e)
	if err != nil {
		return "", fmt.Errorf("explanation: marshal: %w", err)
	}
	return string(raw), nil
}

// HasCriticalContribution returns true if any contribution is critical.
// Used by audit code to flag for review.
func (e *PermissionExplanation) HasCriticalContribution() bool {
	for _, c := range e.Contributions {
		if c.Severity == ExplanationSeverityCritical {
			return true
		}
	}
	return false
}

// CountBySource returns histogram of contribution counts grouped by
// source. Always returns ALL bounded sources with 0 default — dashboards
// have stable axes.
func (e *PermissionExplanation) CountBySource() map[ExplanationSource]int {
	hist := map[ExplanationSource]int{}
	for _, s := range allExplanationSources {
		hist[s] = 0
	}
	for _, c := range e.Contributions {
		hist[c.Source]++
	}
	return hist
}
