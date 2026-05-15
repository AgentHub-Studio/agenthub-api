package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBDD_FrontmatterSchema(t *testing.T) {
	t.Run("Scenario_AgentWithoutDescriptionFailsValidation", func(t *testing.T) {
		// Given description is the only required frontmatter field for agents,
		// When an operator publishes a CLAUDE.md without a description,
		// Then validation emits an error so the agent is blocked at config time.
		p := ParsedFrontmatter{Fields: map[string]string{
			"model":       "claude-sonnet-4-6",
			"temperature": "0.7",
		}}
		vs := AgentFrontmatterSchema.Validate(p)
		assert.True(t, HasErrors(vs), "missing description must be an error")
		hasDescErr := false
		for _, v := range vs {
			if v.FieldName == "description" {
				hasDescErr = true
			}
		}
		assert.True(t, hasDescErr)
	})

	t.Run("Scenario_TemperatureAbove2IsRejected", func(t *testing.T) {
		// Given LLMs accept temperature 0.0–2.0 and values above 2.0 cause
		// unpredictable behavior,
		// When an operator sets temperature: 3.5,
		// Then the schema validator rejects it with an error before the run starts.
		p := ParsedFrontmatter{Fields: map[string]string{
			"description": "An agent",
			"temperature": "3.5",
		}}
		vs := AgentFrontmatterSchema.Validate(p)
		assert.True(t, HasErrors(vs))
		hasTempErr := false
		for _, v := range vs {
			if v.FieldName == "temperature" && v.Severity == FrontmatterViolationError {
				hasTempErr = true
			}
		}
		assert.True(t, hasTempErr)
	})

	t.Run("Scenario_ContextWindowPctOutOfRangeIsRejected", func(t *testing.T) {
		// Given context_window_pct must be 1–100 (it's a percentage),
		// When an operator sets context_window_pct: 0 or 101,
		// Then the schema validator rejects it.
		for _, bad := range []string{"0", "101", "999"} {
			p := ParsedFrontmatter{Fields: map[string]string{
				"description":        "An agent",
				"context_window_pct": bad,
			}}
			vs := AgentFrontmatterSchema.Validate(p)
			assert.True(t, HasErrors(vs), "context_window_pct=%s should be error", bad)
		}
	})

	t.Run("Scenario_UnknownFieldsAreWarningsNotErrors", func(t *testing.T) {
		// Given operators may annotate agents with custom metadata fields,
		// When a CLAUDE.md contains a key not in the schema,
		// Then a warning is emitted (not an error) — the agent can still run.
		p := ParsedFrontmatter{Fields: map[string]string{
			"description":    "An agent",
			"custom_project": "my-project",
		}}
		vs := AgentFrontmatterSchema.Validate(p)
		assert.False(t, HasErrors(vs), "unknown field should not be an error")
		hasWarning := false
		for _, v := range vs {
			if v.FieldName == "custom_project" && v.Severity == FrontmatterViolationWarning {
				hasWarning = true
			}
		}
		assert.True(t, hasWarning, "unknown field must produce a warning")
	})

	t.Run("Scenario_FullValidAgentFrontmatterPassesClean", func(t *testing.T) {
		// Given a fully specified agent frontmatter with all 15 fields populated,
		// When the operator submits it for validation,
		// Then no violations are returned.
		p := ParsedFrontmatter{Fields: map[string]string{
			"description":        "Research assistant with RAG access",
			"model":              "claude-sonnet-4-6",
			"schema_version":     "1",
			"author":             "Platform Team",
			"version":            "2.1.0",
			"temperature":        "0.3",
			"max_tokens":         "4096",
			"max_turns":          "15",
			"timeout":            "600",
			"context_window_pct": "80",
			"tools":              "document_search,sql",
			"allowed_tools":      "document_search,sql",
			"enabled":            "true",
			"tags":               "research,knowledge-base",
			"priority":           "5",
		}}
		vs := AgentFrontmatterSchema.Validate(p)
		assert.Empty(t, vs, "all 15 valid fields must produce no violations")
	})

	t.Run("Scenario_SkillSchemaRejectsTemperature", func(t *testing.T) {
		// Given temperature is an agent-level concern (not a skill concern),
		// When a skill file includes temperature,
		// Then the skill schema emits a warning (unknown field).
		p := ParsedFrontmatter{Fields: map[string]string{
			"description": "A document search skill",
			"temperature": "0.7",
		}}
		vs := SkillFrontmatterSchema.Validate(p)
		hasWarning := false
		for _, v := range vs {
			if v.FieldName == "temperature" && v.Severity == FrontmatterViolationWarning {
				hasWarning = true
			}
		}
		assert.True(t, hasWarning, "temperature in skill must warn, not error")
		assert.False(t, HasErrors(vs))
	})

	t.Run("Scenario_7FieldKindsCoverAllAgentHubDataTypes", func(t *testing.T) {
		// Given AgentHub frontmatter needs to represent strings, booleans,
		// integers, floats, lists, model references, and slugs,
		// When admin enumerates the closed kind set,
		// Then exactly 7 distinct kinds exist — one per data type.
		kinds := AllFrontmatterFieldKinds()
		assert.Len(t, kinds, 7)
		seen := map[FrontmatterFieldKind]bool{}
		for _, k := range kinds {
			assert.True(t, k.IsValid())
			assert.False(t, seen[k], "duplicate kind %q", k)
			seen[k] = true
		}
	})

	t.Run("Scenario_SchemaRejectsNegativeIntegers", func(t *testing.T) {
		// Given negative integers are invalid for all int fields (max_tokens,
		// max_turns, timeout, priority, context_window_pct),
		// When an operator sets max_tokens: -1,
		// Then kind validation rejects it immediately.
		s, err := NewFrontmatterSchema(
			&FrontmatterField{Name: "max_tokens", Kind: FrontmatterFieldKindInt},
		)
		require.NoError(t, err)
		vs := s.Validate(ParsedFrontmatter{Fields: map[string]string{"max_tokens": "-1"}})
		assert.True(t, HasErrors(vs))
	})
}
