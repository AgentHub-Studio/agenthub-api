package agentic

import (
	"testing"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

func dummyTools() []ai.Tool {
	return []ai.Tool{{Type: "function", Function: ai.ToolSchema{Name: "noop"}}}
}

func TestResolveToolChoice_EmptyModeReturnsNil(t *testing.T) {
	if got := resolveToolChoice("", dummyTools(), "openrouter"); got != nil {
		t.Errorf("empty mode should be nil, got %+v", got)
	}
	if got := resolveToolChoice(ai.ToolChoiceAuto, dummyTools(), "openrouter"); got != nil {
		t.Errorf("auto mode should be nil, got %+v", got)
	}
}

func TestResolveToolChoice_NoneAlwaysRespected(t *testing.T) {
	for _, provider := range []string{"openrouter", "ollama", "unknown"} {
		got := resolveToolChoice(ai.ToolChoiceNone, dummyTools(), provider)
		if got == nil || got.Type != ai.ToolChoiceNone {
			t.Errorf("none must always pass through (provider=%s), got %+v", provider, got)
		}
	}
}

func TestResolveToolChoice_AnyDowngradesWhenNoTools(t *testing.T) {
	if got := resolveToolChoice(ai.ToolChoiceAny, nil, "openrouter"); got != nil {
		t.Errorf("any with 0 tools must downgrade to nil, got %+v", got)
	}
}

func TestResolveToolChoice_AnyDowngradesForIncapableProvider(t *testing.T) {
	if got := resolveToolChoice(ai.ToolChoiceAny, dummyTools(), "ollama"); got != nil {
		t.Errorf("any with ollama must downgrade to nil, got %+v", got)
	}
}

func TestResolveToolChoice_AnyAppliedForCapableProvider(t *testing.T) {
	for _, p := range []string{"openrouter", "openai", "OpenAI", "anthropic"} {
		got := resolveToolChoice(ai.ToolChoiceAny, dummyTools(), p)
		if got == nil || got.Type != ai.ToolChoiceAny {
			t.Errorf("any must apply for provider=%s, got %+v", p, got)
		}
	}
}

func TestResolveToolChoice_ProviderSlugStripped(t *testing.T) {
	got := resolveToolChoice(ai.ToolChoiceAny, dummyTools(), "openrouter/anthropic")
	if got == nil || got.Type != ai.ToolChoiceAny {
		t.Errorf("provider slug must be stripped to openrouter, got %+v", got)
	}
}

func TestNormaliseToolMode(t *testing.T) {
	cases := map[string]ai.ToolChoiceType{
		"":          "",
		"auto":      ai.ToolChoiceAuto,
		"AUTO":      ai.ToolChoiceAuto,
		" auto ":    ai.ToolChoiceAuto,
		"required":  ai.ToolChoiceAny,
		"REQUIRED":  ai.ToolChoiceAny,
		"any":       ai.ToolChoiceAny,
		"none":      ai.ToolChoiceNone,
		"garbage":   "",
		"tool_once": "", // unknown custom mode → coerce to empty/auto
	}
	for in, want := range cases {
		if got := normaliseToolMode(in); got != want {
			t.Errorf("normaliseToolMode(%q) = %q, want %q", in, got, want)
		}
	}
}
