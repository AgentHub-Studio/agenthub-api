package agentic

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestExplanation_SourceEnumIsBounded(t *testing.T) {
	for _, s := range AllExplanationSources() {
		assert.True(t, IsValidExplanationSource(s))
	}
	assert.False(t, IsValidExplanationSource(ExplanationSource("unknown")))
	assert.False(t, IsValidExplanationSource(""))
}

func TestExplanation_AllSourcesCount(t *testing.T) {
	// 6 sources in bounded set.
	assert.Equal(t, 6, len(AllExplanationSources()),
		"6 sources expected (refactor must update tests + UI dropdown)")
}

func TestExplanation_SeverityEnumIsBounded(t *testing.T) {
	for _, s := range []ExplanationSeverity{
		ExplanationSeverityInfo, ExplanationSeverityWarn, ExplanationSeverityCritical,
	} {
		assert.True(t, IsValidExplanationSeverity(s))
	}
	assert.False(t, IsValidExplanationSeverity(ExplanationSeverity("urgent")))
}

func TestExplanation_NewSetsRequiredFields(t *testing.T) {
	e := NewPermissionExplanation(PermissionAllow, "shell", "user authorized")
	assert.Equal(t, PermissionAllow, e.Decision)
	assert.Equal(t, "shell", e.ToolName)
	assert.Equal(t, "user authorized", e.PrimaryReason)
	assert.False(t, e.CreatedAt.IsZero())
	assert.Empty(t, e.Contributions)
}

func TestExplanation_AddContribution_AppendsAndChains(t *testing.T) {
	e := NewPermissionExplanation(PermissionAllow, "x", "ok").
		AddContribution(ExplanationContribution{
			Source: ExplanationSourceRuleMatch, Severity: ExplanationSeverityInfo, Message: "rule allowed",
		}).
		AddContribution(ExplanationContribution{
			Source: ExplanationSourcePolicyEngine, Severity: ExplanationSeverityWarn, Message: "policy advised",
		})
	assert.Len(t, e.Contributions, 2)
}

func TestExplanation_AddContribution_RejectsInvalidSourceSilently(t *testing.T) {
	e := NewPermissionExplanation(PermissionAllow, "x", "ok").
		AddContribution(ExplanationContribution{
			Source: ExplanationSource("bogus"), Severity: ExplanationSeverityInfo, Message: "x",
		})
	assert.Empty(t, e.Contributions, "invalid source must be skipped")
}

func TestExplanation_AddContribution_RejectsInvalidSeveritySilently(t *testing.T) {
	e := NewPermissionExplanation(PermissionAllow, "x", "ok").
		AddContribution(ExplanationContribution{
			Source: ExplanationSourceRuleMatch, Severity: ExplanationSeverity("urgent"), Message: "x",
		})
	assert.Empty(t, e.Contributions, "invalid severity must be skipped")
}

func TestExplanation_AddContribution_TruncatesLongMessage(t *testing.T) {
	long := strings.Repeat("x", 800)
	e := NewPermissionExplanation(PermissionAllow, "x", "ok").
		AddContribution(ExplanationContribution{
			Source: ExplanationSourceRuleMatch, Severity: ExplanationSeverityInfo, Message: long,
		})
	assert.Len(t, e.Contributions, 1)
	assert.Equal(t, 500, len(e.Contributions[0].Message),
		"message must be truncated to 500 chars")
	assert.Equal(t, "...", e.Contributions[0].Message[497:])
}

func TestExplanation_AddContribution_PopulatesAtIfZero(t *testing.T) {
	e := NewPermissionExplanation(PermissionAllow, "x", "ok").
		AddContribution(ExplanationContribution{
			Source: ExplanationSourceRuleMatch, Severity: ExplanationSeverityInfo, Message: "x",
		})
	assert.False(t, e.Contributions[0].At.IsZero(), "At must be auto-populated")
}

func TestExplanation_AddRuleMatch_HasInfoSeverity(t *testing.T) {
	e := NewPermissionExplanation(PermissionAllow, "x", "ok").
		AddRuleMatch("shell(*)", "rule matched")
	assert.Equal(t, ExplanationSeverityInfo, e.Contributions[0].Severity)
	assert.Equal(t, "shell(*)", e.Contributions[0].SourceID)
}

func TestExplanation_AddPolicyDecision_DerivesSeverityFromOutcome(t *testing.T) {
	deny := NewPermissionExplanation(PermissionDeny, "x", "denied").
		AddPolicyDecision("opa", PolicyDeny, "policy denied")
	assert.Equal(t, ExplanationSeverityCritical, deny.Contributions[0].Severity)

	approval := NewPermissionExplanation(PermissionConfirm, "x", "approval").
		AddPolicyDecision("opa", PolicyRequireApproval, "human required")
	assert.Equal(t, ExplanationSeverityWarn, approval.Contributions[0].Severity)

	allow := NewPermissionExplanation(PermissionAllow, "x", "allow").
		AddPolicyDecision("opa", PolicyAllow, "policy allowed")
	assert.Equal(t, ExplanationSeverityInfo, allow.Contributions[0].Severity)
}

