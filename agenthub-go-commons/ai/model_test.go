package ai_test

import (
	"testing"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

func TestForcedToolChoice(t *testing.T) {
	tc := ai.ForcedToolChoice("classify_result")
	if tc == nil {
		t.Fatal("expected non-nil ToolChoice")
	}
	if tc.Type != ai.ToolChoiceTool {
		t.Errorf("expected type %q, got %q", ai.ToolChoiceTool, tc.Type)
	}
	if tc.Name != "classify_result" {
		t.Errorf("expected name %q, got %q", "classify_result", tc.Name)
	}
}

func TestChatOptions_Merge_ToolChoice(t *testing.T) {
	base := ai.ChatOptions{Model: "test"}
	override := ai.ChatOptions{ToolChoice: ai.ForcedToolChoice("my_tool")}
	merged := base.Merge(override)
	if merged.ToolChoice == nil {
		t.Fatal("expected ToolChoice to be set after merge")
	}
	if merged.ToolChoice.Name != "my_tool" {
		t.Errorf("expected name %q, got %q", "my_tool", merged.ToolChoice.Name)
	}
}

func TestChatOptions_Merge_StopSequences(t *testing.T) {
	base := ai.ChatOptions{Model: "test"}
	override := ai.ChatOptions{StopSequences: []string{"END", "STOP"}}
	merged := base.Merge(override)
	if len(merged.StopSequences) != 2 {
		t.Errorf("expected 2 stop sequences, got %d", len(merged.StopSequences))
	}
}

func TestChatOptions_Merge_Effort(t *testing.T) {
	base := ai.ChatOptions{Model: "test"}
	medium := ai.EffortMedium
	override := ai.ChatOptions{Effort: &medium}
	merged := base.Merge(override)
	if merged.Effort == nil {
		t.Fatal("expected Effort to be set after merge")
	}
	if *merged.Effort != ai.EffortMedium {
		t.Errorf("expected %q, got %q", ai.EffortMedium, *merged.Effort)
	}
}

func TestEffortLevelConstants(t *testing.T) {
	levels := []ai.EffortLevel{ai.EffortLow, ai.EffortMedium, ai.EffortHigh, ai.EffortMax}
	expected := []string{"low", "medium", "high", "max"}
	for i, l := range levels {
		if string(l) != expected[i] {
			t.Errorf("expected %q, got %q", expected[i], l)
		}
	}
}
