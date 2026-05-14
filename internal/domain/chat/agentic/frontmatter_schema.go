package agentic

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// EXT-006a — Typed frontmatter schema definitions.
//
// PDF arXiv:2604.14228v1 §9 (agent/skill configuration via CLAUDE.md and
// tool-spec files uses YAML frontmatter); §3.1 (model, temperature, and
// max_tokens are frontmatter-configurable per agent); §11 (max_turns
// controls how many tool-call cycles are allowed before stopping).
//
// This file adds a typed schema layer on top of frontmatter.go's parser:
//
//   - FrontmatterFieldKind  — 7-value type enum
//   - FrontmatterField      — typed field definition with optional validation
//   - FrontmatterSchema     — ordered registry + Validate()
//   - FrontmatterViolation  — validation finding (error or warning)
//   - AgentFrontmatterSchema  — canonical 15-field schema for agent files
//   - SkillFrontmatterSchema  — 10-field subset for skill/tool files
//
// Distinction from frontmatter.go:
//   frontmatter.go  — parsing and text transformations (no type system).
//   frontmatter_schema.go — type declarations, field contracts, validation.

// FrontmatterFieldKind classifies the wire-type of a frontmatter field.
type FrontmatterFieldKind string

const (
	// FrontmatterFieldKindString accepts any non-empty string.
	FrontmatterFieldKindString FrontmatterFieldKind = "string"
	// FrontmatterFieldKindBool accepts "true" or "false".
	FrontmatterFieldKindBool FrontmatterFieldKind = "bool"
	// FrontmatterFieldKindInt accepts a non-negative integer string.
	FrontmatterFieldKindInt FrontmatterFieldKind = "int"
	// FrontmatterFieldKindFloat accepts a non-negative decimal string.
	FrontmatterFieldKindFloat FrontmatterFieldKind = "float"
	// FrontmatterFieldKindList accepts a comma-separated string.
	FrontmatterFieldKindList FrontmatterFieldKind = "list"
	// FrontmatterFieldKindModelRef accepts a model identifier string
	// (e.g. "claude-sonnet-4-6"). Not further validated at schema level.
	FrontmatterFieldKindModelRef FrontmatterFieldKind = "model_ref"
	// FrontmatterFieldKindSlug accepts a lowercase kebab-case string.
	FrontmatterFieldKindSlug FrontmatterFieldKind = "slug"
)

// IsValid returns true when k is in the closed set.
func (k FrontmatterFieldKind) IsValid() bool {
	switch k {
	case FrontmatterFieldKindString, FrontmatterFieldKindBool,
		FrontmatterFieldKindInt, FrontmatterFieldKindFloat,
		FrontmatterFieldKindList, FrontmatterFieldKindModelRef,
		FrontmatterFieldKindSlug:
		return true
	}
	return false
}

// AllFrontmatterFieldKinds returns a defensive copy of every valid kind.
func AllFrontmatterFieldKinds() []FrontmatterFieldKind {
	return []FrontmatterFieldKind{
		FrontmatterFieldKindString, FrontmatterFieldKindBool,
		FrontmatterFieldKindInt, FrontmatterFieldKindFloat,
		FrontmatterFieldKindList, FrontmatterFieldKindModelRef,
		FrontmatterFieldKindSlug,
	}
}

// FrontmatterViolationSeverity classifies how critical a violation is.
type FrontmatterViolationSeverity string

const (
	FrontmatterViolationError   FrontmatterViolationSeverity = "error"
	FrontmatterViolationWarning FrontmatterViolationSeverity = "warning"
)

// FrontmatterViolation is one validation finding from FrontmatterSchema.Validate.
type FrontmatterViolation struct {
	FieldName string
	Message   string
	Severity  FrontmatterViolationSeverity
}

func (v FrontmatterViolation) String() string {
	return fmt.Sprintf("[%s] %s: %s", v.Severity, v.FieldName, v.Message)
}

