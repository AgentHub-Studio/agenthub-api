package agentic

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- FrontmatterFieldKind ----

func TestFrontmatterFieldKind_IsValid_AllValid(t *testing.T) {
	for _, k := range AllFrontmatterFieldKinds() {
		assert.True(t, k.IsValid(), "expected %q to be valid", k)
	}
}

func TestFrontmatterFieldKind_IsValid_Invalid(t *testing.T) {
	assert.False(t, FrontmatterFieldKind("").IsValid())
	assert.False(t, FrontmatterFieldKind("number").IsValid())
	assert.False(t, FrontmatterFieldKind("String").IsValid())
}

func TestFrontmatterFieldKind_AllKinds_Count(t *testing.T) {
	assert.Equal(t, 7, len(AllFrontmatterFieldKinds()))
}

func TestFrontmatterFieldKind_AllKinds_Unique(t *testing.T) {
	seen := map[FrontmatterFieldKind]bool{}
	for _, k := range AllFrontmatterFieldKinds() {
		assert.False(t, seen[k], "duplicate kind %q", k)
		seen[k] = true
	}
}

// ---- NewFrontmatterSchema ----

func TestNewFrontmatterSchema_Empty(t *testing.T) {
	s, err := NewFrontmatterSchema()
	require.NoError(t, err)
	assert.Equal(t, 0, s.Size())
}

func TestNewFrontmatterSchema_ValidFields(t *testing.T) {
	s, err := NewFrontmatterSchema(
		&FrontmatterField{Name: "description", Kind: FrontmatterFieldKindString, Required: true},
		&FrontmatterField{Name: "enabled", Kind: FrontmatterFieldKindBool},
	)
	require.NoError(t, err)
	assert.Equal(t, 2, s.Size())
}

func TestNewFrontmatterSchema_EmptyNameReturnsError(t *testing.T) {
	_, err := NewFrontmatterSchema(
		&FrontmatterField{Name: "", Kind: FrontmatterFieldKindString},
	)
	assert.ErrorIs(t, err, ErrFrontmatterFieldNameEmpty)
}

func TestNewFrontmatterSchema_InvalidKindReturnsError(t *testing.T) {
	_, err := NewFrontmatterSchema(
		&FrontmatterField{Name: "x", Kind: "bogus"},
	)
	assert.ErrorIs(t, err, ErrFrontmatterFieldKindInvalid)
}

func TestNewFrontmatterSchema_DuplicateNameReturnsError(t *testing.T) {
	_, err := NewFrontmatterSchema(
		&FrontmatterField{Name: "description", Kind: FrontmatterFieldKindString},
		&FrontmatterField{Name: "description", Kind: FrontmatterFieldKindString},
	)
	assert.ErrorIs(t, err, ErrFrontmatterSchemaDuplicateField)
}

func TestFrontmatterSchema_Field_Found(t *testing.T) {
	s, _ := NewFrontmatterSchema(
		&FrontmatterField{Name: "model", Kind: FrontmatterFieldKindModelRef},
	)
	f, ok := s.Field("model")
	require.True(t, ok)
	assert.Equal(t, "model", f.Name)
}

func TestFrontmatterSchema_Field_NotFound(t *testing.T) {
	s, _ := NewFrontmatterSchema()
	_, ok := s.Field("nope")
	assert.False(t, ok)
}

func TestFrontmatterSchema_AllFields_IsDefensiveCopy(t *testing.T) {
	s, _ := NewFrontmatterSchema(
		&FrontmatterField{Name: "x", Kind: FrontmatterFieldKindString},
	)
	first := s.AllFields()
	first[0] = nil
	second := s.AllFields()
	assert.NotNil(t, second[0])
}

// ---- FrontmatterSchema.Validate ----

func simpleSchema(t *testing.T) *FrontmatterSchema {
	t.Helper()
	s, err := NewFrontmatterSchema(
		&FrontmatterField{Name: "description", Kind: FrontmatterFieldKindString, Required: true},
		&FrontmatterField{Name: "enabled", Kind: FrontmatterFieldKindBool},
		&FrontmatterField{Name: "timeout", Kind: FrontmatterFieldKindInt},
		&FrontmatterField{Name: "temperature", Kind: FrontmatterFieldKindFloat},
		&FrontmatterField{Name: "model", Kind: FrontmatterFieldKindModelRef},
		&FrontmatterField{Name: "slug", Kind: FrontmatterFieldKindSlug},
		&FrontmatterField{Name: "tags", Kind: FrontmatterFieldKindList},
	)
	require.NoError(t, err)
	return s
}

func parsedWith(fields map[string]string) ParsedFrontmatter {
	return ParsedFrontmatter{Fields: fields}
}