func TestExplanation_AddCheckpointDecision_DerivesSeverity(t *testing.T) {
	rejected := NewPermissionExplanation(PermissionDeny, "x", "rejected").
		AddCheckpointDecision("cp-1", "pre_destructive", CheckpointRejected, "user rejected")
	assert.Equal(t, ExplanationSeverityCritical, rejected.Contributions[0].Severity)

	timedOut := NewPermissionExplanation(PermissionDeny, "x", "timeout").
		AddCheckpointDecision("cp-2", "pre_pii_export", CheckpointTimedOut, "no response")
	assert.Equal(t, ExplanationSeverityWarn, timedOut.Contributions[0].Severity)

	approved := NewPermissionExplanation(PermissionAllow, "x", "approved").
		AddCheckpointDecision("cp-3", "pre_irreversible", CheckpointApproved, "user approved")
	assert.Equal(t, ExplanationSeverityInfo, approved.Contributions[0].Severity)
}

func TestExplanation_SortContributionsBySeverity(t *testing.T) {
	e := NewPermissionExplanation(PermissionDeny, "x", "blocked")
	earliest := time.Now()
	e.AddContribution(ExplanationContribution{
		Source: ExplanationSourceRuleMatch, Severity: ExplanationSeverityInfo, Message: "info-1", At: earliest,
	})
	e.AddContribution(ExplanationContribution{
		Source: ExplanationSourcePolicyEngine, Severity: ExplanationSeverityCritical, Message: "crit", At: earliest.Add(time.Second),
	})
	e.AddContribution(ExplanationContribution{
		Source: ExplanationSourceCheckpoint, Severity: ExplanationSeverityWarn, Message: "warn", At: earliest.Add(2 * time.Second),
	})
	e.SortContributionsBySeverity()

	assert.Equal(t, ExplanationSeverityCritical, e.Contributions[0].Severity, "critical first")
	assert.Equal(t, ExplanationSeverityWarn, e.Contributions[1].Severity, "warn second")
	assert.Equal(t, ExplanationSeverityInfo, e.Contributions[2].Severity, "info last")
}

func TestExplanation_PlainText_Format(t *testing.T) {
	e := NewPermissionExplanation(PermissionDeny, "shell", "blocked by policy").
		AddRuleMatch("shell(*)", "rule blocked")
	out := e.PlainText()
	assert.Contains(t, out, "[DENY]")
	assert.Contains(t, out, "shell")
	assert.Contains(t, out, "blocked by policy")
	assert.Contains(t, out, "1 contributions")
}

func TestExplanation_Markdown_Format(t *testing.T) {
	e := NewPermissionExplanation(PermissionDeny, "shell", "blocked").
		AddPolicyDecision("opa", PolicyDeny, "policy denied")
	out := e.Markdown()
	assert.Contains(t, out, "**DENY**")
	assert.Contains(t, out, "`shell`")
	assert.Contains(t, out, "blocked")
	assert.Contains(t, out, "[critical]")
	assert.Contains(t, out, "[policy_engine]")
	assert.Contains(t, out, "policy denied")
}

func TestExplanation_Markdown_NoContributions(t *testing.T) {
	e := NewPermissionExplanation(PermissionAllow, "x", "ok")
	out := e.Markdown()
	assert.Contains(t, out, "**ALLOW**")
	assert.Contains(t, out, "ok")
	assert.NotContains(t, out, "[info]", "no list section when no contributions")
}

func TestExplanation_JSON_RoundTrip(t *testing.T) {
	e := NewPermissionExplanation(PermissionConfirm, "x", "ask user").
		AddRuleMatch("x(*)", "matched")
	jsonStr, err := e.JSON()
	assert.NoError(t, err)

	var roundtrip PermissionExplanation
	assert.NoError(t, json.Unmarshal([]byte(jsonStr), &roundtrip))
	assert.Equal(t, PermissionConfirm, roundtrip.Decision)
	assert.Equal(t, "x", roundtrip.ToolName)
	assert.Len(t, roundtrip.Contributions, 1)
}

func TestExplanation_HasCriticalContribution(t *testing.T) {
	e := NewPermissionExplanation(PermissionDeny, "x", "blocked").
		AddRuleMatch("x(*)", "info-only")
	assert.False(t, e.HasCriticalContribution())

	e.AddContribution(ExplanationContribution{
		Source: ExplanationSourceSafetyImmune, Severity: ExplanationSeverityCritical, Message: "safety",
	})
	assert.True(t, e.HasCriticalContribution())
}

func TestExplanation_CountBySource_AlwaysIncludesAllSources(t *testing.T) {
	e := NewPermissionExplanation(PermissionAllow, "x", "ok").
		AddRuleMatch("x(*)", "matched").
		AddRuleMatch("y(*)", "matched too")

	hist := e.CountBySource()
	for _, s := range AllExplanationSources() {
		_, exists := hist[s]
		assert.True(t, exists, "source %q must appear in histogram with 0 default", s)
	}
	assert.Equal(t, 2, hist[ExplanationSourceRuleMatch])
	assert.Equal(t, 0, hist[ExplanationSourcePolicyEngine])
}