// FrontmatterField is the typed definition of a single frontmatter key.
type FrontmatterField struct {
	// Name is the YAML key (e.g. "temperature").
	Name string
	// Kind classifies the wire type.
	Kind FrontmatterFieldKind
	// Required causes Validate to emit an error when the field is absent.
	Required bool
	// DefaultStr is the raw string default shown in documentation;
	// it does not participate in validation (defaults are advisory).
	DefaultStr string
	// Description explains the field to operators.
	Description string
	// ExtraValidate is an optional domain-level check called after
	// kind validation succeeds. Returns a non-nil error to add a
	// violation; the field name is attached automatically.
	ExtraValidate func(raw string) error
}

// Sentinel errors used inside FrontmatterSchema.
var (
	ErrFrontmatterFieldNameEmpty = errors.New("frontmatter schema: field name must not be empty")
	ErrFrontmatterFieldKindInvalid = errors.New("frontmatter schema: field kind not in closed set")
	ErrFrontmatterSchemaDuplicateField = errors.New("frontmatter schema: duplicate field name")
)

// Compiled regex for slug validation.
var frontmatterSlugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

// validateKind checks whether raw passes the wire-type rules.
func validateKind(raw string, kind FrontmatterFieldKind) error {
	switch kind {
	case FrontmatterFieldKindBool:
		if raw != "true" && raw != "false" {
			return fmt.Errorf("must be \"true\" or \"false\", got %q", raw)
		}
	case FrontmatterFieldKindInt:
		n, err := strconv.Atoi(raw)
		if err != nil {
			return fmt.Errorf("must be an integer, got %q", raw)
		}
		if n < 0 {
			return fmt.Errorf("must be non-negative, got %d", n)
		}
	case FrontmatterFieldKindFloat:
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return fmt.Errorf("must be a decimal, got %q", raw)
		}
		if f < 0 {
			return fmt.Errorf("must be non-negative, got %g", f)
		}
	case FrontmatterFieldKindSlug:
		if !frontmatterSlugRE.MatchString(raw) {
			return fmt.Errorf("must be lowercase kebab-case (≥2 chars), got %q", raw)
		}
	// string, list, model_ref: accept any non-empty value (length checked by Required)
	}
	return nil
}

// FrontmatterSchema is an ordered registry of typed field definitions.
// It validates a ParsedFrontmatter and returns all findings.
type FrontmatterSchema struct {
	fields []*FrontmatterField
	byName map[string]*FrontmatterField
}

// NewFrontmatterSchema constructs a schema from the given field definitions.
// Returns an error when a field has an empty name, an invalid kind, or a
// duplicate name.
func NewFrontmatterSchema(fields ...*FrontmatterField) (*FrontmatterSchema, error) {
	s := &FrontmatterSchema{
		fields: make([]*FrontmatterField, 0, len(fields)),
		byName: make(map[string]*FrontmatterField, len(fields)),
	}
	for _, f := range fields {
		if f.Name == "" {
			return nil, ErrFrontmatterFieldNameEmpty
		}
		if !f.Kind.IsValid() {
			return nil, ErrFrontmatterFieldKindInvalid
		}
		if _, dup := s.byName[f.Name]; dup {
			return nil, fmt.Errorf("%w: %q", ErrFrontmatterSchemaDuplicateField, f.Name)
		}
		s.byName[f.Name] = f
		s.fields = append(s.fields, f)
	}
	return s, nil
}

// Field looks up a field by name. Returns (nil, false) when not found.
func (s *FrontmatterSchema) Field(name string) (*FrontmatterField, bool) {
	f, ok := s.byName[name]
	return f, ok
}

// AllFields returns a defensive copy of all fields in declaration order.
func (s *FrontmatterSchema) AllFields() []*FrontmatterField {
	out := make([]*FrontmatterField, len(s.fields))
	copy(out, s.fields)
	return out
}

// Size returns the number of declared fields.
func (s *FrontmatterSchema) Size() int { return len(s.fields) }

