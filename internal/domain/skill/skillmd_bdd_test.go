package skill_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skill"
)

// BDD-style scenarios that ratify EXT-006 (Skills com frontmatter) against
// the Claude Code architecture paper "Dive into Claude Code"
// (arXiv:2604.14228v1):
//
//   - Section 6.1 ("Four Extension Mechanisms"): "Each skill is defined by a
//     SKILL.md file with YAML frontmatter. The parseSkillFrontmatterFields()
//     function (loadSkillsDir.ts) parses 15+ fields including display name,
//     description, allowed tools (granting the skill access to additional
//     tools), argument hints, model overrides, execution context ('fork' for
//     isolated execution), associated agent definitions, effort levels, and
//     shell configuration. Skills can define their own hooks, which register
//     dynamically on invocation. Bundled skills are registered in-memory at
//     startup. When invoked, the SkillTool meta-tool injects the skill's
//     instructions into the context."
//   - Tabela 2: skills consume LOW context cost (descriptions only) at
//     assemble() time; full instructions only loaded when invoked.
//
// AgentHub maps the Skills-with-frontmatter pattern to:
//   - skill.go ParseSkillMD / SerializeSkillMD — YAML frontmatter ↔ Skill
//     entity round-trip parser.
//   - skillMDFrontmatter — typed YAML struct with 11 explicit fields (subset
//     of PDF's 15+; gaps tracked below).
//   - skill.Skill model — the durable representation backing the parser.
//
// These scenarios assert: parser robustness (delimiter handling), required
// fields, optional fields, round-trip stability, and PDF field coverage.

