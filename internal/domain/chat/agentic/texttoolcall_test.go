package agentic

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseTextToolCalls_GptOssShape(t *testing.T) {
	// Observed in production: gpt-oss-120b emitting {"tool":"...","call":{...}}
	content := `{"tool":"agenthub_get_agent_config","call":{"agent_id":"3b8b935d-82c5-4f2d-9583-102f912f7434"}}`

	calls, consumed := parseTextToolCalls(content)
	if !consumed {
		t.Fatalf("expected consumed=true, got false")
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Function.Name != "agenthub_get_agent_config" {
		t.Errorf("name=%q, want agenthub_get_agent_config", calls[0].Function.Name)
	}
	if !strings.Contains(calls[0].Function.Arguments, "3b8b935d") {
		t.Errorf("arguments=%q should contain agent_id", calls[0].Function.Arguments)
	}
	if calls[0].Type != "function" {
		t.Errorf("type=%q, want function", calls[0].Type)
	}
	if calls[0].ID == "" {
		t.Errorf("id should not be empty")
	}
}

func TestParseTextToolCalls_OpenAIShape(t *testing.T) {
	content := `{"name":"search","arguments":{"query":"hello"}}`

	calls, consumed := parseTextToolCalls(content)
	if !consumed {
		t.Fatalf("expected consumed=true, got false")
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Function.Name != "search" {
		t.Errorf("name=%q, want search", calls[0].Function.Name)
	}
}

func TestParseTextToolCalls_ToolNameShape(t *testing.T) {
	content := `{"tool_name":"weather","arguments":{"city":"SP"}}`

	calls, consumed := parseTextToolCalls(content)
	if !consumed || len(calls) != 1 || calls[0].Function.Name != "weather" {
		t.Fatalf("unexpected: consumed=%v calls=%+v", consumed, calls)
	}
}

func TestParseTextToolCalls_ParametersShape(t *testing.T) {
	content := `{"name":"lookup","parameters":{"id":42}}`

	calls, consumed := parseTextToolCalls(content)
	if !consumed || len(calls) != 1 {
		t.Fatalf("expected 1 call, got consumed=%v calls=%d", consumed, len(calls))
	}
	if !strings.Contains(calls[0].Function.Arguments, "42") {
		t.Errorf("arguments should contain id=42, got %q", calls[0].Function.Arguments)
	}
}

func TestParseTextToolCalls_Array(t *testing.T) {
	content := `[{"tool":"a","call":{}},{"tool":"b","call":{"x":1}}]`

	calls, consumed := parseTextToolCalls(content)
	if !consumed {
		t.Fatalf("expected consumed=true")
	}
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(calls))
	}
	if calls[0].Function.Name != "a" || calls[1].Function.Name != "b" {
		t.Errorf("names=[%s,%s], want [a,b]", calls[0].Function.Name, calls[1].Function.Name)
	}
	if calls[0].ID == calls[1].ID {
		t.Errorf("synthetic IDs must be unique, both are %q", calls[0].ID)
	}
}

func TestParseTextToolCalls_LeadingWhitespace(t *testing.T) {
	content := "\n  \t" + `{"tool":"X","call":{}}` + "\n"

	calls, consumed := parseTextToolCalls(content)
	if !consumed || len(calls) != 1 {
		t.Fatalf("expected 1 call after trim, got consumed=%v", consumed)
	}
}

func TestParseTextToolCalls_NullCall(t *testing.T) {
	// "call":null is treated as an empty-args tool call, not rejected.
	content := `{"tool":"reset","call":null}`

	calls, consumed := parseTextToolCalls(content)
	if !consumed || len(calls) != 1 {
		t.Fatalf("expected 1 call for null args, got consumed=%v", consumed)
	}
	if calls[0].Function.Arguments != "{}" {
		t.Errorf("expected args normalized to {}, got %q", calls[0].Function.Arguments)
	}
}

func TestParseTextToolCalls_PreservesNestedJSON(t *testing.T) {
	content := `{"tool":"publish","call":{"filters":[1,2,3],"meta":{"k":"v"}}}`

	calls, consumed := parseTextToolCalls(content)
	if !consumed || len(calls) != 1 {
		t.Fatalf("unexpected: consumed=%v calls=%d", consumed, len(calls))
	}
	// Arguments must be valid JSON that round-trips.
	var got map[string]any
	if err := json.Unmarshal([]byte(calls[0].Function.Arguments), &got); err != nil {
		t.Fatalf("arguments not valid JSON: %v (raw=%q)", err, calls[0].Function.Arguments)
	}
	if _, ok := got["filters"]; !ok {
		t.Errorf("filters field missing from arguments")
	}
}

// --- rejection paths --------------------------------------------------------

func TestParseTextToolCalls_PlainText(t *testing.T) {
	if _, consumed := parseTextToolCalls("Hello world, here is my answer."); consumed {
		t.Errorf("plain text must not be consumed")
	}
}

func TestParseTextToolCalls_EmptyString(t *testing.T) {
	if _, consumed := parseTextToolCalls(""); consumed {
		t.Errorf("empty string must not be consumed")
	}
	if _, consumed := parseTextToolCalls("   \n\t"); consumed {
		t.Errorf("whitespace-only must not be consumed")
	}
}

func TestParseTextToolCalls_MissingName(t *testing.T) {
	content := `{"call":{"foo":"bar"}}`
	if _, consumed := parseTextToolCalls(content); consumed {
		t.Errorf("JSON without name field must not be consumed")
	}
}

func TestParseTextToolCalls_ProseWithNameField(t *testing.T) {
	// A JSON object with a "name" key but scalar args (not a tool call).
	// Must NOT be coerced — this is a failure mode we specifically guard against.
	content := `{"name":"Alice","age":30}`
	if _, consumed := parseTextToolCalls(content); consumed {
		t.Errorf("prose-like JSON {name, age} must not be coerced into a tool call")
	}
}

func TestParseTextToolCalls_MalformedJSON(t *testing.T) {
	if _, consumed := parseTextToolCalls(`{"tool":"X", "call":{broken}}`); consumed {
		t.Errorf("malformed JSON must not be consumed")
	}
}

func TestParseTextToolCalls_MixedTextAndJSON(t *testing.T) {
	// Prose that happens to contain JSON — do not intercept.
	content := `Sure, I'll help. Calling: {"tool":"search","call":{}} Let me know.`
	if _, consumed := parseTextToolCalls(content); consumed {
		t.Errorf("prose wrapped around JSON must not be consumed (preserves explanatory text)")
	}
}

func TestParseTextToolCalls_EmptyArray(t *testing.T) {
	if _, consumed := parseTextToolCalls("[]"); consumed {
		t.Errorf("empty array must not be consumed")
	}
}

func TestParseTextToolCalls_ArrayWithInvalidItem(t *testing.T) {
	// One valid + one invalid = reject all (avoid partial silent drops).
	content := `[{"tool":"a","call":{}},{"no":"name"}]`
	if _, consumed := parseTextToolCalls(content); consumed {
		t.Errorf("array containing a non-tool-call item must not be consumed")
	}
}