// Validate checks parsed against s. It returns all violations found
// (errors for required/type failures, warnings for unknown fields).
// An empty slice means the frontmatter is valid.
func (s *FrontmatterSchema) Validate(parsed ParsedFrontmatter) []FrontmatterViolation {
	var out []FrontmatterViolation

	// Check required fields and type-validate present fields.
	for _, f := range s.fields {
		raw, present := parsed.Fields[f.Name]
		if !present || strings.TrimSpace(raw) == "" {
			if f.Required {
				out = append(out, FrontmatterViolation{
					FieldName: f.Name,
					Message:   "required field is missing",
					Severity:  FrontmatterViolationError,
				})
			}
			continue
		}
		// Kind validation.
		if err := validateKind(raw, f.Kind); err != nil {
			out = append(out, FrontmatterViolation{
				FieldName: f.Name,
				Message:   err.Error(),
				Severity:  FrontmatterViolationError,
			})
			continue
		}
		// Extra domain validation.
		if f.ExtraValidate != nil {
			if err := f.ExtraValidate(raw); err != nil {
				out = append(out, FrontmatterViolation{
					FieldName: f.Name,
					Message:   err.Error(),
					Severity:  FrontmatterViolationError,
				})
			}
		}
	}

	// Warn about unknown fields not declared in the schema.
	for key := range parsed.Fields {
		if _, ok := s.byName[key]; !ok {
			out = append(out, FrontmatterViolation{
				FieldName: key,
				Message:   "unknown field — not declared in schema",
				Severity:  FrontmatterViolationWarning,
			})
		}
	}
	return out
}

// HasErrors returns true when any violation in vs is an error.
func HasErrors(vs []FrontmatterViolation) bool {
	for _, v := range vs {
		if v.Severity == FrontmatterViolationError {
			return true
		}
	}
	return false
}

// --- Pre-built canonical schemas ---

// AgentFrontmatterSchema is the canonical 15-field schema for agent CLAUDE.md files.
// Adapted from Claude Code §3.1 + §9 + §11.
var AgentFrontmatterSchema = mustBuildAgentSchema()

func mustBuildAgentSchema() *FrontmatterSchema {
	temperatureCheck := func(raw string) error {
		f, _ := strconv.ParseFloat(raw, 64)
		if f > 2.0 {
			return fmt.Errorf("temperature must be ≤ 2.0, got %g", f)
		}
		return nil
	}
	ctxPctCheck := func(raw string) error {
		n, _ := strconv.Atoi(raw)
		if n < 1 || n > 100 {
			return fmt.Errorf("context_window_pct must be 1–100, got %d", n)
		}
		return nil
	}
	positiveCheck := func(field string) func(string) error {
		return func(raw string) error {
			n, _ := strconv.Atoi(raw)
			if n <= 0 {
				return fmt.Errorf("%s must be > 0, got %d", field, n)
			}
			return nil
		}
	}

	fields := []*FrontmatterField{
		// Core identity
		{Name: "description", Kind: FrontmatterFieldKindString, Required: true,
			Description: "One-sentence description of what this agent does."},
		{Name: "model", Kind: FrontmatterFieldKindModelRef, Required: false,
			DefaultStr:  "claude-sonnet-4-6",
			Description: "LLM model identifier (e.g. claude-sonnet-4-6)."},
		{Name: "schema_version", Kind: FrontmatterFieldKindString, Required: false,
			DefaultStr:  "1",
			Description: "Frontmatter schema version for forward compatibility."},
		{Name: "author", Kind: FrontmatterFieldKindString, Required: false,
			Description: "Author or team responsible for this agent."},
		{Name: "version", Kind: FrontmatterFieldKindString, Required: false,
			DefaultStr:  "1.0.0",
			Description: "Semantic version of the agent definition."},
		// Execution parameters
		{Name: "temperature", Kind: FrontmatterFieldKindFloat, Required: false,
			DefaultStr:    "0.7",
			Description:   "Sampling temperature (0.0–2.0). Lower = more deterministic.",
			ExtraValidate: temperatureCheck},
		{Name: "max_tokens", Kind: FrontmatterFieldKindInt, Required: false,
			DefaultStr:    "2048",
			Description:   "Maximum output tokens per LLM call.",
			ExtraValidate: positiveCheck("max_tokens")},
		{Name: "max_turns", Kind: FrontmatterFieldKindInt, Required: false,
			DefaultStr:    "10",
			Description:   "Maximum agentic tool-call cycles before stopping.",
			ExtraValidate: positiveCheck("max_turns")},
		{Name: "timeout", Kind: FrontmatterFieldKindInt, Required: false,
			DefaultStr:    "300",
			Description:   "Run timeout in seconds.",
			ExtraValidate: positiveCheck("timeout")},
		{Name: "context_window_pct", Kind: FrontmatterFieldKindInt, Required: false,
			DefaultStr:    "80",
			Description:   "Alert threshold as % of context window used (1–100).",
			ExtraValidate: ctxPctCheck},
		// Capability control
		{Name: "tools", Kind: FrontmatterFieldKindList, Required: false,
			Description: "Comma-separated list of allowed tool slugs."},
		{Name: "allowed_tools", Kind: FrontmatterFieldKindList, Required: false,
			Description: "Alias for tools — comma-separated allowed tool slugs."},
		{Name: "enabled", Kind: FrontmatterFieldKindBool, Required: false,
			DefaultStr:  "true",
			Description: "Whether the agent is enabled for invocation."},
		// Discovery
		{Name: "tags", Kind: FrontmatterFieldKindList, Required: false,
			Description: "Comma-separated discovery tags (e.g. \"security,audit\")."},
		{Name: "priority", Kind: FrontmatterFieldKindInt, Required: false,
			DefaultStr:  "0",
			Description: "Ordering priority (0 = default; higher = preferred)."},
	}
	s, err := NewFrontmatterSchema(fields...)
	if err != nil {
		panic("AgentFrontmatterSchema build failed: " + err.Error())
	}
	return s
}

