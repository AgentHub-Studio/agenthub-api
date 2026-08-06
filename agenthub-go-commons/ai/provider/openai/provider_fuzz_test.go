package openai

import (
	"encoding/json"
	"testing"
	"unicode/utf8"
)

func FuzzStreamDeltaThinkingDeltaSupportedShapes(f *testing.F) {
	seeds := []struct {
		thinking string
		visible  string
		shape    int
	}{
		{"reasoning content ", "visible text", 0},
		{"reasoning field ", "visible text", 1},
		{"thinking string ", "visible text", 2},
		{"thinking content ", "visible text", 3},
		{"thinking text ", "visible text", 4},
		{"thinking delta ", "visible text", 5},
		{"", "", 0},
	}
	for _, seed := range seeds {
		f.Add(seed.thinking, seed.visible, seed.shape)
	}

	f.Fuzz(func(t *testing.T, thinking string, visible string, shape int) {
		if len(thinking) > 2048 || len(visible) > 2048 {
			return
		}
		if !utf8.ValidString(thinking) || !utf8.ValidString(visible) {
			return
		}

		delta := map[string]any{"content": visible}
		switch normalizedThinkingShape(shape) {
		case 0:
			delta["reasoning_content"] = thinking
		case 1:
			delta["reasoning"] = thinking
		case 2:
			delta["thinking"] = thinking
		case 3:
			delta["thinking"] = map[string]any{"content": thinking}
		case 4:
			delta["thinking"] = map[string]any{"text": thinking}
		default:
			delta["thinking"] = map[string]any{"delta": thinking}
		}

		raw, err := json.Marshal(map[string]any{
			"choices": []any{
				map[string]any{
					"delta":         delta,
					"finish_reason": nil,
				},
			},
		})
		if err != nil {
			t.Fatalf("marshal event: %v", err)
		}

		var event streamEvent
		if err := json.Unmarshal(raw, &event); err != nil {
			t.Fatalf("unmarshal event: %v", err)
		}
		if len(event.Choices) != 1 {
			t.Fatalf("choices length = %d, want 1", len(event.Choices))
		}

		gotThinking := event.Choices[0].Delta.thinkingDelta()
		if gotThinking != thinking {
			t.Fatalf("thinkingDelta() = %q, want %q, raw=%s", gotThinking, thinking, string(raw))
		}

		gotVisible := event.Choices[0].Delta.Content
		if gotVisible == nil {
			t.Fatalf("content pointer is nil, raw=%s", string(raw))
		}
		if *gotVisible != visible {
			t.Fatalf("content = %q, want %q, raw=%s", *gotVisible, visible, string(raw))
		}
	})
}

func normalizedThinkingShape(shape int) int {
	shape %= 6
	if shape < 0 {
		shape += 6
	}
	return shape
}