func TestFrontmatterSchema_Validate_Happy(t *testing.T) {
	s := simpleSchema(t)
	p := parsedWith(map[string]string{
		"description": "A test agent",
		"enabled":     "true",
		"timeout":     "30",
		"temperature": "0.7",
		"model":       "claude-sonnet-4-6",
		"slug":        "my-agent",
		"tags":        "security,audit",
	})
	vs := s.Validate(p)
	assert.Empty(t, vs)
}

func TestFrontmatterSchema_Validate_RequiredMissing(t *testing.T) {
	s := simpleSchema(t)
	vs := s.Validate(parsedWith(map[string]string{}))
	assert.True(t, HasErrors(vs))
	found := false
	for _, v := range vs {
		if v.FieldName == "description" && v.Severity == FrontmatterViolationError {
			found = true
		}
	}
	assert.True(t, found, "expected error for missing required field 'description'")
}

func TestFrontmatterSchema_Validate_BoolInvalid(t *testing.T) {
	s := simpleSchema(t)
	vs := s.Validate(parsedWith(map[string]string{
		"description": "ok",
		"enabled":     "yes",
	}))
	hasEnabledError := false
	for _, v := range vs {
		if v.FieldName == "enabled" {
			hasEnabledError = true
		}
	}
	assert.True(t, hasEnabledError)
}

func TestFrontmatterSchema_Validate_IntInvalid(t *testing.T) {
	s := simpleSchema(t)
	vs := s.Validate(parsedWith(map[string]string{
		"description": "ok",
		"timeout":     "abc",
	}))
	hasTimeoutError := false
	for _, v := range vs {
		if v.FieldName == "timeout" {
			hasTimeoutError = true
		}
	}
	assert.True(t, hasTimeoutError)
}

func TestFrontmatterSchema_Validate_FloatInvalid(t *testing.T) {
	s := simpleSchema(t)
	vs := s.Validate(parsedWith(map[string]string{
		"description": "ok",
		"temperature": "very-warm",
	}))
	hasError := false
	for _, v := range vs {
		if v.FieldName == "temperature" {
			hasError = true
		}
	}
	assert.True(t, hasError)
}

func TestFrontmatterSchema_Validate_SlugInvalid(t *testing.T) {
	s := simpleSchema(t)
	vs := s.Validate(parsedWith(map[string]string{
		"description": "ok",
		"slug":        "Bad-SLUG",
	}))
	hasError := false
	for _, v := range vs {
		if v.FieldName == "slug" {
			hasError = true
		}
	}
	assert.True(t, hasError)
}

func TestFrontmatterSchema_Validate_UnknownFieldIsWarning(t *testing.T) {
	s := simpleSchema(t)
	vs := s.Validate(parsedWith(map[string]string{
		"description": "ok",
		"unknown_key": "value",
	}))
	hasWarning := false
	for _, v := range vs {
		if v.FieldName == "unknown_key" && v.Severity == FrontmatterViolationWarning {
			hasWarning = true
		}
	}
	assert.True(t, hasWarning)
}

func TestFrontmatterSchema_Validate_ExtraValidateHook(t *testing.T) {
	s, err := NewFrontmatterSchema(
		&FrontmatterField{
			Name: "temperature", Kind: FrontmatterFieldKindFloat,
			ExtraValidate: func(raw string) error {
				// temperature must be ≤ 2.0
				f, _ := parseFlt(raw)
				if f > 2.0 {
					return errors.New("temperature must be ≤ 2.0")
				}
				return nil
			},
		},
	)
	require.NoError(t, err)
	vs := s.Validate(parsedWith(map[string]string{"temperature": "5.0"}))
	hasError := false
	for _, v := range vs {
		if v.FieldName == "temperature" {
			hasError = true
		}
	}
	assert.True(t, hasError)
}

func parseFlt(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}

// ---- HasErrors ----

func TestHasErrors_EmptySlice(t *testing.T) {
	assert.False(t, HasErrors(nil))
	assert.False(t, HasErrors([]FrontmatterViolation{}))
}

func TestHasErrors_OnlyWarnings(t *testing.T) {
	vs := []FrontmatterViolation{
		{FieldName: "x", Message: "unknown", Severity: FrontmatterViolationWarning},
	}
	assert.False(t, HasErrors(vs))
}

func TestHasErrors_WithError(t *testing.T) {
	vs := []FrontmatterViolation{
		{FieldName: "description", Message: "required", Severity: FrontmatterViolationError},
	}
	assert.True(t, HasErrors(vs))
}

// ---- AgentFrontmatterSchema ----

func TestAgentFrontmatterSchema_Size(t *testing.T) {
	assert.Equal(t, 15, AgentFrontmatterSchema.Size())
}