// SkillFrontmatterSchema is a 10-field subset schema for skill/tool definition files.
var SkillFrontmatterSchema = mustBuildSkillSchema()

func mustBuildSkillSchema() *FrontmatterSchema {
	positiveCheck := func(field string) func(string) error {
		return func(raw string) error {
			n, _ := strconv.Atoi(raw)
			if n <= 0 {
				return fmt.Errorf("%s must be > 0, got %d", field, n)
			}
			return nil
		}
	}

	fields := []*FrontmatterField{
		{Name: "description", Kind: FrontmatterFieldKindString, Required: true,
			Description: "One-sentence description of what this skill does."},
		{Name: "tools", Kind: FrontmatterFieldKindList, Required: false,
			Description: "Comma-separated list of tool slugs this skill bundles."},
		{Name: "allowed_tools", Kind: FrontmatterFieldKindList, Required: false,
			Description: "Alias for tools."},
		{Name: "enabled", Kind: FrontmatterFieldKindBool, Required: false,
			DefaultStr:  "true",
			Description: "Whether this skill is available for agent binding."},
		{Name: "timeout", Kind: FrontmatterFieldKindInt, Required: false,
			DefaultStr:    "60",
			Description:   "Execution timeout in seconds.",
			ExtraValidate: positiveCheck("timeout")},
		{Name: "tags", Kind: FrontmatterFieldKindList, Required: false,
			Description: "Discovery tags."},
		{Name: "schema_version", Kind: FrontmatterFieldKindString, Required: false,
			Description: "Schema version for forward compatibility."},
		{Name: "author", Kind: FrontmatterFieldKindString, Required: false,
			Description: "Author or team."},
		{Name: "version", Kind: FrontmatterFieldKindString, Required: false,
			Description: "Semantic version."},
		{Name: "priority", Kind: FrontmatterFieldKindInt, Required: false,
			DefaultStr:  "0",
			Description: "Ordering priority."},
	}
	s, err := NewFrontmatterSchema(fields...)
	if err != nil {
		panic("SkillFrontmatterSchema build failed: " + err.Error())
	}
	return s
}