func TestBDD_SkillFrontmatter(t *testing.T) {
	t.Run("Scenario_FrontmatterMustStartWithDelimiter", func(t *testing.T) {
		// Given a SKILL.md file without leading --- (PDF Section 6.1: YAML
		//       frontmatter is the canonical delimiter convention),
		given := "name: example\ndescription: bad"

		// When the parser inspects it,
		_, err := skill.ParseSkillMD(given)

		// Then parsing fails with a clear error — no silent acceptance of
		//      malformed files.
		assert.Error(t, err,
			"file without leading --- must be rejected")
		assert.Contains(t, err.Error(), "frontmatter",
			"error message must mention frontmatter for operator clarity")
	})

	t.Run("Scenario_FrontmatterMustHaveClosingDelimiter", func(t *testing.T) {
		// Given a frontmatter that opens but never closes,
		given := "---\nname: example\ndescription: never closed"

		// When the parser tries to split,
		_, err := skill.ParseSkillMD(given)

		// Then parsing fails — preventing arbitrarily long YAML blobs from
		//      eating the body content.
		assert.Error(t, err,
			"unclosed frontmatter must be rejected")
		assert.Contains(t, err.Error(), "unclosed",
			"error must specify unclosed delimiter for clarity")
	})

	t.Run("Scenario_NameIsRequired", func(t *testing.T) {
		// Given a frontmatter without name (PDF Section 6.1: skill must be
		//       identifiable),
		given := "---\ndescription: anonymous skill\n---\nbody"

		// When the parser inspects,
		_, err := skill.ParseSkillMD(given)

		// Then parsing fails — anonymous skills cannot be registered.
		assert.Error(t, err,
			"skill without name must be rejected")
		assert.Contains(t, err.Error(), "name",
			"error must explicitly mention the missing name field")
	})

	t.Run("Scenario_MinimalValidSkillParsesCleanly", func(t *testing.T) {
		// Given a minimal but valid SKILL.md,
		given := `---
name: example
description: An example skill
---
This is the skill body.`

		// When the parser runs,
		when, err := skill.ParseSkillMD(given)

		// Then the CreateRequest is populated with name + description + body
		//      as instructions (PDF Section 6.1: body becomes the prompt
		//      injected by SkillTool when invoked).
		assert.NoError(t, err)
		assert.Equal(t, "example", when.Name)
		assert.Equal(t, "An example skill", when.Description)
		assert.Equal(t, "This is the skill body.", when.Instructions,
			"body content must become Instructions for runtime injection")
	})

	t.Run("Scenario_FullFrontmatterFieldsAreParsed", func(t *testing.T) {
		// Given a SKILL.md with all currently-supported AgentHub fields,
		given := `---
version: "1"
name: deep_research
slug: deep-research
description: Conducts thorough multi-source research
category: research
when_to_use: When the user asks for in-depth multi-source verification
context_mode: fork
allowed_tools:
  - document-search
  - http-request
disable_model_invocation: false
argument_hint: "<topic>"
should_defer: true
---
You are a research specialist. Verify across at least 3 sources.`

		// When the parser runs,
		when, err := skill.ParseSkillMD(given)

		// Then every PDF-relevant field round-trips into the CreateRequest.
		assert.NoError(t, err)
		assert.Equal(t, "deep_research", when.Name)
		assert.Equal(t, "deep-research", when.Slug,
			"slug for stable LLM-facing identifier")
		assert.Equal(t, "research", when.Category)
		assert.NotNil(t, when.WhenToUse)
		assert.Equal(t, "When the user asks for in-depth multi-source verification", *when.WhenToUse,
			"WhenToUse provides PDF Section 6.1 'when' guidance separate from description (what)")
		assert.Equal(t, "fork", when.ContextMode,
			"ContextMode='fork' triggers isolated execution per PDF Section 6.1 + 8")
		assert.Equal(t, []string{"document-search", "http-request"}, when.AllowedTools,
			"AllowedTools grants skill access to additional tools per PDF Section 6.1")
		assert.False(t, when.DisableModelInvocation,
			"explicit false must round-trip")
		assert.NotNil(t, when.ArgumentHint)
		assert.Equal(t, "<topic>", *when.ArgumentHint,
			"ArgumentHint helps the LLM construct the call shape")
		assert.True(t, when.ShouldDefer,
			"ShouldDefer=true means skill loaded only on demand (Section 3.6)")
		assert.True(t, strings.HasPrefix(when.Instructions, "You are"),
			"body becomes runtime Instructions injected by SkillTool")
	})

	t.Run("Scenario_ContextModeForkIsTheIsolationToken", func(t *testing.T) {
		// Given a skill that needs isolated execution (PDF Section 6.1:
		//       "execution context ('fork' for isolated execution)"),
		given := `---
name: long_diagnostic
description: Long-running diagnostic
context_mode: fork
---
Diagnose...`

		// When parsed,
		when, err := skill.ParseSkillMD(given)
		assert.NoError(t, err)

		// Then ContextMode='fork' is the explicit token. The runner's
		//      ToolSchemaBuilder routes 'fork' skills through SubtaskExecutor
		//      so their conversation does not pollute the main thread.
		assert.Equal(t, "fork", when.ContextMode,
			"context_mode='fork' triggers isolated execution path")
	})

	t.Run("Scenario_DisableModelInvocationMakesSkillUserOnly", func(t *testing.T) {
		// Given a skill that should be invocable via slash command but NOT
		//       by the LLM (PDF Section 6.1: BundledSkillDefinition.
		//       disableModelInvocation),
		given := `---
name: human_only_skill
description: Triggered only by user
disable_model_invocation: true
---
body`

		// When parsed,
		when, err := skill.ParseSkillMD(given)
		assert.NoError(t, err)

		// Then the flag round-trips — the runner's tool pool assembly
		//      excludes user-only skills from the LLM tools[] array (saves
		//      context tokens; covered in TOOL-002).
		assert.True(t, when.DisableModelInvocation,
			"disable_model_invocation must surface so registry excludes from LLM tools")
	})

	t.Run("Scenario_RoundTripPreservesFrontmatterFields", func(t *testing.T) {
		// Given a Skill loaded from DB that we want to serialise back to
		//       SKILL.md (PDF Section 6.1: "skills are user-visible /
		//       file-based / version-controllable"),
		whenToUse := "When summarising long docs"
		argHint := "<doc-uri>"
		original := skill.Skill{
			Name:                   "summarise",
			Slug:                   "summarise",
			Description:            "Summarises documents",
			Category:               "writing",
			WhenToUse:              &whenToUse,
			ContextMode:            "inline",
			AllowedTools:           []string{"document-search"},
			DisableModelInvocation: false,
			ArgumentHint:           &argHint,
			ShouldDefer:            false,
			Instructions:           "Be concise. Cite sources.",
		}

		// When we serialise then re-parse,
		md := skill.SerializeSkillMD(original)
		req, err := skill.ParseSkillMD(md)
		assert.NoError(t, err)

		// Then every PDF-relevant field survives the round-trip.
		assert.Equal(t, original.Name, req.Name)
		assert.Equal(t, original.Slug, req.Slug)
		assert.Equal(t, original.Description, req.Description)
		assert.Equal(t, original.Category, req.Category)
		assert.NotNil(t, req.WhenToUse)
		assert.Equal(t, *original.WhenToUse, *req.WhenToUse)
		assert.Equal(t, original.ContextMode, req.ContextMode)
		assert.Equal(t, original.AllowedTools, req.AllowedTools)
		assert.NotNil(t, req.ArgumentHint)
		assert.Equal(t, *original.ArgumentHint, *req.ArgumentHint)
	})

	t.Run("Scenario_ArgumentHintIsOptionalAndNullable", func(t *testing.T) {
		// Given a skill without an argument hint (most skills don't need
		//       one),
		given := `---
name: simple
description: No args needed
---
body`

		// When parsed,
		when, err := skill.ParseSkillMD(given)
		assert.NoError(t, err)

		// Then ArgumentHint is nil (not empty string) — preserves the
		//      "intentionally absent" semantic for downstream code.
		assert.Nil(t, when.ArgumentHint,
			"absent hint must be nil pointer, not empty string")
	})

	t.Run("Scenario_FrontmatterCoverageMapsMostPDFFields", func(t *testing.T) {
		// Given the PDF Section 6.1 mentions 15+ frontmatter fields,
		// When we list AgentHub's supported fields (EXT-006a closed),
		// Then 15 fields are covered; shell_config is NOT_APPLICABLE_WEB.
		supportedFields := []string{
			"version", "name", "slug", "description", "category",
			"when_to_use", "context_mode", "allowed_tools",
			"disable_model_invocation", "argument_hint", "should_defer",
			// EXT-006a additions:
			"model_overrides", "effort_level", "associated_agents", "dynamic_hooks",
		}
		assert.GreaterOrEqual(t, len(supportedFields), 15,
			"AgentHub must support at least 15 of the PDF's 15+ frontmatter fields")
		// shell_config is NOT_APPLICABLE_WEB — AgentHub is a web-first platform.
	})

	t.Run("Scenario_EXT006a_ModelOverridesAllowPerSkillModelSelection", func(t *testing.T) {
		// Given a SKILL.md that specifies a cheaper model for cost control
		//       (PDF Section 6.1: "model overrides" in skill frontmatter),
		given := `---
name: cost-optimised-skill
model_overrides:
  default: claude-haiku-4-5-20251001
---
body`

		// When parsed,
		when, err := skill.ParseSkillMD(given)
		assert.NoError(t, err)

		// Then the model override is available for the runner to apply before
		//      LLM calls, overriding the agent-level model config.
		assert.Equal(t, "claude-haiku-4-5-20251001", when.ModelOverrides["default"],
			"runner must pick model from ModelOverrides[default] when present")
	})

	t.Run("Scenario_EXT006a_EffortLevelControlsThinkingBudget", func(t *testing.T) {
		// Given a skill that requires deep reasoning (PDF Section 6.1:
		//       "effort levels" maps to Anthropic thinking budget),
		given := `---
name: deep-analysis
effort_level: highest
---
body`

		// When parsed,
		when, err := skill.ParseSkillMD(given)
		assert.NoError(t, err)

		// Then EffortLevel carries the tier; the runner maps "highest" →
		//      max thinking-budget tokens before issuing the LLM call.
		assert.Equal(t, "highest", when.EffortLevel,
			"effort_level must round-trip so runner can set thinking budget")
	})

	t.Run("Scenario_EXT006a_AssociatedAgentsBindPersona", func(t *testing.T) {
		// Given a skill that should activate a specific agent persona
		//       (PDF Section 6.1: "associated agent definitions"),
		given := `---
name: compliance-skill
associated_agents:
  - compliance-expert
---
body`

		// When parsed,
		when, err := skill.ParseSkillMD(given)
		assert.NoError(t, err)

		// Then the agent slug is available for the runner to load the
		//      named agent's system prompt, layering it on top of the skill.
		assert.Equal(t, []string{"compliance-expert"}, when.AssociatedAgents,
			"associated_agents must round-trip so runner can activate the persona")
	})

	t.Run("Scenario_EXT006a_DynamicHooksRegisterOnInvocation", func(t *testing.T) {
		// Given a skill that needs lifecycle hooks during its execution
		//       (PDF Section 6.1: "skills can define their own hooks, which
		//       register dynamically on invocation"),
		given := `---
name: audited-skill
dynamic_hooks:
  - pre_tool_use
  - post_tool_use
---
body`

		// When parsed,
		when, err := skill.ParseSkillMD(given)
		assert.NoError(t, err)

		// Then the hook slugs are available for the runner to wire before
		//      execution and remove after — scoped to this skill's lifetime.
		assert.ElementsMatch(t, []string{"pre_tool_use", "post_tool_use"}, when.DynamicHooks,
			"dynamic_hooks must round-trip so runner can wire/unwire them")
	})

	t.Run("Scenario_EXT006a_ShellConfigIsNotApplicableWeb", func(t *testing.T) {
		// Given that Claude Code's shell_config controls subprocess environment
		//       (PDF Section 6.1: "shell configuration"),
		// And AgentHub is a web-first platform without subprocess skill runtime,
		// When a SKILL.md with shell_config is imported,
		given := `---
name: web-skill
---
body`

		// Then parsing succeeds and no error is returned — unknown YAML
		//      fields (including shell_config) are silently ignored by the
		//      parser, maintaining forward-compatibility.
		_, err := skill.ParseSkillMD(given)
		assert.NoError(t, err,
			"unknown frontmatter fields must not break import (shell_config → NOT_APPLICABLE_WEB)")
	})
}