func TestAgentFrontmatterSchema_DescriptionRequired(t *testing.T) {
	f, ok := AgentFrontmatterSchema.Field("description")
	require.True(t, ok)
	assert.True(t, f.Required)
	assert.Equal(t, FrontmatterFieldKindString, f.Kind)
}

func TestAgentFrontmatterSchema_ModelIsModelRef(t *testing.T) {
	f, ok := AgentFrontmatterSchema.Field("model")
	require.True(t, ok)
	assert.Equal(t, FrontmatterFieldKindModelRef, f.Kind)
}

func TestAgentFrontmatterSchema_TemperatureIsFloat(t *testing.T) {
	f, ok := AgentFrontmatterSchema.Field("temperature")
	require.True(t, ok)
	assert.Equal(t, FrontmatterFieldKindFloat, f.Kind)
}

func TestAgentFrontmatterSchema_MaxTokensIsInt(t *testing.T) {
	f, ok := AgentFrontmatterSchema.Field("max_tokens")
	require.True(t, ok)
	assert.Equal(t, FrontmatterFieldKindInt, f.Kind)
}

func TestAgentFrontmatterSchema_MaxTurnsIsInt(t *testing.T) {
	f, ok := AgentFrontmatterSchema.Field("max_turns")
	require.True(t, ok)
	assert.Equal(t, FrontmatterFieldKindInt, f.Kind)
}

func TestAgentFrontmatterSchema_TagsIsList(t *testing.T) {
	f, ok := AgentFrontmatterSchema.Field("tags")
	require.True(t, ok)
	assert.Equal(t, FrontmatterFieldKindList, f.Kind)
}

func TestAgentFrontmatterSchema_ContextWindowPctIsInt(t *testing.T) {
	f, ok := AgentFrontmatterSchema.Field("context_window_pct")
	require.True(t, ok)
	assert.Equal(t, FrontmatterFieldKindInt, f.Kind)
}

func TestAgentFrontmatterSchema_TemperatureOver2IsError(t *testing.T) {
	p := parsedWith(map[string]string{
		"description": "ok",
		"temperature": "3.0",
	})
	vs := AgentFrontmatterSchema.Validate(p)
	hasTempError := false
	for _, v := range vs {
		if v.FieldName == "temperature" && v.Severity == FrontmatterViolationError {
			hasTempError = true
		}
	}
	assert.True(t, hasTempError)
}

func TestAgentFrontmatterSchema_ContextWindowPctOutOfRangeIsError(t *testing.T) {
	p := parsedWith(map[string]string{
		"description":        "ok",
		"context_window_pct": "150",
	})
	vs := AgentFrontmatterSchema.Validate(p)
	hasCtxError := false
	for _, v := range vs {
		if v.FieldName == "context_window_pct" && v.Severity == FrontmatterViolationError {
			hasCtxError = true
		}
	}
	assert.True(t, hasCtxError)
}

func TestAgentFrontmatterSchema_ValidFullSpec(t *testing.T) {
	p := parsedWith(map[string]string{
		"description":        "A research assistant agent",
		"model":              "claude-sonnet-4-6",
		"temperature":        "0.7",
		"max_tokens":         "2048",
		"max_turns":          "10",
		"timeout":            "300",
		"context_window_pct": "80",
		"tools":              "document_search,sql",
		"allowed_tools":      "document_search,sql",
		"enabled":            "true",
		"tags":               "research,knowledge-base",
		"priority":           "0",
		"schema_version":     "1",
		"author":             "AgentHub Team",
		"version":            "1.0.0",
	})
	vs := AgentFrontmatterSchema.Validate(p)
	assert.False(t, HasErrors(vs))
}

// ---- SkillFrontmatterSchema ----

func TestSkillFrontmatterSchema_Size(t *testing.T) {
	assert.Equal(t, 10, SkillFrontmatterSchema.Size())
}

func TestSkillFrontmatterSchema_DescriptionRequired(t *testing.T) {
	f, ok := SkillFrontmatterSchema.Field("description")
	require.True(t, ok)
	assert.True(t, f.Required)
}

func TestSkillFrontmatterSchema_NoTemperatureField(t *testing.T) {
	// Skills don't have temperature or max_tokens — those are agent-level.
	_, ok := SkillFrontmatterSchema.Field("temperature")
	assert.False(t, ok)
	_, ok = SkillFrontmatterSchema.Field("max_tokens")
	assert.False(t, ok)
}

func TestFrontmatterViolation_String(t *testing.T) {
	v := FrontmatterViolation{
		FieldName: "description",
		Message:   "required field is missing",
		Severity:  FrontmatterViolationError,
	}
	s := v.String()
	assert.Contains(t, s, "error")
	assert.Contains(t, s, "description")
}
