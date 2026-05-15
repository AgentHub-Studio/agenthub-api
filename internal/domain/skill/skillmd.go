package skill

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// SkillMDVersion is the format version embedded in serialized SKILL.md files.
const SkillMDVersion = "1"

// skillMDFrontmatter is the YAML structure inside the --- delimiters.
// Fields map 1-to-1 to the Skill model; snake_case keys match the DB columns.
//
// PDF Section 6.1 gap closure (EXT-006a): model_overrides, effort_level,
// associated_agents, dynamic_hooks added below.
// shell_config is NOT_APPLICABLE_WEB — AgentHub is a web platform.
type skillMDFrontmatter struct {
	Version                string            `yaml:"version"`
	Name                   string            `yaml:"name"`
	Slug                   string            `yaml:"slug,omitempty"`
	Description            string            `yaml:"description,omitempty"`
	Category               string            `yaml:"category,omitempty"`
	WhenToUse              string            `yaml:"when_to_use,omitempty"`
	ContextMode            string            `yaml:"context_mode,omitempty"`
	AllowedTools           []string          `yaml:"allowed_tools,omitempty"`
	DisableModelInvocation bool              `yaml:"disable_model_invocation,omitempty"`
	ArgumentHint           string            `yaml:"argument_hint,omitempty"`
	ShouldDefer            bool              `yaml:"should_defer,omitempty"`
	// ModelOverrides lets a skill request a different model than the agent default.
	// Keys are usage contexts ("default", "vision"); values are model IDs.
	// Inspired by Claude Code BundledSkillDefinition.modelOverrides.
	ModelOverrides map[string]string `yaml:"model_overrides,omitempty"`
	// EffortLevel sets the Anthropic thinking-budget tier for this skill's turns.
	// Valid values: lowest, low, medium, high, highest.
	// Inspired by Claude Code BundledSkillDefinition.effortLevel.
	EffortLevel string `yaml:"effort_level,omitempty"`
	// AssociatedAgents lists agent slugs whose persona this skill is bound to.
	// On invocation the runner activates the named agent's system prompt.
	// Inspired by Claude Code BundledSkillDefinition.associatedAgentDefinitions.
	AssociatedAgents []string `yaml:"associated_agents,omitempty"`
	// DynamicHooks lists hook-event slugs registered for the duration of the skill.
	// The runner wires these hooks before executing and removes them after.
	// Inspired by Claude Code skill-level dynamic hooks (Section 6.1).
	DynamicHooks []string `yaml:"dynamic_hooks,omitempty"`
}

// ParseSkillMD parses a SKILL.md file into a CreateRequest.
// The file must start with a YAML frontmatter block delimited by "---" lines.
// The content after the second "---" delimiter becomes the skill instructions.
func ParseSkillMD(content string) (CreateRequest, error) {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "---") {
		return CreateRequest{}, fmt.Errorf("skillmd: missing frontmatter — file must start with ---")
	}

	// Split on frontmatter delimiters.
	// Format: ---\n<yaml>\n---\n<body>
	rest := content[3:] // skip leading ---
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return CreateRequest{}, fmt.Errorf("skillmd: unclosed frontmatter — missing closing ---")
	}
	frontmatterRaw := strings.TrimSpace(rest[:idx])
	body := strings.TrimSpace(rest[idx+4:]) // skip \n---

	var fm skillMDFrontmatter
	if err := yaml.Unmarshal([]byte(frontmatterRaw), &fm); err != nil {
		return CreateRequest{}, fmt.Errorf("skillmd: parse frontmatter: %w", err)
	}

	if fm.Name == "" {
		return CreateRequest{}, fmt.Errorf("skillmd: missing required field: name")
	}

	var whenToUse *string
	if fm.WhenToUse != "" {
		whenToUse = &fm.WhenToUse
	}

	var argumentHint *string
	if fm.ArgumentHint != "" {
		argumentHint = &fm.ArgumentHint
	}

	return CreateRequest{
		Name:                   fm.Name,
		Slug:                   fm.Slug,
		Description:            fm.Description,
		Instructions:           body,
		Category:               fm.Category,
		WhenToUse:              whenToUse,
		ContextMode:            fm.ContextMode,
		AllowedTools:           fm.AllowedTools,
		DisableModelInvocation: fm.DisableModelInvocation,
		ArgumentHint:           argumentHint,
		ShouldDefer:            fm.ShouldDefer,
		ModelOverrides:         fm.ModelOverrides,
		EffortLevel:            fm.EffortLevel,
		AssociatedAgents:       fm.AssociatedAgents,
		DynamicHooks:           fm.DynamicHooks,
	}, nil
}

// SerializeSkillMD serializes a Skill into the SKILL.md portable format.
// The frontmatter contains metadata; the body contains skill instructions.
func SerializeSkillMD(s Skill) string {
	fm := skillMDFrontmatter{
		Version:                SkillMDVersion,
		Name:                   s.Name,
		Slug:                   s.Slug,
		Description:            s.Description,
		Category:               s.Category,
		ContextMode:            s.ContextMode,
		AllowedTools:           s.AllowedTools,
		DisableModelInvocation: s.DisableModelInvocation,
		ShouldDefer:            s.ShouldDefer,
	}
	if s.WhenToUse != nil {
		fm.WhenToUse = *s.WhenToUse
	}
	if s.ArgumentHint != nil {
		fm.ArgumentHint = *s.ArgumentHint
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	_ = enc.Encode(fm)
	_ = enc.Close()

	frontmatter := strings.TrimSpace(buf.String())

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(frontmatter)
	sb.WriteString("\n---\n")
	if s.Instructions != "" {
		sb.WriteString("\n")
		sb.WriteString(s.Instructions)
		sb.WriteString("\n")
	}
	return sb.String()
}
